#!/usr/bin/env bash
# Контрактные проверки frontend SPA (без полного docker-стека).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "frontend-smoke FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

[[ -f frontend/package.json ]] || fail "missing frontend/package.json"
[[ -f frontend/package-lock.json ]] || fail "missing frontend/package-lock.json"
[[ -f frontend/vite.config.ts ]] || fail "missing frontend/vite.config.ts"
[[ -f frontend/index.html ]] || fail "missing frontend/index.html"
[[ -f frontend/Dockerfile ]] || fail "missing frontend/Dockerfile"
[[ -f frontend/nginx.conf ]] || fail "missing frontend/nginx.conf"
[[ -f frontend/nginx-app.inc ]] || fail "missing frontend/nginx-app.inc"
[[ -f frontend/docker-entrypoint.sh ]] || fail "missing frontend/docker-entrypoint.sh"
[[ -f frontend/src/main.tsx ]] || fail "missing frontend/src/main.tsx"
[[ -f frontend/src/App.tsx ]] || fail "missing frontend/src/App.tsx"
[[ -f frontend/src/api/client.ts ]] || fail "missing frontend/src/api/client.ts"
[[ -f frontend/src/auth/AuthContext.tsx ]] || fail "missing frontend/src/auth/AuthContext.tsx"
[[ -f frontend/src/auth/RequireAuth.tsx ]] || fail "missing frontend/src/auth/RequireAuth.tsx"
[[ -f frontend/src/lib/search.ts ]] || fail "missing frontend/src/lib/search.ts"
[[ -f frontend/src/lib/search.test.ts ]] || fail "missing frontend/src/lib/search.test.ts"
[[ -f frontend/src/styles/theme.css ]] || fail "missing frontend/src/styles/theme.css"
[[ -f frontend/src/styles/common.css ]] || fail "missing frontend/src/styles/common.css"
[[ -f frontend/src/styles/auth-form.css ]] || fail "missing frontend/src/styles/auth-form.css"
[[ -f frontend/public/favicon.svg ]] || fail "missing frontend/public/favicon.svg"
[[ -f frontend/public/data/countries.geojson ]] || fail "missing frontend/public/data/countries.geojson"
[[ -f frontend/src/pages/Map/MapPage.tsx ]] || fail "missing MapPage"
[[ -f frontend/src/pages/Map/mapHeatmap.ts ]] || fail "missing mapHeatmap"
[[ -f frontend/src/pages/Map/mapDetail.tsx ]] || fail "missing mapDetail"
[[ -f frontend/src/pages/Map/mapReputation.ts ]] || fail "missing mapReputation"
ok "spa scaffold present"

grep -q 'react-router-dom' frontend/package.json || fail "package.json: react-router-dom"
grep -q 'maplibre-gl' frontend/package.json || fail "package.json: maplibre-gl"
grep -q 'uplot' frontend/package.json || fail "package.json: uplot"
grep -q '"vitest"' frontend/package.json || fail "package.json: vitest"
grep -q 'npm ci' frontend/Dockerfile || fail "Dockerfile: npm ci"
ok "dependencies"

grep -q 'X-CSRF-Token' frontend/src/api/client.ts || fail "client.ts: CSRF header"
grep -q 'nm_csrf' frontend/src/api/client.ts || fail "client.ts: CSRF cookie"
grep -q 'RequireAuth\|requireLogin' frontend/src/auth/RequireAuth.tsx || fail "RequireAuth"
grep -q '/api/auth/login' frontend/src/api/auth.ts || fail "login endpoint"
grep -q 'id="username"' frontend/src/pages/Login/LoginPage.tsx || fail "login username"
grep -q 'id="password"' frontend/src/pages/Login/LoginPage.tsx || fail "login password"
grep -q 'nm-meta-version\|/api/system/version' frontend/src/components/Shell.tsx || fail "version in user menu"
grep -q 'escapeHTML' frontend/src/lib/format.ts || fail "escapeHTML"
grep -q "kind === 'error' ? 'alert'" frontend/src/components/Toast.tsx || fail "toast alert role"
grep -q 'toast-host' frontend/src/styles/common.css || fail "common.css: toast"
grep -q -- '--bg:' frontend/src/styles/theme.css || fail "theme.css: dark tokens"
grep -q 'data-theme="light"' frontend/src/styles/theme.css || fail "theme.css: light theme"
grep -q 'country:Россия' frontend/src/lib/search.test.ts || fail "search test: Россия"
grep -q 'chartAxisStroke' frontend/src/pages/System/systemCharts.ts || fail "system: chartAxisStroke"
grep -q 'nm-theme-change' frontend/src/pages/System/SystemPage.tsx || fail "system: theme listener"
grep -q 'id="map-main"' frontend/src/pages/Map/MapPage.tsx || fail "map main landmark"
grep -q 'countries.geojson\|loadCountriesGeoJSON\|mapHeatmap' frontend/src/pages/Map/mapHeatmap.ts || fail "heatmap geojson"
grep -q 'autoRotate\|startGlobeAutoRotate\|auto-rotate' \
  frontend/src/pages/Map/MapPage.tsx \
  frontend/src/pages/Map/useGlobeAutoRotate.ts \
  frontend/src/pages/Map/mapViewport.ts || fail "auto-rotate"
grep -q 'reputation' frontend/src/pages/Map/mapReputation.ts || fail "map reputation"
grep -q 'events/series\|sparkline' frontend/src/pages/Map/mapDetail.tsx || fail "detail sparkline"
grep -q 'upload-geo' \
  frontend/src/pages/Map/MapPage.tsx \
  frontend/src/pages/Map/useMapUploads.ts \
  frontend/src/pages/Map/MapSidebar.tsx || fail "map upload-geo"
[[ -f frontend/src/pages/Map/useMapEvents.ts ]] || fail "missing useMapEvents"
[[ -f frontend/src/pages/Map/MapSidebar.tsx ]] || fail "missing MapSidebar"
[[ -f frontend/src/pages/Map/MapTopbar.tsx ]] || fail "missing MapTopbar"
[[ -f frontend/eslint.config.js ]] || fail "missing eslint.config.js"
[[ -f frontend/src/pages/Map/mapReputation.test.ts ]] || fail "missing mapReputation.test.ts"
[[ -f frontend/src/pages/Map/mapHeatmap.test.ts ]] || fail "missing mapHeatmap.test.ts"
[[ -f frontend/src/pages/Map/mapLayers.test.ts ]] || fail "missing mapLayers.test.ts"
ok "auth/ui/search/map contracts"

grep -q 'statusDrops\|id="statusDrops"' frontend/src/pages/System/SystemPage.tsx || fail "system: statusDrops tile"
grep -q 'Buffer drops' frontend/src/pages/System/SystemPipelineTab.tsx || fail "system: Buffer drops kv"
grep -q 'Queue bytes\|queue_bytes' frontend/src/pages/System/SystemPipelineTab.tsx || fail "system: queue_bytes"
ok "system ingest drop visibility"

grep -q 'geo-ranges/clear' frontend/src/pages/GeoRanges/GeoRangesPage.tsx || fail "geo-ranges clear API"
grep -q 'Очистить базу' frontend/src/pages/GeoRanges/GeoRangesPage.tsx || fail "geo-ranges clear button"
grep -q 'Загрузить CSV' frontend/src/pages/GeoRanges/GeoRangesPage.tsx || fail "geo-ranges upload button"
ok "geo-ranges clear/upload"

if grep -qE 'unpkg\.com|jsdelivr\.net' frontend/index.html frontend/src/pages/Map/MapPage.tsx frontend/src/pages/System/SystemPage.tsx; then
  fail "CDN script references still present"
fi
ok "no CDN script tags"

# Legacy vanilla must be gone after cutover
[[ ! -f frontend/js/map-app.js ]] || fail "vanilla map-app.js still present"
[[ ! -f frontend/auth.js ]] || fail "vanilla auth.js still present"
[[ ! -f frontend/login.html ]] || fail "vanilla login.html still present"
ok "vanilla removed"

PYTHON_BIN=""
if command -v python >/dev/null 2>&1; then
  PYTHON_BIN="$(command -v python)"
elif command -v python3 >/dev/null 2>&1; then
  PYTHON_BIN="$(command -v python3)"
fi
# Windows Store python3 stub is not a real interpreter.
case "$PYTHON_BIN" in
  *WindowsApps*) PYTHON_BIN="$(command -v python || true)" ;;
esac
[[ -n "$PYTHON_BIN" ]] || fail "python not found"
"$PYTHON_BIN" - <<'PY' || fail "countries.geojson invalid"
import json
from pathlib import Path
p = Path("frontend/public/data/countries.geojson")
d = json.loads(p.read_text(encoding="utf-8"))
assert d.get("type") == "FeatureCollection", d.get("type")
feats = d.get("features") or []
assert len(feats) > 50, len(feats)
props = feats[0].get("properties") or {}
assert props.get("NAME") or props.get("ADMIN") or props.get("name") or props.get("NAME_EN"), sorted(props)[:12]
print(f"features={len(feats)} size={p.stat().st_size}")
PY
ok "countries.geojson"

grep -q 'auth_request /auth-check' frontend/nginx-app.inc || fail "nginx-app: auth_request"
grep -q '@login_redirect' frontend/nginx-app.inc || fail "nginx-app: login redirect"
grep -q 'try_files \$uri @spa' frontend/nginx-app.inc || fail "nginx-app: SPA try_files @spa"
grep -q 'location @spa' frontend/nginx-app.inc || fail "nginx-app: @spa named location"
grep -q 'return 302 /login' frontend/nginx-app.inc || fail "nginx-app: login redirect path"
grep -q 'location = /system.html' frontend/nginx-app.inc || fail "nginx-app: legacy system.html redirect"
if grep -q 'location = /index.html' frontend/nginx-app.inc; then
  fail "nginx-app: /index.html must not redirect (SPA try_files loop)"
fi
grep -q 'include /etc/nginx/includes/app.inc' frontend/nginx.conf || fail "nginx.conf: app.inc include"
grep -q 'HTTPS_ENABLED' frontend/docker-entrypoint.sh || fail "entrypoint: HTTPS_ENABLED"
grep -q 'listen 443 ssl' frontend/docker-entrypoint.sh || fail "entrypoint: listen 443 ssl"
ok "nginx SPA / HTTPS"

echo "frontend-smoke: all checks passed"
