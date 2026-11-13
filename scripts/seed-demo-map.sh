#!/bin/bash
# Clear traffic tables and insert a sparse hub-and-spoke demo for map screenshots.
# Usage (repo root): bash scripts/seed-demo-map.sh

set -Eeuo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." &>/dev/null && pwd)"
cd "$ROOT"

ch() {
  docker compose exec -T clickhouse \
    sh -c 'exec clickhouse-client --password "$CLICKHOUSE_PASSWORD" "$@"' sh "$@"
}

echo "[seed] inserting demo traffic..."
ch --multiquery < "$ROOT/scripts/seed-demo-map.sql"

echo "[seed] row count:"
ch --query "SELECT count() FROM traffic_logs"

echo "[seed] restart backend (edges agg / caches)..."
docker compose restart backend
echo "[seed] done — open map, period 1h, group by city."
