#!/usr/bin/env bash
set -Eeuo pipefail

_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

_detect_os_id() {
    if [[ -f /etc/os-release ]]; then
        # shellcheck disable=SC1091
        . /etc/os-release
        echo "${ID:-unknown}"
    else
        echo "unknown"
    fi
}

OS_ID="$(_detect_os_id)"

case "$OS_ID" in
    ubuntu|debian|linuxmint|pop)
        exec "${_SCRIPT_DIR}/ubuntu/uninstall_ubuntu.sh" "$@"
        ;;
    ol|rhel|rocky|almalinux|centos|fedora)
        exec "${_SCRIPT_DIR}/oracle_linux/uninstall_oraclelinux.sh" "$@"
        ;;
    *)
        echo "[$(date +'%F %T')] ОШИБКА: неподдерживаемая ОС: ${OS_ID}"
        echo "Запустите явно:"
        echo "  sudo bash ${_SCRIPT_DIR}/ubuntu/uninstall_ubuntu.sh"
        echo "  sudo bash ${_SCRIPT_DIR}/oracle_linux/uninstall_oraclelinux.sh"
        exit 1
        ;;
esac
