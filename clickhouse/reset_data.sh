#!/bin/bash
# Очистка данных ClickHouse (схема и MV не трогаются).
# Запуск из корня проекта:
#   bash clickhouse/reset_data.sh

set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
PROJECT_DIR="$(cd -- "$SCRIPT_DIR/.." &>/dev/null && pwd)"
cd "$PROJECT_DIR"

log() { echo "[$(date +'%F %T')] $*"; }

log "Truncating data tables..."
docker compose exec -T clickhouse clickhouse-client --multiquery < "$SCRIPT_DIR/reset_data.sql"

log "Restarting backend (reload geo index + edges agg state)..."
docker compose restart backend

log "Done. Row counts:"
docker compose exec -T clickhouse clickhouse-client --query "
    SELECT 'traffic_logs' AS tbl, count() AS rows FROM traffic_logs
    UNION ALL SELECT 'traffic_edges_daily', count() FROM traffic_edges_daily
    UNION ALL SELECT 'geo_ranges', count() FROM geo_ranges
    UNION ALL SELECT 'parse_errors', count() FROM parse_errors
    UNION ALL SELECT 'system_metrics', count() FROM system_metrics
    FORMAT PrettyCompact
"
