package migrate

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// EnsureHTTPSchema — минимум, без которого /api/events падает (нет таблиц/колонок).
// Идемпотентно. bootstrap.RunStartup вызывает тот же набор плюс remainder.
func EnsureHTTPSchema(ctx context.Context, ch clickhouse.Conn) error {
	if err := EnsureBaseSchema(ctx, ch); err != nil {
		return fmt.Errorf("base: %w", err)
	}
	if err := EnsureTrafficLogsIPv4(ctx, ch); err != nil {
		return fmt.Errorf("traffic_logs ipv4: %w", err)
	}
	if err := EnsureGeoEdgesAggSchema(ctx, ch); err != nil {
		return fmt.Errorf("geo edges: %w", err)
	}
	return nil
}
