#!/usr/bin/env bash
# Cosign keyless sign-blob for the release tarball and git tag attestation (Sigstore / GitHub OIDC).
#   bash scripts/ci-cosign-sign.sh [dist/geoatlas-X.Y.Z.tar.gz]
#
# Also writes/signs dist/geoatlas-X.Y.Z.gittag (+ .cosign.bundle) when GITHUB_REF_NAME
# and GITHUB_SHA are set (release-tag workflow). Same verify story as the tarball.
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

# Git tag attestation (OpenSSF version_tags_signed): same cosign keyless as signed_releases.
tag_name="${GITHUB_REF_NAME:-}"
commit_sha="${GITHUB_SHA:-}"
if [[ -n "$tag_name" && -n "$commit_sha" ]]; then
  gittag_path="$(dirname -- "$tarball")/${base}.gittag"
  {
    echo "tag=${tag_name}"
    echo "commit=${commit_sha}"
    echo "product=${base#geoatlas-}"
  } >"$gittag_path"
  gittag_bundle="${gittag_path}.cosign.bundle"
  echo "::group::cosign sign-blob ${base}.gittag"
  "$COSIGN" sign-blob --yes \
    --bundle "$gittag_bundle" \
    "$gittag_path"
  echo "::endgroup::"
  [[ -s "$gittag_bundle" ]] || fail "empty bundle ${gittag_bundle}"
  ok "$gittag_bundle"
else
  echo "skip gittag cosign (GITHUB_REF_NAME / GITHUB_SHA unset)"
fi

echo "ci-cosign-sign: all checks passed"
