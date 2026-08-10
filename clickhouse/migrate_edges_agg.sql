-- Миграция: предварительная агрегация рёбер для карты.
-- Источник истины для IN-списков и MV при старте backend — storage.EnsureEdgesAgg
-- (model.BlockedInClause). Этот файл — ручной fallback для существующих установок.
-- Ручной запуск:
--   docker compose exec -T clickhouse clickhouse-client --multiquery < clickhouse/migrate_edges_agg.sql

CREATE TABLE IF NOT EXISTS traffic_edges_daily
(
    day           Date,
    src_ip        String,
    dst_ip        String,
    cnt           SimpleAggregateFunction(sum, UInt64),
    blocked_cnt   SimpleAggregateFunction(sum, UInt64),
    allowed_cnt   SimpleAggregateFunction(sum, UInt64),
    bytes_sent    SimpleAggregateFunction(sum, UInt64),
    bytes_recv    SimpleAggregateFunction(sum, UInt64),
    packets_sent  SimpleAggregateFunction(sum, UInt64),
    packets_recv  SimpleAggregateFunction(sum, UInt64),
    last_action   AggregateFunction(argMax, String, DateTime64(3)),
    rule          AggregateFunction(any, String),
    proto         AggregateFunction(any, String),
    src_port      AggregateFunction(any, UInt32),
    dst_port      AggregateFunction(any, UInt32),
    device        AggregateFunction(any, String),
    src_zone      AggregateFunction(any, String),
    dst_zone      AggregateFunction(any, String),
    src_country   AggregateFunction(any, String),
    dst_country   AggregateFunction(any, String)
)
ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (day, src_ip, dst_ip)
TTL day + INTERVAL 30 DAY DELETE;

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
    argMaxState(action, timestamp) AS last_action,
    anyState(rule)          AS rule,
    anyState(proto)         AS proto,
    anyState(src_port)      AS src_port,
    anyState(dst_port)      AS dst_port,
    anyState(device)        AS device,
    anyState(src_zone)      AS src_zone,
    anyState(dst_zone)      AS dst_zone,
    anyState(src_country)   AS src_country,
    anyState(dst_country)   AS dst_country
FROM traffic_logs
GROUP BY day, src_ip, dst_ip;
