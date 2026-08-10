#!/usr/bin/env bash
# Контрактные проверки frontend SPA (без полного docker-стека).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "frontend-smoke FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

[[ -f frontend/package.json ]] || fail "missing frontend/package.json"
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
ok "spa scaffold present"

grep -q 'react-router-dom' frontend/package.json || fail "package.json: react-router-dom"
grep -q 'maplibre-gl' frontend/package.json || fail "package.json: maplibre-gl"
grep -q 'uplot' frontend/package.json || fail "package.json: uplot"
grep -q '"vitest"' frontend/package.json || fail "package.json: vitest"
ok "dependencies"

grep -q 'X-CSRF-Token' frontend/src/api/client.ts || fail "client.ts: CSRF header"
grep -q 'nm_csrf' frontend/src/api/client.ts || fail "client.ts: CSRF cookie"
grep -q 'requireLogin\|RequireAuth' frontend/src/auth/RequireAuth.tsx || fail "RequireAuth"
grep -q '/api/auth/login' frontend/src/api/auth.ts || fail "login endpoint"
grep -q 'htmlFor="username"\|id="username"' frontend/src/pages/Login/LoginPage.tsx || fail "login username"
grep -q 'htmlFor="password"\|id="password"' frontend/src/pages/Login/LoginPage.tsx || fail "login password"
grep -q 'nm-meta-version\|/api/system/version' frontend/src/components/Shell.tsx || fail "version in user menu"
grep -q 'escapeHTML' frontend/src/lib/format.ts || fail "escapeHTML"
grep -q "kind === 'error' ? 'alert'" frontend/src/components/Toast.tsx || fail "toast alert role"
grep -q 'toast-host' frontend/src/styles/common.css || fail "common.css: toast"
grep -q -- '--bg:' frontend/src/styles/theme.css || fail "theme.css: dark tokens"
grep -q 'data-theme="light"' frontend/src/styles/theme.css || fail "theme.css: light theme"
grep -q 'country:Россия' frontend/src/lib/search.test.ts || fail "search test: Россия"
grep -q 'chartAxisStroke' frontend/src/pages/System/SystemPage.tsx || fail "system: chartAxisStroke"
grep -q 'nm-theme-change' frontend/src/pages/System/SystemPage.tsx || fail "system: theme listener"
grep -q 'id="map-main"' frontend/src/pages/Map/MapPage.tsx || fail "map main landmark"
ok "auth/ui/search contracts"

if grep -qE 'unpkg\.com|jsdelivr\.net' frontend/index.html frontend/src/pages/Map/MapPage.tsx frontend/src/pages/System/SystemPage.tsx; then
  fail "CDN script references still present"
fi
ok "no CDN script tags"

python - <<'PY' || fail "countries.geojson invalid"
import json
from pathlib import Path
p = Path("frontend/public/data/countries.geojson")
d = json.loads(p.read_text(encoding="utf-8"))
assert d.get("type") == "FeatureCollection", d.get("type")
feats = d.get("features") or []
assert len(feats) > 50, len(feats)
props = feats[0].get("properties") or {}
assert props.get("NAME") or props.get("ADMIN") or props.get("name"), sorted(props)[:12]
print(f"features={len(feats)} size={p.stat().st_size}")
PY
ok "countries.geojson"

grep -q 'auth_request /auth-check' frontend/nginx-app.inc || fail "nginx-app: auth_request"
grep -q '@login_redirect' frontend/nginx-app.inc || fail "nginx-app: login redirect"
grep -q 'try_files \$uri /index.html' frontend/nginx-app.inc || fail "nginx-app: SPA try_files"
grep -q 'return 302 /login' frontend/nginx-app.inc || fail "nginx-app: login redirect path"
grep -q 'location = /system.html' frontend/nginx-app.inc || fail "nginx-app: legacy system.html redirect"
grep -q 'include /etc/nginx/includes/app.inc' frontend/nginx.conf || fail "nginx.conf: app.inc include"
grep -q 'HTTPS_ENABLED' frontend/docker-entrypoint.sh || fail "entrypoint: HTTPS_ENABLED"
grep -q 'listen 443 ssl' frontend/docker-entrypoint.sh || fail "entrypoint: listen 443 ssl"
ok "nginx SPA / HTTPS"

echo "frontend-smoke: all checks passed"
