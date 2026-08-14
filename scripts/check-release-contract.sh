#!/usr/bin/env bash
# Контракт релизов: VERSION ↔ CHANGELOG ↔ openapi.yaml ↔ README.
# На каждом PR / push в main. При GITHUB_REF=refs/tags/vX.Y.Z ещё сверяет тег с VERSION.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "release-contract FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

[[ -f VERSION ]] || fail "missing VERSION"
[[ -f CHANGELOG.md ]] || fail "missing CHANGELOG.md"
[[ -f openapi.yaml ]] || fail "missing openapi.yaml"
[[ -f README.md ]] || fail "missing README.md"

VERSION="$(tr -d '[:space:]' < VERSION)"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "VERSION is not X.Y.Z: ${VERSION:-<empty>}"
ok "VERSION $VERSION"

OA="$(awk '
  /^info:/ { ininfo = 1; next }
  ininfo && /^  version: "/ {
    if (match($0, /[0-9]+\.[0-9]+\.[0-9]+/)) {
      print substr($0, RSTART, RLENGTH)
      exit
    }
  }
  ininfo && /^[^ ]/ { exit }
' openapi.yaml)"
[[ "$OA" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "openapi.yaml info.version not found"
ok "OpenAPI $OA"

grep -qE "OpenAPI \\*\\*${OA}\\*\\*" README.md || fail "README.md must mention OpenAPI **${OA}**"
while IFS= read -r line; do
  ver="$(echo "$line" | sed -n 's/.*OpenAPI \*\*\([0-9][0-9.]*\)\*\*.*/\1/p')"
  if [[ -n "$ver" && "$ver" != "$OA" ]]; then
    fail "README.md OpenAPI **${ver}** ≠ openapi.yaml ${OA}"
  fi
done < <(grep -E 'OpenAPI \*\*[0-9]' README.md || true)
ok "README OpenAPI **${OA}**"

grep -qE "^## \[${VERSION}\]" CHANGELOG.md || fail "CHANGELOG.md missing ## [${VERSION}]"
grep -qE "^\[${VERSION}\]:" CHANGELOG.md || fail "CHANGELOG.md missing footer link [${VERSION}]:"
ok "CHANGELOG has section and footer for $VERSION"

# Секция от заголовка ## [name] до следующего ## [ (не включая).
changelog_section() {
  local name="$1"
  awk -v h="$name" '
    $0 ~ "^## \\[" h "\\]" { p = 1; next }
    p && /^## \[/ { exit }
    p { print }
  ' CHANGELOG.md
}

released_oa="$(changelog_section "$VERSION" | sed -n 's/.*OpenAPI API doc version: \*\*\([0-9][0-9.]*\)\*\*.*/\1/p' | head -n1)"
[[ -n "$released_oa" ]] || fail "CHANGELOG [${VERSION}] Notes missing «OpenAPI API doc version: **X.Y.Z**»"
ok "CHANGELOG [${VERSION}] Notes OpenAPI $released_oa"

if [[ "$OA" != "$released_oa" ]]; then
  unreleased="$(changelog_section "Unreleased")"
  [[ -n "$unreleased" ]] || fail "openapi ${OA} ≠ released ${released_oa}, but Unreleased is empty"
  echo "$unreleased" | grep -qF "$OA" || fail "openapi ${OA} ≠ released ${released_oa}; Unreleased must mention ${OA}"
  ok "Unreleased mentions OpenAPI $OA (ahead of ${VERSION})"
else
  ok "OpenAPI matches last release Notes"
fi

if [[ "${GITHUB_REF:-}" == refs/tags/v* ]]; then
  tag="${GITHUB_REF#refs/tags/v}"
  [[ "$tag" == "$VERSION" ]] || fail "git tag v${tag} ≠ VERSION ${VERSION}"
  [[ "$OA" == "$released_oa" ]] || fail "tag v${tag}: OpenAPI ${OA} still ahead of Notes ${released_oa} — move Unreleased into [${VERSION}] first"
  ok "tag v${tag} matches VERSION and OpenAPI Notes"
fi

echo "release-contract: all checks passed"
