#!/usr/bin/env bash
# Общий вызов docker compose с опциональным docker-compose.https.yml.
# Использование: source deploy/common/compose.sh && ga_compose up -d
#
# HTTPS включается, если:
#   - HTTPS_ENABLED=1|true|yes|on, или
#   - HTTPS_ENABLED=auto|пусто и есть ./certs/fullchain.pem + privkey.pem
# Выключить явно: HTTPS_ENABLED=0

_ga_compose_truthy() {
    case "${1:-}" in
        1|true|TRUE|yes|YES|on|ON) return 0 ;;
        *) return 1 ;;
    esac
}

_ga_compose_falsy() {
    case "${1:-}" in
        0|false|FALSE|no|NO|off|OFF) return 0 ;;
        *) return 1 ;;
    esac
}

_ga_compose_env_get() {
    local file="$1" key="$2" v=""
    [[ -f "$file" ]] || { echo ""; return 0; }
    v="$(grep -E "^[[:space:]]*${key}=" "$file" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
    v="${v%%$'\r'}"
    echo "$v"
}

# Пустое / только пробелы / "" — не годится для ${VAR:?} в compose.
_ga_compose_value_blank() {
    local v="${1-}"
    v="${v#"${v%%[![:space:]]*}"}"
    v="${v%"${v##*[![:space:]]}"}"
    if [[ "$v" == \"*\" ]]; then
        v="${v#\"}"
        v="${v%\"}"
    elif [[ "$v" == \'*\' ]]; then
        v="${v#\'}"
        v="${v%\'}"
    fi
    v="${v#"${v%%[![:space:]]*}"}"
    v="${v%"${v##*[![:space:]]}"}"
    [[ -z "$v" ]]
}

# docker compose down/stop тоже парсит YAML и падает на ${CLICKHOUSE_PASSWORD:?},
# даже если пароль для остановки не нужен. Заглушки только в текущем процессе;
# в .env ничего не пишем. ./start.sh / up по-прежнему fail-closed.
_ga_compose_fill_stop_placeholders() {
    local root="$1"
    local env_file="${root}/.env"
    local key val
    for key in CLICKHOUSE_PASSWORD INGEST_SHARED_SECRET API_AUTH_TOKEN SESSION_SECRET AUTH_ADMIN_PASSWORD; do
        val="${!key-}"
        if _ga_compose_value_blank "$val"; then
            val="$(_ga_compose_env_get "$env_file" "$key")"
        fi
        if _ga_compose_value_blank "$val"; then
            export "${key}=ga-compose-stop"
        fi
    done
}

_ga_compose_cmd_allows_placeholder_env() {
    local a
    for a in "$@"; do
        case "$a" in
            down|stop|kill|rm) return 0 ;;
        esac
    done
    return 1
}

# Каталог /opt/geoatlas даёт project=geoatlas, а сеть и контейнеры
# часто уже созданы проектом geoatlas (/opt/geoatlas). Имена
# clickhouse / ga-volume-perms зафиксированы в YAML — второй проект не стартует.
# COMPOSE_PROJECT_NAME из окружения или .env не трогаем.
_ga_compose_adopt_existing_project() {
    local root="${1:-.}"
    local n proj from_env=""
    [[ -n "${COMPOSE_PROJECT_NAME:-}" ]] && return 0
    from_env="$(_ga_compose_env_get "${root}/.env" COMPOSE_PROJECT_NAME)"
    if [[ -n "$from_env" ]]; then
        export COMPOSE_PROJECT_NAME="$from_env"
        return 0
    fi
    command -v docker >/dev/null 2>&1 || return 0
    for n in clickhouse ga-volume-perms backend frontend syslog-ng stats-collector dozzle; do
        proj="$(docker inspect -f '{{ index .Config.Labels "com.docker.compose.project" }}' "$n" 2>/dev/null || true)"
        case "$proj" in
            geoatlas)
                export COMPOSE_PROJECT_NAME="$proj"
                echo "[compose] COMPOSE_PROJECT_NAME=${proj} (контейнер ${n} уже есть; тома не трогаем)" >&2
                if [[ -f "${root}/.env" ]] && ! grep -qE '^[[:space:]]*COMPOSE_PROJECT_NAME=' "${root}/.env"; then
                    printf '\nCOMPOSE_PROJECT_NAME=%s\n' "$proj" >>"${root}/.env"
                    echo "[compose] записали COMPOSE_PROJECT_NAME=${proj} в ${root}/.env" >&2
                fi
                return 0
                ;;
        esac
    done
}

# $1 — корень проекта (по умолчанию cwd)
ga_https_active() {
    local root="${1:-.}"
    local env_file="${root}/.env"
    local enabled cert key

    enabled="${HTTPS_ENABLED:-}"
    [[ -z "$enabled" ]] && enabled="$(_ga_compose_env_get "$env_file" HTTPS_ENABLED)"
    [[ -z "$enabled" ]] && enabled="auto"

    cert="${SSL_CERT_FILE:-}"
    [[ -z "$cert" ]] && cert="$(_ga_compose_env_get "$env_file" SSL_CERT_FILE)"
    [[ -z "$cert" ]] && cert="${root}/certs/fullchain.pem"

    key="${SSL_KEY_FILE:-}"
    [[ -z "$key" ]] && key="$(_ga_compose_env_get "$env_file" SSL_KEY_FILE)"
    [[ -z "$key" ]] && key="${root}/certs/privkey.pem"

    # Host paths for compose decision (container paths differ).
    case "$cert" in
        /etc/nginx/certs/*) cert="${root}/certs/${cert##*/}" ;;
    esac
    case "$key" in
        /etc/nginx/certs/*) key="${root}/certs/${key##*/}" ;;
    esac

    if _ga_compose_falsy "$enabled"; then
        return 1
    fi
    if [[ ! -f "$cert" || ! -f "$key" ]]; then
        if _ga_compose_truthy "$enabled"; then
            return 1
        fi
        return 1
    fi
    if _ga_compose_truthy "$enabled" || [[ "$enabled" == "auto" || -z "$enabled" ]]; then
        return 0
    fi
    return 1
}

ga_compose_args() {
    local root="${1:-.}"
    local args=(-f "${root}/docker-compose.yml")
    if [[ -f "${root}/docker-compose.override.yml" ]]; then
        args+=(-f "${root}/docker-compose.override.yml")
    fi
    if ga_https_active "$root"; then
        args+=(-f "${root}/docker-compose.https.yml")
    fi
    printf '%s\n' "${args[@]}"
}

ga_compose() {
    local root="."
    # If first arg is an existing directory with docker-compose.yml, treat as root.
    if [[ $# -gt 0 && -f "${1}/docker-compose.yml" ]]; then
        root="$1"
        shift
    fi
    local -a files=()
    local line
    while IFS= read -r line; do
        [[ -n "$line" ]] && files+=("$line")
    done < <(ga_compose_args "$root")
    _ga_compose_adopt_existing_project "$root"
    if _ga_compose_cmd_allows_placeholder_env "$@"; then
        _ga_compose_fill_stop_placeholders "$root"
    fi
    (cd "$root" && docker compose "${files[@]}" "$@")
}
