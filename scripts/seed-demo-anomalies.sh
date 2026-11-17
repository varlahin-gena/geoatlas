#!/bin/bash
# Seed presentation demo rows into anomaly_events (fingerprint demo-*).
# Usage (repo root on the stand): bash scripts/seed-demo-anomalies.sh

set -Eeuo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." &>/dev/null && pwd)"
cd "$ROOT"

ch() {
  docker compose exec -T clickhouse \
    sh -c 'exec clickhouse-client --password "$CLICKHOUSE_PASSWORD" "$@"' sh "$@"
}

echo "[seed] removing previous demo-* anomalies..."
ch --query "ALTER TABLE anomaly_events DELETE WHERE startsWith(fingerprint, 'demo-')"

echo "[seed] waiting for delete mutation..."
for _ in $(seq 1 30); do
  pending="$(ch --query "SELECT count() FROM system.mutations WHERE table = 'anomaly_events' AND is_done = 0")"
  if [[ "${pending:-1}" == "0" ]]; then
    break
  fi
  sleep 1
done

echo "[seed] inserting demo anomaly_events..."
ch --multiquery < "$ROOT/scripts/seed-demo-anomalies.sql"

echo "[seed] demo rows:"
ch --query "SELECT code, severity, title FROM anomaly_events WHERE startsWith(fingerprint, 'demo-') ORDER BY detected_at DESC"

echo "[seed] done — open /anomalies (24h, without closed)."
