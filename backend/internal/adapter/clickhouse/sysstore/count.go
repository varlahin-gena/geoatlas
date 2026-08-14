package sysstore

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// CountTableRows — оценка строк из system.parts по allowlist (live-статус edges agg).
func CountTableRows(ctx context.Context, ch clickhouse.Conn, table string) (uint64, error) {
	switch table {
	case "traffic_logs", "traffic_edges_daily":
	default:
		return 0, fmt.Errorf("count: table %q not allowed", table)
	}
	if ch == nil {
		return 0, fmt.Errorf("count: nil conn")
	}
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var n uint64
	err := ch.QueryRow(qctx, `
		SELECT coalesce(sum(rows), 0)
		FROM system.parts
		WHERE database = currentDatabase() AND table = {table:String} AND active
	`, clickhouse.Named("table", table)).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}
