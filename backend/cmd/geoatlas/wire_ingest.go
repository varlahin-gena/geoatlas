package main

import (
	"log/slog"
	"time"

	"geoatlas/internal/adapter/clickhouse/geostore"
	"geoatlas/internal/adapter/clickhouse/ingeststore"
	"geoatlas/internal/adapter/ingestnet"
	"geoatlas/internal/adapter/parseradapter"
	"geoatlas/internal/config"
	"geoatlas/internal/parser"
	usecaseingest "geoatlas/internal/usecase/ingest"
)

func newParserRegistry() *parser.Registry {
	return parser.NewRegistry(
		&parser.UserGateCEF{},
		&parser.FortigateCEF{},
		&parser.CiscoFTD{},
		&parser.CiscoASA{},
		&parser.CowrieJSON{},
		&parser.GenericKV{},
	)
}

func startIngest(a *app, cfg config.Config, geo *geostore.ReloadableGeoIndex, parsers *parser.Registry) {
	lineParser := parseradapter.New(parsers)
	ingestRepo := ingeststore.NewIngestRepository(a.pools.Ingest)
	var insertObs usecaseingest.InsertObserver
	if a.prom != nil {
		insertObs = a.prom
	}
	a.ingestSvc = ingestnet.NewService(ingestnet.Config{
		Bindings:        ingestBindings(cfg),
		BatchSize:       cfg.Ingest.BatchSize,
		FlushInterval:   time.Duration(cfg.Ingest.FlushSec) * time.Second,
		QueueSize:       cfg.Ingest.QueueSize,
		QueueMaxBytes:   cfg.Ingest.QueueMaxBytes,
		Workers:         cfg.Ingest.Workers,
		QueryTimeout:    cfg.QueryTimeout,
		MaxConnections:  cfg.Ingest.MaxConnections,
		ConnIdleTimeout: time.Duration(cfg.Ingest.ConnIdleSec) * time.Second,
		SharedSecret:    cfg.Ingest.SharedSecret,
		AllowFrom:       cfg.Ingest.AllowFrom,
	}, ingestnet.ProcessorDeps{
		Logs: ingestRepo, Errors: ingestRepo, Parser: lineParser,
		Geo: geo, EnrichCountry: cfg.Geo.EnrichOnIngest,
		InsertObs: insertObs,
		Retryable: usecaseingest.InsertErrorClassifyFunc(ingeststore.IsRetryableInsertError),
	})
	if a.heavy != nil {
		heavy := a.heavy
		a.ingestSvc.SetDegradeProbe(func() bool { return heavy.Busy() })
	}
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
}

func ingestBindings(cfg config.Config) []ingestnet.Binding {
	if cfg.Ingest.ListenAddr != "" {
		return []ingestnet.Binding{{Addr: cfg.Ingest.ListenAddr}}
	}
	var bindings []ingestnet.Binding
	if cfg.Ingest.UDPListenAddr != "" {
		bindings = append(bindings, ingestnet.Binding{Addr: cfg.Ingest.UDPListenAddr, Transport: "udp"})
	}
	if cfg.Ingest.TCPListenAddr != "" {
		bindings = append(bindings, ingestnet.Binding{Addr: cfg.Ingest.TCPListenAddr, Transport: "tcp"})
	}
	return bindings
}
