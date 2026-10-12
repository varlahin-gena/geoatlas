#!/usr/bin/env bash
# Longer fuzz budget than CI smoke (ci.yml uses 200000x).
# Count-based -fuzztime avoids wall-clock "context deadline exceeded" (golang/go#75804).
#
#   bash scripts/ci-fuzz-nightly.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/backend"

fail() { echo "ci-fuzz-nightly FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

export GOWORK="${GOWORK:-off}"
export GOTOOLCHAIN="${GOTOOLCHAIN:-local}"

# ~100× PR smoke (200000x). Adjust via FUZZ_NIGHTLY_COUNT if needed.
count="${FUZZ_NIGHTLY_COUNT:-20000000}"
[[ "$count" =~ ^[0-9]+$ ]] || fail "FUZZ_NIGHTLY_COUNT must be digits (got ${count})"

echo "::group::FuzzParseVerbose ${count}x"
go test ./internal/parser/ -run=^$ -fuzz=FuzzParseVerbose -fuzztime="${count}x"
echo "::endgroup::"
ok "FuzzParseVerbose"

echo "::group::FuzzFrameReader ${count}x"
go test ./internal/adapter/ingestnet/ -run=^$ -fuzz=FuzzFrameReader -fuzztime="${count}x"
echo "::endgroup::"
ok "FuzzFrameReader"

echo "ci-fuzz-nightly: all checks passed"
