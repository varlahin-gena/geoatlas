#!/usr/bin/env bash
# Пресет «Сделай мне хорошо» (full-auto install).
# Использование:
#   source deploy/common/full_auto_preset.sh
#   nm_parse_full_auto_argv "$@"
#   nm_apply_full_auto_preset
#   nm_disable_host_firewall   # при NM_FULL_AUTO=1
#
# Включение: NM_FULL_AUTO=1 или argv --full-auto
# Даёт: релиз, все модули, HTTP 8080, автопрофиль, старт стека, firewall OFF.

_nm_fa_log() { echo "[$(date +'%F %T')] [full-auto] $*" >&2; }

# Сканирует argv; при --full-auto выставляет NM_FULL_AUTO=1.
nm_parse_full_auto_argv() {
    local arg
    for arg in "$@"; do
        case "$arg" in
            --full-auto)
                export NM_FULL_AUTO=1
                return 0
                ;;
        esac
    done
    return 0
}

# Применяет пресет, если NM_FULL_AUTO=1.
# Firewall всегда выключается. Остальные knobs — только если ещё не заданы.
nm_apply_full_auto_preset() {
    if [[ "${NM_FULL_AUTO:-0}" != "1" ]]; then
        return 0
    fi

    export NM_AUTO_MODULES=1
    export NM_AUTO_PROFILE=1
    export NM_FORCE=1

    if [[ -z "${NM_INSTALL_SOURCE:-}" ]]; then
        export NM_INSTALL_SOURCE=release
    fi

    if [[ -z "${HTTP_PORT:-}" ]]; then
        if [[ -n "${NM_HTTP_PORT:-}" ]]; then
            export HTTP_PORT="$NM_HTTP_PORT"
        else
            export HTTP_PORT=8080
        fi
    fi

    # Firewall: принудительно выкл. (даже если ENABLE_* задали снаружи).
    export ENABLE_UFW=0
    export ENABLE_FIREWALL=0
    export NM_FIREWALL_FROM_ENV=1

    _nm_fa_log "режим «Сделай мне хорошо»: release=${NM_INSTALL_SOURCE}, HTTP_PORT=${HTTP_PORT}, auto profile, all modules, firewall OFF, start stack"
}

# Активно выключает host firewall (UFW / firewalld). Ошибки не фатальны.
nm_disable_host_firewall() {
    _nm_fa_log "выключаем host firewall…"

    if command -v ufw >/dev/null 2>&1; then
        if ufw --force disable >/dev/null 2>&1; then
            _nm_fa_log "UFW: disabled"
        else
            _nm_fa_log "UFW: disable не удался (продолжаем)"
        fi
    fi

    if command -v systemctl >/dev/null 2>&1; then
        if systemctl list-unit-files firewalld.service >/dev/null 2>&1 \
            || systemctl status firewalld >/dev/null 2>&1; then
            if systemctl disable --now firewalld >/dev/null 2>&1; then
                _nm_fa_log "firewalld: disabled and stopped"
            else
                _nm_fa_log "firewalld: disable/stop не удался (продолжаем)"
            fi
        fi
    fi
}
