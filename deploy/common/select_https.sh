#!/usr/bin/env bash
# Выбор HTTPS для веб-интерфейса (nginx TLS, свои PEM).
# Использование:
#   source deploy/common/select_https.sh
#   confirm_https /opt/network-monitor
#   apply_https /opt/network-monitor
#
# Env (без вопросов / CI):
#   HTTPS_ENABLED=1|0|auto
#   NM_HTTPS_ENABLED=1|0|auto     — алиас
#   HTTPS_PORT=443
#   NM_HTTPS_PORT=443
#   HTTP_REDIRECT=1|0
#   NM_HTTP_REDIRECT=1|0
#   NM_SSL_CERT_SRC=/path/fullchain.pem
#   NM_SSL_KEY_SRC=/path/privkey.pem
#   NM_CERTS_DIR=/path/to/dir     — копирует fullchain.pem+privkey.pem (или cert.pem+key.pem)
#
# Важно: шаг задаётся и в пошаговой установке, и при NM_FULL_AUTO=1
# (если есть TTY и HTTPS_ENABLED / NM_HTTPS_ENABLED ещё не заданы).

set -Eeuo pipefail

_nm_https_log() { echo "[$(date +'%F %T')] [https] $*"; }

_nm_https_ensure_ui() {
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

_nm_https_truthy() {
    case "${1:-}" in
        1|true|TRUE|yes|YES|on|ON) return 0 ;;
        *) return 1 ;;
    esac
}

_nm_https_falsy() {
    case "${1:-}" in
        0|false|FALSE|no|NO|off|OFF) return 0 ;;
        *) return 1 ;;
    esac
}

_nm_https_port_valid() {
    local p="${1:-}"
    [[ "$p" =~ ^[0-9]+$ ]] || return 1
    (( p >= 1 && p <= 65535 ))
}

_nm_https_certs_present() {
    local root="${1:-.}"
    [[ -f "${root}/certs/fullchain.pem" && -f "${root}/certs/privkey.pem" ]]
}

_nm_https_ask_port() {
    local def="${1:-443}"
    local answer=""
    while true; do
        if declare -F nm_ui_inputbox >/dev/null 2>&1; then
            answer="$(nm_ui_inputbox "HTTPS-порт" "TCP-порт TLS (1–65535):" "$def")" || answer=""
        else
            if [[ -r /dev/tty && -w /dev/tty ]]; then
                printf 'HTTPS-порт [%s]: ' "$def" >/dev/tty
                read -r answer </dev/tty || answer=""
            else
                printf 'HTTPS-порт [%s]: ' "$def" >&2
                read -r answer || answer=""
            fi
        fi
        answer="${answer//[[:space:]]/}"
        [[ -z "$answer" ]] && answer="$def"
        if _nm_https_port_valid "$answer"; then
            echo "$answer"
            return 0
        fi
        if declare -F nm_ui_msgbox >/dev/null 2>&1; then
            nm_ui_msgbox "HTTPS-порт" \
                "Некорректный порт «${answer}». Нужно число от 1 до 65535." || true
        else
            echo "  Некорректный порт «${answer}»." >&2
        fi
        def="$answer"
    done
}

# Копирует PEM в $project_dir/certs/{fullchain,privkey}.pem
# Возвращает 0 при успехе.
_nm_https_install_certs() {
    local project_dir="$1"
    local cert_src="${2:-}"
    local key_src="${3:-}"
    local dest="${project_dir}/certs"

    mkdir -p "$dest"

    if [[ -z "$cert_src" && -n "${NM_CERTS_DIR:-}" ]]; then
        local d="${NM_CERTS_DIR}"
        if [[ -f "${d}/fullchain.pem" && -f "${d}/privkey.pem" ]]; then
            cert_src="${d}/fullchain.pem"
            key_src="${d}/privkey.pem"
        elif [[ -f "${d}/cert.pem" && -f "${d}/key.pem" ]]; then
            cert_src="${d}/cert.pem"
            key_src="${d}/key.pem"
        elif [[ -f "${d}/certificate.crt" && -f "${d}/private.key" ]]; then
            cert_src="${d}/certificate.crt"
            key_src="${d}/private.key"
        fi
    fi

    if [[ -z "$cert_src" || -z "$key_src" ]]; then
        return 1
    fi
    if [[ ! -f "$cert_src" || ! -f "$key_src" ]]; then
        _nm_https_log "WARNING: файлы сертификата не найдены: ${cert_src} / ${key_src}"
        return 1
    fi

    cp -f "$cert_src" "${dest}/fullchain.pem"
    cp -f "$key_src" "${dest}/privkey.pem"
    chmod 644 "${dest}/fullchain.pem" 2>/dev/null || true
    chmod 600 "${dest}/privkey.pem" 2>/dev/null || true
    _nm_https_log "Сертификаты скопированы → ${dest}/"
    return 0
}

_nm_https_ask_cert_paths() {
    local project_dir="$1"
    local cert_src="" key_src=""

    if [[ -n "${NM_SSL_CERT_SRC:-}" && -n "${NM_SSL_KEY_SRC:-}" ]]; then
        _nm_https_install_certs "$project_dir" "$NM_SSL_CERT_SRC" "$NM_SSL_KEY_SRC" && return 0
    fi
    if [[ -n "${NM_CERTS_DIR:-}" ]]; then
        _nm_https_install_certs "$project_dir" && return 0
    fi

    if declare -F nm_ui_inputbox >/dev/null 2>&1; then
        cert_src="$(nm_ui_inputbox "Сертификат" \
            "Путь к fullchain.pem (или сертификату с цепочкой):" \
            "${NM_SSL_CERT_SRC:-}")" || cert_src=""
        key_src="$(nm_ui_inputbox "Ключ" \
            "Путь к privkey.pem (приватный ключ):" \
            "${NM_SSL_KEY_SRC:-}")" || key_src=""
    else
        if [[ -r /dev/tty && -w /dev/tty ]]; then
            printf 'Путь к fullchain.pem: ' >/dev/tty
            read -r cert_src </dev/tty || cert_src=""
            printf 'Путь к privkey.pem: ' >/dev/tty
            read -r key_src </dev/tty || key_src=""
        else
            printf 'Путь к fullchain.pem: ' >&2
            read -r cert_src || cert_src=""
            printf 'Путь к privkey.pem: ' >&2
            read -r key_src || key_src=""
        fi
    fi

    cert_src="${cert_src#"${cert_src%%[![:space:]]*}"}"
    cert_src="${cert_src%"${cert_src##*[![:space:]]}"}"
    key_src="${key_src#"${key_src%%[![:space:]]*}"}"
    key_src="${key_src%"${key_src##*[![:space:]]}"}"

    if [[ -z "$cert_src" || -z "$key_src" ]]; then
        return 1
    fi
    _nm_https_install_certs "$project_dir" "$cert_src" "$key_src"
}

_nm_https_prompt_certs_when_enabled() {
    local project_dir="$1"
    local choice=""

    if _nm_https_certs_present "$project_dir"; then
        _nm_https_log "PEM уже есть в ${project_dir}/certs/"
        return 0
    fi

    # Env copy without UI
    if [[ -n "${NM_SSL_CERT_SRC:-}${NM_SSL_KEY_SRC:-}${NM_CERTS_DIR:-}" ]]; then
        if _nm_https_install_certs "$project_dir" "${NM_SSL_CERT_SRC:-}" "${NM_SSL_KEY_SRC:-}"; then
            return 0
        fi
    fi

    if [[ ! -t 0 ]] && [[ "${NM_UI_BACKEND:-}" == "text" || -z "${NM_UI_BACKEND:-}" ]]; then
        # Нет TTY — оставляем без файлов; start.sh/nginx предупредит.
        _nm_https_log "WARNING: HTTPS включён, но PEM нет. Положите certs/fullchain.pem и privkey.pem до старта."
        return 0
    fi

    _nm_https_ensure_ui || true

    if declare -F nm_ui_radiolist >/dev/null 2>&1; then
        if ! choice="$(nm_ui_radiolist \
            "Сертификаты TLS" \
            "HTTPS включён. Где взять PEM?" \
            later "Положить вручную в certs/ позже" ON \
            copy "Указать пути и скопировать сейчас" OFF \
            dir "Каталог с fullchain.pem + privkey.pem" OFF)"; then
            _nm_https_log "Установка отменена пользователем."
            exit 0
        fi
    else
        choice="later"
    fi

    case "$choice" in
        copy)
            if ! _nm_https_ask_cert_paths "$project_dir"; then
                if declare -F nm_ui_msgbox >/dev/null 2>&1; then
                    nm_ui_msgbox "HTTPS" \
                        "Не удалось скопировать сертификаты. Положите PEM в ${project_dir}/certs/ до запуска стека." || true
                fi
                _nm_https_log "WARNING: копирование не удалось — ожидаются файлы в certs/"
            fi
            ;;
        dir)
            local d=""
            if declare -F nm_ui_inputbox >/dev/null 2>&1; then
                d="$(nm_ui_inputbox "Каталог сертификатов" \
                    "Каталог с fullchain.pem и privkey.pem:" \
                    "${NM_CERTS_DIR:-}")" || d=""
            else
                printf 'Каталог с PEM: ' >&2
                read -r d || d=""
            fi
            if [[ -n "$d" ]]; then
                NM_CERTS_DIR="$d"
                export NM_CERTS_DIR
                if ! _nm_https_install_certs "$project_dir"; then
                    _nm_https_log "WARNING: в каталоге нет ожидаемых PEM"
                fi
            fi
            ;;
        *)
            if declare -F nm_ui_msgbox >/dev/null 2>&1 && [[ "${NM_UI_BACKEND:-text}" != "text" ]]; then
                nm_ui_msgbox "HTTPS" \
                    "Перед запуском положите:
  ${project_dir}/certs/fullchain.pem
  ${project_dir}/certs/privkey.pem

Подробнее: certs/README.md" || true
            else
                _nm_https_log "Положите PEM в ${project_dir}/certs/ до ./start.sh"
            fi
            ;;
    esac
}

# $1 — project dir (для проверки/копирования certs)
confirm_https() {
    local project_dir="${1:-.}"
    local enabled="" choice=""

    mkdir -p "${project_dir}/certs"

    # Уже задано через env — без вопросов (CI / повторный прогон).
    if [[ -n "${HTTPS_ENABLED:-}" ]]; then
        enabled="$HTTPS_ENABLED"
    elif [[ -n "${NM_HTTPS_ENABLED:-}" ]]; then
        enabled="$NM_HTTPS_ENABLED"
    fi

    if [[ -n "$enabled" ]]; then
        HTTPS_ENABLED="$enabled"
        export HTTPS_ENABLED
        HTTPS_PORT="${HTTPS_PORT:-${NM_HTTPS_PORT:-443}}"
        HTTP_REDIRECT="${HTTP_REDIRECT:-${NM_HTTP_REDIRECT:-1}}"
        if ! _nm_https_port_valid "${HTTPS_PORT}"; then
            HTTPS_PORT=443
        fi
        export HTTPS_PORT HTTP_REDIRECT
        _nm_https_log "HTTPS_ENABLED=${HTTPS_ENABLED} (из env), порт ${HTTPS_PORT}"
        if _nm_https_truthy "$HTTPS_ENABLED" || [[ "$HTTPS_ENABLED" == "auto" ]]; then
            _nm_https_prompt_certs_when_enabled "$project_dir"
        fi
        return 0
    fi

    # Без TTY (CI / pipe) — не спрашиваем. При TTY спрашиваем всегда,
    # в том числе при NM_FULL_AUTO=1 («Сделай мне хорошо»).
    if [[ ! -t 0 ]]; then
        if _nm_https_certs_present "$project_dir"; then
            HTTPS_ENABLED=auto
        else
            HTTPS_ENABLED=0
        fi
        HTTPS_PORT="${HTTPS_PORT:-${NM_HTTPS_PORT:-443}}"
        HTTP_REDIRECT="${HTTP_REDIRECT:-${NM_HTTP_REDIRECT:-1}}"
        export HTTPS_ENABLED HTTPS_PORT HTTP_REDIRECT
        _nm_https_log "Нет TTY — HTTPS_ENABLED=${HTTPS_ENABLED}"
        if _nm_https_truthy "$HTTPS_ENABLED" || [[ "$HTTPS_ENABLED" == "auto" ]]; then
            _nm_https_prompt_certs_when_enabled "$project_dir"
        fi
        return 0
    fi

    # Явный шаг: пошаговая установка и «Сделай мне хорошо».
    _nm_https_ensure_ui || true
    local default_on="OFF" default_off="ON"
    if _nm_https_certs_present "$project_dir"; then
        default_on="ON"
        default_off="OFF"
    fi

    if declare -F nm_ui_radiolist >/dev/null 2>&1; then
        if ! choice="$(nm_ui_radiolist \
            "HTTPS" \
            "Включить HTTPS для веб-интерфейса (свои PEM-сертификаты)?" \
            off "Только HTTP" "$default_off" \
            on "HTTPS (TLS на nginx)" "$default_on" \
            auto "Авто: HTTPS если PEM уже в certs/" OFF)"; then
            _nm_https_log "Установка отменена пользователем."
            exit 0
        fi
    else
        if _nm_https_certs_present "$project_dir"; then
            choice="auto"
        else
            choice="off"
        fi
        _nm_https_log "UI недоступен — HTTPS_ENABLED=${choice}"
    fi

    case "$choice" in
        on|1|yes|enable)
            HTTPS_ENABLED=1
            ;;
        auto)
            HTTPS_ENABLED=auto
            ;;
        *)
            HTTPS_ENABLED=0
            ;;
    esac
    export HTTPS_ENABLED

    HTTPS_PORT="${HTTPS_PORT:-${NM_HTTPS_PORT:-443}}"
    HTTP_REDIRECT="${HTTP_REDIRECT:-${NM_HTTP_REDIRECT:-1}}"

    if _nm_https_truthy "$HTTPS_ENABLED" || [[ "$HTTPS_ENABLED" == "auto" ]]; then
        if [[ -t 0 ]] || declare -F nm_ui_inputbox >/dev/null 2>&1; then
            if declare -F nm_ui_yesno >/dev/null 2>&1; then
                if ! nm_ui_yesno "HTTPS-порт" \
                    "Использовать порт ${HTTPS_PORT} для HTTPS?
(Нет — указать другой)" 1; then
                    HTTPS_PORT="$(_nm_https_ask_port "$HTTPS_PORT")"
                fi
                if nm_ui_yesno "Редирект HTTP→HTTPS" \
                    "Перенаправлять HTTP на HTTPS?" 1; then
                    HTTP_REDIRECT=1
                else
                    HTTP_REDIRECT=0
                fi
            else
                HTTPS_PORT="$(_nm_https_ask_port "$HTTPS_PORT")"
            fi
        fi
        if ! _nm_https_port_valid "$HTTPS_PORT"; then
            HTTPS_PORT=443
        fi
        export HTTPS_PORT HTTP_REDIRECT
        _nm_https_prompt_certs_when_enabled "$project_dir"
    else
        export HTTPS_PORT HTTP_REDIRECT
    fi

    _nm_https_log "Выбрано: HTTPS_ENABLED=${HTTPS_ENABLED}, HTTPS_PORT=${HTTPS_PORT}, HTTP_REDIRECT=${HTTP_REDIRECT}"
    if declare -F nm_ui_msgbox >/dev/null 2>&1 && [[ "${NM_UI_BACKEND:-text}" != "text" ]]; then
        local summary
        if _nm_https_falsy "$HTTPS_ENABLED"; then
            summary="HTTPS выключен. UI по HTTP (порт ${HTTP_PORT:-80})."
        else
            summary="HTTPS: ${HTTPS_ENABLED}
Порт TLS: ${HTTPS_PORT}
Редирект HTTP→HTTPS: ${HTTP_REDIRECT}
Сертификаты: ${project_dir}/certs/"
        fi
        nm_ui_msgbox "HTTPS" "$summary" || true
    fi
}

# Записать HTTPS_* в .env (не затирая остальные ключи).
apply_https() {
    local project_dir="${1:-.}"
    local env_file="${project_dir}/.env"
    local enabled="${HTTPS_ENABLED:-auto}"
    local port="${HTTPS_PORT:-443}"
    local redirect="${HTTP_REDIRECT:-1}"
    local tmp

    if ! _nm_https_port_valid "$port"; then
        port=443
        HTTPS_PORT=443
    fi

    mkdir -p "${project_dir}/certs"
    touch "$env_file"

    tmp="$(mktemp)"
    grep -vE '^[[:space:]]*(# --- HTTPS \(select_https\.sh\) ---|HTTPS_ENABLED=|HTTPS_PORT=|HTTP_REDIRECT=)' \
        "$env_file" >"$tmp" || true
    # Убрать дубли из блока detect_resources (он тоже пишет эти ключи).
    while [[ -s "$tmp" ]] && [[ -z "$(tail -n1 "$tmp")" ]]; do
        head -n -1 "$tmp" >"${tmp}.2" && mv "${tmp}.2" "$tmp"
    done
    cat >>"$tmp" <<EOF

# --- HTTPS (select_https.sh) ---
HTTPS_ENABLED=${enabled}
HTTPS_PORT=${port}
HTTP_REDIRECT=${redirect}
EOF
    mv "$tmp" "$env_file"
    export HTTPS_ENABLED="$enabled" HTTPS_PORT="$port" HTTP_REDIRECT="$redirect"
    _nm_https_log "Записано HTTPS_ENABLED=${enabled} HTTPS_PORT=${port} → ${env_file}"
}
