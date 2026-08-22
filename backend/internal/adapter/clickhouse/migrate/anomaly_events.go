package migrate

const anomalyEventsDDL = `
CREATE TABLE IF NOT EXISTS anomaly_events
(
    detected_at   DateTime64(3),
    window_start  DateTime64(3),
    window_end    DateTime64(3),
    code          LowCardinality(String),
    severity      LowCardinality(String),
    score         Float32,
    title         String,
    detail        String,
    src_ip        IPv4 DEFAULT toIPv4('0.0.0.0'),
    dst_ip        IPv4 DEFAULT toIPv4('0.0.0.0'),
    src_country   LowCardinality(String) DEFAULT '',
    dst_country   LowCardinality(String) DEFAULT '',
    src_city      LowCardinality(String) DEFAULT '',
    dst_city      LowCardinality(String) DEFAULT '',
    device        LowCardinality(String) DEFAULT '',
    event_count   UInt64,
    fingerprint   String,
    suppression_key String DEFAULT '',
    expires_at    DateTime64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(detected_at)
ORDER BY (detected_at, code, fingerprint)
TTL toDateTime(detected_at) + INTERVAL 30 DAY DELETE
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1
`

const anomalyAcksDDL = `
CREATE TABLE IF NOT EXISTS anomaly_acks
(
    fingerprint String,
    ack_at      DateTime64(3),
    ack_by      String
)
ENGINE = ReplacingMergeTree(ack_at)
ORDER BY fingerprint
`

const anomalySuppressionsDDL = `
CREATE TABLE IF NOT EXISTS anomaly_suppressions
(
    suppression_key   String,
    code              LowCardinality(String),
    source_fingerprint String,
    suppressed_at     DateTime64(3),
    suppressed_until  DateTime64(3),
    suppressed_by     String
)
ENGINE = ReplacingMergeTree(suppressed_at)
ORDER BY suppression_key
TTL toDateTime(suppressed_until) + INTERVAL 1 DAY DELETE
`

const anomalyAssignmentsDDL = `
CREATE TABLE IF NOT EXISTS anomaly_assignments
(
    fingerprint String,
    assigned_to String,
    assigned_at DateTime64(3),
    assigned_by String
)
ENGINE = ReplacingMergeTree(assigned_at)
ORDER BY fingerprint
`
