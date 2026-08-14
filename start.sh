#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
cd "$SCRIPT_DIR"

# shellcheck source=deploy/common/compose.sh
source "${SCRIPT_DIR}/deploy/common/compose.sh"
# shellcheck source=deploy/common/admin_auth.sh
source "${SCRIPT_DIR}/deploy/common/admin_auth.sh"

HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-120}"
DO_BUILD="${DO_BUILD:-1}"          # 1 = пересобирать образы, 0 = только поднять

_nm_env_get() {
    local key="$1" v=""
    if [[ -f .env ]]; then
        v="$(grep -E "^[[:space:]]*${key}=" .env 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
    fi
    echo "$v"
}

# Порт UI из .env (HTTP_PORT); HEALTH_URL можно переопределить явно.
_nm_http_port_from_env() {
    local p
    p="$(_nm_env_get HTTP_PORT)"
    [[ -z "$p" ]] && p="80"
    echo "${HTTP_PORT:-$p}"
}

_nm_https_port_from_env() {
    local p
    p="$(_nm_env_get HTTPS_PORT)"
    [[ -z "$p" ]] && p="443"
    echo "${HTTPS_PORT:-$p}"
}

HTTP_PORT="$(_nm_http_port_from_env)"
HTTPS_PORT="$(_nm_https_port_from_env)"
HTTPS_ON=0
if nm_https_active "$SCRIPT_DIR"; then
    HTTPS_ON=1
fi

if [[ -z "${HEALTH_URL:-}" ]]; then
    if (( HTTPS_ON == 1 )); then
        if [[ "${HTTPS_PORT}" == "443" ]]; then
            HEALTH_URL="https://127.0.0.1/api/ready"
        else
            HEALTH_URL="https://127.0.0.1:${HTTPS_PORT}/api/ready"
        fi
    elif [[ "${HTTP_PORT}" == "80" ]]; then
        HEALTH_URL="http://127.0.0.1/api/ready"
    else
        HEALTH_URL="http://127.0.0.1:${HTTP_PORT}/api/ready"
    fi
fi

log() { echo "[$(date +'%F %T')] $*"; }

trap 'log "ОШИБКА на строке ${LINENO} (код выхода $?)."' ERR

require_docker() {
    if ! command -v docker >/dev/null 2>&1; then
        echo "Docker не найден. Сначала установите Docker."
        exit 1
    fi
    if ! docker compose version >/dev/null 2>&1; then
        echo "Плагин docker compose не найден. Установите docker-compose-plugin."
        exit 1
    fi
}

# syslog-ng.d/zz_profile.conf — gitignored; без него 4.11 берёт medium-дефолты из syslog-ng.conf.
# Сначала копируем example (файл должен появиться всегда), затем уточняем профиль в подпроцессе,
# чтобы сбой source detect_resources.sh не ронял start.sh.
ensure_syslog_profile() {
    local dest="${SCRIPT_DIR}/syslog-ng.d/zz_profile.conf"
    local example="${SCRIPT_DIR}/syslog-ng.d/zz_profile.conf.example"
    local detect="${SCRIPT_DIR}/deploy/common/detect_resources.sh"
    local profile=""

    mkdir -p "${SCRIPT_DIR}/syslog-ng.d"
    if [[ ! -f "$dest" && -f "$example" ]]; then
        cp "$example" "$dest"
        log "syslog-ng буферы: создан syslog-ng.d/zz_profile.conf из example"
    fi

    if [[ -f install-profile.json ]]; then
        profile="$(grep -oE '"profile"[[:space:]]*:[[:space:]]*"[^"]+"' install-profile.json 2>/dev/null \
            | head -1 \
            | sed -E 's/.*"([^"]+)"[[:space:]]*$/\1/' || true)"
    fi

    if [[ -f "$detect" && -n "$profile" ]]; then
        if bash -c '
            set -euo pipefail
            # shellcheck disable=SC1090
            source "$1"
            profile_params "$2"
            write_syslog_profile "$3" "$2"
        ' bash "$detect" "$profile" "$SCRIPT_DIR"; then
            log "syslog-ng буферы: syslog-ng.d/zz_profile.conf (профиль ${profile})"
        else
            log "syslog-ng буферы: профиль ${profile} не применён, оставлен ${dest}"
        fi
    fi
}

warn_old_syslog_conf() {
    local conf="${SCRIPT_DIR}/syslog-ng.conf"
    if grep -qE 'log_iw_size\(' "$conf" 2>/dev/null; then
        log "ВНИМАНИЕ: ${conf} старый (global log_iw_size). На сервере: git fetch origin && git reset --hard origin/main"
    fi
}

prepare_mounts() {
    mkdir -p frontend certs
    if [[ ! -f install-profile.json ]]; then
        echo '{}' > install-profile.json
    fi
    ensure_syslog_profile
    warn_old_syslog_conf
    if [[ -f "${SCRIPT_DIR}/deploy/common/install_meta.sh" ]]; then
        # shellcheck source=deploy/common/install_meta.sh
        source "${SCRIPT_DIR}/deploy/common/install_meta.sh"
        nm_write_install_meta "$SCRIPT_DIR"
    elif [[ ! -f install-meta.json ]]; then
        echo '{"version":"unknown","source":"unknown","ref":"unknown","commit":"","display":"unknown"}' > install-meta.json
    fi
}

# Compose profiles для опциональных сервисов (syslog / stats).
# Если в .env ещё нет COMPOSE_PROFILES — включаем оба (совместимость со старыми установками).
ensure_compose_profiles() {
    local env_file=".env"
    if [[ -f "$env_file" ]] && grep -qE '^[[:space:]]*COMPOSE_PROFILES=' "$env_file"; then
        # docker compose подхватит значение из .env автоматически
        local profiles
        profiles="$(grep -E '^[[:space:]]*COMPOSE_PROFILES=' "$env_file" | tail -n1 | cut -d= -f2- || true)"
        log "Compose-профили: ${profiles:-"(нет — только ядро)"}"
        return 0
    fi
    if [[ -n "${COMPOSE_PROFILES:-}" ]]; then
        log "Compose-профили (env): ${COMPOSE_PROFILES}"
        return 0
    fi
    export COMPOSE_PROFILES="${COMPOSE_PROFILES:-syslog,stats}"
    log "Compose-профили (по умолчанию): ${COMPOSE_PROFILES}"
}

# Генерирует API_AUTH_TOKEN, SESSION_SECRET, пароль admin и CLICKHOUSE_PASSWORD в .env, если ещё нет.
# Operator не сеем — УЗ создают в UI после входа.
ensure_api_auth_token() {
    local env_file=".env"
    local need_token=1 need_session=1 need_admin=1 need_ch=1
    NM_ADMIN_PASSWORD_PRINT="${NM_ADMIN_PASSWORD_PRINT:-0}"
    if [[ -f "$env_file" ]] && grep -qE '^[[:space:]]*API_AUTH_TOKEN=.+' "$env_file"; then
        need_token=0
    fi
    if [[ -f "$env_file" ]] && grep -qE '^[[:space:]]*SESSION_SECRET=.+' "$env_file"; then
        need_session=0
    fi
    if [[ -f "$env_file" ]] && grep -qE '^[[:space:]]*AUTH_ADMIN_PASSWORD=.+' "$env_file"; then
        need_admin=0
    fi
    if [[ -f "$env_file" ]] && grep -qE '^[[:space:]]*CLICKHOUSE_PASSWORD=.+' "$env_file"; then
        need_ch=0
    fi
    if [[ -n "${AUTH_ADMIN_PASSWORD:-}" ]]; then
        need_admin=0
        if [[ ! -f "$env_file" ]] || ! grep -qE '^[[:space:]]*AUTH_ADMIN_PASSWORD=' "$env_file"; then
            {
                echo ""
                echo "AUTH_ADMIN_USER=${AUTH_ADMIN_USER:-admin}"
                echo "AUTH_ADMIN_PASSWORD=${AUTH_ADMIN_PASSWORD}"
                echo "AUTH_ADMIN_MUST_RESET=${AUTH_ADMIN_MUST_RESET:-0}"
            } >>"$env_file"
        fi
    fi
    if (( need_token == 0 && need_session == 0 && need_admin == 0 && need_ch == 0 )); then
        return 0
    fi
    local rand admin_user admin_pass ch_pass
    rand="$(nm_rand_hex 32)"
    admin_user="${AUTH_ADMIN_USER:-admin}"
    if (( need_admin == 1 )); then
        admin_pass="$(nm_rand_hex 12)"
        AUTH_ADMIN_USER="$admin_user"
        AUTH_ADMIN_PASSWORD="$admin_pass"
        AUTH_ADMIN_MUST_RESET=1
        NM_ADMIN_PASSWORD_PRINT=1
        export AUTH_ADMIN_USER AUTH_ADMIN_PASSWORD AUTH_ADMIN_MUST_RESET NM_ADMIN_PASSWORD_PRINT
    fi
    if (( need_ch == 1 )); then
        ch_pass="$(nm_rand_hex 32)"
    fi
    {
        echo ""
        if (( need_token == 1 )); then
            echo "# Сгенерировано start.sh — токен для мутирующих API"
            echo "API_AUTH_TOKEN=${rand}"
            rand="$(nm_rand_hex 32)"
        fi
        if (( need_session == 1 )); then
            echo "# Сгенерировано start.sh — секрет cookie-сессий UI"
            echo "SESSION_SECRET=${rand}"
        fi
        if (( need_admin == 1 )); then
            echo "# Сгенерировано start.sh — пароль admin (смените при первом входе)"
            echo "AUTH_ADMIN_USER=${admin_user}"
            echo "AUTH_ADMIN_PASSWORD=${admin_pass}"
            echo "AUTH_ADMIN_MUST_RESET=1"
        fi
        if (( need_ch == 1 )); then
            echo "# Сгенерировано start.sh — пароль ClickHouse (docker-сеть; порты не публикуются)"
            echo "CLICKHOUSE_PASSWORD=${ch_pass}"
        fi
    } >>"$env_file"
    log "Сгенерированы секреты в .env (API_AUTH_TOKEN / SESSION_SECRET / пароль admin / CLICKHOUSE_PASSWORD, если отсутствовали)"
}

# Не даём поднять стек без секретов / с legacy-плейсхолдерами.
# Обход только через NM_ALLOW_INSECURE=1 (local/dev).
check_auth_secrets() {
    local env_file=".env"
    local allow="${NM_ALLOW_INSECURE:-0}"
    local token="" secret=""
    local admin_pass="" operator_pass=""

    if [[ -f "$env_file" ]]; then
        allow="$(grep -E '^[[:space:]]*NM_ALLOW_INSECURE=' "$env_file" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        [[ -z "$allow" ]] && allow="${NM_ALLOW_INSECURE:-0}"
        token="$(grep -E '^[[:space:]]*API_AUTH_TOKEN=' "$env_file" | tail -n1 | cut -d= -f2- || true)"
        secret="$(grep -E '^[[:space:]]*SESSION_SECRET=' "$env_file" | tail -n1 | cut -d= -f2- || true)"
        admin_pass="$(grep -E '^[[:space:]]*AUTH_ADMIN_PASSWORD=' "$env_file" | tail -n1 | cut -d= -f2- || true)"
        operator_pass="$(grep -E '^[[:space:]]*AUTH_OPERATOR_PASSWORD=' "$env_file" | tail -n1 | cut -d= -f2- || true)"
    else
        token="${API_AUTH_TOKEN:-}"
        secret="${SESSION_SECRET:-}"
        admin_pass="${AUTH_ADMIN_PASSWORD:-}"
        operator_pass="${AUTH_OPERATOR_PASSWORD:-}"
    fi
    # Env overrides file when set explicitly for this run.
    [[ -n "${NM_ALLOW_INSECURE:-}" ]] && allow="$NM_ALLOW_INSECURE"
    [[ -n "${API_AUTH_TOKEN:-}" ]] && token="$API_AUTH_TOKEN"
    [[ -n "${SESSION_SECRET:-}" ]] && secret="$SESSION_SECRET"
    [[ -n "${AUTH_ADMIN_PASSWORD:-}" ]] && admin_pass="$AUTH_ADMIN_PASSWORD"
    [[ -n "${AUTH_OPERATOR_PASSWORD:-}" ]] && operator_pass="$AUTH_OPERATOR_PASSWORD"

    if [[ "$allow" == "1" || "$allow" == "true" || "$allow" == "yes" ]]; then
        log "ВНИМАНИЕ: NM_ALLOW_INSECURE=$allow — небезопасные плейсхолдеры разрешены (только для разработки)"
        return 0
    fi

    if [[ -z "$token" || -z "$secret" ]]; then
        log "ОШИБКА: в .env обязательны API_AUTH_TOKEN и SESSION_SECRET."
        log "  Запустите через ./start.sh (сгенерирует секреты) или задайте вручную."
        log "  Только для локальной разработки: NM_ALLOW_INSECURE=1 ./start.sh"
        exit 1
    fi

    if [[ "$token" == "dev-insecure-change-me" || "$secret" == "dev-session-secret-change-me" ]]; then
        log "ОШИБКА: в .env всё ещё стоят небезопасные плейсхолдеры секретов."
        log "  Запустите через ./start.sh (сгенерирует token/secret) или задайте уникальные API_AUTH_TOKEN / SESSION_SECRET."
        log "  Только для локальной разработки: NM_ALLOW_INSECURE=1 ./start.sh"
        exit 1
    fi

    if [[ "$admin_pass" == "admin" || "$operator_pass" == "operator" ]]; then
        log "ВНИМАНИЕ: в .env пароли по умолчанию (admin/operator) — смените после первого входа"
    fi
}

wait_for_health() {
    local url="$1"
    local timeout="$2"
    local elapsed=0
    local curl_opts=(-fsS)
    # Свои/самоподписанные сертификаты — не валим healthcheck на verify.
    if [[ "$url" == https://* ]]; then
        curl_opts+=(-k)
    fi

    log "Ожидание health endpoint: $url (таймаут ${timeout}с)..."
    while (( elapsed < timeout )); do
        if curl "${curl_opts[@]}" "$url" >/dev/null 2>&1; then
            log "Health OK."
            return 0
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done

    log "Проверка health не успела за ${timeout}с."
    return 1
}

main() {
    require_docker

    [[ -f docker-compose.yml ]] || { log "docker-compose.yml не найден в $SCRIPT_DIR"; exit 1; }

    prepare_mounts
    ensure_api_auth_token
    check_auth_secrets
    ensure_compose_profiles

    if (( HTTPS_ON == 1 )); then
        log "HTTPS: включён (сертификаты на месте; порт хоста ${HTTPS_PORT})"
    else
        log "HTTPS: выкл. (положите PEM в ./certs — см. certs/README.md)"
    fi

    log "Запуск стека Docker Compose..."
    if [[ "$DO_BUILD" == "1" ]]; then
        nm_compose "$SCRIPT_DIR" up -d --build
    else
        nm_compose "$SCRIPT_DIR" up -d
    fi

    if ! wait_for_health "$HEALTH_URL" "$HEALTH_TIMEOUT"; then
        log "Backend не стал healthy вовремя."
        nm_compose "$SCRIPT_DIR" ps || true
        log "----- лог backend (хвост) -----"
        nm_compose "$SCRIPT_DIR" logs --tail=50 backend || true
        exit 1
    fi

    IP_ADDR="$(hostname -I 2>/dev/null | awk '{print $1}')"
    if [[ -z "${IP_ADDR:-}" ]]; then
        IP_ADDR="127.0.0.1"
    fi

    if [[ -f install-profile.json ]]; then
        local profile=""
        if grep -q '"profile"' install-profile.json 2>/dev/null; then
            profile="$(grep -oE '"profile"[[:space:]]*:[[:space:]]*"[^"]+"' install-profile.json 2>/dev/null \
                | head -1 \
                | sed -E 's/.*"([^"]+)"[[:space:]]*$/\1/' || true)"
        fi
        if [[ -n "$profile" ]]; then
            log "Профиль установки: ${profile} (см. install-profile.json)"
        fi
    fi

    log "Стек запущен."
    if (( HTTPS_ON == 1 )); then
        if [[ "${HTTPS_PORT}" == "443" ]]; then
            log "Веб-интерфейс: https://${IP_ADDR}"
            log "Страница входа: https://${IP_ADDR}/login.html"
            log "Health check  : https://${IP_ADDR}/api/ready"
        else
            log "Веб-интерфейс: https://${IP_ADDR}:${HTTPS_PORT}"
            log "Страница входа: https://${IP_ADDR}:${HTTPS_PORT}/login.html"
            log "Health check  : https://${IP_ADDR}:${HTTPS_PORT}/api/ready"
        fi
    elif [[ "${HTTP_PORT}" == "80" ]]; then
        log "Веб-интерфейс: http://${IP_ADDR}"
        log "Страница входа: http://${IP_ADDR}/login.html"
        log "Health check  : http://${IP_ADDR}/api/ready"
    else
        log "Веб-интерфейс: http://${IP_ADDR}:${HTTP_PORT}"
        log "Страница входа: http://${IP_ADDR}:${HTTP_PORT}/login.html"
        log "Health check  : http://${IP_ADDR}:${HTTP_PORT}/api/ready"
    fi
    log "Первый вход: пользователь ${AUTH_ADMIN_USER:-admin}"
    if [[ "${NM_ADMIN_PASSWORD_PRINT:-0}" == "1" && -n "${AUTH_ADMIN_PASSWORD:-}" ]]; then
        log "Пароль (покажется только сейчас): ${AUTH_ADMIN_PASSWORD}"
        log "При входе система попросит сменить пароль."
    else
        log "Пароль задан при установке или лежит в .env (AUTH_ADMIN_PASSWORD)."
    fi
}

main "$@"
