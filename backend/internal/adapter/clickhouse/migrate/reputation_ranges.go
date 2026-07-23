package migrate

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
)

const reputationRangesDDL = `
CREATE TABLE IF NOT EXISTS reputation_ranges
(
    list_name   LowCardinality(String),
    category    LowCardinality(String),
    start_ip    UInt32,
    end_ip      UInt32,
    source      LowCardinality(String),
    updated_at  DateTime64(3)
)
ENGINE = MergeTree()
ORDER BY (list_name, start_ip, end_ip)
`

// EnsureReputationRanges создаёт таблицу списков (идемпотентно).
func EnsureReputationRanges(ctx context.Context, ch clickhouse.Conn) error {
	if ch == nil {
		return fmt.Errorf("clickhouse conn is nil")
	}
	if err := ch.Exec(ctx, reputationRangesDDL); err != nil {
		return fmt.Errorf("ensure reputation_ranges: %w", err)
	}
	return nil
}
