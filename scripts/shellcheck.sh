#!/usr/bin/env bash
# Shellcheck на установочные и ops-скрипты (не clickhouse/*.sh).
# Severity error: предупреждения не валят CI, ловим реальные дыры.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "shellcheck FAIL: $*" >&2; exit 1; }

command -v shellcheck >/dev/null 2>&1 || fail "shellcheck is not installed"

mapfile -t files < <(git ls-files --cached --others --exclude-standard '*.sh' | grep -E '^(start\.sh|stop\.sh|update\.sh|scripts/|deploy/)' | sort)
[[ ${#files[@]} -gt 0 ]] || fail "no shell scripts matched"

echo "shellcheck -S error (${#files[@]} files)"
# SC1091: sourced files may be generated (zz_profile.conf) or optional on host.
shellcheck -S error -e SC1091 "${files[@]}"
echo "shellcheck: all checks passed"
