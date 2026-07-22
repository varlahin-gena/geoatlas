#!/bin/bash
# Soak / smoke: провоцирует drop на полной ingest-очереди и проверяет dropped_total + alerts.
#
# Требования: поднятый стек (./start.sh), curl, jq.
#
# Пример (малая очередь → быстрые drops):
#   INGEST_QUEUE_SIZE=500 docker compose up -d backend
#   ./scripts/soak-queue-drops.sh
#
# Или без перезапуска (надежда на реальную перегрузку worker/CH):
#   SOAK_LINES=500000 SOAK_CONCURRENCY=8 ./scripts/soak-queue-drops.sh

set -euo pipefail

BASE="${SOAK_BASE_URL:-http://127.0.0.1}"
AUTH_HEADER=()
if [[ -n "${API_AUTH_TOKEN:-}" ]]; then
  AUTH_HEADER=(-H "Authorization: Bearer ${API_AUTH_TOKEN}")
fi

LINES="${SOAK_LINES:-50000}"
CONCURRENCY="${SOAK_CONCURRENCY:-4}"
SAMPLE_LINE='CEF:0|UserGate|UG NGFW|7|firewall|Traffic allow|1|src=10.0.0.1 dst=8.8.8.8 spt=12345 dpt=443 proto=TCP act=allow'

echo "== soak-queue-drops: lines=$LINES concurrency=$CONCURRENCY base=$BASE =="

before=$(curl -sf "${AUTH_HEADER[@]}" "$BASE/api/ingest/stats" || true)
if [[ -z "$before" || "$before" == "{}" ]]; then
  echo "ERROR: cannot read $BASE/api/ingest/stats (auth? stack up?)" >&2
  exit 1
fi
dropped_before=$(echo "$before" | jq -r '.dropped_total // 0')
echo "dropped_total before: $dropped_before"

# Flood POST /api/ingest (ops) — попадает в ту же очередь, что syslog TCP.
# При полной очереди API отвечает 503 + stats.dropped (не 200) — это ожидаемо.
# Для именно TCP queue drops нужен syslog→:1514; unit: go test ./internal/ingest/ -run EnqueueFlood.
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
body="$tmpdir/payload.txt"
: >"$body"
for ((i = 0; i < LINES / CONCURRENCY; i++)); do
  printf '%s\n' "$SAMPLE_LINE" >>"$body"
done

pids=()
for ((c = 0; c < CONCURRENCY; c++)); do
  # без -f: 503 при drops — штатный delivery contract
  curl -sS -o /dev/null -w '' "${AUTH_HEADER[@]}" -X POST \
    -H 'Content-Type: text/plain' \
    --data-binary @"$body" \
    "$BASE/api/ingest" >/dev/null &
  pids+=($!)
done
for pid in "${pids[@]}"; do
  wait "$pid" || true
done
echo "POST /api/ingest flood done (HTTP 503 expected when queue saturated)"

sleep 2
after=$(curl -sf "${AUTH_HEADER[@]}" "$BASE/api/ingest/stats")
dropped_after=$(echo "$after" | jq -r '.dropped_total // 0')
echo "dropped_total after:  $dropped_after"
echo "$after" | jq '{state, received_total, inserted_total, dropped_total, queue_depth, queue_capacity}'

sys=$(curl -sf "${AUTH_HEADER[@]}" "$BASE/api/system/stats" || echo '{}')
echo "$sys" | jq '.alerts // [] | map(select(.code|test("ingest_drop|ingest_queue")))'

echo "OK: soak finished. For forced TCP queue drops set INGEST_QUEUE_SIZE low and flood :514."
echo "    Unit coverage: go test ./internal/ingest/ -run EnqueueFlood"
