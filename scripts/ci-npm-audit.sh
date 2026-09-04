#!/usr/bin/env bash
# Production npm audit with retries for flaky registry / retired audit API.
#
# Real high+ findings still fail the job. Transient registry errors (503/502/504,
# "audit endpoint returned an error", etc.) are retried.
#
#   bash scripts/ci-npm-audit.sh
#   (expects cwd or FRONTEND_DIR; default: repo/frontend)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FRONTEND_DIR="${FRONTEND_DIR:-$ROOT/frontend}"
cd "$FRONTEND_DIR"

MAX_ATTEMPTS="${NPM_AUDIT_ATTEMPTS:-5}"
SLEEP_BASE="${NPM_AUDIT_SLEEP_BASE:-15}"
# Registry can hang ~7m before 503; bound each attempt.
ATTEMPT_TIMEOUT_S="${NPM_AUDIT_TIMEOUT_S:-90}"

is_transient() {
  # Match npm registry / transport failures, not "found N vulnerabilities".
  # timeout(1) exits 124 on kill — treat as transient.
  local ec="$2"
  if [[ "$ec" == "124" ]]; then
    return 0
  fi
  grep -qiE \
    'audit endpoint returned an error|Service Unavailable|Bad Gateway|Gateway Time-out|ECONNRESET|ETIMEDOUT|ENOTFOUND|EAI_AGAIN|socket hang up|network timeout|502 Bad Gateway|503 Service Unavailable|504 Gateway' \
    <<<"$1"
}

attempt=1
while ((attempt <= MAX_ATTEMPTS)); do
  echo "::group::npm audit (attempt ${attempt}/${MAX_ATTEMPTS}, timeout ${ATTEMPT_TIMEOUT_S}s)"
  set +e
  out="$(timeout "${ATTEMPT_TIMEOUT_S}" npm audit --omit=dev --audit-level=high 2>&1)"
  ec=$?
  set -e
  printf '%s\n' "$out"
  if ((ec == 124)); then
    echo "npm audit timed out after ${ATTEMPT_TIMEOUT_S}s"
  fi
  echo "::endgroup::"

  if ((ec == 0)); then
    echo "ok: npm audit (production, high+)"
    exit 0
  fi

  if is_transient "$out" "$ec"; then
    if ((attempt == MAX_ATTEMPTS)); then
      echo "::error::npm audit failed after ${MAX_ATTEMPTS} attempts (npm registry)"
      exit 1
    fi
    sleep_s=$((attempt * SLEEP_BASE))
    echo "::warning::transient npm audit registry error; retry in ${sleep_s}s"
    sleep "$sleep_s"
    attempt=$((attempt + 1))
    continue
  fi

  echo "::error::npm audit found high+ production vulnerabilities (or non-transient error)"
  exit "$ec"
done
