#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_DIR="/opt/geoatlas"
if [[ -n "${ENABLE_FIREWALL+x}" ]]; then
    GA_FIREWALL_FROM_ENV=1
else
    GA_FIREWALL_FROM_ENV=0
    ENABLE_FIREWALL=1
fi
DISABLE_SELINUX="${DISABLE_SELINUX:-0}"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"

log() { echo "[$(date +'%F %T')] $*"; }

trap 'log "ОШИБКА на строке ${LINENO} (код выхода $?). Смотрите лог выше / docker compose logs."' ERR

_ga_banner_text() {
    local src_line
    if [[ -n "${GA_INSTALL_PACKAGE:-}" ]]; then
        src_line="Пакет: ${GA_INSTALL_PACKAGE}"
    else
        src_line="Пакет: каталог установщика (или GA_INSTALL_PACKAGE)"
    fi
    cat <<EOF
Каталог: ${PROJECT_DIR}
${src_line}

Шаги:
  1. Пакеты и Docker
  2. SELinux
  3. Наложение пакета
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
    text="$(_ga_banner_text)"
    echo ""
    echo "══════════════════════════════════════════════════════════"
    echo "  ГеоАтлас — установка (Oracle Linux)"
    echo "══════════════════════════════════════════════════════════"
    echo "$text" | sed 's/^/  /'
    echo "══════════════════════════════════════════════════════════"
    echo ""
}

_ga_run_gauge() {
    local title="$1" text="$2"
    shift 2
    if declare -F ga_ui_run_with_gauge >/dev/null 2>&1; then
        ga_ui_run_with_gauge "$title" "$text" "$@"
    else
        "$@"
    fi
}

_ga_run_gauge_fn() {
    local title="$1" text="$2" fn="$3"
    if declare -F ga_ui_run_with_gauge >/dev/null 2>&1; then
        ga_ui_run_with_gauge "$title" "$text" "$fn"
    else
        "$fn"
    fi
}

_ga_source_common() {
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
    return 1
}

_source_ui() {
    if ! _ga_source_common ui.sh; then
        return 1
    fi
    ga_ui_init
    return 0
}

_source_firewall_helpers() {
    _ga_source_common firewall_helpers.sh
}

# Тёмная тема whiptail до появления ui.sh (curl-установка / welcome).
_ensure_dark_newt() {
    if [[ "${GA_UI_DARK:-1}" == "0" ]]; then
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
    # CI / без TTY / авто-модули — без приветствия.
    if [[ "${GA_AUTO_MODULES:-0}" == "1" ]] || [[ ! -t 0 ]]; then
        return 0
    fi
    _ensure_dark_newt
    _source_ui || true
    if declare -F ga_ui_msgbox >/dev/null 2>&1; then
        ga_ui_msgbox "Установка ГеоАтлас" \
"Добро пожаловать в мастер установки.

Каталог: ${PROJECT_DIR}

Далее по шагам: Docker, SELinux, пакет, модули,
HTTPS, порт UI, профиль, файрвол и запуск." || true
    fi
}

require_root() {
    if [[ $EUID -ne 0 ]]; then
        echo "Запустите от имени root."
        exit 1
    fi
}

detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        log "Обнаружена ОС: ${PRETTY_NAME:-unknown}"

        case "${ID:-}" in
            ol|rhel|rocky|almalinux|centos) ;;
            *) log "Внимание: скрипт рассчитан на Oracle Linux / системы на базе RHEL (обнаружено: ${ID:-unknown}).";;
        esac

        OS_MAJOR="${VERSION_ID%%.*}"
        log "Мажорная версия: ${OS_MAJOR}"
    fi
}

remove_conflicting_packages() {
    log "Удаление podman / buildah / runc при наличии (чтобы избежать конфликтов с Docker)..."
    dnf remove -y podman buildah runc containerd container-tools 2>/dev/null || true
}

install_packages() {
    log "Установка зависимостей..."
    dnf install -y \
        ca-certificates \
        curl \
        dnf-plugins-core \
        firewalld \
        policycoreutils \
        policycoreutils-python-utils \
        newt || true

    # whiptail/dialog — TUI (newt даёт whiptail на многих OL)
    dnf install -y newt 2>/dev/null || true
    dnf install -y dialog 2>/dev/null || true
}

install_docker() {
    if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
        log "Docker и плагин Compose уже установлены."
        systemctl enable --now docker || true
        return
    fi

    log "Добавление репозитория Docker CE..."
    dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo

    log "Установка Docker Engine и плагина Compose..."
    dnf install -y \
        docker-ce \
        docker-ce-cli \
        containerd.io \
        docker-buildx-plugin \
        docker-compose-plugin

    log "Включение службы Docker..."
    systemctl enable --now docker
}

configure_selinux() {
    if ! command -v getenforce >/dev/null 2>&1; then
        log "Утилиты SELinux отсутствуют — пропуск."
        return
    fi

    local mode
    mode="$(getenforce 2>/dev/null || echo Disabled)"
    log "Режим SELinux: ${mode}"

    if [[ "$DISABLE_SELINUX" == "1" ]]; then
        log "Перевод SELinux в permissive (DISABLE_SELINUX=1)..."
        setenforce 0 || true
        sed -i 's/^SELINUX=enforcing/SELINUX=permissive/' /etc/selinux/config || true
        return
    fi

    if [[ "$mode" == "Enforcing" ]]; then
        log "SELinux в режиме Enforcing. Включаем bool 'container_manage_cgroup' для Docker..."
        setsebool -P container_manage_cgroup on 2>/dev/null || true
    fi
}

configure_firewall() {
    if [[ "$ENABLE_FIREWALL" != "1" ]]; then
        log "Настройка firewalld пропущена."
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

    log "Настройка firewalld (HTTP ${http_port}/tcp)..."
    systemctl enable --now firewalld || true

    firewall-cmd --permanent --add-port="${http_port}/tcp" || true
    case "${https_enabled}" in
        1|true|TRUE|yes|YES|on|ON)
            [[ -z "$https_port" ]] && https_port=443
            log "firewalld: HTTPS ${https_port}/tcp"
            firewall-cmd --permanent --add-port="${https_port}/tcp" || true
            ;;
    esac
    if [[ "${GA_MODULE_SYSLOG:-1}" == "1" ]]; then
        _source_firewall_helpers || true
        if declare -F ga_firewalld_allow_syslog >/dev/null 2>&1; then
            ga_firewalld_allow_syslog
        else
            firewall-cmd --permanent --add-port=514/tcp  || true
            firewall-cmd --permanent --add-port=514/udp  || true
        fi
    else
        log "Порт 514 пропущен (модуль syslog отключён)."
    fi

    firewall-cmd --reload || true
}

apply_install_package() {
    if ! declare -F ga_fetch_project >/dev/null 2>&1; then
        log "Нет apply_package.sh — запустите установщик из распакованного geoatlas-X.Y.Z."
        exit 1
    fi
    ga_fetch_project "$PROJECT_DIR"
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
    return 1
}

choose_install_source() {
    _ensure_dark_newt
    _source_ui || true
    if _source_select_source && declare -F confirm_install_source >/dev/null 2>&1; then
        confirm_install_source
        return 0
    fi
    log "select_source.sh недоступен. Запустите ./deploy/oracle_linux/install_oraclelinux.sh из распакованного пакета."
    exit 1
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
        GA_MODULE_AUTH=1
        GA_MODULE_API_AUTH=1
        GA_MODULE_SYSLOG=1
        GA_MODULE_STATS=1
        GA_MODULE_REPUTATION=1
        GA_COMPOSE_PROFILES="syslog,stats"
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
    if [[ "${GA_HTTP_PORT_CONFIRMED:-0}" == "1" ]]; then
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
    export GA_HTTP_PORT_CONFIRMED=1
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
        export GA_HTTPS_CONFIRMED=1
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

    [[ -f docker-compose.yml ]] || { log "docker-compose.yml не найден после наложения пакета."; exit 1; }

    log "Выставление прав на исполнение..."
    for f in start.sh stop.sh update.sh \
             scripts/tune-resources.sh \
             scripts/pack-release.sh \
             scripts/backup-clickhouse.sh \
             scripts/restore-clickhouse.sh \
             clickhouse/backfill_edges_agg.sh \
             clickhouse/reset_data.sh \
             deploy/uninstall.sh \
             deploy/common/detect_resources.sh \
             deploy/common/select_modules.sh \
             deploy/common/select_source.sh \
             deploy/common/apply_package.sh \
             deploy/common/select_http_port.sh \
             deploy/common/select_https.sh \
             deploy/common/firewall_helpers.sh \
             deploy/common/ui.sh \
             deploy/common/admin_auth.sh \
             deploy/common/uninstall.sh \
             deploy/oracle_linux/install_oraclelinux.sh \
             deploy/oracle_linux/uninstall_oraclelinux.sh; do
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
        confirm_firewall ENABLE_FIREWALL
        return
    fi
    if [[ "${GA_FIREWALL_FROM_ENV}" == "1" ]] || [[ "${GA_AUTO_MODULES:-0}" == "1" ]] || [[ ! -t 0 ]]; then
        return
    fi
    _source_ui || true
    if declare -F ga_ui_yesno >/dev/null 2>&1; then
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
        if ga_ui_yesno "Файрвол" \
            "Настроить правила firewalld (порты ${ports}, 514)?" 1; then
            ENABLE_FIREWALL=1
        else
            ENABLE_FIREWALL=0
        fi
        return
    fi
    ENABLE_FIREWALL=1
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
    detect_os
    _ensure_dark_newt
    _source_ui || true
    print_banner
    _ga_run_gauge_fn "Конфликты" "Удаление podman/buildah…" remove_conflicting_packages
    _ga_run_gauge_fn "Пакеты" "Установка зависимостей (dnf)…" install_packages
    _source_ui || true
    _welcome_dialog
    _ga_run_gauge_fn "Docker" "Установка Docker Engine и Compose…" install_docker
    _ga_run_gauge_fn "SELinux" "Настройка SELinux для Docker…" configure_selinux
    choose_install_source
    print_banner
    _ga_run_gauge_fn "Пакет" "Наложение geoatlas-*.tar.gz…" apply_install_package
    # После наложения — источник в .env + UI-слой из пакета
    if declare -F apply_install_source >/dev/null 2>&1; then
        apply_install_source "$PROJECT_DIR"
    elif [[ -f "${PROJECT_DIR}/deploy/common/select_source.sh" ]]; then
        # shellcheck source=deploy/common/select_source.sh
        source "${PROJECT_DIR}/deploy/common/select_source.sh"
        apply_install_source "$PROJECT_DIR"
    fi
    _source_ui || true
    prepare_project
    ask_firewall
    if [[ "${ENABLE_FIREWALL}" == "1" ]]; then
        _ga_run_gauge_fn "Файрвол" "Настройка правил firewalld…" configure_firewall
    else
        configure_firewall
    fi
    start_stack
}

main "$@"
