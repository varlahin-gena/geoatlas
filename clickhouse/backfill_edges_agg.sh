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

ch_query() {
    docker compose exec -T clickhouse clickhouse-client --query "$1"
}

ch_multi() {
    docker compose exec -T clickhouse clickhouse-client --multiquery < "$1"
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
    SELECT toString(days.d) AS day
    FROM (
        SELECT DISTINCT toDate(timestamp) AS d FROM traffic_logs
    ) AS days
    LEFT ANTI JOIN (
        SELECT DISTINCT day AS d FROM traffic_edges_daily
    ) AS agg USING (d)
    ORDER BY day DESC
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
        WHERE toDate(timestamp) = toDate('$day')
        GROUP BY day, src_ip, dst_ip
        SETTINGS max_bytes_before_external_group_by = 536870912
    "
done

log "Done."
ch_query "SELECT count() AS agg_rows, count(DISTINCT day) AS days FROM traffic_edges_daily"
