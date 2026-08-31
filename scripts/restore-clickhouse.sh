#!/usr/bin/env bash
# RESTORE ClickHouse из Disk('backups', <name>) + опционально auth tarball.
#
# Использование:
#   ./scripts/restore-clickhouse.sh ga-20260411T023000Z
#   RESTORE_ALLOW_NONEMPTY=1 ./scripts/restore-clickhouse.sh ga-...
#   RESTORE_AUTH=0 ./scripts/restore-clickhouse.sh ga-...
#
# Перед restore лучше остановить ingest (по умолчанию скрипт стопает backend/syslog-ng).
# После restore без edges: POST /api/system/maintenance/backfill
#
# Env:
#   RESTORE_ALLOW_NONEMPTY — 1 = SETTINGS allow_non_empty_tables=true (дефолт 0)
#   RESTORE_AUTH           — 1 = распаковать <name>.auth.tgz в backend:/app/data
#                            (дефолт: 1 если файл есть, иначе 0)
#   RESTORE_STOP_INGEST    — 1 = stop backend + syslog-ng перед restore (дефолт 1)
#   COMPOSE / CLICKHOUSE_SERVICE / BACKEND_SERVICE — как в backup-clickhouse.sh

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

NAME="${1:-}"
if [[ -z "$NAME" ]]; then
  echo "usage: $0 <backup-name>" >&2
  echo "list:  docker compose exec clickhouse ls -1 /var/lib/clickhouse-backups" >&2
  exit 2
fi

RESTORE_ALLOW_NONEMPTY="${RESTORE_ALLOW_NONEMPTY:-0}"
RESTORE_STOP_INGEST="${RESTORE_STOP_INGEST:-1}"
COMPOSE="${COMPOSE:-docker compose}"
CLICKHOUSE_SERVICE="${CLICKHOUSE_SERVICE:-clickhouse}"
BACKEND_SERVICE="${BACKEND_SERVICE:-backend}"
BACKUP_ROOT="/var/lib/clickhouse-backups"

ch_client() {
  $COMPOSE exec -T "$CLICKHOUSE_SERVICE" \
    sh -c 'exec clickhouse-client --password "$CLICKHOUSE_PASSWORD" "$@"' sh "$@"
}

if ! $COMPOSE ps --status running --services 2>/dev/null | grep -qx "$CLICKHOUSE_SERVICE"; then
  echo "restore: service '$CLICKHOUSE_SERVICE' is not running" >&2
  exit 1
fi

exists="$($COMPOSE exec -T "$CLICKHOUSE_SERVICE" sh -c \
  "test -d '${BACKUP_ROOT}/${NAME}' && echo 1 || echo 0")"
exists="${exists//$'\r'/}"
if [[ "$exists" != "1" ]]; then
  echo "restore: backup not found: ${BACKUP_ROOT}/${NAME}" >&2
  $COMPOSE exec -T "$CLICKHOUSE_SERVICE" ls -1 "$BACKUP_ROOT" >&2 || true
  exit 1
fi

STOPPED=()
if [[ "$RESTORE_STOP_INGEST" == "1" || "$RESTORE_STOP_INGEST" == "true" ]]; then
  for svc in backend syslog-ng; do
    if $COMPOSE ps --status running --services 2>/dev/null | grep -qx "$svc"; then
      echo "restore: stopping $svc"
      $COMPOSE stop "$svc"
      STOPPED+=("$svc")
    fi
  done
fi

CANDIDATES=(
  traffic_logs
  geo_ranges
  reputation_ranges
  parse_errors
  system_metrics
  traffic_edges_daily
  traffic_edges_city_daily
  traffic_edges_country_daily
  traffic_edges_continent_daily
)

SETTINGS=""
if [[ "$RESTORE_ALLOW_NONEMPTY" == "1" || "$RESTORE_ALLOW_NONEMPTY" == "true" ]]; then
  SETTINGS=" SETTINGS allow_non_empty_tables = true"
fi

echo "restore: from Disk('backups', '$NAME')"
RESTORED=0
for t in "${CANDIDATES[@]}"; do
  SQL="RESTORE TABLE ${t} FROM Disk('backups', '${NAME}')${SETTINGS}"
  set +e
  err="$(ch_client \
    --receive_timeout 3600 --send_timeout 3600 -q "$SQL" 2>&1)"
  rc=$?
  set -e
  if [[ "$rc" -eq 0 ]]; then
    echo "restore: ok $t"
    RESTORED=$((RESTORED + 1))
    continue
  fi
  if echo "$err" | grep -qiE 'not found|doesn.t exist|UNKNOWN_TABLE|NO_SUCH|cannot find|Backup doesn'; then
    echo "restore: skip $t"
    continue
  fi
  echo "restore: failed $t: $err" >&2
  exit 1
done

if [[ "$RESTORED" -eq 0 ]]; then
  echo "restore: nothing restored" >&2
  exit 1
fi

has_auth="$($COMPOSE exec -T "$CLICKHOUSE_SERVICE" sh -c \
  "test -f '${BACKUP_ROOT}/${NAME}.auth.tgz' && echo 1 || echo 0")"
has_auth="${has_auth//$'\r'/}"
AUTH_DEFAULT=0
[[ "$has_auth" == "1" ]] && AUTH_DEFAULT=1
RESTORE_AUTH="${RESTORE_AUTH:-$AUTH_DEFAULT}"

if [[ "$RESTORE_AUTH" == "1" || "$RESTORE_AUTH" == "true" ]]; then
  if [[ "$has_auth" != "1" ]]; then
    echo "restore: auth tarball missing, skip" >&2
  else
    if ! $COMPOSE ps --status running --services 2>/dev/null | grep -qx "$BACKEND_SERVICE"; then
      echo "restore: starting $BACKEND_SERVICE briefly for auth untar"
      $COMPOSE start "$BACKEND_SERVICE"
      sleep 3
    fi
    $COMPOSE exec -T "$CLICKHOUSE_SERVICE" cat "${BACKUP_ROOT}/${NAME}.auth.tgz" \
      | $COMPOSE exec -T "$BACKEND_SERVICE" tar xzf - -C /app/data
    echo "restore: auth ok → ${BACKEND_SERVICE}:/app/data"
  fi
fi

if [[ ${#STOPPED[@]} -gt 0 ]]; then
  echo "restore: restarting ${STOPPED[*]}"
  $COMPOSE start "${STOPPED[@]}"
fi

echo "restore: done name=$NAME tables=$RESTORED"
echo "restore: if edges were omitted from backup, run POST /api/system/maintenance/backfill"
