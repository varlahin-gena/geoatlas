#!/usr/bin/env bash
# Сборка образов backend / frontend / stats-collector как в docker-compose.yml.
# Без `compose up` и без секретов: интерполяция ${VAR:?} для build не нужна.
#
#   bash scripts/ci-docker-build.sh              # все три
#   bash scripts/ci-docker-build.sh frontend     # один образ
#
# В GitHub Actions: docker/setup-buildx-action + NM_DOCKER_CACHE=gha
# (кэш слоёв между прогонами; --load, чтобы smoke видел образ).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "ci-docker-build FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

command -v docker >/dev/null 2>&1 || fail "docker is not installed"
docker info >/dev/null 2>&1 || fail "docker daemon is not running"

[[ -f docker-compose.yml ]] || fail "missing docker-compose.yml"
[[ -f backend/Dockerfile ]] || fail "missing backend/Dockerfile"
[[ -f stats-collector/Dockerfile ]] || fail "missing stats-collector/Dockerfile"
[[ -f frontend/Dockerfile ]] || fail "missing frontend/Dockerfile"

grep -q 'dockerfile: backend/Dockerfile' docker-compose.yml \
  || fail "docker-compose.yml: backend dockerfile path drifted"
grep -q 'dockerfile: stats-collector/Dockerfile' docker-compose.yml \
  || fail "docker-compose.yml: stats-collector dockerfile path drifted"
grep -q 'context: ./frontend' docker-compose.yml \
  || fail "docker-compose.yml: frontend context drifted"
ok "compose build contexts"

use_gha_cache=0
if [[ "${NM_DOCKER_CACHE:-}" == "gha" ]]; then
  docker buildx version >/dev/null 2>&1 || fail "NM_DOCKER_CACHE=gha requires docker buildx"
  use_gha_cache=1
fi

build_image() {
  local name="$1" file="$2" context="$3" tag="$4"
  echo "::group::docker build ${name}"
  if [[ "$use_gha_cache" -eq 1 ]]; then
    docker buildx build \
      --load \
      --provenance=false \
      --sbom=false \
      --cache-from "type=gha,scope=nm-docker-${name}" \
      --cache-to "type=gha,mode=max,scope=nm-docker-${name}" \
      -t "$tag" -f "$file" "$context"
  else
    docker build -t "$tag" -f "$file" "$context"
  fi
  echo "::endgroup::"
  ok "built ${tag}"
}

assert_user_entrypoint() {
  local tag="$1" user="$2" entrypoint="$3"
  local got_user got_ep
  got_user="$(docker image inspect -f '{{.Config.User}}' "$tag")"
  got_ep="$(docker image inspect -f '{{join .Config.Entrypoint " "}}' "$tag")"
  [[ "$got_user" == "$user" ]] || fail "${tag}: User '${got_user}' != '${user}'"
  [[ "$got_ep" == "$entrypoint" ]] || fail "${tag}: Entrypoint '${got_ep}' != '${entrypoint}'"
  ok "${tag} user=${got_user} entrypoint=${got_ep}"
}

smoke_go_binary() {
  local tag="$1" expect="$2"
  shift 2
  local out exit_code=0
  set +e
  if command -v timeout >/dev/null 2>&1; then
    out="$(timeout 12 docker run --rm "$@" "$tag" 2>&1)"
    exit_code=$?
  else
    out="$(docker run --rm "$@" "$tag" 2>&1)"
    exit_code=$?
  fi
  set -e
  if [[ "$exit_code" -eq 124 ]]; then
    ok "${tag} binary smoke (timeout after start)"
    return 0
  fi
  echo "$out" | grep -qE "$expect" \
    || fail "${tag} binary smoke (exit=${exit_code}): ${out}"
  ok "${tag} binary smoke"
}

smoke_backend() {
  local tag="$1"
  smoke_go_binary "$tag" 'network-monitor starting|build application|clickhouse|configuration|security config' \
    -e NM_ALLOW_INSECURE=1 \
    -e AUTH_DISABLED=1 \
    -e API_AUTH_DISABLED=1 \
    -e CLICKHOUSE_HOST=127.0.0.1 \
    -e CLICKHOUSE_PASSWORD=ci-smoke-test-password-32ch \
    -e SESSION_SECRET=ci-smoke-test-session-secret-32 \
    -e API_AUTH_TOKEN=ci-smoke-test-api-auth-token-32ch \
    -e INGEST_SHARED_SECRET=ci-smoke-test-ingest-secret-32ch
}

smoke_stats() {
  local tag="$1"
  smoke_go_binary "$tag" 'stats-collector starting|clickhouse connection|config:' \
    -e NM_ALLOW_INSECURE=1 \
    -e CLICKHOUSE_HOST=127.0.0.1 \
    -e CLICKHOUSE_PASSWORD=ci-smoke-test-password-32ch \
    -e API_AUTH_TOKEN=ci-smoke-test-api-auth-token-32ch
}

smoke_frontend() {
  local tag="$1"
  local cid="" port=8766
  command -v curl >/dev/null 2>&1 || fail "curl is required for frontend /health smoke"

  dump_logs() {
    docker logs "$cid" >&2 || true
    docker inspect -f 'status={{.State.Status}} exit={{.State.ExitCode}}' "$cid" >&2 || true
  }

  cleanup() {
    if [[ -n "$cid" ]]; then
      docker rm -f "$cid" >/dev/null 2>&1 || true
    fi
  }
  trap cleanup EXIT

  # nginx resolves proxy_pass http://backend:8080 at start; compose DNS is absent here.
  cid="$(docker run -d \
    -e HTTPS_ENABLED=0 \
    --add-host backend:127.0.0.1 \
    -p "127.0.0.1:${port}:80" \
    "$tag")"
  [[ -n "$cid" ]] || fail "docker run ${tag}"

  local i html running
  for i in $(seq 1 40); do
    running="$(docker inspect -f '{{.State.Running}}' "$cid" 2>/dev/null || echo false)"
    if [[ "$running" != "true" ]]; then
      dump_logs
      fail "frontend container exited before /health"
    fi
    if html="$(curl -sf "http://127.0.0.1:${port}/health" 2>/dev/null)"; then
      echo "$html" | grep -q '"ok":true' || fail "frontend /health body: ${html}"
      ok "frontend /health"
      curl -sf "http://127.0.0.1:${port}/" | grep -q 'id="root"' \
        || fail "frontend index: missing id=root"
      ok "frontend index root"
      trap - EXIT
      cleanup
      return 0
    fi
    sleep 0.25
  done
  dump_logs
  fail "frontend /health did not become ready"
}

build_backend() {
  local tag="network_monitor-backend:ci"
  build_image backend backend/Dockerfile . "$tag"
  assert_user_entrypoint "$tag" app /app/network-monitor
  smoke_backend "$tag"
}

build_stats() {
  local tag="network_monitor-stats-collector:ci"
  build_image stats-collector stats-collector/Dockerfile . "$tag"
  assert_user_entrypoint "$tag" app /app/stats-collector
  smoke_stats "$tag"
}

build_frontend() {
  local tag="network_monitor-frontend:ci"
  build_image frontend frontend/Dockerfile frontend "$tag"
  smoke_frontend "$tag"
}

if [[ $# -eq 0 ]]; then
  targets=(backend stats-collector frontend)
else
  targets=("$@")
fi

for t in "${targets[@]}"; do
  case "$t" in
    backend) build_backend ;;
    stats-collector) build_stats ;;
    frontend) build_frontend ;;
    *) fail "unknown image '${t}' (backend|stats-collector|frontend)" ;;
  esac
done

echo "ci-docker-build: all checks passed"
