#!/usr/bin/env bash
# Единая точка входа удаления ГеоАтлас.
# Определяет семейство ОС, подключает OS-бэкенд и общую логику.
#
#   sudo ./deploy/uninstall.sh
#
# Принудительно:
#   GA_FORCE_OS_FAMILY=debian|rhel sudo ./deploy/uninstall.sh
set -Eeuo pipefail

PROJECT_DIR="${PROJECT_DIR:-/opt/geoatlas}"
GA_DEPLOY_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"

_ga_uninstall_log() { echo "[$(date +'%F %T')] $*"; }

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
            _ga_uninstall_log "ОШИБКА: GA_FORCE_OS_FAMILY должен быть debian или rhel (сейчас: ${forced})"
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
        local like
        for like in ${ID_LIKE:-}; do
            family="$(_ga_detect_os_family "$like")"
            if [[ -n "$family" ]]; then
                echo "$family"
                return 0
            fi
        done
        _ga_uninstall_log "ОШИБКА: неподдерживаемая ОС: ${id} (${PRETTY_NAME:-unknown})"
    else
        _ga_uninstall_log "ОШИБКА: нет /etc/os-release — не удалось определить ОС"
    fi

    echo "Задайте семейство вручную:"
    echo "  GA_FORCE_OS_FAMILY=debian sudo bash ${GA_DEPLOY_DIR}/uninstall.sh"
    echo "  GA_FORCE_OS_FAMILY=rhel   sudo bash ${GA_DEPLOY_DIR}/uninstall.sh"
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
            _ga_uninstall_log "ОШИБКА: неизвестное семейство ОС: ${family}"
            exit 1
            ;;
    esac
    if [[ ! -f "$backend" ]]; then
        _ga_uninstall_log "ОШИБКА: OS-бэкенд не найден: ${backend}"
        exit 1
    fi
    # shellcheck source=/dev/null
    source "$backend"
}

main() {
    local family common
    family="$(_ga_resolve_os_family)"
    _ga_load_os_backend "$family"

    common="${GA_DEPLOY_DIR}/common/uninstall.sh"
    if [[ ! -f "$common" ]]; then
        _ga_uninstall_log "ОШИБКА: модуль удаления не найден: ${common}"
        exit 1
    fi
    # shellcheck source=deploy/common/uninstall.sh
    source "$common"
    ga_run_uninstall "$@"
}

main "$@"
