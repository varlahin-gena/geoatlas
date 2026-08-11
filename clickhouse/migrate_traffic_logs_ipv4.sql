-- Ops fallback: traffic_logs.src_ip/dst_ip String → IPv4.
-- Предпочтительно: backend EnsureTrafficLogsIPv4 (EXCHANGE + version bump).
-- Этот скрипт — аварийный путь, если Ensure* недоступен.
--
-- ВНИМАНИЕ: требует место ≈ размера traffic_logs; MV edges пересоздаст Ensure*.

CREATE TABLE IF NOT EXISTS traffic_logs__ipv4_next
(
    timestamp     DateTime64(3),
    parsed_at     DateTime64(3) DEFAULT now64(3),
    ingest_time   DateTime64(3) DEFAULT now64(3),
    vendor        LowCardinality(String) DEFAULT '',
    device        String,
    src_ip        IPv4,
    dst_ip        IPv4,
    src_port      UInt32,
    dst_port      UInt32,
    action        LowCardinality(String),
    success       UInt8 MATERIALIZED 0,
    rule          String,
    proto         LowCardinality(String),
    src_zone      String,
    dst_zone      String,
    src_country   String,
    dst_country   String,
    src_city      String DEFAULT '',
    dst_city      String DEFAULT '',
    src_region    String DEFAULT '',
    dst_region    String DEFAULT '',
    src_lat       Float64 DEFAULT 0,
    src_lon       Float64 DEFAULT 0,
    dst_lat       Float64 DEFAULT 0,
    dst_lon       Float64 DEFAULT 0,
    bytes_sent    UInt64,
    bytes_recv    UInt64,
    packets_sent  UInt64,
    packets_recv  UInt64,
    raw           String CODEC(ZSTD(3)),
    INDEX idx_src_ip      src_ip      TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_dst_ip      dst_ip      TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_dst_port    dst_port    TYPE minmax              GRANULARITY 4,
    INDEX idx_action      action      TYPE set(0)              GRANULARITY 4,
    INDEX idx_dst_country dst_country TYPE bloom_filter(0.01) GRANULARITY 4
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp, src_ip, dst_ip, action)
TTL toDateTime(timestamp) + INTERVAL 30 DAY DELETE
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

INSERT INTO traffic_logs__ipv4_next (
    timestamp, parsed_at, ingest_time, vendor, device,
    src_ip, dst_ip, src_port, dst_port, action,
    rule, proto, src_zone, dst_zone, src_country, dst_country,
    src_city, dst_city, src_region, dst_region,
    src_lat, src_lon, dst_lat, dst_lon,
    bytes_sent, bytes_recv, packets_sent, packets_recv, raw
)
SELECT
    timestamp, parsed_at, ingest_time, vendor, device,
    toIPv4OrZero(src_ip), toIPv4OrZero(dst_ip), src_port, dst_port, action,
    rule, proto, src_zone, dst_zone, src_country, dst_country,
    src_city, dst_city, src_region, dst_region,
    src_lat, src_lon, dst_lat, dst_lon,
    bytes_sent, bytes_recv, packets_sent, packets_recv, raw
FROM traffic_logs;

EXCHANGE TABLES traffic_logs AND traffic_logs__ipv4_next;
DROP TABLE IF EXISTS traffic_logs__ipv4_next;
