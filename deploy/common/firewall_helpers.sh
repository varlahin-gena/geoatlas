#!/usr/bin/env bash
# Хелперы host firewall для установщиков (UFW / firewalld).
# Использование:
#   source deploy/common/firewall_helpers.sh
#   ga_ufw_allow_syslog
#   ga_firewalld_allow_syslog
#
# Env:
#   GA_SYSLOG_ALLOW_FROM=CIDR|IP — открыть :514 только с этого источника; иначе any.

_ga_fw_log() { echo "[$(date +'%F %T')] [firewall] $*" >&2; }

# Syslog :514. GA_SYSLOG_ALLOW_FROM=CIDR|IP — только этот источник; иначе any.
ga_ufw_allow_syslog() {
    local from="${GA_SYSLOG_ALLOW_FROM:-}"
    if [[ -n "$from" ]]; then
        _ga_fw_log "UFW: 514/tcp+udp только с ${from}"
        ufw allow from "$from" to any port 514 proto tcp || true
        ufw allow from "$from" to any port 514 proto udp || true
    else
        ufw allow 514/tcp || true
        ufw allow 514/udp || true
    fi
}

ga_firewalld_allow_syslog() {
    local from="${GA_SYSLOG_ALLOW_FROM:-}"
    local family="ipv4"
    if [[ -n "$from" ]]; then
        [[ "$from" == *:* ]] && family="ipv6"
        _ga_fw_log "firewalld: 514/tcp+udp только с ${from} (${family})"
        firewall-cmd --permanent --add-rich-rule="rule family=\"${family}\" source address=\"${from}\" port port=\"514\" protocol=\"tcp\" accept" || true
        firewall-cmd --permanent --add-rich-rule="rule family=\"${family}\" source address=\"${from}\" port port=\"514\" protocol=\"udp\" accept" || true
    else
        firewall-cmd --permanent --add-port=514/tcp || true
        firewall-cmd --permanent --add-port=514/udp || true
    fi
}
