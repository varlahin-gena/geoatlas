#!/usr/bin/env bash
# Мастер «доступ к веб-интерфейсу»: HTTPS → порты → сертификаты.
# Использование:
#   source deploy/common/select_ui_access.sh
#   confirm_ui_access /opt/geoatlas
#
# Только выставляет переменные (HTTPS_*, HTTP_PORT, GA_*_CONFIRMED).
# Запись в .env — в финале установки (write_env_file / finalize).

set -Eeuo pipefail

_ga_ui_access_log() { echo "[$(date +'%F %T')] [ui-access] $*"; }

_ga_ui_access_dir() {
    cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd
}

_ga_ui_access_source() {
    local name="$1"
    local dir f
    dir="$(_ga_ui_access_dir)"
    f="${dir}/${name}"
    if [[ -f "$f" ]]; then
        # shellcheck source=/dev/null
        source "$f"
        return 0
    fi
    return 1
}

# $1 — каталог проекта
confirm_ui_access() {
    local project_dir="${1:-.}"

    export GA_SKIP_HTTPS_HTTP_CHAIN=1
    export GA_INSTALL_QUIET_DIALOGS=1

    if ! _ga_ui_access_source select_https.sh; then
        _ga_ui_access_log "select_https.sh не найден — HTTPS_ENABLED=0, HTTP_PORT=80"
        HTTPS_ENABLED=0
        HTTP_PORT="${HTTP_PORT:-80}"
        export HTTPS_ENABLED HTTP_PORT
        export GA_HTTPS_CONFIRMED=1 GA_HTTP_PORT_CONFIRMED=1
        return 0
    fi

    confirm_https "$project_dir"

    if ! declare -F confirm_http_port >/dev/null 2>&1; then
        _ga_ui_access_source select_http_port.sh || true
    fi
    if declare -F confirm_http_port >/dev/null 2>&1; then
        confirm_http_port
    else
        HTTP_PORT="${HTTP_PORT:-80}"
        export HTTP_PORT
        export GA_HTTP_PORT_CONFIRMED=1
    fi

    _ga_ui_access_log "Доступ к UI: HTTPS_ENABLED=${HTTPS_ENABLED:-0} HTTPS_PORT=${HTTPS_PORT:-443} HTTP_PORT=${HTTP_PORT:-80}"
}
