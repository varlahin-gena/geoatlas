#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
cd "$SCRIPT_DIR"

# shellcheck source=deploy/common/compose.sh
source "${SCRIPT_DIR}/deploy/common/compose.sh"

REMOVE_DOCKER_VOLUMES="${REMOVE_DOCKER_VOLUMES:-0}"

log() { echo "[$(date +'%F %T')] $*"; }

trap 'log "ОШИБКА на строке ${LINENO} (код выхода $?)."' ERR

require_docker() {
    if ! command -v docker >/dev/null 2>&1; then
        echo "Docker не найден."
        exit 1
    fi
    if ! docker compose version >/dev/null 2>&1; then
        echo "Плагин docker compose не найден."
        exit 1
    fi
}

stop_stack() {
    log "Остановка стека Docker Compose..."
    if [[ "$REMOVE_DOCKER_VOLUMES" == "1" ]]; then
        log "ВНИМАНИЕ: REMOVE_DOCKER_VOLUMES=1 — данные ClickHouse будут УДАЛЕНЫ!"
        nm_compose "$SCRIPT_DIR" down -v --remove-orphans
    else
        nm_compose "$SCRIPT_DIR" down --remove-orphans
        log "Docker volumes сохранены (удалить: REMOVE_DOCKER_VOLUMES=1)."
    fi
}

main() {
    require_docker
    stop_stack
    log "Стек остановлен."
}

main "$@"
