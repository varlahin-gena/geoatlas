#!/usr/bin/env bash
# Grype gate for release SBOM: fail on CRITICAL with a fix available.
#   bash scripts/pack-release.sh
#   bash scripts/ci-sbom.sh
#   bash scripts/ci-grype-release.sh [dist/geoatlas-X.Y.Z.cdx.json]
#
# Нужен grype (в CI: install.sh с пином). Локально:
#   https://github.com/anchore/grype/releases  (пин: v0.120.1)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "ci-grype-release FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

GRYPE="${GRYPE:-grype}"
if [[ "$GRYPE" == */* ]]; then
  [[ -x "$GRYPE" ]] || fail "grype is not executable (${GRYPE})"
else
  command -v "$GRYPE" >/dev/null 2>&1 || fail "grype is not installed (${GRYPE})"
fi

cdx="${1:-}"
if [[ -z "$cdx" ]]; then
  shopt -s nullglob
  cands=(dist/geoatlas-*.cdx.json)
  [[ ${#cands[@]} -eq 1 ]] || fail "pass path to geoatlas-X.Y.Z.cdx.json (found ${#cands[@]} in dist/)"
  cdx="${cands[0]}"
fi
[[ -f "$cdx" ]] || fail "missing SBOM ${cdx}"
[[ -s "$cdx" ]] || fail "empty SBOM ${cdx}"

cdx="$(cd "$(dirname -- "$cdx")" && pwd)/$(basename -- "$cdx")"
base="$(basename -- "$cdx" .cdx.json)"
[[ "$base" =~ ^geoatlas-[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "SBOM name ${base} is not geoatlas-X.Y.Z"

echo "::group::grype sbom:${base} (CRITICAL, only-fixed)"
# Match image-scan: CRITICAL + ignore unfixed. Exit non-zero on match.
"$GRYPE" "sbom:${cdx}" \
  --fail-on critical \
  --only-fixed \
  -o table
echo "::endgroup::"

ok "no fixed CRITICAL findings in ${cdx}"
echo "ci-grype-release: all checks passed"
