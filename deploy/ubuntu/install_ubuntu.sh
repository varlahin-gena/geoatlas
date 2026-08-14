#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_DIR="/opt/network-monitor"
REPO_URL="${REPO_URL:-https://github.com/varlahin-gena/network_monitor.git}"
# BRANCH: если задан снаружи — не спрашиваем источник; иначе default main до confirm_install_source.
if [[ -n "${BRANCH+x}" ]]; then
    NM_BRANCH_FROM_ENV=1
else
    NM_BRANCH_FROM_ENV=0
    BRANCH="main"
fi

# Firewall: если ENABLE_UFW задан снаружи до запуска — не спрашиваем.
if [[ -n "${ENABLE_UFW+x}" ]]; then
    NM_FIREWALL_FROM_ENV=1
else
    NM_FIREWALL_FROM_ENV=0
    ENABLE_UFW=1
fi
UFW_AUTO_ENABLE="${UFW_AUTO_ENABLE:-0}"   # 1 = автоматически включить UFW, если он выключен

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"

log() { echo "[$(date +'%F %T')] $*"; }

trap 'log "ОШИБКА на строке ${LINENO} (код выхода $?). Смотрите лог выше / docker compose logs."' ERR

_nm_banner_text() {
    local src_line
    if [[ -n "${NM_INSTALL_SOURCE:-}" ]]; then
        src_line="Источник: ${NM_INSTALL_SOURCE} → ${BRANCH}"
    else
        src_line="Ссылка: ${BRANCH} (выбор источника — после Docker)"
    fi
    cat <<EOF
Каталог: ${PROJECT_DIR}
${src_line}
Репозиторий: ${REPO_URL}

Шаги:
  1. Пакеты и Docker
  2. Источник (релиз / main)
  3. Клонирование репозитория
  4. Выбор модулей
  5. HTTPS
  6. Порт веб-интерфейса
  7. Профиль производительности
  8. Файрвол
  9. Запуск стека
EOF
}

print_banner() {
    local text
    text="$(_nm_banner_text)"
    echo ""
    echo "══════════════════════════════════════════════════════════"
    echo "  ГеоАтлас — установка (Ubuntu)"
    echo "══════════════════════════════════════════════════════════"
    echo "$text" | sed 's/^/  /'
    echo "══════════════════════════════════════════════════════════"
    echo ""
}

_nm_run_gauge() {
    local title="$1" text="$2"
    shift 2
    if declare -F nm_ui_run_with_gauge >/dev/null 2>&1; then
        nm_ui_run_with_gauge "$title" "$text" "$@"
    else
        "$@"
    fi
}

_nm_run_gauge_fn() {
    local title="$1" text="$2" fn="$3"
    if declare -F nm_ui_run_with_gauge >/dev/null 2>&1; then
        # Функция в том же интерпретаторе (не bash -c) — доступны log/PROJECT_DIR.
        nm_ui_run_with_gauge "$title" "$text" "$fn"
    else
        "$fn"
    fi
}

_nm_github_owner() {
    local owner
    owner="$(echo "$REPO_URL" | sed -E 's#.*github.com[:/]([^/]+/[^/.]+)(\.git)?$#\1#')"
    [[ -n "$owner" ]] || owner="varlahin-gena/network_monitor"
    echo "$owner"
}

_nm_source_common() {
    # $1 — имя файла в deploy/common (например ui.sh)
    local name="$1"
    local candidates=(
        "${PROJECT_DIR}/deploy/common/${name}"
        "${SCRIPT_DIR}/../common/${name}"
    )
    local c
    for c in "${candidates[@]}"; do
        if [[ -f "$c" ]]; then
            # shellcheck source=/dev/null
            source "$c"
            return 0
        fi
    done
    if command -v curl >/dev/null 2>&1; then
        local tmp owner
        owner="$(_nm_github_owner)"
        tmp="$(mktemp)"
        if curl -fsSL --connect-timeout 10 --max-time 30 \
            "https://raw.githubusercontent.com/${owner}/main/deploy/common/${name}" \
            -o "$tmp"; then
            # shellcheck source=/dev/null
            source "$tmp"
            rm -f "$tmp"
            return 0
        fi
        rm -f "$tmp"
    fi
    return 1
}

_source_ui() {
    if ! _nm_source_common ui.sh; then
        return 1
    fi
    nm_ui_init
    return 0
}

_source_full_auto_preset() {
    _nm_source_common full_auto_preset.sh
}

# Тёмная тема whiptail до появления ui.sh (curl-установка / welcome).
_ensure_dark_newt() {
    if [[ "${NM_UI_DARK:-1}" == "0" ]]; then
        return 0
    fi
    if [[ -n "${NEWT_COLORS:-}" ]]; then
        return 0
    fi
    export NEWT_COLORS='root=white,black
border=brightcyan,black
window=white,black
shadow=black,black
title=brightcyan,black
button=black,brightcyan
actbutton=brightcyan,black
compactbutton=white,black
checkbox=white,black
actcheckbox=black,brightcyan
entry=white,black
label=white,black
listbox=white,black
actlistbox=brightcyan,black
sellistbox=black,white
actsellistbox=black,brightcyan
textbox=white,black
acttextbox=black,brightcyan
helpline=black,lightgray
roottext=lightgray,black
emptyscale=black,lightgray
disabledentry=black,lightgray
scale=white,black'
}

_welcome_dialog() {
    # Уже выбран авто-режим (env / --full-auto / нет TTY) — без диалога.
    if [[ "${NM_FULL_AUTO:-0}" == "1" ]] || [[ "${NM_AUTO_MODULES:-0}" == "1" ]] || [[ ! -t 0 ]]; then
        return 0
    fi
    _ensure_dark_newt
    _source_ui || true
    if ! declare -F nm_ui_radiolist >/dev/null 2>&1; then
        if declare -F nm_ui_msgbox >/dev/null 2>&1; then
            nm_ui_msgbox "Установка ГеоАтлас" \
"Добро пожаловать в мастер установки.

Каталог: ${PROJECT_DIR}
Репозиторий: ${REPO_URL}

Далее: Docker, выбор релиза или main, модули и запуск." || true
        fi
        return 0
    fi

    # Короткий tag+item: длинные UTF-8 строки в radiolist ломают рамку newt/whiptail.
    local mode
    local _ui_w="${NM_UI_WIDTH:-72}" _ui_h="${NM_UI_HEIGHT:-18}" _ui_lh="${NM_UI_LIST_HEIGHT:-8}"
    NM_UI_WIDTH=64
    NM_UI_HEIGHT=14
    NM_UI_LIST_HEIGHT=3
    if ! mode="$(nm_ui_radiolist "Установка ГеоАтлас" \
"Режим установки

auto — релиз, модули, :8080, UFW ports
step — спросить каждый шаг

${PROJECT_DIR}" \
        auto "Сделай мне хорошо" ON \
        step "Пошаговая" OFF \
    )"; then
        NM_UI_WIDTH="$_ui_w"
        NM_UI_HEIGHT="$_ui_h"
        NM_UI_LIST_HEIGHT="$_ui_lh"
        log "Установка отменена пользователем."
        exit 0
    fi
    NM_UI_WIDTH="$_ui_w"
    NM_UI_HEIGHT="$_ui_h"
    NM_UI_LIST_HEIGHT="$_ui_lh"

    if [[ "$mode" == "auto" || "$mode" == "full_auto" ]]; then
        export NM_FULL_AUTO=1
        if declare -F nm_apply_full_auto_preset >/dev/null 2>&1; then
            nm_apply_full_auto_preset
        fi
    fi
}

require_root() {
    if [[ $EUID -ne 0 ]]; then
        echo "Запустите от имени root."
        exit 1
    fi
}

detect_ubuntu() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        if [[ "${ID:-}" != "ubuntu" ]]; then
            log "Внимание: скрипт рассчитан на Ubuntu (обнаружено: ${ID:-unknown})."
        fi
        log "Обнаружена ОС: ${PRETTY_NAME:-unknown}"
    fi
}

install_packages() {
    log "Обновление списков пакетов..."
    apt-get update

    log "Установка зависимостей..."
    DEBIAN_FRONTEND=noninteractive apt-get install -y \
        ca-certificates \
        curl \
        git \
        gnupg \
        lsb-release \
        ufw \
        whiptail

    # dialog — запасной TUI, если есть в репозитории
    DEBIAN_FRONTEND=noninteractive apt-get install -y dialog 2>/dev/null || true
}

install_docker() {
    if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
        log "Docker и плагин Compose уже установлены."
        systemctl enable --now docker || true
        return
    fi

    log "Добавление GPG-ключа и репозитория Docker..."
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg

    if [[ ! -s /etc/apt/keyrings/docker.gpg ]]; then
        log "Не удалось скачать GPG-ключ Docker."
        exit 1
    fi

    ARCH="$(dpkg --print-architecture)"
    CODENAME="$(
        . /etc/os-release
        echo "${VERSION_CODENAME}"
    )"

    echo \
      "deb [arch=${ARCH} signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu ${CODENAME} stable" \
      > /etc/apt/sources.list.d/docker.list

    apt-get update

    log "Установка Docker Engine и плагина Compose..."
    DEBIAN_FRONTEND=noninteractive apt-get install -y \
        docker-ce \
        docker-ce-cli \
        containerd.io \
        docker-buildx-plugin \
        docker-compose-plugin

    log "Включение службы Docker..."
    systemctl enable --now docker
}

configure_firewall() {
    if [[ "$ENABLE_UFW" != "1" ]]; then
        log "Настройка UFW пропущена."
        return
    fi

    local http_port="${HTTP_PORT:-80}"
    local https_port=""
    local https_enabled=""
    if [[ -f "${PROJECT_DIR}/.env" ]]; then
        local v
        v="$(grep -E '^[[:space:]]*HTTP_PORT=' "${PROJECT_DIR}/.env" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        [[ -n "$v" ]] && http_port="$v"
        v="$(grep -E '^[[:space:]]*HTTPS_PORT=' "${PROJECT_DIR}/.env" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        [[ -n "$v" ]] && https_port="$v"
        v="$(grep -E '^[[:space:]]*HTTPS_ENABLED=' "${PROJECT_DIR}/.env" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        [[ -n "$v" ]] && https_enabled="$v"
    fi
    [[ -z "$https_port" && -f "${PROJECT_DIR}/certs/fullchain.pem" && -f "${PROJECT_DIR}/certs/privkey.pem" ]] && https_port=443
    if [[ -z "$https_enabled" || "$https_enabled" == "auto" ]]; then
        [[ -f "${PROJECT_DIR}/certs/fullchain.pem" && -f "${PROJECT_DIR}/certs/privkey.pem" ]] && https_enabled=1
    fi

    log "Настройка правил UFW (HTTP ${http_port}/tcp)..."
    ufw allow "${http_port}/tcp" || true
    case "${https_enabled}" in
        1|true|TRUE|yes|YES|on|ON)
            [[ -z "$https_port" ]] && https_port=443
            log "UFW: HTTPS ${https_port}/tcp"
            ufw allow "${https_port}/tcp" || true
            ;;
    esac
    if [[ "${NM_MODULE_SYSLOG:-1}" == "1" ]]; then
        if declare -F nm_ufw_allow_syslog >/dev/null 2>&1; then
            nm_ufw_allow_syslog
        else
            ufw allow 514/tcp || true
            ufw allow 514/udp || true
        fi
    else
        log "Порт 514 пропущен (модуль syslog отключён)."
    fi

    if ufw status | grep -qi "Status: inactive"; then
        if [[ "$UFW_AUTO_ENABLE" == "1" ]]; then
            log "UFW неактивен — включаем без подтверждения..."
            ufw --force enable || log "Не удалось включить UFW автоматически."
        else
            log "UFW неактивен. Правила добавлены, но файрвол НЕ включён (для включения задайте UFW_AUTO_ENABLE=1)."
        fi
    else
        ufw reload || true
    fi
}

clone_or_update_repo() {
    if declare -F nm_clone_or_update_repo >/dev/null 2>&1; then
        nm_clone_or_update_repo "$PROJECT_DIR" "$REPO_URL" "$BRANCH" "${NM_INSTALL_IS_TAG:-0}"
        return
    fi
    mkdir -p /opt

    if [[ -d "$PROJECT_DIR/.git" ]]; then
        log "Проект уже существует, обновляем..."
        cd "$PROJECT_DIR"

        if ! git diff --quiet || ! git diff --cached --quiet; then
            log "Обнаружены локальные изменения — делаем stash перед обновлением."
            git stash push -u -m "install-$(date +%s)" || true
        fi

        if [[ "${NM_INSTALL_IS_TAG:-0}" == "1" ]]; then
            git fetch origin --tags --force
            git checkout --force "$BRANCH"
        else
            git fetch origin
            git checkout "$BRANCH"
            git pull --ff-only origin "$BRANCH"
        fi
    else
        log "Клонирование репозитория..."
        git clone -b "$BRANCH" "$REPO_URL" "$PROJECT_DIR"
        cd "$PROJECT_DIR"
    fi
}

_source_select_source() {
    local candidates=(
        "${SCRIPT_DIR}/../common/select_source.sh"
        "${PROJECT_DIR}/deploy/common/select_source.sh"
    )
    local c
    for c in "${candidates[@]}"; do
        if [[ -f "$c" ]]; then
            # shellcheck source=deploy/common/select_source.sh
            source "$c"
            return 0
        fi
    done
    # curl-установка одним файлом: подтянуть helper с main
    local tmp owner
    if ! command -v curl >/dev/null 2>&1; then
        return 1
    fi
    owner="$(echo "$REPO_URL" | sed -E 's#.*github.com[:/]([^/]+/[^/.]+)(\.git)?$#\1#')"
    [[ -n "$owner" ]] || owner="varlahin-gena/network_monitor"
    tmp="$(mktemp)"
    if curl -fsSL --connect-timeout 10 --max-time 30 \
        "https://raw.githubusercontent.com/${owner}/main/deploy/common/select_source.sh" \
        -o "$tmp"; then
        # shellcheck source=/dev/null
        source "$tmp"
        rm -f "$tmp"
        return 0
    fi
    rm -f "$tmp"
    return 1
}

choose_install_source() {
    _ensure_dark_newt
    _source_ui || true
    if _source_select_source && declare -F confirm_install_source >/dev/null 2>&1; then
        confirm_install_source
        # .env пишем ПОСЛЕ clone — иначе mkdir+/.env ломает git clone в непустой каталог
        return 0
    fi
    log "select_source.sh недоступен — устанавливаем BRANCH=${BRANCH}."
    NM_INSTALL_SOURCE="${NM_INSTALL_SOURCE:-main}"
    NM_INSTALL_IS_TAG=0
}

find_module_helper() {
    local candidates=(
        "${PROJECT_DIR}/deploy/common/select_modules.sh"
        "${SCRIPT_DIR}/../common/select_modules.sh"
    )
    local c
    for c in "${candidates[@]}"; do
        if [[ -f "$c" ]]; then
            echo "$c"
            return 0
        fi
    done
    return 1
}

source_module_helper() {
    local helper
    if ! helper="$(find_module_helper)"; then
        log "Селектор модулей не найден — включены все модули."
        NM_MODULE_AUTH=1
        NM_MODULE_API_AUTH=1
        NM_MODULE_SYSLOG=1
        NM_MODULE_STATS=1
        NM_MODULE_REPUTATION=1
        NM_COMPOSE_PROFILES="syslog,stats"
        return 1
    fi
    # shellcheck source=deploy/common/select_modules.sh
    source "$helper"
    return 0
}

configure_modules() {
    if source_module_helper; then
        confirm_modules
    fi
}

configure_http_port() {
    # Обычно уже вызван из confirm_https (цепочка). Повторно не спрашиваем.
    if [[ "${NM_HTTP_PORT_CONFIRMED:-0}" == "1" ]]; then
        if declare -F apply_http_port >/dev/null 2>&1; then
            apply_http_port "$PROJECT_DIR"
            return 0
        fi
    fi
    local helper="${PROJECT_DIR}/deploy/common/select_http_port.sh"
    if [[ ! -f "$helper" ]]; then
        helper="${SCRIPT_DIR}/../common/select_http_port.sh"
    fi
    if [[ -f "$helper" ]]; then
        # shellcheck source=deploy/common/select_http_port.sh
        source "$helper"
        confirm_http_port
        apply_http_port "$PROJECT_DIR"
        return 0
    fi
    HTTP_PORT="${HTTP_PORT:-80}"
    export HTTP_PORT
    export NM_HTTP_PORT_CONFIRMED=1
    log "select_http_port.sh не найден — HTTP_PORT=${HTTP_PORT}."
}

configure_https() {
    local helper="${PROJECT_DIR}/deploy/common/select_https.sh"
    if [[ ! -f "$helper" ]]; then
        helper="${SCRIPT_DIR}/../common/select_https.sh"
    fi
    if [[ -f "$helper" ]]; then
        # shellcheck source=deploy/common/select_https.sh
        source "$helper"
        confirm_https "$PROJECT_DIR"
        export NM_HTTPS_CONFIRMED=1
        apply_https "$PROJECT_DIR"
        return 0
    fi
    log "select_https.sh не найден — шаг HTTPS пропущен."
}

configure_resources() {
    local detector="${PROJECT_DIR}/deploy/common/detect_resources.sh"
    if [[ ! -f "$detector" ]]; then
        log "Детектор ресурсов не найден ($detector), используются лимиты compose по умолчанию."
        return
    fi

    # shellcheck source=deploy/common/detect_resources.sh
    source "$detector"
    apply_resource_profile "$PROJECT_DIR"
}

prepare_project() {
    cd "$PROJECT_DIR"

    [[ -f docker-compose.yml ]] || { log "docker-compose.yml не найден после клонирования."; exit 1; }

    log "Выставление прав на исполнение..."
    for f in start.sh stop.sh \
             scripts/tune-resources.sh \
             scripts/backup-clickhouse.sh \
             scripts/restore-clickhouse.sh \
             clickhouse/backfill_edges_agg.sh \
             clickhouse/reset_data.sh \
             deploy/uninstall.sh \
             deploy/common/detect_resources.sh \
             deploy/common/select_modules.sh \
             deploy/common/select_source.sh \
             deploy/common/select_http_port.sh \
             deploy/common/select_https.sh \
             deploy/common/full_auto_preset.sh \
             deploy/common/ui.sh \
             deploy/common/admin_auth.sh \
             deploy/common/uninstall.sh \
             deploy/ubuntu/install_ubuntu.sh \
             deploy/ubuntu/uninstall_ubuntu.sh; do
        if [[ -f "$f" ]]; then
            chmod +x "$f"
        fi
    done

    log "Выбор модулей..."
    configure_modules

    log "Выбор HTTPS для веб-интерфейса..."
    configure_https

    log "Выбор HTTP-порта для веб-интерфейса..."
    configure_http_port

    log "Анализ ресурсов сервера и формирование профиля производительности..."
    configure_resources

    # Синхронизируем модули в .env (и на случай NM_SKIP_PROFILE).
    if source_module_helper; then
        apply_module_selection "$PROJECT_DIR"
        print_modules_summary
    fi
    if declare -F apply_https >/dev/null 2>&1; then
        apply_https "$PROJECT_DIR"
    fi
    if declare -F apply_http_port >/dev/null 2>&1; then
        apply_http_port "$PROJECT_DIR"
    fi
}

ask_firewall() {
    if source_module_helper; then
        confirm_firewall ENABLE_UFW
        return
    fi
    if [[ "${NM_FIREWALL_FROM_ENV}" == "1" ]] || [[ "${NM_AUTO_MODULES:-0}" == "1" ]] || [[ ! -t 0 ]]; then
        return
    fi
    _source_ui || true
    if declare -F nm_ui_yesno >/dev/null 2>&1; then
        local ports="${HTTP_PORT:-80}"
        local https_on=0
        case "${HTTPS_ENABLED:-}" in
            1|true|TRUE|yes|YES|on|ON|auto) https_on=1 ;;
        esac
        if [[ "$https_on" != "1" ]] \
            && [[ -f "${PROJECT_DIR}/certs/fullchain.pem" && -f "${PROJECT_DIR}/certs/privkey.pem" ]]; then
            https_on=1
        fi
        if [[ "$https_on" == "1" ]]; then
            ports="${ports}, ${HTTPS_PORT:-443}"
        fi
        if nm_ui_yesno "Файрвол" \
            "Настроить правила UFW (порты ${ports}, 514)?" 1; then
            ENABLE_UFW=1
        else
            ENABLE_UFW=0
        fi
        return
    fi
    ENABLE_UFW=1
}

start_stack() {
    cd "$PROJECT_DIR"

    if source_module_helper && ! confirm_start_stack "$PROJECT_DIR"; then
        log "Запуск стека пропущен пользователем."
        return 0
    fi

    log "Запуск через ./start.sh ..."
    ./start.sh
}

main() {
    require_root
    detect_ubuntu
    _ensure_dark_newt
    _source_full_auto_preset || true
    if declare -F nm_parse_full_auto_argv >/dev/null 2>&1; then
        nm_parse_full_auto_argv "$@"
    fi
    if declare -F nm_apply_full_auto_preset >/dev/null 2>&1; then
        nm_apply_full_auto_preset
    fi
    _source_ui || true
    print_banner
    _nm_run_gauge_fn "Пакеты" "Обновление apt и установка зависимостей…" install_packages
    _source_ui || true
    _source_full_auto_preset || true
    _welcome_dialog
    _nm_run_gauge_fn "Docker" "Установка Docker Engine и Compose…" install_docker
    choose_install_source
    print_banner
    _nm_run_gauge_fn "Репозиторий" "Клонирование / обновление ${BRANCH}…" clone_or_update_repo
    # После clone — источник установки в .env + UI-слой из репозитория
    if declare -F apply_install_source >/dev/null 2>&1; then
        apply_install_source "$PROJECT_DIR"
    elif [[ -f "${PROJECT_DIR}/deploy/common/select_source.sh" ]]; then
        # shellcheck source=deploy/common/select_source.sh
        source "${PROJECT_DIR}/deploy/common/select_source.sh"
        apply_install_source "$PROJECT_DIR"
    fi
    _source_ui || true
    _source_full_auto_preset || true
    if declare -F nm_apply_full_auto_preset >/dev/null 2>&1; then
        nm_apply_full_auto_preset
    fi
    prepare_project
    if [[ "${NM_FULL_AUTO:-0}" == "1" ]]; then
        if [[ "${NM_DISABLE_HOST_FIREWALL:-0}" == "1" ]] && declare -F nm_disable_host_firewall >/dev/null 2>&1; then
            nm_disable_host_firewall "$PROJECT_DIR"
        else
            ENABLE_UFW=1
            UFW_AUTO_ENABLE=1
            configure_firewall
        fi
    else
        ask_firewall
        if [[ "${ENABLE_UFW}" == "1" ]]; then
            _nm_run_gauge_fn "Файрвол" "Настройка правил UFW…" configure_firewall
        else
            configure_firewall
        fi
    fi
    start_stack
    if [[ "${NM_FULL_AUTO:-0}" == "1" ]] && declare -F nm_full_auto_finish >/dev/null 2>&1; then
        nm_full_auto_finish "$PROJECT_DIR"
    fi
}

main "$@"
