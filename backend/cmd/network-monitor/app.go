package main

import (
	"context"
	"sync"

	chadapter "network_monitor/internal/adapter/clickhouse"
	"network_monitor/internal/adapter/backupjob"
	"network_monitor/internal/adapter/geojob"
	httpapi "network_monitor/internal/adapter/httpapi"
	"network_monitor/internal/adapter/reputationjob"
	"network_monitor/internal/config"
	"network_monitor/internal/ingest"
)

type app struct {
	pools      *chadapter.Pools
	ingestSvc  *ingest.Service
	srv        *httpapi.Server
	geoJobs    *geojob.Scheduler
	repJobs    *reputationjob.Scheduler
	backupJobs *backupjob.Scheduler
	bgCancel   context.CancelFunc
	bgWg       sync.WaitGroup
	ingestDone chan error
	ctx        context.Context
	cancel     context.CancelFunc
	listenAddr string
}

func buildApp(ctx context.Context, cfg config.Config) (*app, error) {
	authParts, err := buildAuth(cfg)
	if err != nil {
		return nil, err
	}

	pools, err := connectPools(ctx, cfg)
	if err != nil {
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
	}

	bg := wireBackground(ctx, bgCtx, a, cfg)
	parsers := newParserRegistry()
	startIngest(a, cfg, bg.geo, parsers)
	a.srv = buildHTTP(cfg, a, authParts, bg, parsers)
	if a.repJobs != nil {
		a.repJobs.Start(bgCtx)
	}
	if a.backupJobs != nil {
		a.backupJobs.Start(bgCtx)
	}
	return a, nil
}
