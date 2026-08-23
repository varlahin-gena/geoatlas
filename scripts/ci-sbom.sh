#!/usr/bin/env bash
# SBOM установочного пакета: CycloneDX + SPDX из содержимого tar.gz.
#   bash scripts/pack-release.sh
#   bash scripts/ci-sbom.sh [dist/geoatlas-X.Y.Z.tar.gz]
#
# Нужен syft (в CI: anchore/sbom-action/download-syft). Локально:
#   https://github.com/anchore/syft/releases  (пин: v1.51.0)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "ci-sbom FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

SYFT="${SYFT:-syft}"
if [[ "$SYFT" == */* ]]; then
  [[ -x "$SYFT" ]] || fail "syft is not executable (${SYFT})"
else
  command -v "$SYFT" >/dev/null 2>&1 || fail "syft is not installed (${SYFT})"
fi

tarball="${1:-}"
if [[ -z "$tarball" ]]; then
  shopt -s nullglob
  cands=(dist/geoatlas-*.tar.gz)
  [[ ${#cands[@]} -eq 1 ]] || fail "pass path to geoatlas-X.Y.Z.tar.gz (found ${#cands[@]} in dist/)"
  tarball="${cands[0]}"
fi
[[ -f "$tarball" ]] || fail "missing tarball ${tarball}"

tarball="$(cd "$(dirname -- "$tarball")" && pwd)/$(basename -- "$tarball")"
base="$(basename -- "$tarball" .tar.gz)"
[[ "$base" =~ ^geoatlas-[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "tarball name ${base} is not geoatlas-X.Y.Z"
ver="${base#geoatlas-}"
out_dir="$(dirname -- "$tarball")"
cdx="${out_dir}/${base}.cdx.json"
spdx="${out_dir}/${base}.spdx.json"

stage="$(mktemp -d "${TMPDIR:-/tmp}/ga-sbom.XXXXXX")"
cleanup() { rm -rf "$stage"; }
trap cleanup EXIT

tar -xzf "$tarball" -C "$stage"
root=""
for d in "$stage"/*; do
  [[ -d "$d" ]] || continue
  root="$d"
  break
done
[[ -n "$root" && -f "${root}/VERSION" && -f "${root}/frontend/package-lock.json" ]] \
  || fail "tarball layout (need VERSION + frontend/package-lock.json under one prefix)"
pkg_ver="$(tr -d '[:space:]' <"${root}/VERSION")"
[[ "$pkg_ver" == "$ver" ]] || fail "VERSION ${pkg_ver} ≠ tarball ${ver}"

echo "::group::syft scan ${base}"
"$SYFT" scan "dir:${root}" \
  --source-name "geoatlas" \
  --source-version "$ver" \
  -o "cyclonedx-json=${cdx}" \
  -o "spdx-json=${spdx}"
echo "::endgroup::"

[[ -s "$cdx" ]] || fail "empty ${cdx}"
[[ -s "$spdx" ]] || fail "empty ${spdx}"
grep -q 'CycloneDX' "$cdx" || fail "${cdx}: not CycloneDX"
grep -q 'SPDX' "$spdx" || fail "${spdx}: not SPDX"

ok "${cdx}"
ok "${spdx}"
echo "ci-sbom: all checks passed"
