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
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

CREATE VIEW IF NOT EXISTS v_metrics_latest AS
SELECT
    metric_type, target, metric_name,
    argMax(value, timestamp) AS value,
    argMax(labels, timestamp) AS labels,
    max(timestamp) AS last_seen
FROM system_metrics
WHERE timestamp >= now() - INTERVAL 5 MINUTE
GROUP BY metric_type, target, metric_name;
