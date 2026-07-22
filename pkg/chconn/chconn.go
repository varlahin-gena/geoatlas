// Package chconn — общий native-клиент ClickHouse с retry и лимитами пула.
package chconn

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// Auth — учётные данные native-протокола.
type Auth struct {
	Database string
	Username string
	Password string
}

func (a Auth) Normalized() Auth {
	if a.Database == "" {
		a.Database = "default"
	}
	if a.Username == "" {
		a.Username = "default"
	}
	return a
}

// Options задаёт размер пула и параметры retry.
type Options struct {
	Name           string
	MaxOpenConns   int
	MaxIdleConns   int
	ConnMaxLifetime time.Duration
	MaxAttempts    int
	RetryInterval  time.Duration
	PingTimeout    time.Duration
	// Settings — session settings native-протокола (например async_insert для ingest).
	Settings clickhouse.Settings
}

func (o Options) normalized() Options {
	if o.MaxOpenConns <= 0 {
		o.MaxOpenConns = 5
	}
	if o.MaxIdleConns <= 0 {
		o.MaxIdleConns = o.MaxOpenConns
	}
	if o.MaxIdleConns > o.MaxOpenConns {
		o.MaxIdleConns = o.MaxOpenConns
	}
	if o.ConnMaxLifetime <= 0 {
		o.ConnMaxLifetime = time.Hour
	}
	if o.Name == "" {
		o.Name = "default"
	}
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = 30
	}
	if o.RetryInterval <= 0 {
		o.RetryInterval = 2 * time.Second
	}
	if o.PingTimeout <= 0 {
		o.PingTimeout = 5 * time.Second
	}
	return o
}

// Connect открывает native-соединение с retry до успешного Ping.
func Connect(ctx context.Context, addr string, auth Auth, opts Options) (clickhouse.Conn, error) {
	auth = auth.Normalized()
	opts = opts.normalized()

	var lastErr error
	for i := 0; i < opts.MaxAttempts; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		conn, err := clickhouse.Open(&clickhouse.Options{
			Addr: []string{addr},
			Auth: clickhouse.Auth{
				Database: auth.Database,
				Username: auth.Username,
				Password: auth.Password,
			},
			MaxOpenConns:    opts.MaxOpenConns,
			MaxIdleConns:    opts.MaxIdleConns,
			ConnMaxLifetime: opts.ConnMaxLifetime,
			Settings:        opts.Settings,
		})
		if err == nil {
			pctx, cancel := context.WithTimeout(ctx, opts.PingTimeout)
			pingErr := conn.Ping(pctx)
			cancel()
			if pingErr == nil {
				slog.Info("clickhouse connected",
					"pool", opts.Name,
					"addr", addr,
					"database", auth.Database,
					"user", auth.Username,
					"max_open", opts.MaxOpenConns,
					"max_idle", opts.MaxIdleConns,
					"settings", len(opts.Settings),
				)
				return conn, nil
			}
			_ = conn.Close()
			lastErr = pingErr
		} else {
			lastErr = err
		}

		select {
		case <-time.After(opts.RetryInterval):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("clickhouse connect (%s) failed after %d attempts: %w", opts.Name, opts.MaxAttempts, lastErr)
}
