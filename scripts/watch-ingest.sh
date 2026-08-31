#!/bin/bash
# Мониторинг ingest во время нагрузочного теста.
# Использование: ./scripts/watch-ingest.sh [interval_sec]
# При защищённом /api/ingest/stats: API_AUTH_TOKEN=... ./scripts/watch-ingest.sh
#
# Capacity SLO: drop/s должен быть 0. Любой устойчивый drop/s — инцидент ёмкости
# (алерты ingest_dropping / ingest_dropping_critical на /system.html).
# См. docs/operations.md «Мониторинг ingest».

set -euo pipefail

INTERVAL="${1:-2}"
URL="${INGEST_STATS_URL:-http://127.0.0.1/api/ingest/stats}"
AUTH_HEADER=()
if [[ -n "${API_AUTH_TOKEN:-}" ]]; then
    AUTH_HEADER=(-H "Authorization: Bearer ${API_AUTH_TOKEN}")
fi

prev_recv=0
prev_ins=0
prev_drop=0
prev_ts=$(date +%s)

printf "%-19s %10s %10s %10s %10s %10s %8s %-10s %s\n" \
    "time" "recv/s" "ins/s" "drop/s" "dropped" "buffered" "conn" "state" "queue"
while true; do
    json=$(curl -sf "${AUTH_HEADER[@]}" "$URL" || echo '{}')
    recv=$(echo "$json" | jq -r '.received_total // 0')
    ins=$(echo "$json" | jq -r '.inserted_total // 0')
    drop=$(echo "$json" | jq -r '.dropped_total // 0')
    buf=$(echo "$json" | jq -r '.buffered_lines // 0')
    depth=$(echo "$json" | jq -r '.queue_depth // 0')
    cap=$(echo "$json" | jq -r '.queue_capacity // 0')
    qbytes=$(echo "$json" | jq -r '.queue_bytes // 0')
    qbytes_cap=$(echo "$json" | jq -r '.queue_bytes_capacity // 0')
    conn=$(echo "$json" | jq -r '.connections // 0')
    state=$(echo "$json" | jq -r '.state // "?"')

    now=$(date +%s)
    dt=$((now - prev_ts))
    if (( dt <= 0 )); then dt=1; fi

    recv_rate=$(awk "BEGIN {printf \"%.0f\", ($recv - $prev_recv) / $dt}")
    ins_rate=$(awk "BEGIN {printf \"%.0f\", ($ins - $prev_ins) / $dt}")
    drop_rate=$(awk "BEGIN {printf \"%.0f\", ($drop - $prev_drop) / $dt}")

    qinfo="q=${depth}/${cap}"
    if [[ "$qbytes_cap" != "0" && "$qbytes_cap" != "null" ]]; then
        qinfo="${qinfo} b=${qbytes}/${qbytes_cap}"
    fi

    printf "%-19s %10s %10s %10s %10s %10s %8s %-10s %s\n" \
        "$(date '+%H:%M:%S')" "$recv_rate" "$ins_rate" "$drop_rate" "$drop" "$buf" "$conn" "$state" "$qinfo"

    prev_recv=$recv
    prev_ins=$ins
    prev_drop=$drop
    prev_ts=$now
    sleep "$INTERVAL"
done
