-- ============================================================
-- Ensure* (backend/internal/adapter/clickhouse/migrate) is schema SoT for aggregates/MV.
-- This file is cold bootstrap only (empty ClickHouse volume).
-- migrate_*.sql are ops fallback when Ensure* cannot run.
-- ============================================================

-- ============================================================
-- traffic_logs: основной поток событий МСЭ
-- ============================================================
CREATE TABLE IF NOT EXISTS traffic_logs
(
    timestamp     DateTime64(3),
    parsed_at     DateTime64(3) DEFAULT now64(3),
    ingest_time   DateTime64(3) DEFAULT now64(3),

    vendor        LowCardinality(String) DEFAULT '',
    device        String,

    src_ip        String,
    dst_ip        String,
    src_port      UInt32,
    dst_port      UInt32,

    action        LowCardinality(String),
    -- Должен совпадать с model.AllowedInClause() (backend/internal/model/actions.go).
    success       UInt8 MATERIALIZED
                  if(lower(action) IN (
                      'accept','accepted','allow','allowed','built','close','decrypt',
                      'forward','inspect','mirror','monitor','nat','pass','permit','permitted',
                      'proxy','redirect','route','start','teardown','trust'
                  ), 1, 0),

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

    raw           String CODEC(ZSTD(3))
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp, src_ip, dst_ip, action)
TTL toDateTime(timestamp) + INTERVAL 30 DAY DELETE
-- ttl_only_drop_parts: удаляем целые дневные партиции вместо дорогих TTL-мерджей по строкам.
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- Полезные skip-индексы для частых фильтров
ALTER TABLE traffic_logs ADD INDEX IF NOT EXISTS idx_src_ip      src_ip      TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE traffic_logs ADD INDEX IF NOT EXISTS idx_dst_ip      dst_ip      TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE traffic_logs ADD INDEX IF NOT EXISTS idx_dst_port    dst_port    TYPE minmax              GRANULARITY 4;
ALTER TABLE traffic_logs ADD INDEX IF NOT EXISTS idx_action      action      TYPE set(0)              GRANULARITY 4;
ALTER TABLE traffic_logs ADD INDEX IF NOT EXISTS idx_dst_country dst_country TYPE bloom_filter(0.01) GRANULARITY 4;


-- ============================================================
-- geo_ranges: GeoIP-база (пользовательская + публичная)
-- ============================================================
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
ORDER BY (start_ip, end_ip);


-- ============================================================
-- parse_errors: строки логов, которые не удалось распарсить
-- ============================================================
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
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;


-- ============================================================
-- Удобные представления для аналитики
-- ============================================================

-- Сводка по дням
CREATE VIEW IF NOT EXISTS v_traffic_daily AS
SELECT
    toDate(timestamp)                 AS day,
    vendor,
    count()                           AS events,
    countIf(success = 1)              AS allowed,
    countIf(success = 0)              AS blocked,
    uniqExact(src_ip)                 AS unique_src,
    uniqExact(dst_ip)                 AS unique_dst,
    sum(bytes_sent + bytes_recv)      AS bytes_total
FROM traffic_logs
GROUP BY day, vendor;

-- Топ заблокированных назначений
CREATE VIEW IF NOT EXISTS v_top_blocked_dst AS
SELECT
    dst_ip,
    dst_country,
    dst_port,
    count() AS attempts
FROM traffic_logs
WHERE success = 0
GROUP BY dst_ip, dst_country, dst_port
ORDER BY attempts DESC;

-- ============================================================
-- system_metrics: метрики самой системы (CPU, mem, pipeline rate, etc.)
-- ============================================================
CREATE TABLE IF NOT EXISTS system_metrics
(
    timestamp     DateTime DEFAULT now(),
    metric_type   LowCardinality(String),  -- 'container', 'pipeline', 'storage', 'health'
    target        LowCardinality(String),  -- 'backend', 'clickhouse', 'importer', 'pipeline_rate', ...
    metric_name   LowCardinality(String),  -- 'cpu_pct', 'mem_bytes', 'events_per_sec', 'lag_sec', ...
    value         Float64,
    labels        String DEFAULT ''
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp, metric_type, target, metric_name)
TTL timestamp + INTERVAL 7 DAY DELETE
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- Удобный view для последних значений
CREATE VIEW IF NOT EXISTS v_metrics_latest AS
SELECT
    metric_type, target, metric_name,
    argMax(value, timestamp) AS value,
    argMax(labels, timestamp) AS labels,
    max(timestamp) AS last_seen
FROM system_metrics
WHERE timestamp >= now() - INTERVAL 5 MINUTE
GROUP BY metric_type, target, metric_name;

-- Map aggregates (traffic_edges_daily, geo edges, MVs) are created by storage.Ensure*.
