#!/usr/bin/env bash
# govulncheck по Go-модулям репозитория (source mode: только вызываемый код).
#
#   go install golang.org/x/vuln/cmd/govulncheck@v1.7.0
#   bash scripts/ci-govulncheck.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "ci-govulncheck FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

command -v govulncheck >/dev/null 2>&1 \
  || fail "govulncheck is not installed (go install golang.org/x/vuln/cmd/govulncheck@v1.7.0)"

modules=(backend stats-collector pkg/chconn pkg/syslogngstats)
for dir in "${modules[@]}"; do
  [[ -f "${dir}/go.mod" ]] || fail "missing ${dir}/go.mod"
  echo "::group::govulncheck ${dir}"
  govulncheck -C "$dir" ./...
  echo "::endgroup::"
  ok "$dir"
done

echo "ci-govulncheck: all checks passed"
