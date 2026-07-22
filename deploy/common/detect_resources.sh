#!/usr/bin/env bash
# Определяет ресурсы хоста и генерирует оптимальную конфигурацию для docker compose.
# Использование:
#   source deploy/common/detect_resources.sh
#   apply_resource_profile /opt/network-monitor
#
# Переменные окружения:
#   NM_AUTO_PROFILE=1     — принять рекомендованный профиль без вопросов
#   NM_FORCE_PROFILE=...  — принудительно выбрать профиль (tiny|small|medium|large|xlarge)
#   NM_SKIP_PROFILE=1     — не генерировать override (оставить значения по умолчанию)
# Модули (см. select_modules.sh) сохраняются в .env при пересчёте профиля.

set -Eeuo pipefail

_nm_log() { echo "[$(date +'%F %T')] [resources] $*"; }

_nm_res_ensure_ui() {
    if declare -F nm_ui_radiolist >/dev/null 2>&1; then
        return 0
    fi
    local dir helper
    dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
    helper="${dir}/ui.sh"
    if [[ -f "$helper" ]]; then
        # shellcheck source=deploy/common/ui.sh
        source "$helper"
        return 0
    fi
    return 1
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
            BE_WORKERS=1; BE_QUEUE=50000; BE_BATCH=3000; BE_FLUSH=3
            BE_CH_CONNS=1
            # syslog-ng держит два mem-buf по 128 MiB + свой overhead; ниже 512 MiB тесно.
            SYSLOG_MEM_MB=512; SYSLOG_CPUS=1
            ;;
        small)
            # 4 CPU / 8 GiB: ClickHouse — узкое место INSERT.
            # Меньше concurrent INSERT + крупнее batch; raw не пишем (см. InsertTrafficLogs).
            CH_MEM_GB=3; CH_CPUS=3
            BE_MEM_GB=2; BE_CPUS=2
            BE_WORKERS=2; BE_QUEUE=200000; BE_BATCH=20000; BE_FLUSH=1
            BE_CH_CONNS=2
            SYSLOG_MEM_MB=768; SYSLOG_CPUS=1
            ;;
        medium)
            CH_MEM_GB=6; CH_CPUS=4
            BE_MEM_GB=4; BE_CPUS=3
            BE_WORKERS=3; BE_QUEUE=300000; BE_BATCH=20000; BE_FLUSH=1
            BE_CH_CONNS=3
            SYSLOG_MEM_MB=1024; SYSLOG_CPUS=2
            ;;
        large)
            CH_MEM_GB=12; CH_CPUS=8
            BE_MEM_GB=8; BE_CPUS=6
            BE_WORKERS=4; BE_QUEUE=500000; BE_BATCH=30000; BE_FLUSH=1
            BE_CH_CONNS=4
            SYSLOG_MEM_MB=2048; SYSLOG_CPUS=3
            ;;
        xlarge)
            CH_MEM_GB=24; CH_CPUS=16
            BE_MEM_GB=16; BE_CPUS=12
            BE_WORKERS=6; BE_QUEUE=750000; BE_BATCH=40000; BE_FLUSH=1
            BE_CH_CONNS=6
            SYSLOG_MEM_MB=4096; SYSLOG_CPUS=4
            ;;
        *)
            _nm_log "Unknown profile: $profile"
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

print_host_summary() {
    local cpu="$1" ram_mb="$2" disk_gb="$3" recommended="$4"
    local ram_gib
    ram_gib="$(awk -v mb="$ram_mb" 'BEGIN { printf "%.1f", mb / 1024 }')"

    echo ""
    echo "══════════════════════════════════════════════════════════"
    echo "  Анализ сервера"
    echo "══════════════════════════════════════════════════════════"
    echo "  CPU ядер       : $cpu"
    echo "  RAM            : ${ram_gib} GiB (${ram_mb} MiB)"
    echo "  Свободно диска : ${disk_gb} GiB (/)"
    echo "  Рекомендация   : $(profile_label_ru "$recommended") [$recommended]"
    profile_capacity "$recommended"
    echo "  Расчётная EPS  : до ${EPS_MAX} событий/с"
    echo "══════════════════════════════════════════════════════════"
    echo ""
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
    NM_SELECTED_PROFILE="$recommended"

    if [[ "${NM_AUTO_PROFILE:-0}" == "1" ]]; then
        _nm_log "NM_AUTO_PROFILE=1 — применяем профиль: $recommended"
        return 0
    fi

    if [[ -n "${NM_FORCE_PROFILE:-}" ]]; then
        if ! is_valid_profile "${NM_FORCE_PROFILE}"; then
            _nm_log "ERROR: NM_FORCE_PROFILE='${NM_FORCE_PROFILE}' — неизвестный профиль."
            return 1
        fi
        NM_SELECTED_PROFILE="${NM_FORCE_PROFILE}"
        _nm_log "NM_FORCE_PROFILE — применяем профиль: ${NM_SELECTED_PROFILE}"
        return 0
    fi

    if [[ ! -t 0 ]]; then
        _nm_log "Нет TTY — применяем рекомендованный профиль: $recommended"
        return 0
    fi

    _nm_res_ensure_ui || true

    if declare -F nm_ui_radiolist >/dev/null 2>&1; then
        local -a items=()
        local p mark on_flag desc
        for p in tiny small medium large xlarge; do
            profile_params "$p"
            profile_capacity "$p"
            on_flag=OFF
            mark=""
            [[ "$p" == "$recommended" ]] && on_flag=ON && mark=" ← рекомендуется"
            desc="$(profile_label_ru "$p")${mark} · CH ${CH_MEM_GB}G · BE ${BE_MEM_GB}G · EPS до ${EPS_MAX}/с"
            items+=("$p" "$desc" "$on_flag")
        done
        items+=(skip "Оставить значения docker-compose.yml (без override)" OFF)

        local answer
        if answer="$(nm_ui_radiolist \
            "Профиль производительности" \
            "Рекомендация по ресурсам хоста: ${recommended}
Выберите профиль:" \
            "${items[@]}")"; then
            case "$answer" in
                skip)
                    NM_SELECTED_PROFILE=""
                    _nm_log "Профиль не применён — используются значения docker-compose.yml"
                    return 0
                    ;;
                tiny|small|medium|large|xlarge)
                    NM_SELECTED_PROFILE="$answer"
                    if [[ "$answer" == "$recommended" ]]; then
                        _nm_log "Выбран профиль: ${NM_SELECTED_PROFILE} (рекомендация)"
                    else
                        _nm_log "Выбран профиль: ${NM_SELECTED_PROFILE}"
                    fi
                    return 0
                    ;;
                *)
                    _nm_log "Неизвестный ответ «${answer}» — применяем рекомендацию ${recommended}."
                    NM_SELECTED_PROFILE="$recommended"
                    return 0
                    ;;
            esac
        else
            _nm_log "Выбор профиля отменён — применяем рекомендацию: ${recommended}"
            NM_SELECTED_PROFILE="$recommended"
            return 0
        fi
    fi

    echo "" >&2
    echo "══════════════════════════════════════════════════════════" >&2
    echo "  Выбор профиля производительности" >&2
    echo "══════════════════════════════════════════════════════════" >&2
    print_profile_options "$recommended"
    echo "  Enter / y     — принять рекомендацию [$recommended]" >&2
    echo "  tiny … xlarge — выбрать другой профиль" >&2
    echo "  skip          — оставить значения по умолчанию (без override)" >&2
    echo "" >&2

    while true; do
        local answer
        read -r -p "Ваш выбор [${recommended}]: " answer </dev/tty || answer=""
        answer="${answer,,}"
        answer="${answer//[[:space:]]/}"

        case "$answer" in
            ""|y|yes|д|да)
                NM_SELECTED_PROFILE="$recommended"
                _nm_log "Выбран профиль: ${NM_SELECTED_PROFILE} (рекомендация)"
                return 0
                ;;
            skip|s|пропуск|default)
                NM_SELECTED_PROFILE=""
                _nm_log "Профиль не применён — используются значения docker-compose.yml"
                return 0
                ;;
            tiny|small|medium|large|xlarge)
                NM_SELECTED_PROFILE="$answer"
                _nm_log "Выбран профиль: ${NM_SELECTED_PROFILE}"
                return 0
                ;;
            *)
                echo "  Не понял «${answer}». Enter = [$recommended], или введите tiny/small/medium/large/xlarge/skip." >&2
                ;;
        esac
    done
}

_nm_env_get() {
    # Прочитать значение KEY= из .env (последнее вхождение). $1=file $2=key
    local file="$1" key="$2"
    [[ -f "$file" ]] || return 0
    grep -E "^[[:space:]]*${key}=" "$file" 2>/dev/null | tail -n1 | cut -d= -f2- || true
}

write_env_file() {
    local project_dir="$1" profile="$2"
    local env_file="${project_dir}/.env"
    local ch_max_mem spill existing_token="" existing_session="" token session

    ch_max_mem="$(calc_ch_max_query_bytes "$CH_MEM_GB")"
    spill="$(calc_external_spill_bytes "$CH_MEM_GB")"

    if [[ -f "$env_file" ]]; then
        existing_token="$(_nm_env_get "$env_file" API_AUTH_TOKEN)"
        existing_session="$(_nm_env_get "$env_file" SESSION_SECRET)"
    fi
    if [[ -n "$existing_token" ]]; then
        token="$existing_token"
    elif command -v openssl >/dev/null 2>&1; then
        token="$(openssl rand -hex 32)"
    else
        token="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
    fi
    if [[ -n "$existing_session" ]]; then
        session="$existing_session"
    elif command -v openssl >/dev/null 2>&1; then
        session="$(openssl rand -hex 32)"
    else
        session="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
    fi

    local admin_user="${AUTH_ADMIN_USER:-admin}"
    local admin_pass="${AUTH_ADMIN_PASSWORD:-admin}"
    local operator_user="${AUTH_OPERATOR_USER:-operator}"
    local operator_pass="${AUTH_OPERATOR_PASSWORD:-operator}"
    local auth_disabled="${AUTH_DISABLED:-false}"
    local api_auth_disabled="${API_AUTH_DISABLED:-false}"
    local allow_insecure="${NM_ALLOW_INSECURE:-0}"
    local mod_auth="${NM_MODULE_AUTH:-1}"
    local mod_api_auth="${NM_MODULE_API_AUTH:-1}"
    local mod_syslog="${NM_MODULE_SYSLOG:-1}"
    local mod_stats="${NM_MODULE_STATS:-1}"
    local compose_profiles="${NM_COMPOSE_PROFILES:-${COMPOSE_PROFILES:-syslog,stats}}"

    if [[ -f "$env_file" ]]; then
        local v
        v="$(_nm_env_get "$env_file" AUTH_ADMIN_USER)"; [[ -n "$v" ]] && admin_user="$v"
        v="$(_nm_env_get "$env_file" AUTH_ADMIN_PASSWORD)"; [[ -n "$v" ]] && admin_pass="$v"
        v="$(_nm_env_get "$env_file" AUTH_OPERATOR_USER)"; [[ -n "$v" ]] && operator_user="$v"
        v="$(_nm_env_get "$env_file" AUTH_OPERATOR_PASSWORD)"; [[ -n "$v" ]] && operator_pass="$v"
        # Флаги модулей: текущие env (после confirm_modules) имеют приоритет над файлом.
        if [[ -z "${NM_MODULE_AUTH:-}" ]]; then
            v="$(_nm_env_get "$env_file" NM_MODULE_AUTH)"; [[ -n "$v" ]] && mod_auth="$v"
            v="$(_nm_env_get "$env_file" NM_MODULE_API_AUTH)"; [[ -n "$v" ]] && mod_api_auth="$v"
            v="$(_nm_env_get "$env_file" NM_MODULE_SYSLOG)"; [[ -n "$v" ]] && mod_syslog="$v"
            v="$(_nm_env_get "$env_file" NM_MODULE_STATS)"; [[ -n "$v" ]] && mod_stats="$v"
            v="$(_nm_env_get "$env_file" AUTH_DISABLED)"; [[ -n "$v" ]] && auth_disabled="$v"
            v="$(_nm_env_get "$env_file" API_AUTH_DISABLED)"; [[ -n "$v" ]] && api_auth_disabled="$v"
            v="$(_nm_env_get "$env_file" NM_ALLOW_INSECURE)"; [[ -n "$v" ]] && allow_insecure="$v"
            if grep -qE '^[[:space:]]*COMPOSE_PROFILES=' "$env_file" 2>/dev/null; then
                compose_profiles="$(_nm_env_get "$env_file" COMPOSE_PROFILES)"
            fi
        else
            [[ "${mod_auth}" == "1" ]] && auth_disabled="false" || auth_disabled="true"
            [[ "${mod_api_auth}" == "1" ]] && api_auth_disabled="false" || api_auth_disabled="true"
            compose_profiles="${NM_COMPOSE_PROFILES:-}"
        fi
    elif [[ -n "${NM_MODULE_AUTH:-}" ]]; then
        [[ "${mod_auth}" == "1" ]] && auth_disabled="false" || auth_disabled="true"
        [[ "${mod_api_auth}" == "1" ]] && api_auth_disabled="false" || api_auth_disabled="true"
        compose_profiles="${NM_COMPOSE_PROFILES:-}"
    fi

    if [[ "$auth_disabled" == "true" || "$api_auth_disabled" == "true" ]]; then
        allow_insecure="1"
    fi

    cat >"$env_file" <<EOF
# Сгенерировано detect_resources.sh — не редактируйте вручную, перезапустите tune-resources.sh
NM_INSTALL_PROFILE=${profile}
NM_CH_MEM_GB=${CH_MEM_GB}
NM_BE_MEM_GB=${BE_MEM_GB}
NM_BE_WORKERS=${BE_WORKERS}
NM_BE_QUEUE=${BE_QUEUE}
NM_BE_BATCH=${BE_BATCH}
NM_BE_FLUSH=${BE_FLUSH}
NM_BE_CH_CONNS=${BE_CH_CONNS}
CH_MAX_MEMORY_USAGE=${ch_max_mem}
CH_EXTERNAL_GROUP_BY_BYTES=${spill}
CH_EXTERNAL_SORT_BYTES=${spill}
API_AUTH_TOKEN=${token}
SESSION_SECRET=${session}
AUTH_ADMIN_USER=${admin_user}
AUTH_ADMIN_PASSWORD=${admin_pass}
AUTH_OPERATOR_USER=${operator_user}
AUTH_OPERATOR_PASSWORD=${operator_pass}

# --- Модули (select_modules.sh) ---
NM_MODULE_AUTH=${mod_auth}
NM_MODULE_API_AUTH=${mod_api_auth}
NM_MODULE_SYSLOG=${mod_syslog}
NM_MODULE_STATS=${mod_stats}
AUTH_DISABLED=${auth_disabled}
API_AUTH_DISABLED=${api_auth_disabled}
NM_ALLOW_INSECURE=${allow_insecure}
COMPOSE_PROFILES=${compose_profiles}
EOF
}

write_compose_override() {
    local project_dir="$1" profile="$2"
    local override="${project_dir}/docker-compose.override.yml"
    local ch_max_mem spill

    ch_max_mem="$(calc_ch_max_query_bytes "$CH_MEM_GB")"
    spill="$(calc_external_spill_bytes "$CH_MEM_GB")"

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

write_clickhouse_limits() {
    local project_dir="$1" profile="$2"
    local limits_file="${project_dir}/clickhouse/users.d/zz_install_limits.xml"
    local max_mem spill

    max_mem="$(calc_ch_max_query_bytes "$CH_MEM_GB")"
    spill="$(calc_external_spill_bytes "$CH_MEM_GB")"

    mkdir -p "${project_dir}/clickhouse/users.d"
    cat >"$limits_file" <<EOF
<?xml version="1.0"?>
<clickhouse>
    <!-- Сгенерировано detect_resources.sh, профиль: ${profile} -->
    <!-- ClickHouse container: ${CH_MEM_GB} GiB, per-query limit: ~40% RAM -->
    <profiles>
        <default>
            <max_memory_usage>${max_mem}</max_memory_usage>
            <max_bytes_before_external_group_by>${spill}</max_bytes_before_external_group_by>
            <max_bytes_before_external_sort>${spill}</max_bytes_before_external_sort>
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
      "external_spill_bytes": ${spill}
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
      "cpus": ${SYSLOG_CPUS}
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
    echo "  syslog-ng  : ${SYSLOG_MEM_MB} MiB RAM, ${SYSLOG_CPUS} CPU"
    echo "  Ёмкость    : ${EPS_MIN}–${EPS_MAX} eps"
    echo ""
    echo "Созданы файлы:"
    echo "  docker-compose.override.yml"
    echo "  .env"
    echo "  clickhouse/users.d/zz_install_limits.xml"
    echo "  install-profile.json"
}

warn_if_constraints() {
    local ram_mb="$1" disk_gb="$2" cgroup="$3"

    if (( disk_gb > 0 && disk_gb < 20 )); then
        _nm_log "WARNING: мало свободного места на диске (${disk_gb} GiB). Рекомендуется ≥20 GiB для ClickHouse и буферов syslog-ng."
    fi

    if (( ram_mb > 0 && ram_mb < 3072 )); then
        _nm_log "WARNING: мало RAM (${ram_mb} MiB). Система будет работать, но при высокой нагрузке возможны OOM и отставание ingest."
    fi

    if [[ "$cgroup" == "unknown" ]]; then
        _nm_log "WARNING: не удалось определить версию cgroup — stats-collector может не видеть метрики контейнеров."
    fi
}

apply_resource_profile() {
    local project_dir="${1:-.}"

    if [[ "${NM_SKIP_PROFILE:-0}" == "1" ]]; then
        _nm_log "NM_SKIP_PROFILE=1 — генерация конфигурации пропущена."
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

    profile="${NM_SELECTED_PROFILE:-}"
    if [[ -z "$profile" ]]; then
        return 0
    fi

    if ! profile_params "$profile"; then
        _nm_log "ERROR: некорректный профиль '$profile', откат на $recommended"
        profile="$recommended"
        profile_params "$profile"
    fi
    profile_capacity "$profile"

    write_env_file "$project_dir" "$profile"
    write_compose_override "$project_dir" "$profile"
    write_clickhouse_limits "$project_dir" "$profile"
    write_install_profile_json "$project_dir" "$profile" "$cpu" "$ram_mb" "$disk_gb" "$cgroup"

    echo ""
    print_applied_config "$profile"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    apply_resource_profile "${1:-.}"
fi
