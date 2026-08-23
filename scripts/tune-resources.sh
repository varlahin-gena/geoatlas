#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
PROJECT_DIR="$(cd -- "$SCRIPT_DIR/.." &>/dev/null && pwd)"

log() { echo "[$(date +'%F %T')] $*"; }

if [[ ! -f "${PROJECT_DIR}/deploy/common/detect_resources.sh" ]]; then
    log "detect_resources.sh not found in ${PROJECT_DIR}"
    exit 1
fi

# shellcheck source=../deploy/common/detect_resources.sh
source "${PROJECT_DIR}/deploy/common/detect_resources.sh"
# shellcheck source=../deploy/common/compose.sh
source "${PROJECT_DIR}/deploy/common/compose.sh"

log "Пересчёт конфигурации по ресурсам сервера..."
apply_resource_profile "$PROJECT_DIR"

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    if ga_compose "$PROJECT_DIR" ps --status running -q 2>/dev/null | grep -q .; then
        log "Стек уже запущен — перезапускаем с новыми лимитами..."
        ga_compose "$PROJECT_DIR" up -d
        log "Готово. Проверьте install-profile.json для деталей."
    else
        log "Стек не запущен. Запустите: ./start.sh"
    fi
fi
