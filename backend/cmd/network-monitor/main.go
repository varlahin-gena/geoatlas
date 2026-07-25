package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/adapter/bootstrapadapter"
	chadapter "network_monitor/internal/adapter/clickhouse"
	"network_monitor/internal/adapter/clickhouse/migrate"
	"network_monitor/internal/adapter/clickhouse/query"
	"network_monitor/internal/adapter/geoipcodec"
	"network_monitor/internal/adapter/geojob"
	httpapi "network_monitor/internal/adapter/httpapi"
	"network_monitor/internal/adapter/parseradapter"
	"network_monitor/internal/adapter/reputationfeedsfile"
	"network_monitor/internal/adapter/reputationjob"
	"network_monitor/internal/adapter/retentionfile"
	"network_monitor/internal/adapter/systemlive"
	"network_monitor/internal/auth"
	"network_monitor/internal/config"
	"network_monitor/internal/ingest"
	"network_monitor/internal/logging"
	"network_monitor/internal/parser"
	usecaseauth "network_monitor/internal/usecase/auth"
	"network_monitor/internal/usecase/bootstrap"
	usecaseevents "network_monitor/internal/usecase/events"
	usecasegeo "network_monitor/internal/usecase/geo"
	"network_monitor/internal/usecase/parseerrors"
	"network_monitor/internal/usecase/parsetest"
	usecasereputation "network_monitor/internal/usecase/reputation"
	usecaseretention "network_monitor/internal/usecase/retention"
	usecasesystem "network_monitor/internal/usecase/system"
)

func main() {
	cfg := config.FromEnv()
	logging.Setup(cfg.LogLevel, cfg.LogFormat)

	if err := cfg.ValidateSecurity(); err != nil {
		slog.Error("security config", "err", err)
		os.Exit(1)
	}
	for _, w := range cfg.SecurityWarnings() {
		slog.Warn(w)
	}
	if cfg.APIAuthDisabled {
		slog.Warn("API auth disabled — mutating endpoints are open")
	}

	var users *auth.UserStore
	var sessions *auth.SessionManager
	var apiTokens *auth.TokenStore
	if cfg.AuthDisabled {
		slog.Warn("UI auth disabled — login and role checks are off")
	} else {
		seed, err := auth.SeedUsersFromEnv(
			cfg.AuthAdminUser, cfg.AuthAdminPassword,
			cfg.AuthOperatorUser, cfg.AuthOperatorPassword,
		)
		if err != nil {
			slog.Error("auth seed", "err", err)
			os.Exit(1)
		}
		store, err := auth.OpenOrSeed(cfg.AuthUsersFile, seed)
		if err != nil {
			slog.Error("auth users file", "path", cfg.AuthUsersFile, "err", err)
			os.Exit(1)
		}
		ttl := time.Duration(cfg.SessionTTLHours) * time.Hour
		mgr, err := auth.NewSessionManager(cfg.SessionSecret, ttl)
		if err != nil {
			slog.Error("session manager", "err", err)
			os.Exit(1)
		}
		users = store
		sessions = mgr
		slog.Info("UI auth enabled", "users", users.Len(), "users_file", cfg.AuthUsersFile, "session_ttl", ttl.String())
	}
	if !cfg.APIAuthDisabled {
		ts, err := auth.OpenOrCreateTokenStore(cfg.APITokensFile)
		if err != nil {
			slog.Error("api tokens file", "path", cfg.APITokensFile, "err", err)
			os.Exit(1)
		}
		apiTokens = ts
		slog.Info("API token store ready", "tokens", apiTokens.Len(), "file", cfg.APITokensFile)
	}

	slog.Info("network-monitor starting", "edges_agg", true, "geo_enrich_on_ingest", cfg.GeoEnrichOnIngest)

	query.ConfigureQuerySettings(cfg.CHMaxMemoryUsage, cfg.CHExternalGroupBy, cfg.CHExternalSort)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ingestOpts := chadapter.PoolOptions{MaxOpenConns: cfg.CHIngestMaxOpen, MaxIdleConns: cfg.CHIngestMaxOpen}
	if cfg.CHIngestAsyncInsert {
		ingestOpts.Settings = clickhouse.Settings{
			"async_insert":          1,
			"wait_for_async_insert": 1,
		}
		slog.Info("clickhouse ingest: async_insert enabled", "wait_for_async_insert", 1)
	}

	pools, err := chadapter.ConnectPools(ctx, cfg.ClickHouseAddr(),
		chadapter.Auth{
			Database: cfg.ClickHouseDatabase,
			Username: cfg.ClickHouseUser,
			Password: cfg.ClickHousePassword,
		},
		ingestOpts,
		chadapter.PoolOptions{MaxOpenConns: cfg.CHAPIMaxOpen, MaxIdleConns: cfg.CHAPIMaxOpen},
		chadapter.PoolOptions{MaxOpenConns: cfg.CHBackgroundMaxOpen, MaxIdleConns: cfg.CHBackgroundMaxOpen},
	)
	if err != nil {
		slog.Error("clickhouse connection failed", "err", err)
		os.Exit(1)
	}
	defer pools.Close()

	// Фоновые Ensure*/enrich привязаны к bgCtx: на shutdown ждём их до pools.Close.
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()
	var bgWg sync.WaitGroup

	geo := chadapter.NewReloadableGeoIndex(pools.Background)
	geoJobs := geojob.New(geo, chadapter.NewMaintenanceStore(pools.Background), cfg.GeoBackfillLookbackDays)
	if err := geo.Reload(ctx); err != nil {
		slog.Warn("geo index not loaded", "err", err)
	}

	repIdx := chadapter.NewReloadableReputationIndex(pools.Background)
	repRepo := chadapter.NewReputationRepository(pools.API, pools.Ingest)
	repFeedStore := reputationfeedsfile.New(cfg.ReputationFeedsFile)
	repFeeds, err := repFeedStore.LoadOrSeed(cfg.ReputationFeeds)
	if err != nil {
		slog.Warn("reputation feeds file load/seed failed", "err", err, "path", cfg.ReputationFeedsFile)
		repFeeds = cfg.ReputationFeeds
		if len(repFeeds) == 0 {
			repFeeds = config.DefaultReputationFeeds()
		}
	} else {
		slog.Info("reputation feeds loaded", "count", len(repFeeds), "path", cfg.ReputationFeedsFile)
	}
	repUC := usecasereputation.New(repRepo, repIdx, usecasereputation.DefaultCodec{}, nil, repFeedStore)
	repJobs := reputationjob.New(repFeeds, cfg.ReputationFetchInterval, cfg.ReputationFetchEnabled, repUC)
	repUC.SetRefresher(repJobs)

	if err := migrate.EnsureReputationRanges(ctx, pools.Background); err != nil {
		slog.Warn("reputation_ranges ensure (early) failed", "err", err)
	}
	if err := repIdx.Reload(ctx); err != nil {
		slog.Warn("reputation index not loaded", "err", err)
	}

	retentionUC := usecaseretention.New(
		retentionfile.New(cfg.RetentionFile),
		chadapter.NewRetentionApplier(pools.Background),
	)

	bgStore := &bootstrapadapter.Storage{CH: pools.Background}
	bgWg.Add(1)
	go func() {
		defer bgWg.Done()
		bootstrap.RunStartup(bgCtx, bootstrap.Dependencies{
			Schema:    bgStore,
			Backfill:  bgStore,
			Ready:     bgStore,
			Enrich:    geoJobs,
			Geo:       geo,
			Retention: retentionUC,
		}, bootstrap.Options{
			SkipStartupBackfill:     cfg.SkipStartupBackfill,
			GeoBackfillLookbackDays: cfg.GeoBackfillLookbackDays,
			Timeout:                 6 * time.Hour,
		}, func(msg string, err error) {
			slog.Warn(msg, "err", err)
		}, func(msg string, args ...any) {
			slog.Info(msg, args...)
		})
	}()

	parsers := parser.NewRegistry(
		&parser.UserGateCEF{},
		&parser.FortigateCEF{},
		&parser.CiscoFTD{},
		&parser.CiscoASA{},
		&parser.CowrieJSON{},
		&parser.GenericKV{},
	)

	bindings := ingestBindings(cfg)

	lineParser := parseradapter.New(parsers)
	ingestRepo := chadapter.NewIngestRepository(pools.Ingest)
	procDeps := ingest.ProcessorDeps{
		Logs: ingestRepo, Errors: ingestRepo, Parser: lineParser,
		Geo: geo, EnrichCountry: cfg.GeoEnrichOnIngest,
	}
	// City/lat/lon/country пишем в traffic_logs из GeoIP (нужны для geo pre-agg / UI).
	ingestSvc := ingest.NewService(ingest.Config{
		Bindings:        bindings,
		BatchSize:       cfg.IngestBatchSize,
		FlushInterval:   time.Duration(cfg.IngestFlushSec) * time.Second,
		QueueSize:       cfg.IngestQueueSize,
		QueueMaxBytes:   cfg.IngestQueueMaxBytes,
		Workers:         cfg.IngestWorkers,
		QueryTimeout:    cfg.QueryTimeout,
		MaxConnections:  cfg.IngestMaxConnections,
		ConnIdleTimeout: time.Duration(cfg.IngestConnIdleSec) * time.Second,
	}, procDeps)

	ingestDone := make(chan error, 1)
	go func() {
		err := ingestSvc.Run(ctx)
		ingestDone <- err
		if ctx.Err() == nil {
			if err != nil {
				slog.Error("ingest service failed", "err", err)
			} else {
				slog.Error("ingest exited unexpectedly")
			}
			stop()
		}
	}()

	trafficRepo := chadapter.NewTrafficRepository(pools.API)
	geoRepo := chadapter.NewGeoRepository(pools.API, pools.Ingest)
	eventsUC := usecaseevents.New(trafficRepo, geo, repIdx)
	geoUC := usecasegeo.New(geoRepo, trafficRepo, geo, geoJobs, geoipcodec.New())
	parseErrorsUC := parseerrors.New(chadapter.NewParseErrorRepository(pools.API, pools.Ingest))
	parseTestAdapter := parseradapter.NewParseTest(parsers)
	parseTestUC := parsetest.New(parseTestAdapter, geo, parseTestAdapter)
	systemRepo := chadapter.NewSystemRepository(pools.API)
	systemUC := usecasesystem.New(usecasesystem.Dependencies{
		Metrics: systemRepo, Edges: systemRepo,
		Ingest:   &systemlive.IngestAdapter{Src: ingestSvc},
		Profiles: systemlive.ProfileAdapter{}, InstallProfilePath: cfg.InstallProfilePath,
		Maintenance: geoJobs,
	})

	authUC := usecaseauth.New(users, sessions)
	srv := httpapi.NewServer(cfg, ingestSvc, eventsUC, geoUC, repUC, parseErrorsUC, systemUC, systemRepo, parseTestUC, retentionUC, authUC, users, sessions, apiTokens)

	repJobs.Start(bgCtx)

	go func() {
		slog.Info("backend listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			stop()
		}
	}()

	// Единая точка остановки: сигнал или fatal от HTTP/ingest.
	<-ctx.Done()
	slog.Info("shutdown signal received")

	httpCtx, httpCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := srv.Shutdown(httpCtx); err != nil {
		slog.Warn("http shutdown failed", "err", err)
	} else {
		slog.Info("http shutdown complete")
	}
	httpCancel()

	geoStopCtx, geoStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	geoJobs.Shutdown(geoStopCtx)
	geoStopCancel()

	repStopCtx, repStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	repJobs.Shutdown(repStopCtx)
	repStopCancel()

	// Отдельный бюджет: не делим 15s HTTP с drain очереди ingest.
	ingestWait := ingestSvc.ShutdownWaitTimeout()
	ingestSnap := ingestSvc.Stats()
	slog.Info("waiting for ingest drain",
		"budget", ingestWait.String(),
		"queue_depth", ingestSnap.QueueDepth,
		"dropped_total", ingestSnap.DroppedTotal,
	)
	ingestWaitCtx, ingestWaitCancel := context.WithTimeout(context.Background(), ingestWait)
	defer ingestWaitCancel()
	if ingestDone != nil {
		select {
		case err := <-ingestDone:
			if err != nil {
				slog.Warn("ingest shutdown error", "err", err)
			} else {
				slog.Info("ingest shutdown complete")
			}
		case <-ingestWaitCtx.Done():
			left := ingestSvc.Stats().QueueDepth
			slog.Warn("ingest drain timeout", "queue_depth_left", left, "budget", ingestWait.String())
			ingestSvc.AbortDrain()
			// Даём workers выйти после AbortDrain до pools.Close.
			select {
			case <-ingestDone:
			case <-time.After(3 * time.Second):
				slog.Warn("ingest workers still draining after AbortDrain")
			}
		}
	}

	bgCancel()
	bgWaitCtx, bgWaitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer bgWaitCancel()
	bgDone := make(chan struct{})
	go func() {
		bgWg.Wait()
		close(bgDone)
	}()
	select {
	case <-bgDone:
		slog.Info("background workers stopped")
	case <-bgWaitCtx.Done():
		slog.Warn("background workers drain timeout")
	}

	slog.Info("shutdown complete")
}

func ingestBindings(cfg config.Config) []ingest.Binding {
	if cfg.IngestListenAddr != "" {
		return []ingest.Binding{{Addr: cfg.IngestListenAddr, Transport: ""}}
	}
	var bindings []ingest.Binding
	if cfg.IngestUDPListenAddr != "" {
		bindings = append(bindings, ingest.Binding{Addr: cfg.IngestUDPListenAddr, Transport: "udp"})
	}
	if cfg.IngestTCPListenAddr != "" {
		bindings = append(bindings, ingest.Binding{Addr: cfg.IngestTCPListenAddr, Transport: "tcp"})
	}
	return bindings
}
