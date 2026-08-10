package main

import (
	"context"
	"log/slog"
	"network_monitor/internal/config"
	"network_monitor/internal/logging"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg := config.FromEnv()
	logging.Setup(cfg.LogLevel, cfg.LogFormat)
	cfg.ResolveGeoUploadLimits()

	if err := cfg.ValidateConfig(); err != nil {
		slog.Error("configuration", "err", err)
		os.Exit(1)
	}
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

	slog.Info("network-monitor starting", "edges_agg", true, "geo_enrich_on_ingest", cfg.GeoEnrichOnIngest)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	a, err := buildApp(ctx, cfg)
	if err != nil {
		slog.Error("build application", "err", err)
		os.Exit(1)
	}
	if err := a.run(ctx); err != nil {
		slog.Error("run application", "err", err)
		os.Exit(1)
	}
}
