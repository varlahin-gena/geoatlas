#!/usr/bin/env bash
# Выбор порта веб-интерфейса (frontend / nginx).
# Использование:
#   source deploy/common/select_http_port.sh
#   confirm_http_port
#   apply_http_port /opt/network-monitor
#
# Env (CI / без TTY):
#   HTTP_PORT=8080              — без вопросов
#   NM_HTTP_PORT=8080           — алиас
#   NM_AUTO_MODULES=1           — порт 80 по умолчанию
#
# После confirm_http_port: HTTP_PORT (1–65535)

set -Eeuo pipefail

_nm_port_log() { echo "[$(date +'%F %T')] [http-port] $*"; }

_nm_port_ensure_ui() {
    if ! declare -F nm_ui_radiolist >/dev/null 2>&1; then
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
    if [[ -z "${NM_UI_BACKEND:-}" ]] && declare -F nm_ui_init >/dev/null 2>&1; then
        nm_ui_init
    fi
    return 0
}

_nm_port_valid() {
    local p="${1:-}"
    [[ "$p" =~ ^[0-9]+$ ]] || return 1
    (( p >= 1 && p <= 65535 ))
}

_nm_port_ask_custom() {
    local def="${1:-8080}"
    local answer=""
    while true; do
        if declare -F nm_ui_inputbox >/dev/null 2>&1; then
            answer="$(nm_ui_inputbox "Порт веб-интерфейса" "Введите TCP-порт (1–65535):" "$def")" || answer=""
        else
            if [[ -r /dev/tty && -w /dev/tty ]]; then
                printf 'Порт веб-интерфейса [%s]: ' "$def" >/dev/tty
                read -r answer </dev/tty || answer=""
            else
                printf 'Порт веб-интерфейса [%s]: ' "$def" >&2
                read -r answer || answer=""
            fi
        fi
        answer="${answer//[[:space:]]/}"
        [[ -z "$answer" ]] && answer="$def"
        if _nm_port_valid "$answer"; then
            echo "$answer"
            return 0
        fi
        if declare -F nm_ui_msgbox >/dev/null 2>&1; then
            nm_ui_msgbox "Порт веб-интерфейса" \
                "Некорректный порт «${answer}». Нужно число от 1 до 65535." || true
        else
            echo "  Некорректный порт «${answer}». Нужно число от 1 до 65535." >&2
        fi
        def="$answer"
    done
}

confirm_http_port() {
    if [[ -n "${HTTP_PORT:-}" ]]; then
        if ! _nm_port_valid "$HTTP_PORT"; then
            _nm_port_log "WARNING: HTTP_PORT=${HTTP_PORT} некорректен — используем 80."
            HTTP_PORT=80
        fi
        export HTTP_PORT
        _nm_port_log "Порт задан через HTTP_PORT=${HTTP_PORT}."
        return 0
    fi
    if [[ -n "${NM_HTTP_PORT:-}" ]]; then
        HTTP_PORT="$NM_HTTP_PORT"
        if ! _nm_port_valid "$HTTP_PORT"; then
            _nm_port_log "WARNING: NM_HTTP_PORT=${NM_HTTP_PORT} некорректен — используем 80."
            HTTP_PORT=80
        fi
        export HTTP_PORT
        _nm_port_log "Порт задан через NM_HTTP_PORT=${HTTP_PORT}."
        return 0
    fi

    if [[ "${NM_AUTO_MODULES:-0}" == "1" ]] || [[ ! -t 0 ]]; then
        HTTP_PORT=80
        export HTTP_PORT
        _nm_port_log "Нет TTY / NM_AUTO_MODULES — HTTP_PORT=80."
        return 0
    fi

    _nm_port_ensure_ui || true
    local choice=""

    if declare -F nm_ui_radiolist >/dev/null 2>&1; then
        choice="$(nm_ui_radiolist \
            "Порт веб-интерфейса" \
            "На каком порту открыть UI (nginx)?" \
            80 "Стандартный HTTP (:80)" ON \
            8080 "Альтернатива (:8080)" OFF \
            443 "HTTPS-порт без TLS на nginx (:443)" OFF \
            8443 "Альтернатива (:8443)" OFF \
            custom "Указать вручную…" OFF)" || choice="80"
    else
        choice="80"
        _nm_port_log "UI недоступен — HTTP_PORT=80."
    fi

    case "${choice}" in
        custom|c|ручн*|other|manual)
            HTTP_PORT="$(_nm_port_ask_custom 8080)"
            ;;
        *)
            if _nm_port_valid "$choice"; then
                HTTP_PORT="$choice"
            else
                _nm_port_log "WARNING: неизвестный выбор «${choice}» — HTTP_PORT=80."
                HTTP_PORT=80
            fi
            ;;
    esac
    export HTTP_PORT
    _nm_port_log "Выбран порт веб-интерфейса: ${HTTP_PORT}"
    if declare -F nm_ui_msgbox >/dev/null 2>&1 && [[ "${NM_UI_BACKEND:-text}" != "text" ]]; then
        nm_ui_msgbox "Порт веб-интерфейса" \
            "Веб-интерфейс будет слушать порт ${HTTP_PORT}." || true
    fi
}

# Записать HTTP_PORT в .env (не затирая остальные ключи).
apply_http_port() {
    local project_dir="${1:-.}"
    local env_file="${project_dir}/.env"
    local port="${HTTP_PORT:-80}"
    local tmp

    if ! _nm_port_valid "$port"; then
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
    _nm_port_log "Записан HTTP_PORT=${port} → ${env_file}"
}
