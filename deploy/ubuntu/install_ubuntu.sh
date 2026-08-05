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

trap 'log "ERROR at line ${LINENO} (exit code $?). Check logs above / docker compose logs."' ERR

_nm_banner_text() {
    local src_line
    if [[ -n "${NM_INSTALL_SOURCE:-}" ]]; then
        src_line="Источник: ${NM_INSTALL_SOURCE} → ${BRANCH}"
    else
        src_line="Ref: ${BRANCH} (выбор источника — после Docker)"
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
  5. Порт веб-интерфейса
  6. Профиль производительности
  7. Firewall
  8. Запуск стека
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

auto — релиз, модули, :8080, firewall off
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
        echo "Please run as root."
        exit 1
    fi
}

detect_ubuntu() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        if [[ "${ID:-}" != "ubuntu" ]]; then
            log "Warning: this script is intended for Ubuntu (detected: ${ID:-unknown})."
        fi
        log "Detected OS: ${PRETTY_NAME:-unknown}"
    fi
}

install_packages() {
    log "Updating package lists..."
    apt-get update

    log "Installing prerequisites..."
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
        log "Docker and compose plugin already installed."
        systemctl enable --now docker || true
        return
    fi

    log "Adding Docker GPG key and repository..."
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg

    if [[ ! -s /etc/apt/keyrings/docker.gpg ]]; then
        log "Failed to download Docker GPG key."
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

    log "Installing Docker Engine and Compose plugin..."
    DEBIAN_FRONTEND=noninteractive apt-get install -y \
        docker-ce \
        docker-ce-cli \
        containerd.io \
        docker-buildx-plugin \
        docker-compose-plugin

    log "Enabling Docker service..."
    systemctl enable --now docker
}

configure_firewall() {
    if [[ "$ENABLE_UFW" != "1" ]]; then
        log "UFW configuration skipped."
        return
    fi

    local http_port="${HTTP_PORT:-80}"
    if [[ -f "${PROJECT_DIR}/.env" ]]; then
        local v
        v="$(grep -E '^[[:space:]]*HTTP_PORT=' "${PROJECT_DIR}/.env" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        [[ -n "$v" ]] && http_port="$v"
    fi

    log "Configuring UFW rules (HTTP ${http_port}/tcp)..."
    ufw allow "${http_port}/tcp" || true
    if [[ "${NM_MODULE_SYSLOG:-1}" == "1" ]]; then
        ufw allow 514/tcp || true
        ufw allow 514/udp || true
    else
        log "Port 514 skipped (syslog module disabled)."
    fi

    if ufw status | grep -qi "Status: inactive"; then
        if [[ "$UFW_AUTO_ENABLE" == "1" ]]; then
            log "UFW is inactive — enabling non-interactively..."
            ufw --force enable || log "Could not enable UFW automatically."
        else
            log "UFW is inactive. Rules added but firewall NOT enabled (set UFW_AUTO_ENABLE=1 to enable)."
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
        log "Project already exists, updating..."
        cd "$PROJECT_DIR"

        if ! git diff --quiet || ! git diff --cached --quiet; then
            log "Local changes detected — stashing before update."
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
        log "Cloning repository..."
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
        log "Module selector not found — all modules enabled."
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
    log "select_http_port.sh not found — HTTP_PORT=${HTTP_PORT}."
}

configure_resources() {
    local detector="${PROJECT_DIR}/deploy/common/detect_resources.sh"
    if [[ ! -f "$detector" ]]; then
        log "Resource detector not found ($detector), using default compose limits."
        return
    fi

    # shellcheck source=deploy/common/detect_resources.sh
    source "$detector"
    apply_resource_profile "$PROJECT_DIR"
}

prepare_project() {
    cd "$PROJECT_DIR"

    [[ -f docker-compose.yml ]] || { log "docker-compose.yml not found after clone."; exit 1; }

    log "Setting executable permissions..."
    for f in start.sh stop.sh \
             scripts/tune-resources.sh \
             deploy/uninstall.sh \
             deploy/common/detect_resources.sh \
             deploy/common/select_modules.sh \
             deploy/common/select_source.sh \
             deploy/common/select_http_port.sh \
             deploy/common/full_auto_preset.sh \
             deploy/common/ui.sh \
             deploy/common/uninstall.sh \
             deploy/ubuntu/install_ubuntu.sh \
             deploy/ubuntu/uninstall_ubuntu.sh; do
        if [[ -f "$f" ]]; then
            chmod +x "$f"
        fi
    done

    log "Selecting modules..."
    configure_modules

    log "Selecting HTTP port for web UI..."
    configure_http_port

    log "Detecting server resources and generating performance profile..."
    configure_resources

    # Синхронизируем модули в .env (и на случай NM_SKIP_PROFILE).
    if source_module_helper; then
        apply_module_selection "$PROJECT_DIR"
        print_modules_summary
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
        if nm_ui_yesno "Firewall" \
            "Настроить правила UFW (порты ${HTTP_PORT:-80}, 514)?" 1; then
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
        log "Stack start skipped by user."
        return 0
    fi

    log "Delegating startup to ./start.sh ..."
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
    # После clone — полноценный UI-слой из репозитория
    _source_ui || true
    _source_full_auto_preset || true
    if declare -F nm_apply_full_auto_preset >/dev/null 2>&1; then
        nm_apply_full_auto_preset
    fi
    prepare_project
    if [[ "${NM_FULL_AUTO:-0}" == "1" ]]; then
        if declare -F nm_disable_host_firewall >/dev/null 2>&1; then
            nm_disable_host_firewall
        else
            ENABLE_UFW=0
            configure_firewall
        fi
    else
        ask_firewall
        if [[ "${ENABLE_UFW}" == "1" ]]; then
            _nm_run_gauge_fn "Firewall" "Настройка правил UFW…" configure_firewall
        else
            configure_firewall
        fi
    fi
    start_stack
}

main "$@"
