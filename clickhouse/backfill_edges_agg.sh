#!/bin/bash
# Создаёт traffic_edges_daily + MV и backfill'ит историю по дням.
# Запуск из корня проекта:
#   bash clickhouse/backfill_edges_agg.sh
#
# Требует: docker compose, работающий clickhouse с traffic_logs.

set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
PROJECT_DIR="$(cd -- "$SCRIPT_DIR/.." &>/dev/null && pwd)"
cd "$PROJECT_DIR"

log() { echo "[$(date +'%F %T')] $*"; }

ch_client() {
    docker compose exec -T clickhouse \
      sh -c 'exec clickhouse-client --password "$CLICKHOUSE_PASSWORD" "$@"' sh "$@"
}

ch_query() {
    ch_client --query "$1"
}

ch_multi() {
    ch_client --multiquery < "$1"
}

log "Applying schema (traffic_edges_daily + MV)..."
ch_multi "$SCRIPT_DIR/migrate_edges_agg.sql"

RAW_ROWS="$(ch_query "SELECT count() FROM traffic_logs")"
log "traffic_logs rows: $RAW_ROWS"

if [[ "$RAW_ROWS" == "0" ]]; then
    log "Nothing to backfill."
    exit 0
fi

DAYS="$(ch_query "
    SELECT toString(d)
    FROM (
        SELECT DISTINCT toDate(parseDateTimeBestEffort(partition)) AS d
        FROM system.parts
        WHERE database = currentDatabase() AND table = 'traffic_logs' AND active
    )
    WHERE d < today() AND d > toDate('2000-01-01')
      AND d NOT IN (
        SELECT DISTINCT toDate(parseDateTimeBestEffort(partition)) AS d
        FROM system.parts
        WHERE database = currentDatabase() AND table = 'traffic_edges_daily' AND active
      )
    ORDER BY d DESC
    FORMAT TSV
")"

if [[ -z "${DAYS// }" ]]; then
    log "All days already backfilled."
    ch_query "SELECT count() AS agg_rows, count(DISTINCT day) AS days FROM traffic_edges_daily"
    exit 0
fi

mapfile -t DAY_LIST <<< "$DAYS"
TOTAL="${#DAY_LIST[@]}"
log "Days to backfill: $TOTAL"

# SoT: model.BlockedInClause(); regenerate: cd backend && go generate ./internal/model/...
# ACTION_VOCAB:BLOCKED_BEGIN
BLOCKED="'block','blocked','denied','deny','discard','discarded','drop','dropped','reject','rejected','reset'"
# ACTION_VOCAB:BLOCKED_END

i=0
for day in "${DAY_LIST[@]}"; do
    [[ -z "$day" ]] && continue
    i=$((i + 1))
    log "Backfill $i/$TOTAL — $day"
    ch_query "
        INSERT INTO traffic_edges_daily
        SELECT
            toDate(timestamp) AS day,
            src_ip,
            dst_ip,
            count() AS cnt,
            sum(toUInt64(lower(action) IN ($BLOCKED))) AS blocked_cnt,
            sum(toUInt64(lower(action) NOT IN ($BLOCKED) AND lower(action) NOT IN ('','unknown'))) AS allowed_cnt,
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
        WHERE timestamp >= toDateTime(toDate('$day')) AND timestamp < toDateTime(toDate('$day')) + INTERVAL 1 DAY
        GROUP BY day, src_ip, dst_ip
        SETTINGS max_bytes_before_external_group_by = 536870912
    "
done

log "Done."
ch_query "SELECT count() AS agg_rows, count(DISTINCT day) AS days FROM traffic_edges_daily"
