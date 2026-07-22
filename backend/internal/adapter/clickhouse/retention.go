package clickhouse

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// RetentionApplier — MODIFY TTL для групп таблиц (usecase/retention.TTLApplier).
type RetentionApplier struct {
	ch clickhouse.Conn
}

func NewRetentionApplier(ch clickhouse.Conn) *RetentionApplier {
	return &RetentionApplier{ch: ch}
}

func (a *RetentionApplier) ApplyTrafficLogs(ctx context.Context, days int) error {
	return a.modifyTTL(ctx, "traffic_logs",
		fmt.Sprintf("toDateTime(timestamp) + INTERVAL %d DAY DELETE", days))
}

func (a *RetentionApplier) ApplyParseErrors(ctx context.Context, days int) error {
	return a.modifyTTL(ctx, "parse_errors",
		fmt.Sprintf("toDateTime(timestamp) + INTERVAL %d DAY DELETE", days))
}

func (a *RetentionApplier) ApplySystemMetrics(ctx context.Context, days int) error {
	return a.modifyTTL(ctx, "system_metrics",
		fmt.Sprintf("timestamp + INTERVAL %d DAY DELETE", days))
}

func (a *RetentionApplier) ApplyEdges(ctx context.Context, days int) error {
	expr := fmt.Sprintf("day + INTERVAL %d DAY DELETE", days)
	tables := []string{
		"traffic_edges_daily",
		"traffic_edges_city_daily",
		"traffic_edges_country_daily",
	}
	for _, table := range tables {
		ok, err := a.tableExists(ctx, table)
		if err != nil {
			return err
		}
		if !ok {
			slog.Info("retention: skip missing table", "table", table)
			continue
		}
		if err := a.modifyTTL(ctx, table, expr); err != nil {
			return err
		}
	}
	return nil
}

func (a *RetentionApplier) modifyTTL(ctx context.Context, table, ttlExpr string) error {
	if a == nil || a.ch == nil {
		return fmt.Errorf("clickhouse not configured")
	}
	ok, err := a.tableExists(ctx, table)
	if err != nil {
		return err
	}
	if !ok {
		slog.Info("retention: skip missing table", "table", table)
		return nil
	}
	qctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	ddl := fmt.Sprintf("ALTER TABLE %s MODIFY TTL %s", table, ttlExpr)
	if err := a.ch.Exec(qctx, ddl); err != nil {
		return fmt.Errorf("%s: %w", table, err)
	}
	slog.Info("retention: TTL updated", "table", table, "ttl", ttlExpr)
	return nil
}

func (a *RetentionApplier) tableExists(ctx context.Context, name string) (bool, error) {
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var n uint64
	err := a.ch.QueryRow(qctx, `
		SELECT count()
		FROM system.tables
		WHERE database = currentDatabase() AND name = ?
	`, name).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
