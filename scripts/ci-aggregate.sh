#!/usr/bin/env bash
# Aggregate CI job results for branch protection (path-filter skips must not look green).
set -euo pipefail

gate() {
  local name="$1" should_run="$2" result="$3"
  if [[ "$should_run" == "true" ]]; then
    if [[ "$result" != "success" ]]; then
      echo "FAIL: $name ($result)" >&2
      return 1
    fi
    echo "ok: $name (ran, success)"
    return 0
  fi
  if [[ "$result" == "success" || "$result" == "skipped" ]]; then
    echo "ok: $name (skipped)"
    return 0
  fi
  echo "FAIL: $name ($result)" >&2
  return 1
}

fail=0
gate release-contract true "${RELEASE_CONTRACT_RESULT:-}" || fail=1
gate frontend-smoke true "${FRONTEND_SMOKE_RESULT:-}" || fail=1
gate trivy true "${TRIVY_RESULT:-}" || fail=1
gate shellcheck "${SHELL_GATE:-false}" "${SHELLCHECK_RESULT:-}" || fail=1
gate backend "${BACKEND_GATE:-false}" "${BACKEND_RESULT:-}" || fail=1
gate chconn "${CHCONN_GATE:-false}" "${CHCONN_RESULT:-}" || fail=1
gate syslogngstats "${SYSLOGNGSTATS_GATE:-false}" "${SYSLOGNGSTATS_RESULT:-}" || fail=1
gate backend-integration "${INTEGRATION_GATE:-false}" "${BACKEND_INTEGRATION_RESULT:-}" || fail=1
gate stats-collector "${STATS_GATE:-false}" "${STATS_COLLECTOR_RESULT:-}" || fail=1
gate govulncheck "${GOVULNCHECK_GATE:-false}" "${GOVULNCHECK_RESULT:-}" || fail=1
gate frontend "${FRONTEND_GATE:-false}" "${FRONTEND_RESULT:-}" || fail=1

# docker-images matrix: any failure fails aggregate when docker_any=true
if [[ "${DOCKER_ANY_GATE:-false}" == "true" ]]; then
  if [[ "${DOCKER_IMAGES_RESULT:-success}" != "success" ]]; then
    echo "FAIL: docker-images (${DOCKER_IMAGES_RESULT:-})" >&2
    fail=1
  else
    echo "ok: docker-images (ran, success)"
  fi
else
  if [[ "${DOCKER_IMAGES_RESULT:-skipped}" == "success" || "${DOCKER_IMAGES_RESULT:-skipped}" == "skipped" ]]; then
    echo "ok: docker-images (skipped)"
  else
    echo "FAIL: docker-images (${DOCKER_IMAGES_RESULT:-})" >&2
    fail=1
  fi
fi

exit "$fail"
