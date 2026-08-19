package main

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"

	"network_monitor/internal/adapter/anomalyjob"
	"network_monitor/internal/adapter/backupjob"
	chadapter "network_monitor/internal/adapter/clickhouse"
	"network_monitor/internal/adapter/datalock"
	"network_monitor/internal/adapter/geojob"
	httpapi "network_monitor/internal/adapter/httpapi"
	"network_monitor/internal/adapter/ingestnet"
	appmetrics "network_monitor/internal/adapter/metrics"
	"network_monitor/internal/adapter/reputationjob"
	"network_monitor/internal/config"
)

type app struct {
	pools       *chadapter.Pools
	ingestSvc   *ingestnet.Service
	prom        *appmetrics.Registry
	srv         *httpapi.Server
	geoJobs     *geojob.Scheduler
	repJobs     *reputationjob.Scheduler
	backupJobs  *backupjob.Scheduler
	anomalyJobs *anomalyjob.Scheduler
	dataLock    *datalock.Lock
	bgCancel    context.CancelFunc
	bgWg        sync.WaitGroup
	ingestDone  chan error
	ctx         context.Context
	cancel      context.CancelFunc
	listenAddr  string
}

func buildApp(ctx context.Context, cfg config.Config) (*app, error) {
	dataDir := filepath.Dir(cfg.AuthUsersFile)
	if dataDir == "" || dataDir == "." {
		dataDir = "/app/data"
	}
	var dataLock *datalock.Lock
	if !cfg.AllowMultiInstance {
		lock, err := datalock.Acquire(dataDir)
		if err != nil {
			return nil, err
		}
		dataLock = lock
		slog.Info("control-plane lock acquired", "dir", dataDir, "file", ".nm_backend.lock")
	} else {
		slog.Warn("NM_ALLOW_MULTI_INSTANCE=1 — control-plane file lock disabled (unsafe for shared /app/data)")
	}

	authParts, err := buildAuth(cfg)
	if err != nil {
		if dataLock != nil {
			_ = dataLock.Close()
		}
		return nil, err
	}

	pools, err := connectPools(ctx, cfg)
	if err != nil {
		if dataLock != nil {
			_ = dataLock.Close()
		}
		return nil, err
	}

	aCtx, cancel := context.WithCancel(ctx)
	// WithoutCancel: bootstrap/jobs живут до явного bgCancel на shutdown,
	// а не обрываются вместе с signal ctx (пока идёт HTTP/ingest drain).
	bgCtx, bgCancel := context.WithCancel(context.WithoutCancel(ctx))
	a := &app{
		pools:      pools,
		bgCancel:   bgCancel,
		ctx:        aCtx,
		cancel:     cancel,
		listenAddr: cfg.ListenAddr,
		dataLock:   dataLock,
	}

	bg := wireBackground(ctx, bgCtx, a, cfg)
	parsers := newParserRegistry()
	a.prom = appmetrics.New(nil)
	startIngest(a, cfg, bg.geo, parsers)
	a.prom.SetIngest(a.ingestSvc)
	a.srv = buildHTTP(cfg, a, authParts, bg, parsers)
	if a.repJobs != nil {
		a.repJobs.Start(bgCtx)
	}
	if a.backupJobs != nil {
		a.backupJobs.Start(bgCtx)
	}
	if a.anomalyJobs != nil {
		a.anomalyJobs.Start(bgCtx)
	}
	return a, nil
}
