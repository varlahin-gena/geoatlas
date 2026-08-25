#!/usr/bin/env bash
# Выбор HTTP-порта веб-интерфейса (frontend / nginx).
# Вызывается ПОСЛЕ confirm_https (см. select_https.sh → цепочка).
# Использование:
#   source deploy/common/select_http_port.sh
#   confirm_http_port
#   apply_http_port /opt/geoatlas
#
# Env (CI / без TTY):
#   HTTP_PORT=8080              — без вопросов
#   GA_HTTP_PORT=8080           — алиас
#   GA_AUTO_MODULES=1           — порт 80 по умолчанию
#
# После confirm_http_port: HTTP_PORT (1–65535), GA_HTTP_PORT_CONFIRMED=1
# Если HTTPS уже включён — спрашиваем порт для HTTP (редирект / параллельный доступ).

set -Eeuo pipefail

_ga_port_log() { echo "[$(date +'%F %T')] [http-port] $*"; }

_ga_port_https_on() {
    case "${HTTPS_ENABLED:-}" in
        1|true|TRUE|yes|YES|on|ON|auto) return 0 ;;
    esac
    return 1
}

_ga_port_ensure_ui() {
    if ! declare -F ga_ui_radiolist >/dev/null 2>&1; then
        local dir helper
        dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
        helper="${dir}/ui.sh"
        if [[ -f "$helper" ]]; then
            # shellcheck source=deploy/common/ui.sh
            source "$helper"
        else
            return 1
        fi
    fi
    if [[ -z "${GA_UI_BACKEND:-}" ]] && declare -F ga_ui_init >/dev/null 2>&1; then
        ga_ui_init
    fi
    return 0
}

_ga_port_valid() {
    local p="${1:-}"
    [[ "$p" =~ ^[0-9]+$ ]] || return 1
    (( p >= 1 && p <= 65535 ))
}

_ga_port_ask_custom() {
    local def="${1:-8080}"
    local answer=""
    while true; do
        if declare -F ga_ui_inputbox >/dev/null 2>&1; then
            answer="$(ga_ui_inputbox "HTTP-порт" "Введите TCP-порт (1–65535):" "$def")" || answer=""
        else
            if [[ -r /dev/tty && -w /dev/tty ]]; then
                printf 'HTTP-порт [%s]: ' "$def" >/dev/tty
                read -r answer </dev/tty || answer=""
            else
                printf 'HTTP-порт [%s]: ' "$def" >&2
                read -r answer || answer=""
            fi
        fi
        answer="${answer//[[:space:]]/}"
        [[ -z "$answer" ]] && answer="$def"
        if _ga_port_valid "$answer"; then
            echo "$answer"
            return 0
        fi
        if declare -F ga_ui_msgbox >/dev/null 2>&1; then
            ga_ui_msgbox "HTTP-порт" \
                "Некорректный порт «${answer}». Нужно число от 1 до 65535." || true
        else
            echo "  Некорректный порт «${answer}». Нужно число от 1 до 65535." >&2
        fi
        def="$answer"
    done
}

_ga_port_set_confirmed() {
    export HTTP_PORT="$1"
    export GA_HTTP_PORT_CONFIRMED=1
}

confirm_http_port() {
    if [[ "${GA_HTTP_PORT_CONFIRMED:-0}" == "1" ]] && [[ -n "${HTTP_PORT:-}" ]]; then
        _ga_port_log "HTTP_PORT уже выбран (${HTTP_PORT}) — пропуск."
        return 0
    fi

    if [[ -n "${HTTP_PORT:-}" ]]; then
        if ! _ga_port_valid "$HTTP_PORT"; then
            _ga_port_log "ВНИМАНИЕ: HTTP_PORT=${HTTP_PORT} некорректен — используем 80."
            HTTP_PORT=80
        fi
        _ga_port_set_confirmed "$HTTP_PORT"
        _ga_port_log "Порт задан через HTTP_PORT=${HTTP_PORT}."
        return 0
    fi
    if [[ -n "${GA_HTTP_PORT:-}" ]]; then
        HTTP_PORT="$GA_HTTP_PORT"
        if ! _ga_port_valid "$HTTP_PORT"; then
            _ga_port_log "ВНИМАНИЕ: GA_HTTP_PORT=${GA_HTTP_PORT} некорректен — используем 80."
            HTTP_PORT=80
        fi
        _ga_port_set_confirmed "$HTTP_PORT"
        _ga_port_log "Порт задан через GA_HTTP_PORT=${HTTP_PORT}."
        return 0
    fi

    if [[ "${GA_AUTO_MODULES:-0}" == "1" ]] || { [[ ! -t 0 ]] && [[ ! -r /dev/tty ]]; }; then
        _ga_port_set_confirmed 80
        _ga_port_log "Нет TTY / GA_AUTO_MODULES — HTTP_PORT=80."
        return 0
    fi

    _ga_port_ensure_ui || true
    local choice="" title prompt
    if _ga_port_https_on; then
        title="HTTP-порт"
        prompt="Порт HTTP (редирект на HTTPS или параллельный доступ).
TLS уже: ${HTTPS_PORT:-443}"
    else
        title="Порт веб-интерфейса"
        prompt="На каком порту открыть UI (только HTTP)?"
    fi

    if declare -F ga_ui_radiolist >/dev/null 2>&1; then
        if ! choice="$(ga_ui_radiolist \
            "$title" \
            "$prompt" \
            80 "HTTP :80" ON \
            8080 "HTTP :8080" OFF \
            custom "Указать вручную" OFF)"; then
            _ga_port_log "Установка отменена пользователем."
            exit 0
        fi
    else
        choice="80"
        _ga_port_log "UI недоступен — HTTP_PORT=80."
    fi

    case "${choice}" in
        custom|c|ручн*|other|manual)
            HTTP_PORT="$(_ga_port_ask_custom 8080)"
            ;;
        *)
            if _ga_port_valid "$choice"; then
                HTTP_PORT="$choice"
            else
                _ga_port_log "ВНИМАНИЕ: неизвестный выбор «${choice}» — HTTP_PORT=80."
                HTTP_PORT=80
            fi
            ;;
    esac
    _ga_port_set_confirmed "$HTTP_PORT"
    _ga_port_log "Выбран HTTP_PORT=${HTTP_PORT}"
    if declare -F ga_ui_msgbox >/dev/null 2>&1 && [[ "${GA_UI_BACKEND:-text}" != "text" ]]; then
        local summary="HTTP: порт ${HTTP_PORT}."
        if _ga_port_https_on; then
            summary="${summary}
HTTPS: порт ${HTTPS_PORT:-443} (TLS)."
        fi
        ga_ui_msgbox "$title" "$summary" || true
    fi
}

# Записать HTTP_PORT в .env (не затирая остальные ключи).
apply_http_port() {
    local project_dir="${1:-.}"
    local env_file="${project_dir}/.env"
    local port="${HTTP_PORT:-80}"
    local tmp

    if ! _ga_port_valid "$port"; then
        port=80
        HTTP_PORT=80
    fi

    mkdir -p "$project_dir"
    touch "$env_file"

    tmp="$(mktemp)"
    grep -vE '^[[:space:]]*(# --- HTTP port \(select_http_port\.sh\) ---|HTTP_PORT=)' \
        "$env_file" >"$tmp" || true
    while [[ -s "$tmp" ]] && [[ -z "$(tail -n1 "$tmp")" ]]; do
        head -n -1 "$tmp" >"${tmp}.2" && mv "${tmp}.2" "$tmp"
    done
    cat >>"$tmp" <<EOF

# --- HTTP port (select_http_port.sh) ---
HTTP_PORT=${port}
EOF
    mv "$tmp" "$env_file"
    export HTTP_PORT="$port"
    _ga_port_log "Записан HTTP_PORT=${port} → ${env_file}"
}
