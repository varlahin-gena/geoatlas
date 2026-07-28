package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
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

type app struct {
	pools      *chadapter.Pools
	ingestSvc  *ingest.Service
	srv        *httpapi.Server
	geoJobs    *geojob.Scheduler
	repJobs    *reputationjob.Scheduler
	bgCancel   context.CancelFunc
	bgWg       sync.WaitGroup
	ingestDone chan error
	ctx        context.Context
	cancel     context.CancelFunc
	listenAddr string
}

func buildApp(ctx context.Context, cfg config.Config) (*app, error) {
	var users *auth.UserStore
	var sessions *auth.SessionManager
	var apiTokens *auth.TokenStore
	if cfg.AuthDisabled {
		slog.Warn("UI auth disabled — login and role checks are off")
	} else {
		seed, err := auth.SeedUsersFromEnv(cfg.AuthAdminUser, cfg.AuthAdminPassword, cfg.AuthOperatorUser, cfg.AuthOperatorPassword)
		if err != nil {
			return nil, fmt.Errorf("auth seed: %w", err)
		}
		users, err = auth.OpenOrSeed(cfg.AuthUsersFile, seed)
		if err != nil {
			return nil, fmt.Errorf("auth users file %q: %w", cfg.AuthUsersFile, err)
		}
		ttl := time.Duration(cfg.SessionTTLHours) * time.Hour
		sessions, err = auth.NewSessionManager(cfg.SessionSecret, ttl)
		if err != nil {
			return nil, fmt.Errorf("session manager: %w", err)
		}
		slog.Info("UI auth enabled", "users", users.Len(), "users_file", cfg.AuthUsersFile, "session_ttl", ttl.String())
	}
	if !cfg.APIAuthDisabled {
		var err error
		apiTokens, err = auth.OpenOrCreateTokenStore(cfg.APITokensFile)
		if err != nil {
			return nil, fmt.Errorf("api tokens file %q: %w", cfg.APITokensFile, err)
		}
		slog.Info("API token store ready", "tokens", apiTokens.Len(), "file", cfg.APITokensFile)
	}

	query.ConfigureQuerySettings(cfg.CHMaxMemoryUsage, cfg.CHExternalGroupBy, cfg.CHExternalSort)
	ingestOpts := chadapter.PoolOptions{MaxOpenConns: cfg.CHIngestMaxOpen, MaxIdleConns: cfg.CHIngestMaxOpen}
	if cfg.CHIngestAsyncInsert {
		ingestOpts.Settings = clickhouse.Settings{"async_insert": 1, "wait_for_async_insert": 1}
		slog.Info("clickhouse ingest: async_insert enabled", "wait_for_async_insert", 1)
	}
	pools, err := chadapter.ConnectPools(ctx, cfg.ClickHouseAddr(), chadapter.Auth{
		Database: cfg.ClickHouseDatabase, Username: cfg.ClickHouseUser, Password: cfg.ClickHousePassword,
	}, ingestOpts,
		chadapter.PoolOptions{MaxOpenConns: cfg.CHAPIMaxOpen, MaxIdleConns: cfg.CHAPIMaxOpen},
		chadapter.PoolOptions{MaxOpenConns: cfg.CHBackgroundMaxOpen, MaxIdleConns: cfg.CHBackgroundMaxOpen},
	)
	if err != nil {
		return nil, fmt.Errorf("clickhouse connection: %w", err)
	}

	aCtx, cancel := context.WithCancel(ctx)
	// WithoutCancel: bootstrap/jobs живут до явного bgCancel на shutdown,
	// а не обрываются вместе с signal ctx (пока идёт HTTP/ingest drain).
	bgCtx, bgCancel := context.WithCancel(context.WithoutCancel(ctx))
	a := &app{pools: pools, bgCancel: bgCancel, ctx: aCtx, cancel: cancel, listenAddr: cfg.ListenAddr}
	geo := chadapter.NewReloadableGeoIndex(pools.Background)
	a.geoJobs = geojob.New(geo, chadapter.NewMaintenanceStore(pools.Background), cfg.GeoBackfillLookbackDays)
	if err := geo.Reload(ctx); err != nil {
		slog.Warn("geo index not loaded", "err", err)
	}

	repIdx := chadapter.NewReloadableReputationIndex(pools.Background)
	repRepo := chadapter.NewReputationRepository(pools.API, pools.Ingest)
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
	repUC := usecasereputation.New(repRepo, repIdx, usecasereputation.DefaultCodec{}, nil, repFeedStore)
	a.repJobs = reputationjob.New(repFeeds, cfg.ReputationFetchInterval, cfg.ReputationFetchEnabled, repUC)
	repUC.SetRefresher(a.repJobs)
	if err := migrate.EnsureReputationRanges(ctx, pools.Background); err != nil {
		slog.Warn("reputation_ranges ensure (early) failed", "err", err)
	}
	if err := repIdx.Reload(ctx); err != nil {
		slog.Warn("reputation index not loaded", "err", err)
	}

	retentionUC := usecaseretention.New(retentionfile.New(cfg.RetentionFile), chadapter.NewRetentionApplier(pools.Background))
	bgStore := &bootstrapadapter.Storage{CH: pools.Background}
	a.bgWg.Add(1)
	go func() {
		defer a.bgWg.Done()
		bootstrap.RunStartup(bgCtx, bootstrap.Dependencies{Schema: bgStore, Backfill: bgStore, Ready: bgStore, Enrich: a.geoJobs, Geo: geo, Retention: retentionUC},
			bootstrap.Options{SkipStartupBackfill: cfg.SkipStartupBackfill, GeoBackfillLookbackDays: cfg.GeoBackfillLookbackDays, Timeout: 6 * time.Hour},
			func(msg string, err error) { slog.Warn(msg, "err", err) }, func(msg string, args ...any) { slog.Info(msg, args...) })
	}()

	parsers := parser.NewRegistry(&parser.UserGateCEF{}, &parser.FortigateCEF{}, &parser.CiscoFTD{}, &parser.CiscoASA{}, &parser.CowrieJSON{}, &parser.GenericKV{})
	lineParser := parseradapter.New(parsers)
	ingestRepo := chadapter.NewIngestRepository(pools.Ingest)
	a.ingestSvc = ingest.NewService(ingest.Config{
		Bindings: ingestBindings(cfg), BatchSize: cfg.IngestBatchSize, FlushInterval: time.Duration(cfg.IngestFlushSec) * time.Second,
		QueueSize: cfg.IngestQueueSize, QueueMaxBytes: cfg.IngestQueueMaxBytes, Workers: cfg.IngestWorkers, QueryTimeout: cfg.QueryTimeout,
		MaxConnections: cfg.IngestMaxConnections, ConnIdleTimeout: time.Duration(cfg.IngestConnIdleSec) * time.Second,
	}, ingest.ProcessorDeps{Logs: ingestRepo, Errors: ingestRepo, Parser: lineParser, Geo: geo, EnrichCountry: cfg.GeoEnrichOnIngest})
	a.ingestDone = make(chan error, 1)
	go func() {
		err := a.ingestSvc.Run(a.ctx)
		a.ingestDone <- err
		if a.ctx.Err() == nil {
			if err != nil {
				slog.Error("ingest service failed", "err", err)
			} else {
				slog.Error("ingest exited unexpectedly")
			}
			a.cancel()
		}
	}()

	trafficRepo := chadapter.NewTrafficRepository(pools.API)
	geoRepo := chadapter.NewGeoRepository(pools.API, pools.Ingest)
	eventsUC := usecaseevents.New(trafficRepo, geo, repIdx)
	geoUC := usecasegeo.New(geoRepo, trafficRepo, geo, a.geoJobs, geoipcodec.New())
	parseErrorsUC := parseerrors.New(chadapter.NewParseErrorRepository(pools.API, pools.Ingest))
	parseTestAdapter := parseradapter.NewParseTest(parsers)
	parseTestUC := parsetest.New(parseTestAdapter, geo, parseTestAdapter)
	systemRepo := chadapter.NewSystemRepository(pools.API)
	systemUC := usecasesystem.New(usecasesystem.Dependencies{Metrics: systemRepo, Edges: systemRepo, Ingest: &systemlive.IngestAdapter{Src: a.ingestSvc}, Profiles: systemlive.ProfileAdapter{}, InstallProfilePath: cfg.InstallProfilePath, Maintenance: a.geoJobs})
	authUC := usecaseauth.New(users, sessions)
	a.srv = httpapi.NewServer(cfg, a.ingestSvc, eventsUC, geoUC, repUC, parseErrorsUC, systemUC, systemRepo, parseTestUC, retentionUC, authUC, users, sessions, apiTokens)
	a.repJobs.Start(bgCtx)
	return a, nil
}

func reputationFeedsFromConfig(feeds []config.ReputationFeed) []usecasereputation.Feed {
	out := make([]usecasereputation.Feed, len(feeds))
	for i, feed := range feeds {
		out[i] = usecasereputation.Feed{
			Name: feed.Name, URL: feed.URL, Category: feed.Category, Format: feed.Format,
		}
	}
	return out
}

func (a *app) run(ctx context.Context) error {
	if a == nil || a.srv == nil {
		return errors.New("application not initialized")
	}
	defer a.pools.Close()
	defer a.cancel()
	go func() {
		slog.Info("backend listening", "addr", a.listenAddr)
		if err := a.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			a.cancel()
		}
	}()
	select {
	case <-ctx.Done():
	case <-a.ctx.Done():
	}
	slog.Info("shutdown signal received")
	// Parent ctx уже Done — бюджеты shutdown наследуют values, но не cancel.
	base := context.WithoutCancel(ctx)
	httpCtx, httpCancel := context.WithTimeout(base, 15*time.Second)
	if err := a.srv.Shutdown(httpCtx); err != nil {
		slog.Warn("http shutdown failed", "err", err)
	} else {
		slog.Info("http shutdown complete")
	}
	httpCancel()
	geoCtx, geoCancel := context.WithTimeout(base, 5*time.Second)
	a.geoJobs.Shutdown(geoCtx)
	geoCancel()
	repCtx, repCancel := context.WithTimeout(base, 5*time.Second)
	a.repJobs.Shutdown(repCtx)
	repCancel()
	ingestWait := a.ingestSvc.ShutdownWaitTimeout()
	snap := a.ingestSvc.Stats()
	slog.Info("waiting for ingest drain", "budget", ingestWait.String(), "queue_depth", snap.QueueDepth, "dropped_total", snap.DroppedTotal)
	waitCtx, waitCancel := context.WithTimeout(base, ingestWait)
	select {
	case err := <-a.ingestDone:
		if err != nil {
			slog.Warn("ingest shutdown error", "err", err)
		} else {
			slog.Info("ingest shutdown complete")
		}
	case <-waitCtx.Done():
		slog.Warn("ingest drain timeout", "queue_depth_left", a.ingestSvc.Stats().QueueDepth, "budget", ingestWait.String())
		a.ingestSvc.AbortDrain()
		select {
		case <-a.ingestDone:
		case <-time.After(3 * time.Second):
			slog.Warn("ingest workers still draining after AbortDrain")
		}
	}
	waitCancel()
	a.bgCancel()
	bgWaitCtx, bgWaitCancel := context.WithTimeout(base, 5*time.Second)
	defer bgWaitCancel()
	done := make(chan struct{})
	go func() { a.bgWg.Wait(); close(done) }()
	select {
	case <-done:
		slog.Info("background workers stopped")
	case <-bgWaitCtx.Done():
		slog.Warn("background workers drain timeout")
	}
	slog.Info("shutdown complete")
	return nil
}

func ingestBindings(cfg config.Config) []ingest.Binding {
	if cfg.IngestListenAddr != "" {
		return []ingest.Binding{{Addr: cfg.IngestListenAddr}}
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
