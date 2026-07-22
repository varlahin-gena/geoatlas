#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
cd "$SCRIPT_DIR"

REMOVE_DOCKER_VOLUMES="${REMOVE_DOCKER_VOLUMES:-0}"

log() { echo "[$(date +'%F %T')] $*"; }

trap 'log "ERROR at line ${LINENO} (exit code $?)."' ERR

require_docker() {
    if ! command -v docker >/dev/null 2>&1; then
        echo "Docker not found."
        exit 1
    fi
    if ! docker compose version >/dev/null 2>&1; then
        echo "docker compose plugin not found."
        exit 1
    fi
}

stop_stack() {
    log "Stopping Docker Compose stack..."
    if [[ "$REMOVE_DOCKER_VOLUMES" == "1" ]]; then
        log "WARNING: REMOVE_DOCKER_VOLUMES=1 — ClickHouse data will be DELETED!"
        docker compose down -v --remove-orphans
    else
        docker compose down --remove-orphans
        log "Docker volumes preserved (set REMOVE_DOCKER_VOLUMES=1 to delete)."
    fi
}

main() {
    require_docker
    stop_stack
    log "Stack stopped."
}

main "$@"
