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
	"network_monitor/internal/adapter/geoipcodec"
	"network_monitor/internal/adapter/geojob"
	httpapi "network_monitor/internal/adapter/httpapi"
	"network_monitor/internal/adapter/parseradapter"
	"network_monitor/internal/adapter/retentionfile"
	"network_monitor/internal/adapter/systemlive"
	"network_monitor/internal/auth"
	"network_monitor/internal/config"
	"network_monitor/internal/ingest"
	"network_monitor/internal/logging"
	"network_monitor/internal/parser"
	"network_monitor/internal/storage"
	usecaseauth "network_monitor/internal/usecase/auth"
	"network_monitor/internal/usecase/bootstrap"
	usecaseevents "network_monitor/internal/usecase/events"
	usecasegeo "network_monitor/internal/usecase/geo"
	"network_monitor/internal/usecase/parseerrors"
	"network_monitor/internal/usecase/parsetest"
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

	slog.Info("network-monitor starting", "edges_agg", true, "geo_enrich_on_ingest", cfg.GeoEnrichOnIngest)

	storage.ConfigureQuerySettings(cfg.CHMaxMemoryUsage, cfg.CHExternalGroupBy, cfg.CHExternalSort)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ingestOpts := storage.PoolOptions{MaxOpenConns: cfg.CHIngestMaxOpen, MaxIdleConns: cfg.CHIngestMaxOpen}
	if cfg.CHIngestAsyncInsert {
		ingestOpts.Settings = clickhouse.Settings{
			"async_insert":          1,
			"wait_for_async_insert": 1,
		}
		slog.Info("clickhouse ingest: async_insert enabled", "wait_for_async_insert", 1)
	}

	pools, err := storage.ConnectPools(ctx, cfg.ClickHouseAddr(),
		storage.Auth{
			Database: cfg.ClickHouseDatabase,
			Username: cfg.ClickHouseUser,
			Password: cfg.ClickHousePassword,
		},
		ingestOpts,
		storage.PoolOptions{MaxOpenConns: cfg.CHAPIMaxOpen, MaxIdleConns: cfg.CHAPIMaxOpen},
		storage.PoolOptions{MaxOpenConns: cfg.CHBackgroundMaxOpen, MaxIdleConns: cfg.CHBackgroundMaxOpen},
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
	geoJobs := geojob.New(geo, pools.Background, cfg.GeoBackfillLookbackDays)
	if err := geo.Reload(ctx); err != nil {
		slog.Warn("geo index not loaded", "err", err)
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
	eventsUC := usecaseevents.New(trafficRepo, geo)
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
	srv := httpapi.NewServer(cfg, ingestSvc, eventsUC, geoUC, parseErrorsUC, systemUC, systemRepo, parseTestUC, retentionUC, authUC, users, sessions)

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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("http shutdown failed", "err", err)
	} else {
		slog.Info("http shutdown complete")
	}

	geoStopCtx, geoStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	geoJobs.Shutdown(geoStopCtx)
	geoStopCancel()

	if ingestDone != nil {
		select {
		case err := <-ingestDone:
			if err != nil {
				slog.Warn("ingest shutdown error", "err", err)
			} else {
				slog.Info("ingest shutdown complete")
			}
		case <-shutdownCtx.Done():
			slog.Warn("ingest drain timeout")
		}
	}

	bgCancel()
	// Отдельный бюджет: HTTP/ingest могли съесть общий shutdownCtx.
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
