package migrate

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// Base-table DDL is the schema SoT together with Ensure* for aggregates.
// clickhouse/init.sql is generated from coldBootstrapStatements (go generate).

const geoRangesDDL = `
CREATE TABLE IF NOT EXISTS geo_ranges
(
    start_ip UInt32,
    end_ip   UInt32,
    country  String,
    region   String,
    city     String,
    lat      Float64,
    lon      Float64
)
ENGINE = MergeTree()
ORDER BY (start_ip, end_ip)
`

const parseErrorsDDL = `
CREATE TABLE IF NOT EXISTS parse_errors
(
    id        UUID DEFAULT generateUUIDv4(),
    timestamp DateTime64(3) DEFAULT now64(3),
    vendor    LowCardinality(String) DEFAULT '',
    reason    String,
    raw       String CODEC(ZSTD(3))
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY timestamp
TTL toDateTime(timestamp) + INTERVAL 7 DAY DELETE
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1
`

const systemMetricsDDL = `
CREATE TABLE IF NOT EXISTS system_metrics
(
    timestamp     DateTime DEFAULT now(),
    metric_type   LowCardinality(String),
    target        LowCardinality(String),
    metric_name   LowCardinality(String),
    value         Float64,
    labels        String DEFAULT ''
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp, metric_type, target, metric_name)
TTL timestamp + INTERVAL 7 DAY DELETE
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1
`

const metricsLatestViewDDL = `
CREATE VIEW IF NOT EXISTS v_metrics_latest AS
SELECT
    metric_type, target, metric_name,
    argMax(value, timestamp) AS value,
    argMax(labels, timestamp) AS labels,
    max(timestamp) AS last_seen
FROM system_metrics
WHERE timestamp >= now() - INTERVAL 5 MINUTE
GROUP BY metric_type, target, metric_name
`

type bootstrapStmt struct {
	title string
	sql   string
}

// ColdBootstrapStatements — базовые таблицы/view для пустого тома.
// Агрегаты карты (traffic_edges_*) создаёт Ensure*, не cold bootstrap.
func coldBootstrapStatements() []bootstrapStmt {
	return []bootstrapStmt{
		{title: "traffic_logs: основной поток событий МСЭ", sql: trafficLogsCreateSQL("traffic_logs", true)},
		{title: "geo_ranges: GeoIP-база", sql: geoRangesDDL},
		{title: "reputation_ranges: офлайн-репутационные списки", sql: reputationRangesDDL},
		{title: "parse_errors: строки логов, которые не удалось распарсить", sql: parseErrorsDDL},
		{title: "system_metrics: метрики самой системы", sql: systemMetricsDDL},
		{title: "v_metrics_latest: последние значения метрик", sql: metricsLatestViewDDL},
	}
}

// EnsureBaseSchema создаёт базовые таблицы, если их нет (идемпотентно).
// Нужна и на пустом томе без init.sql, и при добавлении таблиц на старых установках.
func EnsureBaseSchema(ctx context.Context, ch clickhouse.Conn) error {
	if ch == nil {
		return fmt.Errorf("clickhouse conn is nil")
	}
	for _, stmt := range coldBootstrapStatements() {
		if err := execDDL(ctx, ch, stmt.sql); err != nil {
			return fmt.Errorf("ensure base schema %s: %w", stmt.title, err)
		}
	}
	return nil
}
