#!/usr/bin/env bash
# Контрактные проверки frontend auth (без полного docker-стека).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "frontend-smoke FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

[[ -f frontend/auth.js ]] || fail "missing frontend/auth.js"
[[ -f frontend/common.js ]] || fail "missing frontend/common.js"
[[ -f frontend/common.css ]] || fail "missing frontend/common.css"
[[ -f frontend/theme.css ]] || fail "missing frontend/theme.css"
[[ -f frontend/favicon.svg ]] || fail "missing frontend/favicon.svg"
[[ -f frontend/login.html ]] || fail "missing frontend/login.html"
[[ -f frontend/nginx.conf ]] || fail "missing frontend/nginx.conf"
[[ -f frontend/nginx-app.inc ]] || fail "missing frontend/nginx-app.inc"
[[ -f frontend/docker-entrypoint.sh ]] || fail "missing frontend/docker-entrypoint.sh"
[[ -f frontend/data/countries.geojson ]] || fail "missing frontend/data/countries.geojson"
[[ -f frontend/index.css ]] || fail "missing frontend/index.css"
[[ -f frontend/system.css ]] || fail "missing frontend/system.css"
[[ -f frontend/auth-form.css ]] || fail "missing frontend/auth-form.css"
[[ -f frontend/js/map-app.js ]] || fail "missing frontend/js/map-app.js"
[[ -f frontend/js/map-state.js ]] || fail "missing frontend/js/map-state.js"
[[ -f frontend/js/map-detail.js ]] || fail "missing frontend/js/map-detail.js"
[[ -f frontend/js/map-render.js ]] || fail "missing frontend/js/map-render.js"
[[ -f frontend/js/map-filters.js ]] || fail "missing frontend/js/map-filters.js"
[[ -f frontend/js/map-uploads.js ]] || fail "missing frontend/js/map-uploads.js"
[[ -f frontend/js/system-app.js ]] || fail "missing frontend/js/system-app.js"
[[ -f frontend/vendor/deck.gl.min.js ]] || fail "missing frontend/vendor/deck.gl.min.js"
[[ -f frontend/vendor/uPlot.iife.min.js ]] || fail "missing frontend/vendor/uPlot.iife.min.js"
[[ -f frontend/vendor/uPlot.min.css ]] || fail "missing frontend/vendor/uPlot.min.css"
# Minimal size sanity (avoid empty/corrupt downloads)
deck_sz=$(wc -c < frontend/vendor/deck.gl.min.js | tr -d ' ')
uplot_sz=$(wc -c < frontend/vendor/uPlot.iife.min.js | tr -d ' ')
[[ "$deck_sz" -gt 500000 ]] || fail "deck.gl.min.js too small ($deck_sz)"
[[ "$uplot_sz" -gt 20000 ]] || fail "uPlot.iife.min.js too small ($uplot_sz)"
ok "shared assets present"

grep -q 'function requireLogin' frontend/auth.js || fail "auth.js: requireLogin"
grep -q 'nmAuthHeaders' frontend/auth.js || fail "auth.js: nmAuthHeaders"
grep -q 'X-CSRF-Token' frontend/auth.js || fail "auth.js: CSRF header"
grep -q 'nm_csrf' frontend/auth.js || fail "auth.js: CSRF cookie"
grep -q "loginUrl" frontend/auth.js || fail "auth.js: loginUrl"
grep -q 'nm-meta-version' frontend/auth.js || fail "auth.js: version in user menu"
grep -q '/api/system/version' frontend/auth.js || fail "auth.js: system version fetch"
ok "auth.js helpers"

grep -q 'escapeHTML' frontend/common.js || fail "common.js: escapeHTML"
grep -q 'function toast' frontend/common.js || fail "common.js: toast"
grep -q 'mountAdminTopbar' frontend/common.js || fail "common.js: mountAdminTopbar"
ok "common.js helpers"

grep -q -- '--bg:' frontend/theme.css || fail "theme.css: dark tokens"
grep -q 'data-theme="light"' frontend/theme.css || fail "theme.css: light theme"
grep -q 'body.page-admin .topbar' frontend/common.css || fail "common.css: admin topbar"
grep -q '\.toast-host' frontend/common.css || fail "common.css: toast"
grep -q ':focus-visible' frontend/common.css || fail "common.css: focus-visible"
grep -q '\.skip-link' frontend/common.css || fail "common.css: skip-link"
grep -q 'body.page-auth' frontend/auth-form.css || fail "auth-form.css: page-auth"
ok "shared css"

# GeoJSON: FeatureCollection с NAME/ADMIN
python - <<'PY' || fail "countries.geojson invalid"
import json
from pathlib import Path
p = Path("frontend/data/countries.geojson")
d = json.loads(p.read_text(encoding="utf-8"))
assert d.get("type") == "FeatureCollection", d.get("type")
feats = d.get("features") or []
assert len(feats) > 50, len(feats)
props = feats[0].get("properties") or {}
assert props.get("NAME") or props.get("ADMIN") or props.get("name"), sorted(props)[:12]
print(f"features={len(feats)} size={p.stat().st_size}")
PY
ok "countries.geojson"

grep -q '/api/auth/login' frontend/login.html || fail "login.html: login endpoint"
grep -q 'id="username"' frontend/login.html || fail "login.html: username field"
grep -q 'id="password"' frontend/login.html || fail "login.html: password field"
grep -q '/auth-form.css' frontend/login.html || fail "login.html: auth-form.css"
grep -q '/auth-form.css' frontend/change-password.html || fail "change-password.html: auth-form.css"
ok "login/change-password"

# Карта: внешние модули, без inline handlers
grep -q '/index.css' frontend/index.html || fail "index.html: index.css"
grep -q '/js/map-app.js' frontend/index.html || fail "index.html: map-app.js"
grep -q 'function bindUI' frontend/js/map-app.js || fail "map-app.js: bindUI"
grep -q 'function init' frontend/js/map-app.js || fail "map-app.js: init"
if grep -qE 'onclick=|onchange=|oninput=' frontend/index.html; then
  fail "index.html: inline event handlers still present"
fi
grep -q 'Россия' frontend/js/map-state.js || fail "map-state.js: encoding (Россия)"
grep -q '/vendor/deck.gl.min.js' frontend/index.html || fail "index.html: local deck.gl"
if grep -qE 'unpkg\.com|jsdelivr\.net' frontend/index.html; then
  fail "index.html: still references CDN"
fi
ok "index map modules"

# System
grep -q '/system.css' frontend/system.html || fail "system.html: system.css"
grep -q '/js/system-app.js' frontend/system.html || fail "system.html: system-app.js"
grep -q 'chartAxisStroke' frontend/js/system-app.js || fail "system-app.js: chartAxisStroke"
grep -q 'nm-theme-change' frontend/js/system-app.js || fail "system-app.js: theme listener"
grep -q '/vendor/uPlot.iife.min.js' frontend/system.html || fail "system.html: local uPlot"
if grep -qE 'unpkg\.com|jsdelivr\.net' frontend/system.html; then
  fail "system.html: still references CDN"
fi
ok "system modules"

# A11y basics
grep -q 'role="dialog"' frontend/users.html || fail "users.html: dialog role"
grep -q 'aria-modal' frontend/users.html || fail "users.html: aria-modal"
grep -q 'scope="col"' frontend/users.html || fail "users.html: th scope"
grep -q 'scope="col"' frontend/geo-missing.html || fail "geo-missing.html: th scope"
grep -q 'scope="col"' frontend/parse-errors.html || fail "parse-errors.html: th scope"
grep -q "kind === 'error' ? 'alert'" frontend/common.js || fail "common.js: toast alert role"
grep -q 'id="map-main"' frontend/index.html || fail "index.html: main landmark id"
ok "a11y basics"

# Все страницы тянут shared shell
for page in index.html system.html users.html parse-errors.html parser-test.html geo-missing.html change-password.html login.html; do
  grep -q '/theme.css' "frontend/${page}" || fail "${page}: missing theme.css"
  grep -q '/favicon.svg' "frontend/${page}" || fail "${page}: missing favicon.svg"
done
for page in users.html parse-errors.html parser-test.html geo-missing.html; do
  grep -q 'mountAdminTopbar' "frontend/${page}" || fail "${page}: missing mountAdminTopbar"
  grep -q '/common.js' "frontend/${page}" || fail "${page}: missing common.js"
  grep -q '/config.js' "frontend/${page}" || fail "${page}: missing config.js"
done
ok "pages share theme/favicon/admin shell"

grep -q 'auth_request /auth-check' frontend/nginx-app.inc || fail "nginx-app: auth_request"
grep -q '@login_redirect' frontend/nginx-app.inc || fail "nginx-app: login redirect"
grep -q 'location = /index.html' frontend/nginx-app.inc || fail "nginx-app: index protected"
grep -q 'error_page 401 = @login_redirect' frontend/nginx-app.inc || fail "nginx-app: 401→login"
grep -q 'include /etc/nginx/includes/app.inc' frontend/nginx.conf || fail "nginx.conf: app.inc include"
grep -q 'HTTPS_ENABLED' frontend/docker-entrypoint.sh || fail "entrypoint: HTTPS_ENABLED"
grep -q 'listen 443 ssl' frontend/docker-entrypoint.sh || fail "entrypoint: listen 443 ssl"
ok "nginx auth redirects / HTTPS entrypoint"

for page in system.html users.html parse-errors.html parser-test.html geo-missing.html; do
  grep -q "location = /${page}" frontend/nginx-app.inc || fail "nginx-app: missing ${page}"
done
ok "nginx admin locations"

echo "frontend-smoke: all checks passed"
