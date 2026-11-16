package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"geoatlas/stats-collector/internal/collector"
	"geoatlas/stats-collector/internal/config"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	if cfg.UsingAdminFallback() {
		slog.Warn("API_OPS_TOKEN unset — using API_AUTH_TOKEN (admin). Prefer API_OPS_TOKEN for least privilege")
	}

	slog.Info("stats-collector starting",
		"clickhouse", cfg.ClickHouseAddr,
		"interval", cfg.CollectInterval.String(),
		"cgroup_root", cfg.CgroupRoot,
		"host_proc", cfg.HostProcRoot,
		"backend_url", cfg.BackendHealthURL,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	conn, err := collector.ConnectClickHouse(ctx, cfg)
	if err != nil {
		slog.Error("clickhouse connection failed", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	c := collector.NewCollector(cfg, conn)

	// Диагностика: какие cgroup-пути нашли на старте.
	if paths := c.FindCgroupPaths(); len(paths) > 0 {
		slog.Info("detected cgroups", "count", len(paths), "paths", paths)
	} else {
		slog.Warn("detected cgroups: none, will retry on each cycle")
	}

	c.Collect(ctx)

	ticker := time.NewTicker(cfg.CollectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutdown")
			return
		case <-ticker.C:
			c.Collect(ctx)
		}
	}
}
