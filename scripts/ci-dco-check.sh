#!/usr/bin/env bash
# Verify Developer Certificate of Origin (Signed-off-by) on commits.
# CI PR:  BASE_SHA=... HEAD_SHA=... bash scripts/ci-dco-check.sh
# Local:  bash scripts/ci-dco-check.sh origin/main..HEAD
set -euo pipefail

fail() { echo "dco-check FAIL: $*" >&2; exit 1; }
ok() { echo "dco-check: $*"; }

RANGE="${1:-}"
if [[ -z "$RANGE" ]]; then
  [[ -n "${BASE_SHA:-}" && -n "${HEAD_SHA:-}" ]] || fail "pass RANGE or set BASE_SHA and HEAD_SHA"
  RANGE="${BASE_SHA}..${HEAD_SHA}"
fi

mapfile -t commits < <(git rev-list --reverse "$RANGE")
((${#commits[@]} > 0)) || fail "no commits in range $RANGE"

bad=0
for sha in "${commits[@]}"; do
  author_name=$(git show -s --format='%an' "$sha")
  author_email=$(git show -s --format='%ae' "$sha")
  body=$(git show -s --format='%B' "$sha")
  expected="Signed-off-by: ${author_name} <${author_email}>"
  if ! grep -Fqx "$expected" <<<"$body"; then
    echo "Missing or mismatched Signed-off-by on $sha" >&2
    echo "  author: ${author_name} <${author_email}>" >&2
    echo "  expected: ${expected}" >&2
    bad=$((bad + 1))
  fi
done

((bad == 0)) || fail "$bad commit(s) without valid DCO (use: git commit -s)"
ok "${#commits[@]} commit(s) in $RANGE have Signed-off-by"
