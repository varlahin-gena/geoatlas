#!/usr/bin/env bash
# Заглушки для docker compose down при пустых ${VAR:?} — без Docker.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "test-compose-stop-env FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

# shellcheck source=deploy/common/compose.sh
source deploy/common/compose.sh

_nm_compose_cmd_allows_placeholder_env down --remove-orphans || fail "down должен допускать заглушки"
_nm_compose_cmd_allows_placeholder_env stop || fail "stop должен допускать заглушки"
if _nm_compose_cmd_allows_placeholder_env up -d --build; then
    fail "up не должен подставлять заглушки"
fi
ok "placeholder commands"

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/nm-compose-stop.XXXXXX")"
cleanup() { rm -rf "$tmpdir"; }
trap cleanup EXIT

unset CLICKHOUSE_PASSWORD INGEST_SHARED_SECRET API_AUTH_TOKEN SESSION_SECRET AUTH_ADMIN_PASSWORD

echo 'CLICKHOUSE_PASSWORD=' >"${tmpdir}/.env"
_nm_compose_fill_stop_placeholders "$tmpdir"
[[ "${CLICKHOUSE_PASSWORD}" == "nm-compose-stop" ]] || fail "пустой .env — нужна заглушка"
[[ "${API_AUTH_TOKEN}" == "nm-compose-stop" ]] || fail "нет ключа в .env — нужна заглушка"
ok "empty .env fills placeholders"

unset CLICKHOUSE_PASSWORD
echo 'CLICKHOUSE_PASSWORD=""' >"${tmpdir}/.env"
_nm_compose_fill_stop_placeholders "$tmpdir"
[[ "${CLICKHOUSE_PASSWORD}" == "nm-compose-stop" ]] || fail "CLICKHOUSE_PASSWORD=\"\" — нужна заглушка"
ok "quoted empty fills"

unset CLICKHOUSE_PASSWORD
echo 'CLICKHOUSE_PASSWORD=keep-from-file' >"${tmpdir}/.env"
_nm_compose_fill_stop_placeholders "$tmpdir"
[[ -z "${CLICKHOUSE_PASSWORD:-}" ]] || fail "нельзя экспортировать заглушку, если в .env есть значение"
ok "non-empty .env not overridden"

export CLICKHOUSE_PASSWORD=from-shell
echo 'CLICKHOUSE_PASSWORD=' >"${tmpdir}/.env"
_nm_compose_fill_stop_placeholders "$tmpdir"
[[ "${CLICKHOUSE_PASSWORD}" == "from-shell" ]] || fail "shell env не должен затираться"
ok "shell env preserved"

echo "test-compose-stop-env: all checks passed"
