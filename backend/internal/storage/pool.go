package storage

import (
	"context"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/pkg/chconn"
)

// Auth — учётные данные native-протокола ClickHouse.
type Auth = chconn.Auth

// PoolOptions задаёт размер пула native-соединений clickhouse-go.
type PoolOptions struct {
	Name           string
	MaxOpenConns   int
	MaxIdleConns   int
	ConnMaxLifetime time.Duration
	Settings       clickhouse.Settings
}

// Pools разделяет write / read / background пути, чтобы тяжёлые SELECT
// и edges-backfill не конкурировали с batch INSERT ingest.
type Pools struct {
	Ingest     clickhouse.Conn
	API        clickhouse.Conn
	Background clickhouse.Conn
}

// ConnectWithPool подключается к ClickHouse с заданным размером пула.
func ConnectWithPool(ctx context.Context, addr string, auth Auth, opts PoolOptions) (clickhouse.Conn, error) {
	return chconn.Connect(ctx, addr, auth, chconn.Options{
		Name:           opts.Name,
		MaxOpenConns:   opts.MaxOpenConns,
		MaxIdleConns:   opts.MaxIdleConns,
		ConnMaxLifetime: opts.ConnMaxLifetime,
		Settings:       opts.Settings,
	})
}

// ConnectPools открывает три независимых пула.
func ConnectPools(ctx context.Context, addr string, auth Auth, ingest, api, background PoolOptions) (*Pools, error) {
	ingest.Name = "ingest"
	api.Name = "api"
	background.Name = "background"

	ing, err := ConnectWithPool(ctx, addr, auth, ingest)
	if err != nil {
		return nil, err
	}
	apiConn, err := ConnectWithPool(ctx, addr, auth, api)
	if err != nil {
		_ = ing.Close()
		return nil, err
	}
	bg, err := ConnectWithPool(ctx, addr, auth, background)
	if err != nil {
		_ = ing.Close()
		_ = apiConn.Close()
		return nil, err
	}
	return &Pools{Ingest: ing, API: apiConn, Background: bg}, nil
}

// Close закрывает все пулы. Ошибки логируются, первая возвращается.
func (p *Pools) Close() error {
	if p == nil {
		return nil
	}
	var first error
	for name, c := range map[string]clickhouse.Conn{
		"ingest":     p.Ingest,
		"api":        p.API,
		"background": p.Background,
	} {
		if c == nil {
			continue
		}
		if err := c.Close(); err != nil {
			slog.Warn("clickhouse pool close failed", "pool", name, "err", err)
			if first == nil {
				first = err
			}
		}
	}
	return first
}
