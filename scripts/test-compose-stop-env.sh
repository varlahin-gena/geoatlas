#!/usr/bin/env bash
# Заглушки для docker compose down при пустых ${VAR:?} — без Docker.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "test-compose-stop-env FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

# shellcheck source=deploy/common/compose.sh
source deploy/common/compose.sh

_ga_compose_cmd_allows_placeholder_env down --remove-orphans || fail "down должен допускать заглушки"
_ga_compose_cmd_allows_placeholder_env stop || fail "stop должен допускать заглушки"
if _ga_compose_cmd_allows_placeholder_env up -d --build; then
    fail "up не должен подставлять заглушки"
fi
ok "placeholder commands"

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/ga-compose-stop.XXXXXX")"
cleanup() { rm -rf "$tmpdir"; }
trap cleanup EXIT

unset CLICKHOUSE_PASSWORD INGEST_SHARED_SECRET API_AUTH_TOKEN SESSION_SECRET AUTH_ADMIN_PASSWORD

echo 'CLICKHOUSE_PASSWORD=' >"${tmpdir}/.env"
_ga_compose_fill_stop_placeholders "$tmpdir"
[[ "${CLICKHOUSE_PASSWORD}" == "ga-compose-stop" ]] || fail "пустой .env — нужна заглушка"
[[ "${API_AUTH_TOKEN}" == "ga-compose-stop" ]] || fail "нет ключа в .env — нужна заглушка"
ok "empty .env fills placeholders"

unset CLICKHOUSE_PASSWORD
echo 'CLICKHOUSE_PASSWORD=""' >"${tmpdir}/.env"
_ga_compose_fill_stop_placeholders "$tmpdir"
[[ "${CLICKHOUSE_PASSWORD}" == "ga-compose-stop" ]] || fail "CLICKHOUSE_PASSWORD=\"\" — нужна заглушка"
ok "quoted empty fills"

unset CLICKHOUSE_PASSWORD
echo 'CLICKHOUSE_PASSWORD=keep-from-file' >"${tmpdir}/.env"
_ga_compose_fill_stop_placeholders "$tmpdir"
[[ -z "${CLICKHOUSE_PASSWORD:-}" ]] || fail "нельзя экспортировать заглушку, если в .env есть значение"
ok "non-empty .env not overridden"

export CLICKHOUSE_PASSWORD=from-shell
echo 'CLICKHOUSE_PASSWORD=' >"${tmpdir}/.env"
_ga_compose_fill_stop_placeholders "$tmpdir"
[[ "${CLICKHOUSE_PASSWORD}" == "from-shell" ]] || fail "shell env не должен затираться"
ok "shell env preserved"

unset COMPOSE_PROJECT_NAME
echo 'COMPOSE_PROJECT_NAME=geoatlas' >"${tmpdir}/.env"
_ga_compose_adopt_existing_project "$tmpdir"
[[ "${COMPOSE_PROJECT_NAME}" == "geoatlas" ]] || fail "должен взять COMPOSE_PROJECT_NAME из .env"
cnt="$(grep -cE '^[[:space:]]*COMPOSE_PROJECT_NAME=' "${tmpdir}/.env" || true)"
[[ "$cnt" == "1" ]] || fail ".env не должен дублировать COMPOSE_PROJECT_NAME"
ok "adopt project from .env"

unset COMPOSE_PROJECT_NAME
echo '' >"${tmpdir}/.env"
_ga_compose_adopt_existing_project "$tmpdir"
[[ -z "${COMPOSE_PROJECT_NAME:-}" ]] || fail "без контейнеров и .env не должен выставлять project"
ok "adopt no-op without docker containers"

echo "test-compose-stop-env: all checks passed"
