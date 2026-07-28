package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ClickHouse/clickhouse-go/v2"

	chadapter "network_monitor/internal/adapter/clickhouse"
	"network_monitor/internal/adapter/clickhouse/query"
	"network_monitor/internal/config"
)

func connectPools(ctx context.Context, cfg config.Config) (*chadapter.Pools, error) {
	query.ConfigureQuerySettings(cfg.CHMaxMemoryUsage, cfg.CHExternalGroupBy, cfg.CHExternalSort)

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
		return nil, fmt.Errorf("clickhouse connection: %w", err)
	}
	return pools, nil
}
