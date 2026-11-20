#!/usr/bin/env bash
# Единая точка входа установки ГеоАтлас.
# Определяет семейство ОС, подключает OS-бэкенд и общий оркестратор.
#
#   sudo ./deploy/install.sh
#
# Принудительно:
#   GA_FORCE_OS_FAMILY=debian|rhel sudo ./deploy/install.sh
set -Eeuo pipefail

PROJECT_DIR="${PROJECT_DIR:-/opt/geoatlas}"
GA_DEPLOY_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"

_ga_install_log() { echo "[$(date +'%F %T')] $*"; }

_ga_detect_os_family() {
    local id="${1:-}"
    case "$id" in
        ubuntu|debian|linuxmint|pop)
            echo "debian"
            ;;
        ol|rhel|rocky|almalinux|centos|fedora)
            echo "rhel"
            ;;
        *)
            echo ""
            ;;
    esac
}

_ga_resolve_os_family() {
    local forced="${GA_FORCE_OS_FAMILY:-}"
    local id="" family=""

    case "$forced" in
        debian|rhel)
            echo "$forced"
            return 0
            ;;
        "")
            ;;
        *)
            _ga_install_log "ОШИБКА: GA_FORCE_OS_FAMILY должен быть debian или rhel (сейчас: ${forced})"
            exit 1
            ;;
    esac

    if [[ -f /etc/os-release ]]; then
        # shellcheck disable=SC1091
        . /etc/os-release
        id="${ID:-unknown}"
        family="$(_ga_detect_os_family "$id")"
        if [[ -n "$family" ]]; then
            echo "$family"
            return 0
        fi
        # ID_LIKE: "rhel centos fedora" / "debian"
        local like
        for like in ${ID_LIKE:-}; do
            family="$(_ga_detect_os_family "$like")"
            if [[ -n "$family" ]]; then
                echo "$family"
                return 0
            fi
        done
        _ga_install_log "ОШИБКА: неподдерживаемая ОС: ${id} (${PRETTY_NAME:-unknown})"
    else
        _ga_install_log "ОШИБКА: нет /etc/os-release — не удалось определить ОС"
    fi

    echo "Задайте семейство вручную:"
    echo "  GA_FORCE_OS_FAMILY=debian sudo bash ${GA_DEPLOY_DIR}/install.sh"
    echo "  GA_FORCE_OS_FAMILY=rhel   sudo bash ${GA_DEPLOY_DIR}/install.sh"
    exit 1
}

_ga_load_os_backend() {
    local family="$1"
    local backend=""
    case "$family" in
        debian)
            backend="${GA_DEPLOY_DIR}/ubuntu/os_backend.sh"
            ;;
        rhel)
            backend="${GA_DEPLOY_DIR}/oracle_linux/os_backend.sh"
            ;;
        *)
            _ga_install_log "ОШИБКА: неизвестное семейство ОС: ${family}"
            exit 1
            ;;
    esac
    if [[ ! -f "$backend" ]]; then
        _ga_install_log "ОШИБКА: OS-бэкенд не найден: ${backend}"
        exit 1
    fi
    # shellcheck source=/dev/null
    source "$backend"
}

main() {
    local family orch
    family="$(_ga_resolve_os_family)"
    _ga_load_os_backend "$family"

    orch="${GA_DEPLOY_DIR}/common/install_orchestrator.sh"
    if [[ ! -f "$orch" ]]; then
        _ga_install_log "ОШИБКА: оркестратор не найден: ${orch}"
        exit 1
    fi
    # shellcheck source=deploy/common/install_orchestrator.sh
    source "$orch"
    ga_install_main "$@"
}

main "$@"
