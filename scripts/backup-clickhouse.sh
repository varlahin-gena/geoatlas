#!/usr/bin/env bash
# Native ClickHouse BACKUP на disk `backups` (+ снимок /app/data с backend).
#
# Использование (на хосте appliance, из корня репо / /opt/network-monitor):
#   ./scripts/backup-clickhouse.sh
#   BACKUP_KEEP=3 BACKUP_INCLUDE_EDGES=0 ./scripts/backup-clickhouse.sh
#
# Env:
#   BACKUP_ENABLED     — если 0, выход 0 без работы (для cron-обёрток). Дефолт: 1
#   BACKUP_KEEP        — сколько полных бэкапов оставить (дефолт 7)
#   BACKUP_INCLUDE_EDGES — 1 = включить traffic_edges_* (дефолт 1); 0 = только факты + ranges
#   BACKUP_INCLUDE_AUTH  — 1 = tar /app/data → *.auth.tgz рядом (дефолт 1)
#   COMPOSE            — команда compose (дефолт: docker compose)
#   CLICKHOUSE_SERVICE — имя сервиса (дефолт: clickhouse)
#   BACKEND_SERVICE    — имя сервиса backend (дефолт: backend)
#
# Cron (пример, ежедневно в 02:30):
#   30 2 * * * cd /opt/network-monitor && ./scripts/backup-clickhouse.sh >>/var/log/nm-backup.log 2>&1

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

BACKUP_ENABLED="${BACKUP_ENABLED:-1}"
BACKUP_KEEP="${BACKUP_KEEP:-7}"
BACKUP_INCLUDE_EDGES="${BACKUP_INCLUDE_EDGES:-1}"
BACKUP_INCLUDE_AUTH="${BACKUP_INCLUDE_AUTH:-1}"
COMPOSE="${COMPOSE:-docker compose}"
CLICKHOUSE_SERVICE="${CLICKHOUSE_SERVICE:-clickhouse}"
BACKEND_SERVICE="${BACKEND_SERVICE:-backend}"
BACKUP_ROOT="/var/lib/clickhouse-backups"

if [[ "$BACKUP_ENABLED" == "0" || "$BACKUP_ENABLED" == "false" ]]; then
  echo "backup: disabled (BACKUP_ENABLED=$BACKUP_ENABLED)"
  exit 0
fi

if ! $COMPOSE ps --status running --services 2>/dev/null | grep -qx "$CLICKHOUSE_SERVICE"; then
  echo "backup: service '$CLICKHOUSE_SERVICE' is not running" >&2
  exit 1
fi

NAME="nm-$(date -u +%Y%m%dT%H%M%SZ)"
echo "backup: starting name=$NAME keep=$BACKUP_KEEP edges=$BACKUP_INCLUDE_EDGES auth=$BACKUP_INCLUDE_AUTH"

TABLES=(
  traffic_logs
  geo_ranges
  reputation_ranges
  parse_errors
  system_metrics
)
if [[ "$BACKUP_INCLUDE_EDGES" == "1" || "$BACKUP_INCLUDE_EDGES" == "true" ]]; then
  TABLES+=(
    traffic_edges_daily
    traffic_edges_city_daily
    traffic_edges_country_daily
  )
fi

# Только существующие таблицы (reputation может отсутствовать при выключенном модуле).
EXISTING=()
for t in "${TABLES[@]}"; do
  n="$($COMPOSE exec -T "$CLICKHOUSE_SERVICE" clickhouse-client -q \
    "SELECT count() FROM system.tables WHERE database = currentDatabase() AND name = '$t'")"
  if [[ "${n//$'\r'/}" == "1" ]]; then
    EXISTING+=("$t")
  else
    echo "backup: skip missing table $t"
  fi
done

if [[ ${#EXISTING[@]} -eq 0 ]]; then
  echo "backup: no tables to back up" >&2
  exit 1
fi

LIST="$(IFS=,; echo "${EXISTING[*]}")"
# CH 25: BACKUP TABLE a, TABLE b — keyword TABLE перед каждым объектом.
TABLE_LIST=""
for t in "${EXISTING[@]}"; do
  if [[ -n "$TABLE_LIST" ]]; then
    TABLE_LIST+=", "
  fi
  TABLE_LIST+="TABLE ${t}"
done
SQL="BACKUP ${TABLE_LIST} TO Disk('backups', '${NAME}')"
echo "backup: $SQL"
$COMPOSE exec -T "$CLICKHOUSE_SERVICE" clickhouse-client --receive_timeout 3600 --send_timeout 3600 -q "$SQL"
echo "backup: clickhouse ok → Disk('backups', '$NAME')"

if [[ "$BACKUP_INCLUDE_AUTH" == "1" || "$BACKUP_INCLUDE_AUTH" == "true" ]]; then
  if $COMPOSE ps --status running --services 2>/dev/null | grep -qx "$BACKEND_SERVICE"; then
    $COMPOSE exec -T "$BACKEND_SERVICE" tar czf - -C /app/data . \
      | $COMPOSE exec -T "$CLICKHOUSE_SERVICE" sh -c "cat > '${BACKUP_ROOT}/${NAME}.auth.tgz'"
    echo "backup: auth ok → ${NAME}.auth.tgz"
  else
    echo "backup: skip auth (backend not running)" >&2
  fi
fi

# Prune: каталоги nm-* (полные бэкапы) и парные *.auth.tgz
mapfile -t ALL < <($COMPOSE exec -T "$CLICKHOUSE_SERVICE" sh -c \
  "ls -1 '${BACKUP_ROOT}' 2>/dev/null | grep -E '^nm-[0-9]{8}T[0-9]{6}Z\$' | sort -r" || true)

KEEP=$((BACKUP_KEEP))
if [[ "$KEEP" -lt 1 ]]; then
  KEEP=1
fi

i=0
for dir in "${ALL[@]:-}"; do
  dir="${dir//$'\r'/}"
  [[ -z "$dir" ]] && continue
  i=$((i + 1))
  if [[ "$i" -le "$KEEP" ]]; then
    continue
  fi
  echo "backup: prune $dir"
  $COMPOSE exec -T "$CLICKHOUSE_SERVICE" sh -c \
    "rm -rf '${BACKUP_ROOT}/${dir}' '${BACKUP_ROOT}/${dir}.auth.tgz'"
done

echo "backup: done name=$NAME kept=$KEEP"
