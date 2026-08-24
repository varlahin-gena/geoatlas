#!/usr/bin/env bash
# A1: один Go-модуль (geoatlas), один cmd (geoatlas), один HTTP adapter (httpapi).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "module-identity FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

[[ -f backend/go.mod ]] || fail "missing backend/go.mod"
grep -q '^module geoatlas$' backend/go.mod || fail "backend/go.mod module must be geoatlas"
ok "go.mod module geoatlas"

[[ -d backend/cmd/geoatlas ]] || fail "missing backend/cmd/geoatlas"
[[ ! -d backend/cmd/network-monitor ]] || fail "remove stale backend/cmd/network-monitor"
ok "single cmd/geoatlas"

grep -q './cmd/geoatlas' backend/Dockerfile || fail "backend/Dockerfile must build ./cmd/geoatlas"
ok "Dockerfile builds cmd/geoatlas"

if grep -R --include='*.go' -n 'network_monitor' backend stats-collector pkg 2>/dev/null; then
  fail "network_monitor import path in Go sources (use geoatlas/...)"
fi
ok "no network_monitor import paths"

if grep -R --include='*.go' -n 'cmd/network-monitor' backend 2>/dev/null; then
  fail "reference to cmd/network-monitor in Go sources"
fi
ok "no cmd/network-monitor references"

if ! grep -q 'enum: \[administrator, operator, dashboard\]' openapi.yaml; then
  fail "openapi.yaml AuthUserPublic.role must include dashboard"
fi
ok "openapi role enums include dashboard"

echo "module-identity: all checks passed"
