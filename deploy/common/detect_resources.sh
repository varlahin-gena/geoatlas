#!/usr/bin/env bash
# Определяет ресурсы хоста и генерирует оптимальную конфигурацию для docker compose.
# Использование:
#   source deploy/common/detect_resources.sh
#   apply_resource_profile /opt/geoatlas
#
# Переменные окружения:
#   GA_AUTO_PROFILE=1     — принять рекомендованный профиль без вопросов
#   GA_FORCE_PROFILE=...  — принудительно выбрать профиль (tiny|small|medium|large|xlarge)
#   GA_SKIP_PROFILE=1     — не генерировать override (оставить значения по умолчанию)
# Модули (см. select_modules.sh) сохраняются в .env при пересчёте профиля.

set -Eeuo pipefail

_ga_log() { echo "[$(date +'%F %T')] [resources] $*"; }

_ga_res_ensure_ui() {
    if ! declare -F ga_ui_radiolist >/dev/null 2>&1; then
        local dir helper
        dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
        helper="${dir}/ui.sh"
        if [[ -f "$helper" ]]; then
            # shellcheck source=deploy/common/ui.sh
            source "$helper"
        else
            return 1
        fi
    fi
    if [[ -z "${GA_UI_BACKEND:-}" ]] && declare -F ga_ui_init >/dev/null 2>&1; then
        ga_ui_init
    fi
    return 0
}

detect_cpu_cores() {
    local cores
    cores="$(nproc 2>/dev/null || true)"
    if [[ -z "$cores" || "$cores" -lt 1 ]]; then
        cores="$(grep -c '^processor' /proc/cpuinfo 2>/dev/null || echo 1)"
    fi
    echo "$cores"
}

detect_ram_mb() {
    awk '/MemTotal:/ { printf "%d\n", int($2 / 1024) }' /proc/meminfo 2>/dev/null || echo 0
}

detect_disk_gb_avail() {
    local path="${1:-/}"
    local avail
    avail="$(df -BG "$path" 2>/dev/null | awk 'NR==2 { gsub(/G/, "", $4); print $4 }')"
    if [[ -z "$avail" || ! "$avail" =~ ^[0-9]+$ ]]; then
        echo 0
    else
        echo "$avail"
    fi
}

detect_cgroup_version() {
    local fs_type
    fs_type="$(stat -fc %T /sys/fs/cgroup 2>/dev/null || echo unknown)"
    case "$fs_type" in
        cgroup2fs) echo "v2" ;;
        tmpfs)     echo "v1" ;;
        *)         echo "unknown" ;;
    esac
}

recommend_profile() {
    local cpu="$1"
    local ram_mb="$2"

    if (( ram_mb <= 4096 )) || (( cpu <= 2 && ram_mb <= 6144 )); then
        echo "tiny"
    elif (( cpu <= 4 && ram_mb <= 8192 )); then
        echo "small"
    elif (( cpu <= 8 && ram_mb <= 16384 )); then
        echo "medium"
    elif (( cpu <= 16 && ram_mb <= 32768 )); then
        echo "large"
    else
        echo "xlarge"
    fi
}

profile_params() {
    local profile="$1"
    case "$profile" in
        tiny)
            CH_MEM_GB=2; CH_CPUS=1
            BE_MEM_GB=1; BE_CPUS=1
            # Tiny: минимизируем churn частей; пусть ingest буферизует короткие bursts.
            BE_WORKERS=1; BE_QUEUE=50000; BE_BATCH=5000; BE_FLUSH=5
            BE_CH_CONNS=1
            # 2×fifo сообщений в RAM (flow-control-window) + ~80 MiB процесс.
            SYSLOG_MEM_MB=512; SYSLOG_CPUS=1
            SYSLOG_FIFO=10000; SYSLOG_MEM_BUF=33554432; SYSLOG_DISK_BUF=268435456
            SYSLOG_UDP_RCVBUF=16777216; SYSLOG_IW_SIZE=1000
            ;;
        small)
            # 4 CPU / 8 GiB: CH чаще упирается в inserts/merges, чем ingest.
            # Держим умеренный параллелизм и более редкие flush, чтобы не плодить мелкие parts.
            CH_MEM_GB=3; CH_CPUS=3
            BE_MEM_GB=2; BE_CPUS=2
            BE_WORKERS=2; BE_QUEUE=200000; BE_BATCH=20000; BE_FLUSH=5
            BE_CH_CONNS=2
            # 2×fifo сообщений в RAM (flow-control-window) + ~80 MiB процесс.
            SYSLOG_MEM_MB=768; SYSLOG_CPUS=1
            SYSLOG_FIFO=25000; SYSLOG_MEM_BUF=67108864; SYSLOG_DISK_BUF=536870912
            SYSLOG_UDP_RCVBUF=33554432; SYSLOG_IW_SIZE=2000
            ;;
        medium)
            CH_MEM_GB=6; CH_CPUS=4
            BE_MEM_GB=4; BE_CPUS=3
            BE_WORKERS=2; BE_QUEUE=300000; BE_BATCH=25000; BE_FLUSH=4
            BE_CH_CONNS=2
            SYSLOG_MEM_MB=1024; SYSLOG_CPUS=2
            SYSLOG_FIFO=50000; SYSLOG_MEM_BUF=100663296; SYSLOG_DISK_BUF=1073741824
            SYSLOG_UDP_RCVBUF=67108864; SYSLOG_IW_SIZE=4000
            ;;
        large)
            CH_MEM_GB=12; CH_CPUS=8
            BE_MEM_GB=8; BE_CPUS=6
            BE_WORKERS=3; BE_QUEUE=500000; BE_BATCH=30000; BE_FLUSH=3
            BE_CH_CONNS=3
            SYSLOG_MEM_MB=2048; SYSLOG_CPUS=3
            SYSLOG_FIFO=100000; SYSLOG_MEM_BUF=201326592; SYSLOG_DISK_BUF=2147483648
            SYSLOG_UDP_RCVBUF=134217728; SYSLOG_IW_SIZE=8000
            ;;
        xlarge)
            CH_MEM_GB=24; CH_CPUS=16
            BE_MEM_GB=16; BE_CPUS=12
            BE_WORKERS=4; BE_QUEUE=750000; BE_BATCH=40000; BE_FLUSH=2
            BE_CH_CONNS=4
            SYSLOG_MEM_MB=4096; SYSLOG_CPUS=4
            SYSLOG_FIFO=200000; SYSLOG_MEM_BUF=402653184; SYSLOG_DISK_BUF=4294967296
            SYSLOG_UDP_RCVBUF=134217728; SYSLOG_IW_SIZE=16000
            ;;
        *)
            _ga_log "Неизвестный профиль: $profile"
            return 1
            ;;
    esac
}

profile_capacity() {
    local profile="$1"
    case "$profile" in
        tiny)   EPS_MIN=500;   EPS_MAX=2000 ;;
        small)  EPS_MIN=5000;  EPS_MAX=12000 ;;
        medium) EPS_MIN=10000; EPS_MAX=25000 ;;
        large)  EPS_MIN=25000; EPS_MAX=80000 ;;
        xlarge) EPS_MIN=80000; EPS_MAX=200000 ;;
        *)      EPS_MIN=0;     EPS_MAX=0 ;;
    esac
}

profile_label_ru() {
    case "$1" in
        tiny)   echo "минимальный (≤2 CPU / ≤4 GiB RAM)" ;;
        small)  echo "малый (≤4 CPU / ≤8 GiB RAM)" ;;
        medium) echo "средний (≤8 CPU / ≤16 GiB RAM)" ;;
        large)  echo "большой (≤16 CPU / ≤32 GiB RAM)" ;;
        xlarge) echo "максимальный (>16 CPU / >32 GiB RAM)" ;;
        *)      echo "$1" ;;
    esac
}

calc_ch_max_query_bytes() {
    local ch_mem_gb="$1"
    awk -v gb="$ch_mem_gb" 'BEGIN { printf "%d", gb * 1024 * 1024 * 1024 * 0.40 }'
}

calc_external_spill_bytes() {
    local ch_mem_gb="$1"
    awk -v gb="$ch_mem_gb" 'BEGIN {
        spill = gb * 1024 * 1024 * 64
        if (spill < 134217728) spill = 134217728
        if (spill > 536870912) spill = 536870912
        printf "%d", spill
    }'
}

# Потоки на один тяжёлый GROUP BY: ~половина ядер CH, потолок 4 (не утилизировать все ядра картой).
calc_ch_max_threads() {
    local ch_cpus="$1"
    local t=$(( (ch_cpus + 1) / 2 ))
    (( t < 1 )) && t=1
    (( t > 4 )) && t=4
    echo "$t"
}

print_host_summary() {
    local cpu="$1" ram_mb="$2" disk_gb="$3" recommended="$4"
    local ram_gib
    ram_gib="$(awk -v mb="$ram_mb" 'BEGIN { printf "%.1f", mb / 1024 }')"
    profile_capacity "$recommended"
    local summary
    summary="CPU ядер: ${cpu}
RAM: ${ram_gib} GiB (${ram_mb} MiB)
Свободно диска: ${disk_gb} GiB (/)
Рекомендация: $(profile_label_ru "$recommended") [${recommended}]
Расчётная EPS: до ${EPS_MAX} событий/с"

    # Детальный dump — в лог (stderr/stdout для install log).
    echo ""
    echo "══════════════════════════════════════════════════════════"
    echo "  Анализ сервера"
    echo "══════════════════════════════════════════════════════════"
    echo "  CPU ядер       : $cpu"
    echo "  RAM            : ${ram_gib} GiB (${ram_mb} MiB)"
    echo "  Свободно диска : ${disk_gb} GiB (/)"
    echo "  Рекомендация   : $(profile_label_ru "$recommended") [$recommended]"
    echo "  Расчётная EPS  : до ${EPS_MAX} событий/с"
    echo "══════════════════════════════════════════════════════════"
    echo ""

    _ga_res_ensure_ui || true
    if declare -F ga_ui_msgbox >/dev/null 2>&1 && [[ "${GA_UI_BACKEND:-text}" != "text" ]]; then
        ga_ui_msgbox "Анализ сервера" "$summary" || true
    fi
}

is_valid_profile() {
    case "$1" in
        tiny|small|medium|large|xlarge) return 0 ;;
        *) return 1 ;;
    esac
}

print_profile_options() {
    local recommended="$1" p mark
    echo "" >&2
    echo "Доступные профили:" >&2
    echo "" >&2
    for p in tiny small medium large xlarge; do
        profile_params "$p"
        profile_capacity "$p"
        mark=""
        [[ "$p" == "$recommended" ]] && mark="  ← рекомендуется"
        printf "  %-8s %s%s\n" "$p" "$(profile_label_ru "$p")" "$mark" >&2
        printf "           ClickHouse %d GiB · Backend %d GiB · workers %d · EPS до %d/с\n" \
            "$CH_MEM_GB" "$BE_MEM_GB" "$BE_WORKERS" "$EPS_MAX" >&2
        echo "" >&2
    done
}

confirm_profile() {
    local recommended="$1"
    GA_SELECTED_PROFILE="$recommended"

    if [[ "${GA_AUTO_PROFILE:-0}" == "1" ]]; then
        _ga_log "GA_AUTO_PROFILE=1 — применяем профиль: $recommended"
        return 0
    fi

    if [[ -n "${GA_FORCE_PROFILE:-}" ]]; then
        if ! is_valid_profile "${GA_FORCE_PROFILE}"; then
            _ga_log "ОШИБКА: GA_FORCE_PROFILE='${GA_FORCE_PROFILE}' — неизвестный профиль."
            return 1
        fi
        GA_SELECTED_PROFILE="${GA_FORCE_PROFILE}"
        _ga_log "GA_FORCE_PROFILE — применяем профиль: ${GA_SELECTED_PROFILE}"
        return 0
    fi

    if [[ ! -t 0 ]]; then
        _ga_log "Нет TTY — применяем рекомендованный профиль: $recommended"
        return 0
    fi

    _ga_res_ensure_ui || true

    if declare -F ga_ui_radiolist >/dev/null 2>&1; then
        local -a items=()
        local p mark on_flag desc
        for p in tiny small medium large xlarge; do
            profile_params "$p"
            profile_capacity "$p"
            on_flag=OFF
            mark=""
            [[ "$p" == "$recommended" ]] && on_flag=ON && mark=" ← рекомендуется"
            desc="$(profile_label_ru "$p")${mark} · ${CH_MEM_GB}/${BE_MEM_GB}G"
            items+=("$p" "$desc" "$on_flag")
        done

        local answer
        if answer="$(ga_ui_radiolist \
            "Профиль производительности" \
            "Рекомендация по ресурсам хоста: ${recommended}
Выберите профиль:" \
            "${items[@]}")"; then
            case "$answer" in
                tiny|small|medium|large|xlarge)
                    GA_SELECTED_PROFILE="$answer"
                    if [[ "$answer" == "$recommended" ]]; then
                        _ga_log "Выбран профиль: ${GA_SELECTED_PROFILE} (рекомендация)"
                    else
                        _ga_log "Выбран профиль: ${GA_SELECTED_PROFILE}"
                    fi
                    return 0
                    ;;
                *)
                    _ga_log "Неизвестный ответ «${answer}» — применяем рекомендацию ${recommended}."
                    GA_SELECTED_PROFILE="$recommended"
                    return 0
                    ;;
            esac
        else
            _ga_log "Выбор профиля отменён — установка прервана."
            exit 0
        fi
    fi

    _ga_log "UI недоступен — применяем рекомендованный профиль: ${recommended}"
    GA_SELECTED_PROFILE="$recommended"
    return 0
}

_ga_env_get() {
    # Прочитать значение KEY= из .env (последнее вхождение). $1=file $2=key
    local file="$1" key="$2"
    [[ -f "$file" ]] || return 0
    grep -E "^[[:space:]]*${key}=" "$file" 2>/dev/null | tail -n1 | cut -d= -f2- || true
}

_ga_ensure_admin_auth() {
    if declare -F ga_rand_hex >/dev/null 2>&1; then
        return 0
    fi
    local dir helper
    dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
    helper="${dir}/admin_auth.sh"
    if [[ ! -f "$helper" ]]; then
        return 1
    fi
    # shellcheck source=deploy/common/admin_auth.sh
    source "$helper"
}

write_env_file() {
    local project_dir="$1" profile="$2"
    local env_file="${project_dir}/.env"
    local ch_max_mem spill existing_token="" existing_session="" existing_ch="" existing_ingest="" existing_ops="" token session ch_password ingest_secret ops_token

    ch_max_mem="$(calc_ch_max_query_bytes "$CH_MEM_GB")"
    spill="$(calc_external_spill_bytes "$CH_MEM_GB")"

    if [[ -f "$env_file" ]]; then
        existing_token="$(_ga_env_get "$env_file" API_AUTH_TOKEN)"
        existing_session="$(_ga_env_get "$env_file" SESSION_SECRET)"
        existing_ch="$(_ga_env_get "$env_file" CLICKHOUSE_PASSWORD)"
        existing_ingest="$(_ga_env_get "$env_file" INGEST_SHARED_SECRET)"
        existing_ops="$(_ga_env_get "$env_file" API_OPS_TOKEN)"
    fi
    if [[ -n "$existing_token" ]]; then
        token="$existing_token"
    elif command -v openssl >/dev/null 2>&1; then
        token="$(openssl rand -hex 32)"
    else
        token="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
    fi
    if [[ -n "$existing_ops" ]]; then
        ops_token="$existing_ops"
    elif command -v openssl >/dev/null 2>&1; then
        ops_token="$(openssl rand -hex 32)"
    else
        ops_token="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
    fi
    if [[ -n "$existing_session" ]]; then
        session="$existing_session"
    elif command -v openssl >/dev/null 2>&1; then
        session="$(openssl rand -hex 32)"
    else
        session="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
    fi
    if [[ -n "$existing_ingest" ]]; then
        ingest_secret="$existing_ingest"
    elif command -v openssl >/dev/null 2>&1; then
        ingest_secret="$(openssl rand -hex 32)"
    else
        ingest_secret="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
    fi
    ch_password="${CLICKHOUSE_PASSWORD:-$existing_ch}"
    if [[ -z "$ch_password" ]]; then
        if command -v openssl >/dev/null 2>&1; then
            ch_password="$(openssl rand -hex 32)"
        else
            ch_password="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
        fi
    fi

    local admin_user="${AUTH_ADMIN_USER:-admin}"
    local admin_pass="${AUTH_ADMIN_PASSWORD:-}"
    local admin_must="${AUTH_ADMIN_MUST_RESET:-}"
    local operator_user="${AUTH_OPERATOR_USER:-}"
    local operator_pass="${AUTH_OPERATOR_PASSWORD:-}"
    local operator_block=""
    local can_ask=0
    GA_ADMIN_PASSWORD_PRINT="${GA_ADMIN_PASSWORD_PRINT:-0}"
    local auth_disabled="${AUTH_DISABLED:-false}"
    local api_auth_disabled="${API_AUTH_DISABLED:-false}"
    local allow_insecure="${GA_ALLOW_INSECURE:-0}"
    local mod_auth="${GA_MODULE_AUTH:-1}"
    local mod_api_auth="${GA_MODULE_API_AUTH:-1}"
    local mod_syslog="${GA_MODULE_SYSLOG:-1}"
    local mod_stats="${GA_MODULE_STATS:-1}"
    local mod_reputation="${GA_MODULE_REPUTATION:-1}"
    local reputation_fetch_enabled="${REPUTATION_FETCH_ENABLED:-true}"
    local compose_profiles="${GA_COMPOSE_PROFILES:-${COMPOSE_PROFILES:-syslog,stats}}"
    local http_port="${HTTP_PORT:-80}"
    local https_enabled="${HTTPS_ENABLED:-auto}"
    local https_port="${HTTPS_PORT:-443}"
    local http_redirect="${HTTP_REDIRECT:-1}"

    if [[ -f "$env_file" ]]; then
        local v
        v="$(_ga_env_get "$env_file" AUTH_ADMIN_USER)"; [[ -n "$v" ]] && admin_user="$v"
        if [[ -z "$admin_pass" ]]; then
            v="$(_ga_env_get "$env_file" AUTH_ADMIN_PASSWORD)"; [[ -n "$v" ]] && admin_pass="$v"
        fi
        if [[ -z "$admin_must" ]]; then
            v="$(_ga_env_get "$env_file" AUTH_ADMIN_MUST_RESET)"; [[ -n "$v" ]] && admin_must="$v"
        fi
        if [[ -z "$operator_pass" ]]; then
            v="$(_ga_env_get "$env_file" AUTH_OPERATOR_PASSWORD)"; [[ -n "$v" ]] && operator_pass="$v"
        fi
        if [[ -z "$operator_user" ]]; then
            v="$(_ga_env_get "$env_file" AUTH_OPERATOR_USER)"; [[ -n "$v" ]] && operator_user="$v"
        fi
        # Флаги модулей: текущие env (после confirm_modules) имеют приоритет над файлом.
        if [[ -z "${GA_MODULE_AUTH:-}" ]]; then
            v="$(_ga_env_get "$env_file" GA_MODULE_AUTH)"; [[ -n "$v" ]] && mod_auth="$v"
            v="$(_ga_env_get "$env_file" GA_MODULE_API_AUTH)"; [[ -n "$v" ]] && mod_api_auth="$v"
            v="$(_ga_env_get "$env_file" GA_MODULE_SYSLOG)"; [[ -n "$v" ]] && mod_syslog="$v"
            v="$(_ga_env_get "$env_file" GA_MODULE_STATS)"; [[ -n "$v" ]] && mod_stats="$v"
            v="$(_ga_env_get "$env_file" GA_MODULE_REPUTATION)"; [[ -n "$v" ]] && mod_reputation="$v"
            v="$(_ga_env_get "$env_file" AUTH_DISABLED)"; [[ -n "$v" ]] && auth_disabled="$v"
            v="$(_ga_env_get "$env_file" API_AUTH_DISABLED)"; [[ -n "$v" ]] && api_auth_disabled="$v"
            v="$(_ga_env_get "$env_file" REPUTATION_FETCH_ENABLED)"; [[ -n "$v" ]] && reputation_fetch_enabled="$v"
            v="$(_ga_env_get "$env_file" GA_ALLOW_INSECURE)"; [[ -n "$v" ]] && allow_insecure="$v"
            v="$(_ga_env_get "$env_file" HTTP_PORT)"; [[ -n "$v" ]] && http_port="$v"
            v="$(_ga_env_get "$env_file" HTTPS_ENABLED)"; [[ -n "$v" ]] && https_enabled="$v"
            v="$(_ga_env_get "$env_file" HTTPS_PORT)"; [[ -n "$v" ]] && https_port="$v"
            v="$(_ga_env_get "$env_file" HTTP_REDIRECT)"; [[ -n "$v" ]] && http_redirect="$v"
            if grep -qE '^[[:space:]]*COMPOSE_PROFILES=' "$env_file" 2>/dev/null; then
                compose_profiles="$(_ga_env_get "$env_file" COMPOSE_PROFILES)"
            fi
        else
            [[ "${mod_auth}" == "1" ]] && auth_disabled="false" || auth_disabled="true"
            [[ "${mod_api_auth}" == "1" ]] && api_auth_disabled="false" || api_auth_disabled="true"
            [[ "${mod_reputation}" == "1" ]] && reputation_fetch_enabled="true" || reputation_fetch_enabled="false"
            compose_profiles="${GA_COMPOSE_PROFILES:-}"
            [[ -n "${HTTP_PORT:-}" ]] && http_port="$HTTP_PORT"
            [[ -n "${HTTPS_ENABLED:-}" ]] && https_enabled="$HTTPS_ENABLED"
            [[ -n "${HTTPS_PORT:-}" ]] && https_port="$HTTPS_PORT"
            [[ -n "${HTTP_REDIRECT:-}" ]] && http_redirect="$HTTP_REDIRECT"
        fi
    elif [[ -n "${GA_MODULE_AUTH:-}" ]]; then
        [[ "${mod_auth}" == "1" ]] && auth_disabled="false" || auth_disabled="true"
        [[ "${mod_api_auth}" == "1" ]] && api_auth_disabled="false" || api_auth_disabled="true"
        [[ "${mod_reputation}" == "1" ]] && reputation_fetch_enabled="true" || reputation_fetch_enabled="false"
        compose_profiles="${GA_COMPOSE_PROFILES:-}"
        [[ -n "${HTTP_PORT:-}" ]] && http_port="$HTTP_PORT"
        [[ -n "${HTTPS_ENABLED:-}" ]] && https_enabled="$HTTPS_ENABLED"
        [[ -n "${HTTPS_PORT:-}" ]] && https_port="$HTTPS_PORT"
        [[ -n "${HTTP_REDIRECT:-}" ]] && http_redirect="$HTTP_REDIRECT"
    fi

    if [[ "$auth_disabled" == "true" || "$api_auth_disabled" == "true" ]]; then
        allow_insecure="1"
    fi

    if [[ -z "$admin_pass" ]]; then
        _ga_res_ensure_ui || true
        if ! _ga_ensure_admin_auth; then
            _ga_log "ОШИБКА: не найден deploy/common/admin_auth.sh"
            exit 1
        fi
        if [[ "${GA_FULL_AUTO:-0}" != "1" ]] \
            && [[ "${GA_UI_AVAILABLE:-0}" == "1" ]] \
            && declare -F ga_ui_passwordbox >/dev/null 2>&1; then
            can_ask=1
        fi
        if (( can_ask == 1 )); then
            admin_pass="$(ga_prompt_admin_password "$admin_user")" || {
                _ga_log "Пароль администратора не задан — установка прервана."
                exit 1
            }
            admin_must=0
        else
            admin_pass="$(ga_rand_hex 12)"
            admin_must=1
            GA_ADMIN_PASSWORD_PRINT=1
            _ga_log "Сгенерирован пароль администратора (один раз в конце установки / ./start.sh)."
        fi
    elif [[ -z "$admin_must" ]]; then
        if _ga_ensure_admin_auth && ga_password_is_weak "$admin_user" "$admin_pass"; then
            admin_must=1
        else
            admin_must=0
        fi
    fi
    export AUTH_ADMIN_USER="$admin_user"
    export AUTH_ADMIN_PASSWORD="$admin_pass"
    export AUTH_ADMIN_MUST_RESET="$admin_must"
    export GA_ADMIN_PASSWORD_PRINT

    if [[ -n "$operator_pass" && -z "$operator_user" ]]; then
        operator_user="operator"
    fi
    if [[ -n "$operator_pass" ]]; then
        operator_block="AUTH_OPERATOR_USER=${operator_user}
AUTH_OPERATOR_PASSWORD=${operator_pass}"
    fi

    local syslog_stats_url=""
    if [[ "$mod_syslog" == "1" ]]; then
        syslog_stats_url="http://syslog-ng:9577/stats"
    fi

    cat >"$env_file" <<EOF
# Сгенерировано detect_resources.sh — не редактируйте вручную, перезапустите tune-resources.sh
GA_INSTALL_PROFILE=${profile}
GA_CH_MEM_GB=${CH_MEM_GB}
GA_BE_MEM_GB=${BE_MEM_GB}
GA_BE_WORKERS=${BE_WORKERS}
GA_BE_QUEUE=${BE_QUEUE}
GA_BE_BATCH=${BE_BATCH}
GA_BE_FLUSH=${BE_FLUSH}
GA_BE_CH_CONNS=${BE_CH_CONNS}
CH_MAX_MEMORY_USAGE=${ch_max_mem}
CH_EXTERNAL_GROUP_BY_BYTES=${spill}
CH_EXTERNAL_SORT_BYTES=${spill}
API_AUTH_TOKEN=${token}
API_OPS_TOKEN=${ops_token}
SESSION_SECRET=${session}
CLICKHOUSE_PASSWORD=${ch_password}
INGEST_SHARED_SECRET=${ingest_secret}
INGEST_ALLOW_FROM=${INGEST_ALLOW_FROM:-syslog-ng}
GA_TRUSTED_PROXIES=${GA_TRUSTED_PROXIES:-frontend}
AUTH_ADMIN_USER=${admin_user}
AUTH_ADMIN_PASSWORD=${admin_pass}
AUTH_ADMIN_MUST_RESET=${admin_must}
${operator_block}

# --- Модули (select_modules.sh) ---
GA_MODULE_AUTH=${mod_auth}
GA_MODULE_API_AUTH=${mod_api_auth}
GA_MODULE_SYSLOG=${mod_syslog}
GA_MODULE_STATS=${mod_stats}
GA_MODULE_REPUTATION=${mod_reputation}
AUTH_DISABLED=${auth_disabled}
API_AUTH_DISABLED=${api_auth_disabled}
REPUTATION_FETCH_ENABLED=${reputation_fetch_enabled}
GA_ALLOW_INSECURE=${allow_insecure}
COMPOSE_PROFILES=${compose_profiles}
SYSLOG_STATS_URL=${syslog_stats_url}

# --- HTTP / HTTPS (select_http_port.sh, certs/) ---
HTTP_PORT=${http_port}
HTTPS_ENABLED=${https_enabled}
HTTPS_PORT=${https_port}
HTTP_REDIRECT=${http_redirect}
EOF
}

write_compose_override() {
    local project_dir="$1" profile="$2"
    local override="${project_dir}/docker-compose.override.yml"
    local ch_max_mem spill ch_threads

    ch_max_mem="$(calc_ch_max_query_bytes "$CH_MEM_GB")"
    spill="$(calc_external_spill_bytes "$CH_MEM_GB")"
    ch_threads="$(calc_ch_max_threads "$CH_CPUS")"

    cat >"$override" <<EOF
# Сгенерировано detect_resources.sh — профиль: ${profile}
# Пересоздать: ./scripts/tune-resources.sh

services:
  clickhouse:
    deploy:
      resources:
        limits:
          cpus: "${CH_CPUS}.0"
          memory: ${CH_MEM_GB}g
    volumes:
      - ./clickhouse/users.d/zz_install_limits.xml:/etc/clickhouse-server/users.d/zz_install_limits.xml:ro

  backend:
    environment:
      INGEST_WORKERS: "${BE_WORKERS}"
      INGEST_BATCH_SIZE: "${BE_BATCH}"
      INGEST_QUEUE_SIZE: "${BE_QUEUE}"
      INGEST_FLUSH_SEC: "${BE_FLUSH}"
      CH_INGEST_MAX_OPEN_CONNS: "${BE_CH_CONNS}"
      CH_INGEST_ASYNC_INSERT: "true"
      GOMAXPROCS: "${BE_CPUS}"
      CH_MAX_MEMORY_USAGE: "${ch_max_mem}"
      CH_EXTERNAL_GROUP_BY_BYTES: "${spill}"
      CH_EXTERNAL_SORT_BYTES: "${spill}"
      CH_MAX_THREADS: "${ch_threads}"
    deploy:
      resources:
        limits:
          cpus: "${BE_CPUS}.0"
          memory: ${BE_MEM_GB}g

  syslog-ng:
    deploy:
      resources:
        limits:
          cpus: "${SYSLOG_CPUS}.0"
          memory: ${SYSLOG_MEM_MB}m
EOF
}

# 4.11 делит TCP log-iw-size на max-connections; минимум 100 на соединение.
syslog_tcp_max_conn() { echo 64; }

syslog_tcp_iw_size() {
    local tcp_max min_tcp_iw tcp_iw
    tcp_max="$(syslog_tcp_max_conn)"
    min_tcp_iw=$((tcp_max * 100))
    tcp_iw="${SYSLOG_IW_SIZE}"
    if (( tcp_iw < min_tcp_iw )); then
        tcp_iw=$min_tcp_iw
    fi
    echo "$tcp_iw"
}

write_syslog_profile() {
    local project_dir="$1" profile="$2"
    local conf_dir="${project_dir}/syslog-ng.d"
    local tcp_rcv=16777216
    local tcp_max tcp_iw
    tcp_max="$(syslog_tcp_max_conn)"
    tcp_iw="$(syslog_tcp_iw_size)"

    mkdir -p "$conf_dir"
    cat >"${conf_dir}/zz_profile.conf" <<EOF
# Сгенерировано detect_resources.sh — профиль: ${profile}
@define fifo_size ${SYSLOG_FIFO}
@define disk_buf ${SYSLOG_DISK_BUF}
@define udp_rcvbuf ${SYSLOG_UDP_RCVBUF}
@define tcp_rcvbuf ${tcp_rcv}
@define iw_size ${SYSLOG_IW_SIZE}
@define tcp_iw_size ${tcp_iw}
@define tcp_max_conn ${tcp_max}
EOF

    local ingest_secret=""
    ingest_secret="$(_ga_env_get "${project_dir}/.env" INGEST_SHARED_SECRET)"
    if [[ -n "$ingest_secret" ]]; then
        local esc="${ingest_secret//\\/\\\\}"
        esc="${esc//\"/\\\"}"
        cat >"${conf_dir}/zz_ingest_auth.conf" <<EOF
# Generated by detect_resources.sh — do not commit.
@define ga_ingest_token "${esc}"
EOF
    fi
}

write_clickhouse_limits() {
    local project_dir="$1" profile="$2"
    local limits_file="${project_dir}/clickhouse/users.d/zz_install_limits.xml"
    local max_mem spill ch_threads

    max_mem="$(calc_ch_max_query_bytes "$CH_MEM_GB")"
    spill="$(calc_external_spill_bytes "$CH_MEM_GB")"
    ch_threads="$(calc_ch_max_threads "$CH_CPUS")"

    mkdir -p "${project_dir}/clickhouse/users.d"
    cat >"$limits_file" <<EOF
<?xml version="1.0"?>
<clickhouse>
    <!-- Сгенерировано detect_resources.sh, профиль: ${profile} -->
    <!-- ClickHouse container: ${CH_MEM_GB} GiB / ${CH_CPUS} CPU; per-query ~40% RAM, max_threads=${ch_threads} -->
    <profiles>
        <default>
            <max_memory_usage>${max_mem}</max_memory_usage>
            <max_bytes_before_external_group_by>${spill}</max_bytes_before_external_group_by>
            <max_bytes_before_external_sort>${spill}</max_bytes_before_external_sort>
            <max_threads>${ch_threads}</max_threads>
        </default>
    </profiles>
</clickhouse>
EOF
}

write_install_profile_json() {
    local project_dir="$1" profile="$2"
    local cpu="$3" ram_mb="$4" disk_gb="$5" cgroup="$6"
    local json_file="${project_dir}/install-profile.json"
    local ch_max_mem spill

    ch_max_mem="$(calc_ch_max_query_bytes "$CH_MEM_GB")"
    spill="$(calc_external_spill_bytes "$CH_MEM_GB")"

    cat >"$json_file" <<EOF
{
  "generated_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "host": {
    "cpu_cores": ${cpu},
    "ram_mb": ${ram_mb},
    "disk_gb_avail": ${disk_gb},
    "cgroup": "${cgroup}"
  },
  "profile": "${profile}",
  "profile_label": "$(profile_label_ru "$profile")",
  "limits": {
    "clickhouse": {
      "memory_gb": ${CH_MEM_GB},
      "cpus": ${CH_CPUS},
      "max_query_memory_bytes": ${ch_max_mem},
      "external_spill_bytes": ${spill},
      "max_threads": $(calc_ch_max_threads "$CH_CPUS")
    },
    "backend": {
      "memory_gb": ${BE_MEM_GB},
      "cpus": ${BE_CPUS},
      "ingest_workers": ${BE_WORKERS},
      "ingest_queue_size": ${BE_QUEUE},
      "ingest_batch_size": ${BE_BATCH},
      "ingest_flush_sec": ${BE_FLUSH},
      "ch_ingest_max_open_conns": ${BE_CH_CONNS}
    },
    "syslog_ng": {
      "memory_mb": ${SYSLOG_MEM_MB},
      "cpus": ${SYSLOG_CPUS},
      "fifo_size": ${SYSLOG_FIFO},
      "mem_buf_bytes": ${SYSLOG_MEM_BUF},
      "disk_buf_bytes": ${SYSLOG_DISK_BUF},
      "udp_rcvbuf_bytes": ${SYSLOG_UDP_RCVBUF},
      "iw_size": ${SYSLOG_IW_SIZE},
      "tcp_iw_size": $(syslog_tcp_iw_size),
      "tcp_max_conn": $(syslog_tcp_max_conn)
    }
  },
  "capacity": {
    "expected_eps_min": ${EPS_MIN},
    "expected_eps_max": ${EPS_MAX}
  }
}
EOF
}

print_applied_config() {
    local profile="$1"
    echo "Применён профиль: $(profile_label_ru "$profile") [$profile]"
    echo "  ClickHouse : ${CH_MEM_GB} GiB RAM, ${CH_CPUS} CPU"
    echo "  Backend    : ${BE_MEM_GB} GiB RAM, ${BE_CPUS} CPU, workers=${BE_WORKERS}, queue=${BE_QUEUE}, batch=${BE_BATCH}, flush=${BE_FLUSH}s, ch_conns=${BE_CH_CONNS}"
    echo "  syslog-ng  : ${SYSLOG_MEM_MB} MiB RAM, ${SYSLOG_CPUS} CPU, fifo=${SYSLOG_FIFO}, window=${SYSLOG_FIFO} msgs, disk=$((SYSLOG_DISK_BUF / 1048576)) MiB"
    echo "  Ёмкость    : ${EPS_MIN}–${EPS_MAX} eps"
    echo ""
    echo "Созданы файлы:"
    echo "  docker-compose.override.yml"
    echo "  .env"
    echo "  clickhouse/users.d/zz_install_limits.xml"
    echo "  syslog-ng.d/zz_profile.conf"
    echo "  syslog-ng.d/zz_ingest_auth.conf"
    echo "  install-profile.json"
}

warn_if_constraints() {
    local ram_mb="$1" disk_gb="$2" cgroup="$3"

    if (( disk_gb > 0 && disk_gb < 20 )); then
        _ga_log "ВНИМАНИЕ: мало свободного места на диске (${disk_gb} GiB). Рекомендуется ≥20 GiB для ClickHouse и буферов syslog-ng."
    fi

    if (( ram_mb > 0 && ram_mb < 3072 )); then
        _ga_log "ВНИМАНИЕ: мало RAM (${ram_mb} MiB). Система будет работать, но при высокой нагрузке возможны OOM и отставание ingest."
    fi

    if [[ "$cgroup" == "unknown" ]]; then
        _ga_log "ВНИМАНИЕ: не удалось определить версию cgroup — stats-collector может не видеть метрики контейнеров."
    fi
}

apply_resource_profile() {
    local project_dir="${1:-.}"

    if [[ "${GA_SKIP_PROFILE:-0}" == "1" ]]; then
        _ga_log "GA_SKIP_PROFILE=1 — генерация конфигурации пропущена."
        return 0
    fi

    local cpu ram_mb disk_gb cgroup recommended profile

    cpu="$(detect_cpu_cores)"
    ram_mb="$(detect_ram_mb)"
    disk_gb="$(detect_disk_gb_avail "$project_dir")"
    if (( disk_gb == 0 )); then
        disk_gb="$(detect_disk_gb_avail "/")"
    fi
    cgroup="$(detect_cgroup_version)"
    recommended="$(recommend_profile "$cpu" "$ram_mb")"

    print_host_summary "$cpu" "$ram_mb" "$disk_gb" "$recommended"
    warn_if_constraints "$ram_mb" "$disk_gb" "$cgroup"

    if ! confirm_profile "$recommended"; then
        return 1
    fi

    profile="${GA_SELECTED_PROFILE:-}"
    if [[ -z "$profile" ]]; then
        return 0
    fi

    if ! profile_params "$profile"; then
        _ga_log "ОШИБКА: некорректный профиль '$profile', откат на $recommended"
        profile="$recommended"
        profile_params "$profile"
    fi
    profile_capacity "$profile"

    write_env_file "$project_dir" "$profile"
    write_compose_override "$project_dir" "$profile"
    write_clickhouse_limits "$project_dir" "$profile"
    write_syslog_profile "$project_dir" "$profile"
    write_install_profile_json "$project_dir" "$profile" "$cpu" "$ram_mb" "$disk_gb" "$cgroup"

    echo ""
    print_applied_config "$profile"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    apply_resource_profile "${1:-.}"
fi
