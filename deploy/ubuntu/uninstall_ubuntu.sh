#!/usr/bin/env bash
set -Eeuo pipefail

_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_COMMON="${_SCRIPT_DIR}/../common/uninstall.sh"

nm_audit_firewall() {
    if command -v ufw >/dev/null 2>&1; then
        if ufw status 2>/dev/null | grep -qi "Status: active"; then
            echo "  Firewall (UFW)   : active, правила HTTP/HTTPS/514/8080 будут проверены при удалении"
        else
            echo "  Firewall (UFW)   : установлен, но не active"
        fi
    else
        echo "  Firewall (UFW)   : не установлен"
    fi
}

nm_remove_firewall_rules() {
    if [[ "${REMOVE_FIREWALL_RULES:-1}" != "1" ]]; then
        return
    fi

    local http_port="80"
    local https_port=""
    # NM_PROJECT_DIR — канон common/uninstall.sh; PROJECT_DIR — legacy alias.
    local project_dir="${NM_PROJECT_DIR:-${PROJECT_DIR:-/opt/network-monitor}}"
    if [[ -f "${project_dir}/.env" ]]; then
        local v
        v="$(grep -E '^[[:space:]]*HTTP_PORT=' "${project_dir}/.env" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        [[ -n "$v" ]] && http_port="$v"
        v="$(grep -E '^[[:space:]]*HTTPS_PORT=' "${project_dir}/.env" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        [[ -n "$v" ]] && https_port="$v"
    fi
    [[ -z "$https_port" ]] && https_port="443"

    if command -v ufw >/dev/null 2>&1; then
        echo "[$(date +'%F %T')] Removing UFW rules..."
        ufw delete allow "${http_port}/tcp" || true
        ufw delete allow "${https_port}/tcp" || true
        ufw delete allow 80/tcp   || true
        ufw delete allow 443/tcp  || true
        ufw delete allow 514/tcp  || true
        ufw delete allow 514/udp  || true
        # Legacy rule from previous versions (backend was exposed on 8080):
        ufw delete allow 8080/tcp || true

        if ufw status | grep -qi "Status: active"; then
            ufw reload || true
        fi
    fi
}

# shellcheck source=deploy/common/uninstall.sh
source "$_COMMON"
nm_run_uninstall "$@"
