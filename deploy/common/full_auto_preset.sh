#!/usr/bin/env bash
# Пресет «Сделай мне хорошо» (full-auto install).
# Использование:
#   source deploy/common/full_auto_preset.sh
#   nm_parse_full_auto_argv "$@"
#   nm_apply_full_auto_preset
#   nm_disable_host_firewall   # при NM_FULL_AUTO=1
#   nm_full_auto_finish /opt/network-monitor   # после start.sh
#
# Включение: NM_FULL_AUTO=1 или argv --full-auto
# Даёт: релиз, все модули, HTTP 8080, автопрофиль, старт стека, firewall OFF.
# HTTPS не выключается: при TTY select_https спросит интерактивно.

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

    # HTTPS: не трогаем HTTPS_ENABLED — select_https спросит при TTY.
    _nm_fa_log "режим «Сделай мне хорошо»: release=${NM_INSTALL_SOURCE}, HTTP_PORT=${HTTP_PORT}, auto profile, all modules, firewall OFF, start stack"
    _nm_fa_log "HTTPS будет спрошен интерактивно (select_https), если есть TTY и HTTPS_ENABLED / NM_HTTPS_ENABLED ещё не заданы"
}

_nm_fa_http_port() {
    local port="${HTTP_PORT:-}"
    local project_dir="${1:-}"
    local v=""
    if [[ -z "$port" && -n "$project_dir" && -f "${project_dir}/.env" ]]; then
        v="$(grep -E '^[[:space:]]*HTTP_PORT=' "${project_dir}/.env" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        [[ -n "$v" ]] && port="$v"
    fi
    echo "${port:-8080}"
}

# HTTPS активен: PEM в certs/ и HTTPS_ENABLED не выключен.
# Читает .env при необходимости; печатает порт TLS в stdout, rc=0 если HTTPS on.
_nm_fa_https_active() {
    local project_dir="${1:-}"
    local enabled="${HTTPS_ENABLED:-}"
    local port="${HTTPS_PORT:-}"
    local v=""

    if [[ -n "$project_dir" && -f "${project_dir}/.env" ]]; then
        if [[ -z "$enabled" ]]; then
            v="$(grep -E '^[[:space:]]*HTTPS_ENABLED=' "${project_dir}/.env" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
            [[ -n "$v" ]] && enabled="$v"
        fi
        if [[ -z "$port" ]]; then
            v="$(grep -E '^[[:space:]]*HTTPS_PORT=' "${project_dir}/.env" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
            [[ -n "$v" ]] && port="$v"
        fi
    fi
    port="${port:-443}"

    case "${enabled}" in
        0|false|FALSE|no|NO|off|OFF)
            return 1
            ;;
    esac

    if [[ -n "$project_dir" ]] \
        && [[ -f "${project_dir}/certs/fullchain.pem" && -f "${project_dir}/certs/privkey.pem" ]]; then
        echo "$port"
        return 0
    fi

    case "${enabled}" in
        1|true|TRUE|yes|YES|on|ON)
            echo "$port"
            return 0
            ;;
    esac
    return 1
}

_nm_fa_ufw_active() {
    command -v ufw >/dev/null 2>&1 || return 1
    ufw status 2>/dev/null | grep -qi "Status: active"
}

_nm_fa_firewalld_active() {
    command -v firewall-cmd >/dev/null 2>&1 || return 1
    firewall-cmd --state >/dev/null 2>&1
}

# Если host firewall всё ещё активен — открываем HTTP(/HTTPS) и syslog.
_nm_fa_open_ports_if_fw_active() {
    local port="$1"
    local open_syslog="${2:-1}"
    local https_port="${3:-}"

    if _nm_fa_ufw_active; then
        _nm_fa_log "UFW всё ещё active — открываем ${port}/tcp (fallback)"
        ufw allow "${port}/tcp" >/dev/null 2>&1 || true
        if [[ -n "$https_port" ]]; then
            _nm_fa_log "UFW: HTTPS ${https_port}/tcp (fallback)"
            ufw allow "${https_port}/tcp" >/dev/null 2>&1 || true
        fi
        if [[ "$open_syslog" == "1" ]]; then
            ufw allow 514/tcp >/dev/null 2>&1 || true
            ufw allow 514/udp >/dev/null 2>&1 || true
        fi
        ufw reload >/dev/null 2>&1 || true
    fi

    if _nm_fa_firewalld_active; then
        _nm_fa_log "firewalld всё ещё active — открываем ${port}/tcp (fallback)"
        firewall-cmd --permanent --add-port="${port}/tcp" >/dev/null 2>&1 || true
        if [[ -n "$https_port" ]]; then
            _nm_fa_log "firewalld: HTTPS ${https_port}/tcp (fallback)"
            firewall-cmd --permanent --add-port="${https_port}/tcp" >/dev/null 2>&1 || true
        fi
        if [[ "$open_syslog" == "1" ]]; then
            firewall-cmd --permanent --add-port=514/tcp >/dev/null 2>&1 || true
            firewall-cmd --permanent --add-port=514/udp >/dev/null 2>&1 || true
        fi
        firewall-cmd --reload >/dev/null 2>&1 || true
    fi
}

# Активно выключает host firewall (UFW / firewalld). Ошибки не фатальны.
# $1 — опционально PROJECT_DIR (чтобы взять HTTP_PORT из .env).
nm_disable_host_firewall() {
    local project_dir="${1:-}"
    local port https_port=""
    port="$(_nm_fa_http_port "$project_dir")"

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

    # Если disable не сработал — иначе :8080 остаётся закрытым (правил allow нет).
    local syslog_on=1
    if [[ -n "$project_dir" && -f "${project_dir}/.env" ]]; then
        local mod
        mod="$(grep -E '^[[:space:]]*NM_MODULE_SYSLOG=' "${project_dir}/.env" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        [[ "$mod" == "0" ]] && syslog_on=0
    elif [[ "${NM_MODULE_SYSLOG:-1}" == "0" ]]; then
        syslog_on=0
    fi
    https_port="$(_nm_fa_https_active "$project_dir" 2>/dev/null || true)"
    _nm_fa_open_ports_if_fw_active "$port" "$syslog_on" "$https_port"
}

_nm_fa_host_ip() {
    local ip=""
    ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
    [[ -n "$ip" ]] || ip="127.0.0.1"
    echo "$ip"
}

# $1 port, $2 ip, $3 scheme (http|https)
_nm_fa_ui_url() {
    local port="$1" ip="$2" scheme="${3:-http}"
    if [[ "$scheme" == "https" ]]; then
        if [[ "$port" == "443" ]]; then
            echo "https://${ip}"
        else
            echo "https://${ip}:${port}"
        fi
    else
        if [[ "$port" == "80" ]]; then
            echo "http://${ip}"
        else
            echo "http://${ip}:${port}"
        fi
    fi
}

# Проверка login.html, подсказка URL/логина, попытка открыть браузер.
# $1 — PROJECT_DIR
nm_full_auto_finish() {
    if [[ "${NM_FULL_AUTO:-0}" != "1" ]]; then
        return 0
    fi

    local project_dir="${1:-.}"
    local port ip base login_url health_url ok=0
    local scheme="http" curl_opts=()
    local https_port=""

    port="$(_nm_fa_http_port "$project_dir")"
    ip="$(_nm_fa_host_ip)"

    if https_port="$(_nm_fa_https_active "$project_dir" 2>/dev/null)"; then
        scheme="https"
        port="$https_port"
        curl_opts=(-k)
    fi

    base="$(_nm_fa_ui_url "$port" "$ip" "$scheme")"
    login_url="${base}/login.html"
    health_url="${base}/api/health"

    _nm_fa_log "проверяем UI: ${login_url}"
    if command -v curl >/dev/null 2>&1; then
        if curl -fsS "${curl_opts[@]}" --connect-timeout 3 --max-time 8 "$login_url" >/dev/null 2>&1; then
            ok=1
            _nm_fa_log "login.html OK"
        elif curl -fsS "${curl_opts[@]}" --connect-timeout 3 --max-time 8 \
            "${scheme}://127.0.0.1:${port}/login.html" >/dev/null 2>&1; then
            ok=1
            base="$(_nm_fa_ui_url "$port" "127.0.0.1" "$scheme")"
            login_url="${base}/login.html"
            _nm_fa_log "login.html OK на 127.0.0.1:${port} (внешний IP ${ip} может быть недоступен — SG/маршрут)"
        else
            _nm_fa_log "WARNING: login.html недоступен на :${port} (${scheme})"
            _nm_fa_log "  Проверьте: grep -E 'HTTP_PORT|HTTPS_' ${project_dir}/.env; docker compose -f ${project_dir}/docker-compose.yml ps"
            if [[ "$scheme" == "https" ]]; then
                _nm_fa_log "  curl -k -I https://127.0.0.1:${port}/login.html"
            else
                _nm_fa_log "  curl -I http://127.0.0.1:${port}/login.html"
            fi
        fi
    fi

    # Попытка открыть браузер на хосте с GUI (на headless SSH обычно noop).
    if [[ "$ok" == "1" ]] && [[ -n "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ]]; then
        if command -v xdg-open >/dev/null 2>&1; then
            xdg-open "$login_url" >/dev/null 2>&1 || true
        fi
    fi

    local msg
    msg="Установка завершена.

Веб-интерфейс: ${base}
Вход:          ${login_url}

Логин:  admin
Пароль: admin
(при первом входе система попросит сменить пароль)

Health: ${health_url}"

    if [[ "$ok" != "1" ]]; then
        msg="${msg}

⚠ Страница входа сейчас не отвечает на :${port} (${scheme}).
На сервере: curl $([ "$scheme" = https ] && echo -k ) -I ${scheme}://127.0.0.1:${port}/login.html
Если заходите с другой машины — откройте TCP ${port} в Security Group / облачном фаерволе."
    fi

    echo "" >&2
    echo "══════════════════════════════════════════════════════════" >&2
    echo "$msg" >&2
    echo "══════════════════════════════════════════════════════════" >&2

    if declare -F nm_ui_msgbox >/dev/null 2>&1 && [[ -t 0 ]] && [[ "${NM_UI_BACKEND:-}" != "text" ]]; then
        nm_ui_msgbox "ГеоАтлас готов" "$msg" || true
    fi
}
