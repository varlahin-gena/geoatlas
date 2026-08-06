#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
cd "$SCRIPT_DIR"

HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-120}"
DO_BUILD="${DO_BUILD:-1}"          # 1 = пересобирать образы, 0 = только поднять
# Порт UI из .env (HTTP_PORT); HEALTH_URL можно переопределить явно.
_nm_http_port_from_env() {
    local p="80"
    if [[ -f .env ]]; then
        local v
        v="$(grep -E '^[[:space:]]*HTTP_PORT=' .env 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        [[ -n "$v" ]] && p="$v"
    fi
    echo "${HTTP_PORT:-$p}"
}
HTTP_PORT="$(_nm_http_port_from_env)"
if [[ "${HTTP_PORT}" == "80" ]]; then
    HEALTH_URL="${HEALTH_URL:-http://127.0.0.1/api/health}"
else
    HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:${HTTP_PORT}/api/health}"
fi

log() { echo "[$(date +'%F %T')] $*"; }

trap 'log "ERROR at line ${LINENO} (exit code $?)."' ERR

require_docker() {
    if ! command -v docker >/dev/null 2>&1; then
        echo "Docker not found. Please install Docker first."
        exit 1
    fi
    if ! docker compose version >/dev/null 2>&1; then
        echo "docker compose plugin not found. Install docker-compose-plugin."
        exit 1
    fi
}

prepare_mounts() {
    mkdir -p frontend
    if [[ ! -f install-profile.json ]]; then
        echo '{}' > install-profile.json
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
        log "Compose profiles: ${profiles:-"(none — core only)"}"
        return 0
    fi
    if [[ -n "${COMPOSE_PROFILES:-}" ]]; then
        log "Compose profiles (env): ${COMPOSE_PROFILES}"
        return 0
    fi
    export COMPOSE_PROFILES="${COMPOSE_PROFILES:-syslog,stats}"
    log "Compose profiles (default): ${COMPOSE_PROFILES}"
}

# Генерирует API_AUTH_TOKEN и SESSION_SECRET в .env, если ещё нет.
ensure_api_auth_token() {
    local env_file=".env"
    local need_token=1 need_session=1
    if [[ -f "$env_file" ]] && grep -qE '^[[:space:]]*API_AUTH_TOKEN=' "$env_file"; then
        need_token=0
    fi
    if [[ -f "$env_file" ]] && grep -qE '^[[:space:]]*SESSION_SECRET=' "$env_file"; then
        need_session=0
    fi
    if (( need_token == 0 && need_session == 0 )); then
        return 0
    fi
    local rand
    if command -v openssl >/dev/null 2>&1; then
        rand="$(openssl rand -hex 32)"
    else
        rand="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
    fi
    {
        echo ""
        if (( need_token == 1 )); then
            echo "# Сгенерировано start.sh — токен для мутирующих API"
            echo "API_AUTH_TOKEN=${rand}"
            rand="$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
        fi
        if (( need_session == 1 )); then
            echo "# Сгенерировано start.sh — секрет cookie-сессий UI"
            echo "SESSION_SECRET=${rand}"
        fi
        if ! grep -qE '^[[:space:]]*AUTH_ADMIN_PASSWORD=' "$env_file" 2>/dev/null; then
            echo "# Смените пароли после первого входа"
            echo "AUTH_ADMIN_USER=admin"
            echo "AUTH_ADMIN_PASSWORD=admin"
            echo "AUTH_OPERATOR_USER=operator"
            echo "AUTH_OPERATOR_PASSWORD=operator"
        fi
    } >>"$env_file"
    log "Generated auth secrets in .env (API_AUTH_TOKEN / SESSION_SECRET / default users if missing)"
}

# Не даём поднять стек с compose-плейсхолдерами, если забыли сгенерировать .env.
# Обход только через NM_ALLOW_INSECURE=1 (local/dev).
check_auth_secrets() {
    local env_file=".env"
    local allow="${NM_ALLOW_INSECURE:-0}"
    local token="" secret=""

    if [[ -f "$env_file" ]]; then
        allow="$(grep -E '^[[:space:]]*NM_ALLOW_INSECURE=' "$env_file" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        [[ -z "$allow" ]] && allow="${NM_ALLOW_INSECURE:-0}"
        token="$(grep -E '^[[:space:]]*API_AUTH_TOKEN=' "$env_file" | tail -n1 | cut -d= -f2- || true)"
        secret="$(grep -E '^[[:space:]]*SESSION_SECRET=' "$env_file" | tail -n1 | cut -d= -f2- || true)"
    else
        token="${API_AUTH_TOKEN:-}"
        secret="${SESSION_SECRET:-}"
    fi
    # Env overrides file when set explicitly for this run.
    [[ -n "${NM_ALLOW_INSECURE:-}" ]] && allow="$NM_ALLOW_INSECURE"
    [[ -n "${API_AUTH_TOKEN:-}" ]] && token="$API_AUTH_TOKEN"
    [[ -n "${SESSION_SECRET:-}" ]] && secret="$SESSION_SECRET"

    if [[ "$allow" == "1" || "$allow" == "true" || "$allow" == "yes" ]]; then
        log "WARNING: NM_ALLOW_INSECURE=$allow — insecure placeholders allowed (dev only)"
        return 0
    fi

    if [[ "$token" == "dev-insecure-change-me" || "$secret" == "dev-session-secret-change-me" ]]; then
        log "ERROR: .env still has docker-compose placeholder secrets."
        log "  Run via ./start.sh (generates token/secret) or set unique API_AUTH_TOKEN / SESSION_SECRET."
        log "  For local only: NM_ALLOW_INSECURE=1 ./start.sh"
        exit 1
    fi
}

wait_for_health() {
    local url="$1"
    local timeout="$2"
    local elapsed=0

    log "Waiting for health endpoint: $url (timeout ${timeout}s)..."
    while (( elapsed < timeout )); do
        if curl -fsS "$url" >/dev/null 2>&1; then
            log "Health OK."
            return 0
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done

    log "Health check timed out after ${timeout}s."
    return 1
}

main() {
    require_docker

    [[ -f docker-compose.yml ]] || { log "docker-compose.yml not found in $SCRIPT_DIR"; exit 1; }

    prepare_mounts
    ensure_api_auth_token
    check_auth_secrets
    ensure_compose_profiles

    log "Starting Docker Compose stack..."
    if [[ "$DO_BUILD" == "1" ]]; then
        docker compose up -d --build
    else
        docker compose up -d
    fi

    if ! wait_for_health "$HEALTH_URL" "$HEALTH_TIMEOUT"; then
        log "Backend did not become healthy in time."
        docker compose ps || true
        log "----- backend logs (tail) -----"
        docker compose logs --tail=50 backend || true
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
            log "Install profile: ${profile} (see install-profile.json)"
        fi
    fi

    log "Stack is up."
    if [[ "${HTTP_PORT}" == "80" ]]; then
        log "Web interface: http://${IP_ADDR}"
        log "Login page   : http://${IP_ADDR}/login.html"
        log "Health check : http://${IP_ADDR}/api/health"
    else
        log "Web interface: http://${IP_ADDR}:${HTTP_PORT}"
        log "Login page   : http://${IP_ADDR}:${HTTP_PORT}/login.html"
        log "Health check : http://${IP_ADDR}:${HTTP_PORT}/api/health"
    fi
    log "Default login: admin / admin (смена пароля при первом входе)"
}

main "$@"
