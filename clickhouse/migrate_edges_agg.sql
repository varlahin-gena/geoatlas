-- Миграция: предварительная агрегация рёбер для карты (IP daily + coords).
-- Источник истины для IN-списков и MV при старте backend — storage.EnsureEdgesAgg
-- (model.BlockedInClause). Этот файл — ручной fallback для существующих установок.
-- Ручной запуск:
--   docker compose exec -T clickhouse sh -c \
--     'clickhouse-client --password "$CLICKHOUSE_PASSWORD" --multiquery' \
--     < clickhouse/migrate_edges_agg.sql
--
-- src_ip/dst_ip: IPv4. На старом томе лучше дать backend Ensure* (EXCHANGE).

CREATE TABLE IF NOT EXISTS traffic_edges_daily
(
    day           Date,
    src_ip        IPv4,
    dst_ip        IPv4,
    cnt           SimpleAggregateFunction(sum, UInt64),
    blocked_cnt   SimpleAggregateFunction(sum, UInt64),
    allowed_cnt   SimpleAggregateFunction(sum, UInt64),
    bytes_sent    SimpleAggregateFunction(sum, UInt64),
    bytes_recv    SimpleAggregateFunction(sum, UInt64),
    packets_sent  SimpleAggregateFunction(sum, UInt64),
    packets_recv  SimpleAggregateFunction(sum, UInt64),
    src_lat_sum   SimpleAggregateFunction(sum, Float64),
    src_lon_sum   SimpleAggregateFunction(sum, Float64),
    dst_lat_sum   SimpleAggregateFunction(sum, Float64),
    dst_lon_sum   SimpleAggregateFunction(sum, Float64),
    coord_weight  SimpleAggregateFunction(sum, UInt64),
    last_action   AggregateFunction(argMax, String, DateTime64(3)),
    rule          AggregateFunction(any, String),
    proto         AggregateFunction(any, String),
    src_port      AggregateFunction(any, UInt32),
    dst_port      AggregateFunction(any, UInt32),
    device        AggregateFunction(any, String),
    src_zone      AggregateFunction(any, String),
    dst_zone      AggregateFunction(any, String),
    src_country   AggregateFunction(any, String),
    dst_country   AggregateFunction(any, String),
    src_city      AggregateFunction(any, String),
    dst_city      AggregateFunction(any, String)
)
ENGINE = AggregatingMergeTree()
PARTITION BY day
ORDER BY (day, src_ip, dst_ip)
TTL day + INTERVAL 30 DAY DELETE
SETTINGS ttl_only_drop_parts = 1;

CREATE MATERIALIZED VIEW IF NOT EXISTS traffic_edges_daily_mv
TO traffic_edges_daily AS
SELECT
    toDate(timestamp) AS day,
    src_ip,
    dst_ip,
    count() AS cnt,
    -- SoT blocked list: model.BlockedInClause(); go generate ./internal/model/...
    sum(if(lower(action) IN (
/* ACTION_VOCAB:BLOCKED_BEGIN */
'block','blocked','denied','deny','discard','discarded','drop','dropped','reject','rejected','reset'
/* ACTION_VOCAB:BLOCKED_END */
    ), toUInt64(1), toUInt64(0))) AS blocked_cnt,
    sum(if(lower(action) NOT IN (
/* ACTION_VOCAB:BLOCKED_BEGIN */
'block','blocked','denied','deny','discard','discarded','drop','dropped','reject','rejected','reset'
/* ACTION_VOCAB:BLOCKED_END */
    ) AND lower(action) NOT IN ('','unknown'), toUInt64(1), toUInt64(0))) AS allowed_cnt,
    sum(bytes_sent) AS bytes_sent,
    sum(bytes_recv) AS bytes_recv,
    sum(packets_sent) AS packets_sent,
    sum(packets_recv) AS packets_recv,
    sumIf(src_lat, (src_lat != 0 OR src_lon != 0) AND (dst_lat != 0 OR dst_lon != 0)) AS src_lat_sum,
    sumIf(src_lon, (src_lat != 0 OR src_lon != 0) AND (dst_lat != 0 OR dst_lon != 0)) AS src_lon_sum,
    sumIf(dst_lat, (src_lat != 0 OR src_lon != 0) AND (dst_lat != 0 OR dst_lon != 0)) AS dst_lat_sum,
    sumIf(dst_lon, (src_lat != 0 OR src_lon != 0) AND (dst_lat != 0 OR dst_lon != 0)) AS dst_lon_sum,
    sum(if((src_lat != 0 OR src_lon != 0) AND (dst_lat != 0 OR dst_lon != 0), toUInt64(1), toUInt64(0))) AS coord_weight,
    argMaxState(action, timestamp) AS last_action,
    anyState(rule)          AS rule,
    anyState(proto)         AS proto,
    anyState(src_port)      AS src_port,
    anyState(dst_port)      AS dst_port,
    anyState(device)        AS device,
    anyState(src_zone)      AS src_zone,
    anyState(dst_zone)      AS dst_zone,
    anyState(src_country)   AS src_country,
    anyState(dst_country)   AS dst_country,
    anyState(src_city)      AS src_city,
    anyState(dst_city)      AS dst_city
FROM traffic_logs
GROUP BY day, src_ip, dst_ip;
