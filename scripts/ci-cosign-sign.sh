#!/usr/bin/env bash
# Cosign keyless sign-blob for the release tarball (Sigstore / GitHub OIDC).
#   bash scripts/ci-cosign-sign.sh [dist/geoatlas-X.Y.Z.tar.gz]
#
# В GitHub Actions нужен permissions.id-token: write.
# Локально keyless обычно недоступен — скрипт для CI / отладки в GHA.
#
# Нужен cosign в PATH (в CI: sigstore/cosign-installer).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "ci-cosign-sign FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

COSIGN="${COSIGN:-cosign}"
if [[ "$COSIGN" == */* ]]; then
  [[ -x "$COSIGN" ]] || fail "cosign is not executable (${COSIGN})"
else
  command -v "$COSIGN" >/dev/null 2>&1 || fail "cosign is not installed (${COSIGN})"
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
bundle="${tarball}.cosign.bundle"

echo "::group::cosign sign-blob ${base}.tar.gz"
"$COSIGN" sign-blob --yes \
  --bundle "$bundle" \
  "$tarball"
echo "::endgroup::"

[[ -s "$bundle" ]] || fail "empty bundle ${bundle}"
ok "$bundle"
echo "ci-cosign-sign: all checks passed"
