#!/usr/bin/env bash
# Прикрепить к уже опубликованным релизам подпись, которую видит OpenSSF Scorecard.
#
# Scorecard Signed-Releases смотрит последние 5 релизов и суффикс .sigstore
# (не .cosign.bundle). Архив не пересобирается.
#
#   TAGS=v2.5.1,v2.5.0 bash scripts/ci-sign-existing-releases.sh
#
# Если на релизе уже есть .tar.gz.cosign.bundle — копируются те же байты в
# .tar.gz.sigstore (идентичность release-tag.yml сохраняется).
# Если bundle нет — keyless cosign sign-blob опубликованного tar.gz и заливаются
# оба имени. В GitHub Actions нужны contents: write и id-token: write.
set -euo pipefail

fail() { echo "ci-sign-existing-releases FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

repo="${GITHUB_REPOSITORY:-}"
[[ -n "$repo" ]] || fail "GITHUB_REPOSITORY is unset"
command -v gh >/dev/null 2>&1 || fail "gh is not installed"

TAGS="${TAGS:-}"
[[ -n "$TAGS" ]] || fail "TAGS is empty (comma-separated vX.Y.Z)"

tags=()
IFS=',' read -ra raw_tags <<< "$TAGS"
for raw in "${raw_tags[@]}"; do
  tag="${raw//[[:space:]]/}"
  [[ -n "$tag" ]] || continue
  [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "tag must be vX.Y.Z, got ${tag}"
  tags+=("$tag")
done
[[ ${#tags[@]} -gt 0 ]] || fail "no tags"
[[ ${#tags[@]} -le 10 ]] || fail "too many tags (${#tags[@]}); pass at most 10"

has_asset() {
  local want=$1 n
  for n in "${asset_names[@]}"; do
    [[ "$n" == "$want" ]] && return 0
  done
  return 1
}

for tag in "${tags[@]}"; do
  ver="${tag#v}"
  tarball_name="geoatlas-${ver}.tar.gz"
  bundle_name="${tarball_name}.cosign.bundle"
  sigstore_name="${tarball_name}.sigstore"

  asset_names=()
  while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    asset_names+=("$line")
  done < <(gh release view "$tag" --repo "$repo" --json assets --jq '.assets[].name')
  [[ ${#asset_names[@]} -gt 0 ]] || fail "no assets on ${tag}"

  if has_asset "$sigstore_name"; then
    ok "${tag}: ${sigstore_name} already attached"
    continue
  fi

  workdir="$(mktemp -d)"
  if has_asset "$bundle_name"; then
    gh release download "$tag" --repo "$repo" --pattern "$bundle_name" --dir "$workdir"
    cp -f "${workdir}/${bundle_name}" "${workdir}/${sigstore_name}"
    [[ -s "${workdir}/${sigstore_name}" ]] || fail "empty ${sigstore_name}"
    gh release upload "$tag" --repo "$repo" --clobber "${workdir}/${sigstore_name}"
    ok "${tag}: copied ${bundle_name} -> ${sigstore_name}"
  else
    has_asset "$tarball_name" || fail "${tag} has no ${tarball_name}"
    command -v cosign >/dev/null 2>&1 || fail "cosign is not installed"
    gh release download "$tag" --repo "$repo" --pattern "$tarball_name" --dir "$workdir"
    [[ -s "${workdir}/${tarball_name}" ]] || fail "empty ${tarball_name}"
    echo "::group::cosign sign-blob ${tarball_name}"
    cosign sign-blob --yes \
      --bundle "${workdir}/${sigstore_name}" \
      "${workdir}/${tarball_name}"
    echo "::endgroup::"
    [[ -s "${workdir}/${sigstore_name}" ]] || fail "empty ${sigstore_name}"
    cp -f "${workdir}/${sigstore_name}" "${workdir}/${bundle_name}"
    gh release upload "$tag" --repo "$repo" --clobber \
      "${workdir}/${sigstore_name}" \
      "${workdir}/${bundle_name}"
    ok "${tag}: signed ${tarball_name}"
  fi
  rm -rf "$workdir"
done

echo "ci-sign-existing-releases: all checks passed"
