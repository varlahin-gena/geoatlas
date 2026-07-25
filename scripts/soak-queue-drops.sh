#!/bin/bash
# Soak / smoke: провоцирует drop на полной ingest-очереди и проверяет dropped_total + alerts.
#
# Требования: поднятый стек (./start.sh), curl, jq.
#
# Пример (малая очередь → быстрые drops по depth):
#   INGEST_QUEUE_SIZE=500 docker compose up -d --force-recreate backend
#   API_AUTH_TOKEN="$TOKEN" ./scripts/soak-queue-drops.sh
#
# Byte-cap (P1):
#   INGEST_QUEUE_MAX_BYTES=1048576 docker compose up -d --force-recreate backend
#   API_AUTH_TOKEN="$TOKEN" SOAK_LINES=200000 SOAK_CONCURRENCY=8 ./scripts/soak-queue-drops.sh
#
# Без перезапуска (нужен реальный backpressure workers/CH):
#   API_AUTH_TOKEN="$TOKEN" SOAK_LINES=500000 SOAK_CONCURRENCY=8 ./scripts/soak-queue-drops.sh
#
# Напрямую в backend (минуя nginx), если через :80 мало received:
#   SOAK_BASE_URL=http://127.0.0.1:8080 API_AUTH_TOKEN="$TOKEN" ./scripts/soak-queue-drops.sh

set -euo pipefail

BASE="${SOAK_BASE_URL:-http://127.0.0.1}"
AUTH_HEADER=()
if [[ -n "${API_AUTH_TOKEN:-}" ]]; then
  AUTH_HEADER=(-H "Authorization: Bearer ${API_AUTH_TOKEN}")
fi

LINES="${SOAK_LINES:-50000}"
CONCURRENCY="${SOAK_CONCURRENCY:-4}"
if (( CONCURRENCY < 1 )); then
  echo "ERROR: SOAK_CONCURRENCY must be >= 1" >&2
  exit 1
fi
SAMPLE_LINE='CEF:0|UserGate|UG NGFW|7|firewall|Traffic allow|1|src=10.0.0.1 dst=8.8.8.8 spt=12345 dpt=443 proto=TCP act=allow'
PER_REQ=$((LINES / CONCURRENCY))
if (( PER_REQ < 1 )); then
  echo "ERROR: SOAK_LINES/SOAK_CONCURRENCY < 1" >&2
  exit 1
fi
EXPECTED=$((PER_REQ * CONCURRENCY))

echo "== soak-queue-drops: lines=$LINES concurrency=$CONCURRENCY per_req=$PER_REQ expected≈$EXPECTED base=$BASE =="

before=$(curl -sf "${AUTH_HEADER[@]}" "$BASE/api/ingest/stats" || true)
if [[ -z "$before" || "$before" == "{}" ]]; then
  echo "ERROR: cannot read $BASE/api/ingest/stats (auth? stack up?)" >&2
  exit 1
fi
dropped_before=$(echo "$before" | jq -r '.dropped_total // 0')
recv_before=$(echo "$before" | jq -r '.received_total // 0')
echo "dropped_total before: $dropped_before  received_total before: $recv_before"
echo "$before" | jq '{state, queue_depth, queue_capacity, queue_bytes, queue_bytes_capacity, dropped_total}'

# Smoke POST: один маленький запрос — сразу видно 401/403/502.
smoke=$(curl -sS -o /tmp/soak-smoke-body.$$ -w '%{http_code}' "${AUTH_HEADER[@]}" \
  -X POST -H 'Content-Type: text/plain' -H 'Expect:' \
  --data-binary "$SAMPLE_LINE"$'\n' \
  "$BASE/api/ingest" || true)
echo "smoke POST /api/ingest → HTTP $smoke"
if [[ "$smoke" != "200" && "$smoke" != "503" ]]; then
  echo "ERROR: smoke ingest failed (want 200/503). body:" >&2
  head -c 500 /tmp/soak-smoke-body.$$ >&2 || true
  echo >&2
  rm -f /tmp/soak-smoke-body.$$
  exit 1
fi
rm -f /tmp/soak-smoke-body.$$

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
body="$tmpdir/payload.txt"
# Быстрее bash-цикла: yes обрезает по числу строк.
yes "$SAMPLE_LINE" | head -n "$PER_REQ" >"$body"
bytes=$(wc -c <"$body" | tr -d ' ')
lines_in_file=$(wc -l <"$body" | tr -d ' ')
echo "payload: $lines_in_file lines, $bytes bytes (~$(awk "BEGIN{printf \"%.1f\", $bytes/1048576}") MiB) × $CONCURRENCY POSTs"

# Flood POST /api/ingest (ops). Expect: отключён — иначе nginx auth_request + 100-continue
# часто рвёт крупные тела (симптом: received<<expected).
status_dir="$tmpdir/status"
mkdir -p "$status_dir"
pids=()
for ((c = 0; c < CONCURRENCY; c++)); do
  (
    code=$(curl -sS -o "$status_dir/body.$c" -w '%{http_code}' "${AUTH_HEADER[@]}" \
      -X POST \
      -H 'Content-Type: text/plain' \
      -H 'Expect:' \
      --data-binary @"$body" \
      "$BASE/api/ingest" || echo "000")
    echo "$code" >"$status_dir/code.$c"
  ) &
  pids+=($!)
done
for pid in "${pids[@]}"; do
  wait "$pid" || true
done

echo "HTTP status histogram:"
cat "$status_dir"/code.* | sort | uniq -c
okish=0
for f in "$status_dir"/code.*; do
  code=$(cat "$f")
  if [[ "$code" == "200" || "$code" == "503" ]]; then
    okish=$((okish + 1))
  else
    c=${f##*.}
    echo "  worker $c → HTTP $code body: $(head -c 200 "$status_dir/body.$c" | tr '\n' ' ')"
  fi
done
echo "POST /api/ingest flood done ($okish/$CONCURRENCY returned 200/503)"

sleep 2
after=$(curl -sf "${AUTH_HEADER[@]}" "$BASE/api/ingest/stats")
dropped_after=$(echo "$after" | jq -r '.dropped_total // 0')
recv_after=$(echo "$after" | jq -r '.received_total // 0')
recv_delta=$((recv_after - recv_before))
echo "dropped_total after:  $dropped_after"
echo "received_total delta: $recv_delta (expected ≈ $EXPECTED + smoke)"
echo "$after" | jq '{
  state, received_total, inserted_total, dropped_total,
  queue_depth, queue_capacity, queue_bytes, queue_bytes_capacity
}'

delta=$((dropped_after - dropped_before))
echo "dropped delta: $delta"

sys=$(curl -sf "${AUTH_HEADER[@]}" "$BASE/api/system/stats" || echo '{}')
echo "$sys" | jq '.alerts // [] | map(select(.code|test("ingest_drop|ingest_queue")))'

# Flood считается доставленным, если приняли хотя бы половину ожидаемого.
min_recv=$((EXPECTED / 2))
if (( recv_delta < min_recv )); then
  echo "ERROR: flood barely reached ingest (received_delta=$recv_delta, want ≥ $min_recv)." >&2
  echo "    Try: SOAK_BASE_URL=http://127.0.0.1:8080 (bypass nginx)" >&2
  echo "    Or check: curl -sS -D- -H \"Authorization: Bearer \$TOKEN\" -H 'Expect:' -X POST --data-binary @/etc/hosts $BASE/api/ingest | head" >&2
  exit 1
fi

if (( delta > 0 )); then
  echo "OK: soak saw drops (queue pressure confirmed)."
else
  echo "WARN: flood delivered but no drops — queue absorbed it."
  echo "    Force drops: INGEST_QUEUE_SIZE=500 docker compose up -d --force-recreate backend"
  echo "    Then re-run this script with SOAK_LINES=20000 SOAK_CONCURRENCY=4"
fi
