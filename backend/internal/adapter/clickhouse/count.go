package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// CountTableRows — count() по allowlist-таблицам (для live-статуса edges agg).
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
	if err := ch.QueryRow(qctx, fmt.Sprintf("SELECT count() FROM %s", table)).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
