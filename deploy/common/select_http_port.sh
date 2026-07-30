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
    if declare -F nm_ui_radiolist >/dev/null 2>&1; then
        return 0
    fi
    local dir helper
    dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
    helper="${dir}/ui.sh"
    if [[ -f "$helper" ]]; then
        # shellcheck source=deploy/common/ui.sh
        source "$helper"
        return 0
    fi
    return 1
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
        if command -v whiptail >/dev/null 2>&1; then
            answer="$(whiptail --backtitle "ГеоАтлас" --title "Порт веб-интерфейса" \
                --inputbox "Введите TCP-порт (1–65535):" 10 50 "$def" \
                3>&1 1>&2 2>&3)" || answer=""
        elif declare -F nm_ui_yesno >/dev/null 2>&1; then
            # text fallback via tty
            if [[ -r /dev/tty && -w /dev/tty ]]; then
                printf 'Порт веб-интерфейса [%s]: ' "$def" >/dev/tty
                read -r answer </dev/tty || answer=""
            else
                printf 'Порт веб-интерфейса [%s]: ' "$def" >&2
                read -r answer || answer=""
            fi
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
        echo "  Некорректный порт «${answer}». Нужно число от 1 до 65535." >&2
        def="$answer"
    done
}

confirm_http_port() {
    # Уже задан снаружи
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
    local eighty_on=ON p8080_on=OFF p443_on=OFF p8443_on=OFF custom_on=OFF

    if declare -F nm_ui_radiolist >/dev/null 2>&1; then
        choice="$(nm_ui_radiolist \
            "Порт веб-интерфейса" \
            "На каком порту открыть UI (nginx)?" \
            80 "Стандартный HTTP (:80)" "$eighty_on" \
            8080 "Альтернатива (:8080)" "$p8080_on" \
            443 "HTTPS-порт без TLS на nginx (:443)" "$p443_on" \
            8443 "Альтернатива (:8443)" "$p8443_on" \
            custom "Указать вручную…" "$custom_on")" || choice="80"
    elif command -v whiptail >/dev/null 2>&1; then
        choice="$(whiptail --backtitle "ГеоАтлас" --title "Порт веб-интерфейса" \
            --radiolist "На каком порту открыть UI (nginx)?" 16 70 5 \
            80 "Стандартный HTTP (:80)" ON \
            8080 "Альтернатива (:8080)" OFF \
            443 "HTTPS-порт без TLS на nginx (:443)" OFF \
            8443 "Альтернатива (:8443)" OFF \
            custom "Указать вручную…" OFF \
            3>&1 1>&2 2>&3)" || choice="80"
        choice="${choice//\"/}"
        choice="${choice%%[[:space:]]*}"
    else
        echo "" >&2
        echo "══════════════════════════════════════════════════════════" >&2
        echo "  Порт веб-интерфейса" >&2
        echo "══════════════════════════════════════════════════════════" >&2
        echo "  [80]     стандартный HTTP" >&2
        echo "  [8080]   альтернатива" >&2
        echo "  [443]    порт 443 (TLS на nginx не настраивается)" >&2
        echo "  [8443]   альтернатива" >&2
        echo "  [custom] указать вручную" >&2
        echo "" >&2
        local answer
        if [[ -r /dev/tty && -w /dev/tty ]]; then
            printf 'Ваш выбор [80]: ' >/dev/tty
            read -r answer </dev/tty || answer=""
        else
            printf 'Ваш выбор [80]: ' >&2
            read -r answer || answer=""
        fi
        answer="${answer,,}"
        answer="${answer//[[:space:]]/}"
        [[ -z "$answer" ]] && answer="80"
        choice="$answer"
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
