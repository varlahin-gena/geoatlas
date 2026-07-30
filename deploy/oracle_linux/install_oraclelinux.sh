#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_DIR="/opt/network-monitor"
REPO_URL="${REPO_URL:-https://github.com/varlahin-gena/network_monitor.git}"
if [[ -n "${BRANCH+x}" ]]; then
    NM_BRANCH_FROM_ENV=1
else
    NM_BRANCH_FROM_ENV=0
    BRANCH="main"
fi

if [[ -n "${ENABLE_FIREWALL+x}" ]]; then
    NM_FIREWALL_FROM_ENV=1
else
    NM_FIREWALL_FROM_ENV=0
    ENABLE_FIREWALL=1
fi
DISABLE_SELINUX="${DISABLE_SELINUX:-0}"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"

log() { echo "[$(date +'%F %T')] $*"; }

trap 'log "ERROR at line ${LINENO} (exit code $?). Check logs above / docker compose logs."' ERR

print_banner() {
    echo ""
    echo "══════════════════════════════════════════════════════════"
    echo "  ГеоАтлас — интерактивная установка (Oracle Linux)"
    echo "══════════════════════════════════════════════════════════"
    echo "  Каталог : ${PROJECT_DIR}"
    if [[ -n "${NM_INSTALL_SOURCE:-}" ]]; then
        echo "  Источник: ${NM_INSTALL_SOURCE} → ${BRANCH}"
    else
        echo "  Ref     : ${BRANCH} (выбор источника — после Docker)"
    fi
    echo "  Репо    : ${REPO_URL}"
    echo ""
    echo "  Шаги:"
    echo "    1. Пакеты и Docker"
    echo "    2. SELinux"
    echo "    3. Источник (релиз / main)"
    echo "    4. Клонирование репозитория"
    echo "    5. Выбор модулей"
    echo "    6. Порт веб-интерфейса"
    echo "    7. Профиль производительности"
    echo "    8. Firewall"
    echo "    9. Запуск стека"
    echo "══════════════════════════════════════════════════════════"
    echo ""
}

_source_ui() {
    local candidates=(
        "${PROJECT_DIR}/deploy/common/ui.sh"
        "${SCRIPT_DIR}/../common/ui.sh"
    )
    local c
    for c in "${candidates[@]}"; do
        if [[ -f "$c" ]]; then
            # shellcheck source=deploy/common/ui.sh
            source "$c"
            nm_ui_init
            return 0
        fi
    done
    return 1
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
    if [[ "${NM_AUTO_MODULES:-0}" == "1" ]] || [[ ! -t 0 ]]; then
        return 0
    fi
    _ensure_dark_newt
    if _source_ui && declare -F nm_ui_msgbox >/dev/null 2>&1; then
        nm_ui_msgbox "Установка ГеоАтлас" \
"Добро пожаловать в мастер установки.

Каталог: ${PROJECT_DIR}
Репозиторий: ${REPO_URL}

Далее: выбор релиза или ветки main, зависимости, Docker и модули." || true
        return 0
    fi
    if command -v whiptail >/dev/null 2>&1; then
        whiptail --backtitle "ГеоАтлас" --title "Установка ГеоАтлас" \
            --msgbox "Добро пожаловать в мастер установки.

Каталог: ${PROJECT_DIR}

Далее: выбор релиза или ветки main, зависимости и Docker." 14 60 || true
    fi
}

require_root() {
    if [[ $EUID -ne 0 ]]; then
        echo "Please run as root."
        exit 1
    fi
}

detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        log "Detected OS: ${PRETTY_NAME:-unknown}"

        case "${ID:-}" in
            ol|rhel|rocky|almalinux|centos) ;;
            *) log "Warning: this script is intended for Oracle Linux / RHEL-based systems (detected: ${ID:-unknown}).";;
        esac

        OS_MAJOR="${VERSION_ID%%.*}"
        log "Major version: ${OS_MAJOR}"
    fi
}

remove_conflicting_packages() {
    log "Removing podman / buildah / runc if present (to avoid conflicts with Docker)..."
    dnf remove -y podman buildah runc containerd container-tools 2>/dev/null || true
}

install_packages() {
    log "Installing prerequisites..."
    dnf install -y \
        ca-certificates \
        curl \
        git \
        dnf-plugins-core \
        firewalld \
        policycoreutils \
        policycoreutils-python-utils \
        newt || true

    # YAD только при наличии дисплея; сбой не блокирует установку.
    if [[ -n "${DISPLAY:-}" || -n "${WAYLAND_DISPLAY:-}" ]]; then
        log "Graphical display detected — installing yad (optional)..."
        dnf install -y yad || log "yad not available — continuing with whiptail/text UI."
    fi
}

install_docker() {
    if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
        log "Docker and compose plugin already installed."
        systemctl enable --now docker || true
        return
    fi

    log "Adding Docker CE repository..."
    dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo

    log "Installing Docker Engine and Compose plugin..."
    dnf install -y \
        docker-ce \
        docker-ce-cli \
        containerd.io \
        docker-buildx-plugin \
        docker-compose-plugin

    log "Enabling Docker service..."
    systemctl enable --now docker
}

configure_selinux() {
    if ! command -v getenforce >/dev/null 2>&1; then
        log "SELinux tools not present, skipping."
        return
    fi

    local mode
    mode="$(getenforce 2>/dev/null || echo Disabled)"
    log "SELinux mode: ${mode}"

    if [[ "$DISABLE_SELINUX" == "1" ]]; then
        log "Setting SELinux to permissive (DISABLE_SELINUX=1)..."
        setenforce 0 || true
        sed -i 's/^SELINUX=enforcing/SELINUX=permissive/' /etc/selinux/config || true
        return
    fi

    if [[ "$mode" == "Enforcing" ]]; then
        log "SELinux is Enforcing. Setting bool 'container_manage_cgroup' to allow Docker..."
        setsebool -P container_manage_cgroup on 2>/dev/null || true
    fi
}

configure_firewall() {
    if [[ "$ENABLE_FIREWALL" != "1" ]]; then
        log "Firewalld configuration skipped."
        return
    fi

    local http_port="${HTTP_PORT:-80}"
    if [[ -f "${PROJECT_DIR}/.env" ]]; then
        local v
        v="$(grep -E '^[[:space:]]*HTTP_PORT=' "${PROJECT_DIR}/.env" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        [[ -n "$v" ]] && http_port="$v"
    fi

    log "Configuring firewalld (HTTP ${http_port}/tcp)..."
    systemctl enable --now firewalld || true

    firewall-cmd --permanent --add-port="${http_port}/tcp" || true
    if [[ "${NM_MODULE_SYSLOG:-1}" == "1" ]]; then
        firewall-cmd --permanent --add-port=514/tcp  || true
        firewall-cmd --permanent --add-port=514/udp  || true
    else
        log "Port 514 skipped (syslog module disabled)."
    fi

    firewall-cmd --reload || true
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
             deploy/common/ui.sh \
             deploy/common/uninstall.sh \
             deploy/oracle_linux/install_oraclelinux.sh \
             deploy/oracle_linux/uninstall_oraclelinux.sh; do
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
        confirm_firewall ENABLE_FIREWALL
        return
    fi
    if [[ "${NM_FIREWALL_FROM_ENV}" == "1" ]] || [[ "${NM_AUTO_MODULES:-0}" == "1" ]] || [[ ! -t 0 ]]; then
        return
    fi
    local answer
    if [[ -r /dev/tty && -w /dev/tty ]]; then
        printf 'Настроить правила firewalld (порты %s, 514)? [Y/n]: ' "${HTTP_PORT:-80}" >/dev/tty
        read -r answer </dev/tty || answer=""
    else
        printf 'Настроить правила firewalld (порты %s, 514)? [Y/n]: ' "${HTTP_PORT:-80}" >&2
        read -r answer || answer=""
    fi
    answer="${answer,,}"
    if [[ "$answer" =~ ^(n|no|н|нет)$ ]]; then
        ENABLE_FIREWALL=0
    fi
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
    print_banner
    detect_os
    remove_conflicting_packages
    install_packages
    _welcome_dialog
    install_docker
    configure_selinux
    choose_install_source
    print_banner
    clone_or_update_repo
    _source_ui || true
    prepare_project
    ask_firewall
    configure_firewall
    start_stack
}

main "$@"
