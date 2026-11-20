#!/usr/bin/env bash
# OS-бэкенд: Oracle Linux / RHEL / Rocky / Alma (dnf + firewalld + SELinux).
# Подключается из deploy/install.sh и deploy/uninstall.sh.

GA_OS_FAMILY="rhel"
GA_OS_LABEL="Oracle Linux"
GA_OS_FIREWALL_VAR="ENABLE_FIREWALL"
GA_OS_FIREWALL_NAME="firewalld"
GA_OS_PACKAGES_GAUGE_TEXT="Установка системных пакетов…"
GA_OS_FIREWALL_GAUGE_TEXT="Открытие портов…"

if [[ -n "${ENABLE_FIREWALL+x}" ]]; then
    GA_FIREWALL_FROM_ENV=1
else
    GA_FIREWALL_FROM_ENV=0
    ENABLE_FIREWALL=1
fi
DISABLE_SELINUX="${DISABLE_SELINUX:-0}"

_ga_os_backend_dir() {
    cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd
}

_ga_os_source_firewall_helpers() {
    local dir f
    dir="$(_ga_os_backend_dir)"
    f="${dir}/../common/firewall_helpers.sh"
    if [[ -f "$f" ]]; then
        # shellcheck source=deploy/common/firewall_helpers.sh
        source "$f"
        return 0
    fi
    if [[ -n "${PROJECT_DIR:-}" && -f "${PROJECT_DIR}/deploy/common/firewall_helpers.sh" ]]; then
        # shellcheck source=/dev/null
        source "${PROJECT_DIR}/deploy/common/firewall_helpers.sh"
        return 0
    fi
    return 1
}

ga_os_banner_steps() {
    cat <<EOF
Шаги:
  1. Подготовка системы
  2. Наложение пакета
  3. Доступ к сайту
  4. Режим производительности
  5. Пароль администратора
  6. Сетевой экран
  7. Запуск
EOF
}

ga_os_welcome_lines() {
    cat <<EOF
Дальше: подготовка системы, пакет, доступ к сайту,
режим работы, пароль администратора, сетевой экран и запуск.
EOF
}

ga_os_before_docker() {
    log "Удаление конфликтующих пакетов контейнеров (если есть)..."
    dnf remove -y podman buildah runc containerd container-tools 2>/dev/null || true
}

ga_os_after_docker() {
    if ! command -v getenforce >/dev/null 2>&1; then
        log "Утилиты SELinux отсутствуют — пропуск."
        return
    fi

    local mode
    mode="$(getenforce 2>/dev/null || echo Disabled)"
    log "Режим SELinux: ${mode}"

    if [[ "${DISABLE_SELINUX}" == "1" ]]; then
        log "Перевод SELinux в permissive (DISABLE_SELINUX=1)..."
        setenforce 0 || true
        sed -i 's/^SELINUX=enforcing/SELINUX=permissive/' /etc/selinux/config || true
        return
    fi

    if [[ "$mode" == "Enforcing" ]]; then
        log "Разрешение Docker управлять cgroup (SELinux)..."
        setsebool -P container_manage_cgroup on 2>/dev/null || true
    fi
}

ga_os_chmod_extra() {
    cat <<EOF
deploy/install.sh
deploy/uninstall.sh
deploy/common/install_orchestrator.sh
deploy/common/select_ui_access.sh
deploy/oracle_linux/os_backend.sh
EOF
}

# --- Uninstall hooks (вызываются из common/uninstall.sh) ---

ga_os_audit_firewall() {
    if command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld 2>/dev/null; then
        echo "  Сетевой экран     : включён, правила портов будут сняты при удалении"
        echo "[$(date +'%F %T')] [firewall] audit: firewalld active" >&2
    elif command -v firewall-cmd >/dev/null 2>&1; then
        echo "  Сетевой экран     : выключен"
        echo "[$(date +'%F %T')] [firewall] audit: firewalld installed, inactive" >&2
    else
        echo "  Сетевой экран     : не найден"
        echo "[$(date +'%F %T')] [firewall] audit: firewalld not found" >&2
    fi
}

ga_audit_firewall() { ga_os_audit_firewall; }

ga_os_remove_firewall_rules() {
    if [[ "${REMOVE_FIREWALL_RULES:-1}" != "1" ]]; then
        return
    fi

    local http_port="80"
    local https_port=""
    local project_dir="${GA_PROJECT_DIR:-${PROJECT_DIR:-/opt/geoatlas}}"
    if [[ -f "${project_dir}/.env" ]]; then
        local v
        v="$(grep -E '^[[:space:]]*HTTP_PORT=' "${project_dir}/.env" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        [[ -n "$v" ]] && http_port="$v"
        v="$(grep -E '^[[:space:]]*HTTPS_PORT=' "${project_dir}/.env" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        [[ -n "$v" ]] && https_port="$v"
    fi
    [[ -z "$https_port" ]] && https_port="443"

    if ! command -v firewall-cmd >/dev/null 2>&1 || ! systemctl is-active --quiet firewalld; then
        echo "[$(date +'%F %T')] Сетевой экран не активен — пропуск снятия правил."
        return
    fi

    echo "[$(date +'%F %T')] Снятие правил сетевого экрана..."
    firewall-cmd --permanent --remove-port="${http_port}/tcp" || true
    firewall-cmd --permanent --remove-port="${https_port}/tcp" || true
    firewall-cmd --permanent --remove-port=80/tcp || true
    firewall-cmd --permanent --remove-port=443/tcp || true
    # Legacy: backend был на 8080
    firewall-cmd --permanent --remove-port=8080/tcp || true

    _ga_os_source_firewall_helpers || true
    if declare -F ga_firewalld_remove_syslog >/dev/null 2>&1; then
        ga_firewalld_remove_syslog
    else
        firewall-cmd --permanent --remove-port=514/tcp || true
        firewall-cmd --permanent --remove-port=514/udp || true
    fi

    firewall-cmd --reload || true
}

ga_remove_firewall_rules() { ga_os_remove_firewall_rules; }

ga_os_install_packages() {
    log "Установка зависимостей..."
    dnf install -y \
        ca-certificates \
        curl \
        dnf-plugins-core \
        firewalld \
        policycoreutils \
        policycoreutils-python-utils \
        newt || true

    dnf install -y newt 2>/dev/null || true
    dnf install -y dialog 2>/dev/null || true
}

ga_os_install_docker() {
    if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
        log "Docker уже установлен."
        systemctl enable --now docker || true
        return
    fi

    log "Добавление репозитория Docker..."
    dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo

    log "Установка Docker..."
    dnf install -y \
        docker-ce \
        docker-ce-cli \
        containerd.io \
        docker-buildx-plugin \
        docker-compose-plugin

    log "Включение службы Docker..."
    systemctl enable --now docker
}

ga_os_configure_firewall() {
    if [[ "${ENABLE_FIREWALL}" != "1" ]]; then
        log "Настройка сетевого экрана пропущена."
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

    log "Открытие HTTP ${http_port}/tcp..."
    systemctl enable --now firewalld || true

    firewall-cmd --permanent --add-port="${http_port}/tcp" || true
    case "${https_enabled}" in
        1|true|TRUE|yes|YES|on|ON)
            [[ -z "$https_port" ]] && https_port=443
            log "Открытие HTTPS ${https_port}/tcp..."
            firewall-cmd --permanent --add-port="${https_port}/tcp" || true
            ;;
    esac
    if [[ "${GA_MODULE_SYSLOG:-1}" == "1" ]]; then
        _ga_os_source_firewall_helpers || true
        if declare -F ga_firewalld_allow_syslog >/dev/null 2>&1; then
            ga_firewalld_allow_syslog
        else
            firewall-cmd --permanent --add-port=514/tcp  || true
            firewall-cmd --permanent --add-port=514/udp  || true
        fi
    else
        log "Порт 514 пропущен (syslog отключён)."
    fi

    firewall-cmd --reload || true
}
