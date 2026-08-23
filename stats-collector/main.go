package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"geoatlas/stats-collector/internal/collector"
	"geoatlas/stats-collector/internal/config"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config: %v", err)
	}
	if cfg.UsingAdminFallback() {
		log.Printf("WARNING: API_OPS_TOKEN unset — using API_AUTH_TOKEN (admin). Prefer API_OPS_TOKEN for least privilege")
	}

	log.Printf("stats-collector starting")
	log.Printf("  ClickHouse:  %s", cfg.ClickHouseAddr)
	log.Printf("  Interval:    %s", cfg.CollectInterval)
	log.Printf("  Cgroup root: %s", cfg.CgroupRoot)
	log.Printf("  Host proc:   %s", cfg.HostProcRoot)
	log.Printf("  Backend URL: %s", cfg.BackendHealthURL)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	conn, err := collector.ConnectClickHouse(ctx, cfg)
	if err != nil {
		log.Fatalf("clickhouse connection failed: %v", err)
	}
	defer conn.Close()

	c := collector.NewCollector(cfg, conn)

	// Диагностика: какие cgroup-пути нашли на старте.
	if paths := c.FindCgroupPaths(); len(paths) > 0 {
		log.Printf("  Detected cgroups (%d):", len(paths))
		for target, path := range paths {
			log.Printf("    %-18s -> %s", target, path)
		}
	} else {
		log.Printf("  Detected cgroups: NONE (will retry on each cycle)")
	}

	c.Collect(ctx)

	ticker := time.NewTicker(cfg.CollectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("shutdown")
			return
		case <-ticker.C:
			c.Collect(ctx)
		}
	}
}
