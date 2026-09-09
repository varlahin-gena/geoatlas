#!/usr/bin/env bash
# Ingest hot-path benches with allocs. Run twice (before/after a change) and
# compare with benchstat:
#
#   bash scripts/bench-ingest.sh /tmp/ingest-before.txt
#   # ... patch ...
#   bash scripts/bench-ingest.sh /tmp/ingest-after.txt
#   benchstat /tmp/ingest-before.txt /tmp/ingest-after.txt
#
#   go install golang.org/x/perf/cmd/benchstat@latest
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${1:-}"
COUNT="${BENCH_COUNT:-6}"
BENCHTIME="${BENCH_TIME:-200ms}"

cd "$ROOT/backend"

cmd=(go test
  ./internal/parser/
  ./internal/usecase/ingest/
  ./internal/adapter/clickhouse/ingeststore/
  ./internal/geoip/
  -run=NONE
  -bench=.
  -benchmem
  -count="$COUNT"
  -benchtime="$BENCHTIME"
)

if [[ -n "$OUT" ]]; then
  mkdir -p "$(dirname "$OUT")"
  "${cmd[@]}" | tee "$OUT"
else
  "${cmd[@]}"
fi
