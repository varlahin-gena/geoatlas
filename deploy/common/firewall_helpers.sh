#!/usr/bin/env bash
# Хелперы host firewall для установщиков (UFW / firewalld).
# Использование:
#   source deploy/common/firewall_helpers.sh
#   ga_ufw_allow_syslog
#   ga_firewalld_allow_syslog
#
# Env:
#   GA_SYSLOG_ALLOW_FROM=CIDR|IP — открыть syslog-порты только с этого источника; иначе any.
#   GA_CONNECTOR_EXTRA_PORTS=5514-5529 — доп. порты (как в compose); плюс всегда 514.

_ga_fw_log() { echo "[$(date +'%F %T')] [firewall] $*" >&2; }

# Список портов: 514 + extras из GA_CONNECTOR_EXTRA_PORTS.
ga_syslog_ports() {
    local ports=(514)
    local range="${GA_CONNECTOR_EXTRA_PORTS:-5514-5529}"
    range="${range// /}"
    if [[ -n "$range" ]]; then
        if [[ "$range" == *-* ]]; then
            local lo="${range%-*}"
            local hi="${range#*-}"
            local p
            for ((p=lo; p<=hi; p++)); do
                ports+=("$p")
            done
        else
            ports+=("$range")
        fi
    fi
    printf '%s\n' "${ports[@]}"
}

# Syslog :514 (+ extra connector ports). GA_SYSLOG_ALLOW_FROM=CIDR|IP — только этот источник; иначе any.
ga_ufw_allow_syslog() {
    local from="${GA_SYSLOG_ALLOW_FROM:-}"
    local port
    while read -r port; do
        [[ -z "$port" ]] && continue
        if [[ -n "$from" ]]; then
            _ga_fw_log "UFW: ${port}/tcp+udp только с ${from}"
            ufw allow from "$from" to any port "$port" proto tcp || true
            ufw allow from "$from" to any port "$port" proto udp || true
        else
            ufw allow "${port}/tcp" || true
            ufw allow "${port}/udp" || true
        fi
    done < <(ga_syslog_ports)
}

ga_firewalld_allow_syslog() {
    local from="${GA_SYSLOG_ALLOW_FROM:-}"
    local family="ipv4"
    local port
    while read -r port; do
        [[ -z "$port" ]] && continue
        if [[ -n "$from" ]]; then
            [[ "$from" == *:* ]] && family="ipv6"
            _ga_fw_log "firewalld: ${port}/tcp+udp только с ${from} (${family})"
            firewall-cmd --permanent --add-rich-rule="rule family=\"${family}\" source address=\"${from}\" port port=\"${port}\" protocol=\"tcp\" accept" || true
            firewall-cmd --permanent --add-rich-rule="rule family=\"${family}\" source address=\"${from}\" port port=\"${port}\" protocol=\"udp\" accept" || true
        else
            firewall-cmd --permanent --add-port="${port}/tcp" || true
            firewall-cmd --permanent --add-port="${port}/udp" || true
        fi
    done < <(ga_syslog_ports)
}
