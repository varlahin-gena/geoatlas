package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ClickHouse/clickhouse-go/v2"

	chadapter "geoatlas/internal/adapter/clickhouse"
	"geoatlas/internal/adapter/clickhouse/query"
	"geoatlas/internal/config"
)

func connectPools(ctx context.Context, cfg config.Config) (*chadapter.Pools, error) {
	ch := cfg.ClickHouse
	query.ConfigureQuerySettings(ch.MaxMemoryUsage, ch.ExternalGroupBy, ch.ExternalSort, ch.MaxThreads)

	ingestOpts := chadapter.PoolOptions{MaxOpenConns: ch.IngestMaxOpen, MaxIdleConns: ch.IngestMaxOpen}
	if ch.IngestAsyncInsert {
		ingestOpts.Settings = clickhouse.Settings{
			"async_insert":          1,
			"wait_for_async_insert": 1,
		}
		slog.Info("clickhouse ingest: async_insert enabled", "wait_for_async_insert", 1)
	}

	pools, err := chadapter.ConnectPools(ctx, ch.Addr(),
		chadapter.Auth{
			Database: ch.Database,
			Username: ch.User,
			Password: ch.Password,
		},
		ingestOpts,
		chadapter.PoolOptions{MaxOpenConns: ch.APIMaxOpen, MaxIdleConns: ch.APIMaxOpen},
		chadapter.PoolOptions{MaxOpenConns: ch.BackgroundMaxOpen, MaxIdleConns: ch.BackgroundMaxOpen},
	)
	if err != nil {
		return nil, fmt.Errorf("clickhouse connection: %w", err)
	}
	return pools, nil
}
