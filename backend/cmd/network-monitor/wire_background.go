package main

import (
	"context"
	"log/slog"
	"time"

	"network_monitor/internal/adapter/bootstrapadapter"
	chadapter "network_monitor/internal/adapter/clickhouse"
	"network_monitor/internal/adapter/clickhouse/geostore"
	"network_monitor/internal/adapter/clickhouse/migrate"
	"network_monitor/internal/adapter/clickhouse/repstore"
	"network_monitor/internal/adapter/geojob"
	"network_monitor/internal/adapter/reputationcodec"
	"network_monitor/internal/adapter/reputationfeedsfile"
	"network_monitor/internal/adapter/reputationjob"
	"network_monitor/internal/adapter/retentionfile"
	"network_monitor/internal/config"
	usecasereputation "network_monitor/internal/usecase/reputation"
	usecaseretention "network_monitor/internal/usecase/retention"
	"network_monitor/internal/usecase/bootstrap"
)

// backgroundParts — индексы/UC, нужные ingest и HTTP после wireBackground.
type backgroundParts struct {
	geo         *geostore.ReloadableGeoIndex
	repIdx      *repstore.ReloadableReputationIndex
	repUC       *usecasereputation.Service
	retentionUC *usecaseretention.Service
}

func wireBackground(ctx, bgCtx context.Context, a *app, cfg config.Config) backgroundParts {
	geo := geostore.NewReloadableGeoIndex(a.pools.Background, cfg.GeoSnapshotFile)
	a.geoJobs = geojob.New(geo, chadapter.NewMaintenanceStore(a.pools.Background), cfg.GeoBackfillLookbackDays)
	// Disk snapshot: карта работает сразу после рестарта. Полный Reload из CH
	// сверяет stamp и при расхождении перечитывает таблицу.
	if geo.LoadDisk() {
		slog.Info("geo index: serving disk snapshot until clickhouse reload")
	}
	// Не блокируем старт HTTP на полной загрузке GeoIP (миллионы диапазонов → минуты).
	// Иначе после OOM/рестарта nginx auth_request падает → клиенту 500 на всех страницах.
	a.bgWg.Add(1)
	go func() {
		defer a.bgWg.Done()
		if err := geo.Reload(bgCtx); err != nil {
			slog.Warn("geo index not loaded", "err", err)
		}
	}()

	// Schema для /api/events до HTTP: иначе city/days бьётся о missing columns/tables.
	// IP-тип логов — до geo MV/enrich JOIN (IPv4 = IPv4).
	if err := migrate.EnsureHTTPSchema(ctx, a.pools.Background); err != nil {
		slog.Warn("http schema ensure (early) failed", "err", err)
	}

	var repIdx *repstore.ReloadableReputationIndex
	var repUC *usecasereputation.Service
	if cfg.ReputationFetchEnabled {
		repIdx = repstore.NewReloadableReputationIndex(a.pools.Background)
		repRepo := repstore.NewReputationRepository(a.pools.API, a.pools.Ingest)
		repFeedStore := reputationfeedsfile.New(cfg.ReputationFeedsFile)
		repFeeds, err := repFeedStore.LoadOrSeed(reputationFeedsFromConfig(cfg.ReputationFeeds))
		if err != nil {
			slog.Warn("reputation feeds file load/seed failed", "err", err, "path", cfg.ReputationFeedsFile)
			repFeeds = reputationFeedsFromConfig(cfg.ReputationFeeds)
			if len(repFeeds) == 0 {
				repFeeds = reputationFeedsFromConfig(config.DefaultReputationFeeds())
			}
		} else {
			slog.Info("reputation feeds loaded", "count", len(repFeeds), "path", cfg.ReputationFeedsFile)
		}
		if cleaned, dropped := dropRetiredReputationFeeds(repFeeds); dropped > 0 {
			repFeeds = cleaned
			if err := repFeedStore.Save(repFeeds); err != nil {
				slog.Warn("reputation feeds: failed to persist after dropping retired", "err", err, "dropped", dropped)
			} else {
				slog.Info("reputation feeds: dropped retired upstream lists", "dropped", dropped, "remaining", len(repFeeds))
			}
		}
		repUC = usecasereputation.New(repRepo, repIdx, reputationcodec.New(), nil, repFeedStore)
		a.repJobs = reputationjob.New(repFeeds, cfg.ReputationFetchInterval, true, repUC)
		repUC.SetRefresher(a.repJobs)
		if err := migrate.EnsureReputationRanges(ctx, a.pools.Background); err != nil {
			slog.Warn("reputation_ranges ensure (early) failed", "err", err)
		}
		a.bgWg.Add(1)
		go func() {
			defer a.bgWg.Done()
			if err := repIdx.Reload(bgCtx); err != nil {
				slog.Warn("reputation index not loaded", "err", err)
			}
		}()
	} else {
		slog.Info("reputation module disabled (REPUTATION_FETCH_ENABLED=false)")
		a.repJobs = nil
	}

	retentionUC := usecaseretention.New(
		retentionfile.New(cfg.RetentionFile),
		chadapter.NewRetentionApplier(a.pools.Background),
	)
	bgStore := &bootstrapadapter.Storage{CH: a.pools.Background}
	a.bgWg.Add(1)
	go func() {
		defer a.bgWg.Done()
		bootstrap.RunStartup(bgCtx, bootstrap.Dependencies{
			Schema: bgStore, Backfill: bgStore, Ready: bgStore,
			Enrich: a.geoJobs, Geo: geo, Retention: retentionUC,
		}, bootstrap.Options{
			SkipStartupBackfill:     cfg.SkipStartupBackfill,
			GeoBackfillLookbackDays: cfg.GeoBackfillLookbackDays,
			ReputationEnabled:       cfg.ReputationFetchEnabled,
			Timeout:                 6 * time.Hour,
		}, func(msg string, err error) {
			slog.Warn(msg, "err", err)
		}, func(msg string, args ...any) {
			slog.Info(msg, args...)
		})
	}()

	return backgroundParts{
		geo:         geo,
		repIdx:      repIdx,
		repUC:       repUC,
		retentionUC: retentionUC,
	}
}

func reputationFeedsFromConfig(feeds []config.ReputationFeed) []usecasereputation.Feed {
	feeds, _ = config.WithoutRetiredReputationFeeds(feeds)
	out := make([]usecasereputation.Feed, 0, len(feeds))
	for _, feed := range feeds {
		out = append(out, usecasereputation.Feed{
			Name: feed.Name, URL: feed.URL, Category: feed.Category, Format: feed.Format,
		})
	}
	return out
}

func dropRetiredReputationFeeds(feeds []usecasereputation.Feed) (cleaned []usecasereputation.Feed, dropped int) {
	if len(feeds) == 0 {
		return nil, dropped
	}
	cleaned = make([]usecasereputation.Feed, 0, len(feeds))
	for _, f := range feeds {
		if _, retired := config.RetiredReputationFeedNames[f.Name]; retired {
			dropped++
			continue
		}
		cleaned = append(cleaned, f)
	}
	if len(cleaned) == 0 {
		return nil, dropped
	}
	return cleaned, dropped
}
