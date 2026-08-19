package migrate

const drEventsDDL = `
CREATE TABLE IF NOT EXISTS dr_events
(
    timestamp DateTime64(3),
    actor     LowCardinality(String),
    action    LowCardinality(String),
    target    String,
    status    LowCardinality(String),
    message   String,
    meta      String
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp, action, target)
TTL toDateTime(timestamp) + INTERVAL 180 DAY DELETE
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1
`

const auditEventsDDL = `
CREATE TABLE IF NOT EXISTS audit_events
(
    timestamp     DateTime64(3),
    actor         LowCardinality(String),
    action        LowCardinality(String),
    resource_type LowCardinality(String),
    resource_id   String,
    result        LowCardinality(String),
    ip            String,
    details       String
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp, action, actor)
TTL toDateTime(timestamp) + INTERVAL 180 DAY DELETE
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1
`
