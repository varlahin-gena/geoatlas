#!/usr/bin/env bash
# Fail if Go or npm production deps introduce toxic licenses for an Apache-2.0 appliance.
# Deny: AGPL*, GPL-3.0*, SSPL*, BUSL*, Commons-Clause. (GPL-2.0 / LGPL not denied here.)
#
#   bash scripts/ci-license-check.sh
#
# Needs: go, go-licenses (CI installs), node/npm for frontend.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "ci-license-check FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

export GOWORK="${GOWORK:-off}"
export GOTOOLCHAIN="${GOTOOLCHAIN:-local}"

GO_LICENSES="${GO_LICENSES:-go-licenses}"
command -v "$GO_LICENSES" >/dev/null 2>&1 || fail "go-licenses not installed (${GO_LICENSES})"

# License name is the last CSV field from `go-licenses report`.
toxic_go_license() {
  local lic="$1"
  case "$lic" in
    AGPL*|GPL-3.0*|GPL3*|GPLv3*|SSPL*|BUSL*|Commons-Clause*|commons-clause*)
      return 0
      ;;
  esac
  # Classifier sometimes emits "GNU Affero General Public License v3.0"
  if echo "$lic" | grep -qiE 'affero|sspl|business source|commons.clause'; then
    return 0
  fi
  if echo "$lic" | grep -qiE '(^|[^L])GPL.?3'; then
    # Exclude LGPL
    if echo "$lic" | grep -qiE 'LGPL|Lesser'; then
      return 1
    fi
    return 0
  fi
  return 1
}

check_go_module() {
  local dir="$1"
  local pkg="${2:-./...}"
  [[ -f "${dir}/go.mod" ]] || fail "missing ${dir}/go.mod"
  echo "::group::go-licenses report ${dir}"
  local csv tmp bad=0
  tmp="$(mktemp)"
  # stderr: expected warnings (asm, empty version); don't fail the pipe on those.
  if ! (cd "$dir" && "$GO_LICENSES" report ${pkg} >"$tmp"); then
    echo "go-licenses report failed in ${dir} (see above)" >&2
    cat "$tmp" >&2 || true
    rm -f "$tmp"
    echo "::endgroup::"
    return 1
  fi
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ -z "$line" ]] && continue
    # CSV: module,URL,license — license may contain commas rarely; take last field.
    lic="${line##*,}"
    mod="${line%%,*}"
    if toxic_go_license "$lic"; then
      echo "TOXIC: ${dir}: ${mod} → ${lic}" >&2
      bad=1
    fi
  done <"$tmp"
  rm -f "$tmp"
  echo "::endgroup::"
  if [[ "$bad" -ne 0 ]]; then
    return 1
  fi
  ok "go ${dir}"
  return 0
}

go_fail=0
check_go_module backend ./... || go_fail=1
check_go_module stats-collector ./... || go_fail=1
check_go_module pkg/chconn ./... || go_fail=1
check_go_module pkg/syslogngstats ./... || go_fail=1
[[ "$go_fail" -eq 0 ]] || fail "toxic Go license(s) found"

# npm production only (runtime ship in frontend image).
command -v npx >/dev/null 2>&1 || fail "npx not found"
[[ -f frontend/package-lock.json ]] || fail "missing frontend/package-lock.json"

echo "::group::npm license-checker (production)"
# Pin for reproducibility; --production ≈ runtime deps.
fail_on='AGPL;AGPL-1.0;AGPL-3.0;GPL-3.0;GPL-3.0-only;GPL-3.0-or-later;SSPL-1.0;BUSL-1.1;Commons-Clause'
(
  cd frontend
  npx --yes license-checker@25.0.1 \
    --production \
    --excludePrivatePackages \
    --failOn "$fail_on"
)
echo "::endgroup::"
ok "npm production"

echo "ci-license-check: all checks passed"
