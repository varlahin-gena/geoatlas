package main

import (
	"log/slog"
	"time"

	"network_monitor/internal/adapter/clickhouse/geostore"
	"network_monitor/internal/adapter/clickhouse/ingeststore"
	"network_monitor/internal/adapter/parseradapter"
	"network_monitor/internal/config"
	"network_monitor/internal/ingest"
	"network_monitor/internal/parser"
	usecaseingest "network_monitor/internal/usecase/ingest"
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
	a.ingestSvc = ingest.NewService(ingest.Config{
		Bindings:        ingestBindings(cfg),
		BatchSize:       cfg.IngestBatchSize,
		FlushInterval:   time.Duration(cfg.IngestFlushSec) * time.Second,
		QueueSize:       cfg.IngestQueueSize,
		QueueMaxBytes:   cfg.IngestQueueMaxBytes,
		Workers:         cfg.IngestWorkers,
		QueryTimeout:    cfg.QueryTimeout,
		MaxConnections:  cfg.IngestMaxConnections,
		ConnIdleTimeout: time.Duration(cfg.IngestConnIdleSec) * time.Second,
	}, ingest.ProcessorDeps{
		Logs: ingestRepo, Errors: ingestRepo, Parser: lineParser,
		Geo: geo, EnrichCountry: cfg.GeoEnrichOnIngest,
		InsertObs: insertObs,
	})
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
