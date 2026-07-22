#!/usr/bin/env bash
set -Eeuo pipefail

_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_COMMON="${_SCRIPT_DIR}/../common/uninstall.sh"

nm_audit_firewall() {
    if command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld 2>/dev/null; then
        echo "  Firewall         : firewalld active, порты 80/514/8080 будут проверены при удалении"
    elif command -v firewall-cmd >/dev/null 2>&1; then
        echo "  Firewall         : firewalld установлен, но не active"
    else
        echo "  Firewall         : firewalld не найден"
    fi
}

nm_remove_firewall_rules() {
    if [[ "${REMOVE_FIREWALL_RULES:-1}" != "1" ]]; then
        return
    fi

    if command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld; then
        echo "[$(date +'%F %T')] Removing firewalld rules..."
        firewall-cmd --permanent --remove-port=80/tcp   || true
        firewall-cmd --permanent --remove-port=514/tcp  || true
        firewall-cmd --permanent --remove-port=514/udp  || true
        # Legacy rule from previous versions (backend was exposed on 8080):
        firewall-cmd --permanent --remove-port=8080/tcp || true
        firewall-cmd --reload || true
    else
        echo "[$(date +'%F %T')] firewalld not active or not installed — skipping rule removal."
    fi
}

# shellcheck source=deploy/common/uninstall.sh
source "$_COMMON"
nm_run_uninstall "$@"
