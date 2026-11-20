#!/usr/bin/env bash
# OS-бэкенд установщика: Debian/Ubuntu (apt + UFW).
# Подключается из deploy/install.sh до install_orchestrator.sh.

GA_OS_FAMILY="debian"
GA_OS_LABEL="Ubuntu"
GA_OS_FIREWALL_VAR="ENABLE_UFW"
GA_OS_FIREWALL_NAME="UFW"
GA_OS_PACKAGES_GAUGE_TEXT="Установка системных пакетов…"
GA_OS_FIREWALL_GAUGE_TEXT="Открытие портов…"

if [[ -n "${ENABLE_UFW+x}" ]]; then
    GA_FIREWALL_FROM_ENV=1
else
    GA_FIREWALL_FROM_ENV=0
    ENABLE_UFW=1
fi
UFW_AUTO_ENABLE="${UFW_AUTO_ENABLE:-0}"

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

ga_os_before_docker() { :; }

ga_os_after_docker() { :; }

ga_os_chmod_extra() {
    cat <<EOF
deploy/install.sh
deploy/common/install_orchestrator.sh
deploy/common/select_ui_access.sh
deploy/ubuntu/os_backend.sh
deploy/ubuntu/uninstall_ubuntu.sh
EOF
}

ga_os_install_packages() {
    log "Обновление списков пакетов..."
    apt-get update

    log "Установка зависимостей..."
    DEBIAN_FRONTEND=noninteractive apt-get install -y \
        ca-certificates \
        curl \
        gnupg \
        lsb-release \
        ufw \
        whiptail

    DEBIAN_FRONTEND=noninteractive apt-get install -y dialog 2>/dev/null || true
}

ga_os_install_docker() {
    if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
        log "Docker уже установлен."
        systemctl enable --now docker || true
        return
    fi

    log "Добавление репозитория Docker..."
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg

    if [[ ! -s /etc/apt/keyrings/docker.gpg ]]; then
        log "Не удалось скачать ключ Docker."
        exit 1
    fi

    local arch codename
    arch="$(dpkg --print-architecture)"
    codename="$(
        . /etc/os-release
        echo "${VERSION_CODENAME}"
    )"

    echo \
      "deb [arch=${arch} signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu ${codename} stable" \
      > /etc/apt/sources.list.d/docker.list

    apt-get update

    log "Установка Docker..."
    DEBIAN_FRONTEND=noninteractive apt-get install -y \
        docker-ce \
        docker-ce-cli \
        containerd.io \
        docker-buildx-plugin \
        docker-compose-plugin

    log "Включение службы Docker..."
    systemctl enable --now docker
}

ga_os_configure_firewall() {
    if [[ "${ENABLE_UFW}" != "1" ]]; then
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
    ufw allow "${http_port}/tcp" || true
    case "${https_enabled}" in
        1|true|TRUE|yes|YES|on|ON)
            [[ -z "$https_port" ]] && https_port=443
            log "Открытие HTTPS ${https_port}/tcp..."
            ufw allow "${https_port}/tcp" || true
            ;;
    esac
    if [[ "${GA_MODULE_SYSLOG:-1}" == "1" ]]; then
        _ga_os_source_firewall_helpers || true
        if declare -F ga_ufw_allow_syslog >/dev/null 2>&1; then
            ga_ufw_allow_syslog
        else
            ufw allow 514/tcp || true
            ufw allow 514/udp || true
        fi
    else
        log "Порт 514 пропущен (syslog отключён)."
    fi

    if ufw status | grep -qi "Status: inactive"; then
        if [[ "${UFW_AUTO_ENABLE}" == "1" ]]; then
            log "Включение сетевого экрана..."
            ufw --force enable || log "Не удалось включить сетевой экран автоматически."
        else
            log "Правила добавлены, сетевой экран пока выключен (UFW_AUTO_ENABLE=1 — включить)."
        fi
    else
        ufw reload || true
    fi
}
