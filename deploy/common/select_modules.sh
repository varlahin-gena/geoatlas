#!/usr/bin/env bash
# Интерактивный выбор модулей ГеоАтлас.
# Использование:
#   source deploy/common/select_modules.sh
#   confirm_modules
#   apply_module_selection /opt/network-monitor
#
# UI: deploy/common/ui.sh (whiptail → dialog → текст).
#
# Переменные окружения (CI / без TTY):
#   NM_AUTO_MODULES=1              — принять все модули по умолчанию (вкл.)
#   NM_MODULES=auth,syslog,stats,reputation — явный список (через запятую); пусто = только ядро
#   NM_ENABLE_AUTH=0|1             — UI-авторизация (логин / роли)
#   NM_ENABLE_API_AUTH=0|1         — Bearer-токен для мутирующих API
#   NM_ENABLE_SYSLOG=0|1           — контейнер syslog-ng
#   NM_ENABLE_STATS=0|1            — контейнер stats-collector
#   NM_ENABLE_REPUTATION=0|1       — модуль репутации IP (API, UI, фиды)
#   NM_UI=whiptail|dialog|text     — бэкенд диалогов
#
# После confirm_modules доступны:
#   NM_MODULE_AUTH, NM_MODULE_API_AUTH, NM_MODULE_SYSLOG, NM_MODULE_STATS,
#   NM_MODULE_REPUTATION  (0|1)
#   NM_COMPOSE_PROFILES  — строка для COMPOSE_PROFILES в .env

set -Eeuo pipefail

_nm_mod_log() { echo "[$(date +'%F %T')] [modules] $*"; }

_nm_mod_ensure_ui() {
    if ! declare -F nm_ui_yesno >/dev/null 2>&1; then
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
    if [[ -z "${NM_UI_BACKEND:-}" ]] && declare -F nm_ui_init >/dev/null 2>&1; then
        nm_ui_init
    fi
    return 0
}

_nm_mod_truthy() {
    case "${1:-}" in
        1|true|TRUE|yes|YES|y|Y|on|ON) return 0 ;;
        *) return 1 ;;
    esac
}

_nm_mod_yesno() {
    # $1 prompt, $2 default 0|1 → returns 0 if yes
    local prompt="$1"
    local def="${2:-1}"
    if _nm_mod_ensure_ui; then
        nm_ui_yesno "ГеоАтлас" "$prompt" "$def"
        return
    fi
    local hint answer
    if [[ "$def" == "1" ]]; then
        hint="Y/n"
    else
        hint="y/N"
    fi
    if [[ -r /dev/tty && -w /dev/tty ]]; then
        printf '%s [%s]: ' "$prompt" "$hint" >/dev/tty
        read -r answer </dev/tty || answer=""
    else
        printf '%s [%s]: ' "$prompt" "$hint" >&2
        read -r answer || answer=""
    fi
    answer="${answer,,}"
    answer="${answer//[[:space:]]/}"
    if [[ -z "$answer" ]]; then
        [[ "$def" == "1" ]]
        return
    fi
    case "$answer" in
        y|yes|д|да|1) return 0 ;;
        n|no|н|нет|0) return 1 ;;
        *)
            echo "  Введите y или n." >/dev/tty 2>/dev/null || echo "  Введите y или n." >&2
            _nm_mod_yesno "$prompt" "$def"
            ;;
    esac
}

_nm_mod_tag_selected() {
    # $1 = comma-list, $2 = tag
    local list=",${1},"
    local tag="$2"
    [[ "$list" == *",${tag},"* ]]
}

_nm_mod_parse_list() {
    # Разбор NM_MODULES=auth,syslog,... → выставляет NM_MODULE_*
    local raw="${1:-}"
    local part
    NM_MODULE_AUTH=0
    NM_MODULE_API_AUTH=0
    NM_MODULE_SYSLOG=0
    NM_MODULE_STATS=0
    NM_MODULE_REPUTATION=0
    IFS=',' read -ra parts <<<"$raw"
    for part in "${parts[@]}"; do
        part="${part,,}"
        part="${part//[[:space:]]/}"
        [[ -z "$part" ]] && continue
        case "$part" in
            auth|ui-auth|ui_auth) NM_MODULE_AUTH=1 ;;
            api-auth|api_auth|api) NM_MODULE_API_AUTH=1 ;;
            syslog|syslog-ng|ingest) NM_MODULE_SYSLOG=1 ;;
            stats|stats-collector|monitoring|system) NM_MODULE_STATS=1 ;;
            reputation|rep|reputation-ip|reputation_ip) NM_MODULE_REPUTATION=1 ;;
            all)
                NM_MODULE_AUTH=1
                NM_MODULE_API_AUTH=1
                NM_MODULE_SYSLOG=1
                NM_MODULE_STATS=1
                NM_MODULE_REPUTATION=1
                ;;
            core|none) ;;
            *)
                _nm_mod_log "WARNING: неизвестный модуль «${part}» в NM_MODULES — пропущен."
                ;;
        esac
    done
}

_nm_mod_set_defaults() {
    NM_MODULE_AUTH="${NM_MODULE_AUTH:-1}"
    NM_MODULE_API_AUTH="${NM_MODULE_API_AUTH:-1}"
    NM_MODULE_SYSLOG="${NM_MODULE_SYSLOG:-1}"
    NM_MODULE_STATS="${NM_MODULE_STATS:-1}"
    NM_MODULE_REPUTATION="${NM_MODULE_REPUTATION:-1}"
}

_nm_mod_from_env_flags() {
    # Индивидуальные NM_ENABLE_* перекрывают defaults / NM_MODULES
    if [[ -n "${NM_ENABLE_AUTH:-}" ]]; then
        _nm_mod_truthy "$NM_ENABLE_AUTH" && NM_MODULE_AUTH=1 || NM_MODULE_AUTH=0
    fi
    if [[ -n "${NM_ENABLE_API_AUTH:-}" ]]; then
        _nm_mod_truthy "$NM_ENABLE_API_AUTH" && NM_MODULE_API_AUTH=1 || NM_MODULE_API_AUTH=0
    fi
    if [[ -n "${NM_ENABLE_SYSLOG:-}" ]]; then
        _nm_mod_truthy "$NM_ENABLE_SYSLOG" && NM_MODULE_SYSLOG=1 || NM_MODULE_SYSLOG=0
    fi
    if [[ -n "${NM_ENABLE_STATS:-}" ]]; then
        _nm_mod_truthy "$NM_ENABLE_STATS" && NM_MODULE_STATS=1 || NM_MODULE_STATS=0
    fi
    if [[ -n "${NM_ENABLE_REPUTATION:-}" ]]; then
        _nm_mod_truthy "$NM_ENABLE_REPUTATION" && NM_MODULE_REPUTATION=1 || NM_MODULE_REPUTATION=0
    fi
}

_nm_mod_build_compose_profiles() {
    local profiles=()
    [[ "${NM_MODULE_SYSLOG:-0}" == "1" ]] && profiles+=("syslog")
    [[ "${NM_MODULE_STATS:-0}" == "1" ]] && profiles+=("stats")
    local IFS=','
    NM_COMPOSE_PROFILES="${profiles[*]-}"
}

print_modules_summary() {
    local a s t p r
    [[ "${NM_MODULE_AUTH:-0}" == "1" ]] && a="вкл." || a="выкл. (AUTH_DISABLED)"
    [[ "${NM_MODULE_API_AUTH:-0}" == "1" ]] && t="вкл." || t="выкл. (API_AUTH_DISABLED)"
    [[ "${NM_MODULE_SYSLOG:-0}" == "1" ]] && s="вкл." || s="выкл."
    [[ "${NM_MODULE_STATS:-0}" == "1" ]] && p="вкл." || p="выкл."
    [[ "${NM_MODULE_REPUTATION:-0}" == "1" ]] && r="вкл." || r="выкл. (модуль отключён)"
    local profiles="${NM_COMPOSE_PROFILES:-}"
    [[ -n "$profiles" ]] || profiles="(нет — только ядро)"
    echo ""
    echo "══════════════════════════════════════════════════════════"
    echo "  Выбранные модули"
    echo "══════════════════════════════════════════════════════════"
    echo "  Ядро (всегда)     : ClickHouse + Backend + Frontend"
    echo "  UI-авторизация    : ${a}"
    echo "  API Bearer-токен  : ${t}"
    echo "  syslog-ng         : ${s}"
    echo "  stats-collector   : ${p}"
    echo "  Репутация IP      : ${r}"
    echo "  Compose profiles  : ${profiles}"
    echo "══════════════════════════════════════════════════════════"
    echo ""
    _nm_mod_ensure_ui || true
    if declare -F nm_ui_msgbox >/dev/null 2>&1 && [[ "${NM_UI_BACKEND:-text}" != "text" ]]; then
        nm_ui_msgbox "Выбранные модули" \
"Ядро: ClickHouse + Backend + Frontend
UI-авторизация: ${a}
API Bearer-токен: ${t}
syslog-ng: ${s}
stats-collector: ${p}
Репутация IP: ${r}
Compose profiles: ${profiles}" || true
    fi
}

confirm_modules() {
    _nm_mod_set_defaults

    # Явный список имеет приоритет над defaults (но не над NM_ENABLE_*).
    if [[ -n "${NM_MODULES+x}" ]]; then
        _nm_mod_parse_list "${NM_MODULES}"
    fi
    _nm_mod_from_env_flags

    if [[ "${NM_AUTO_MODULES:-0}" == "1" ]]; then
        _nm_mod_build_compose_profiles
        _nm_mod_log "NM_AUTO_MODULES=1 — модули: auth=${NM_MODULE_AUTH} api_auth=${NM_MODULE_API_AUTH} syslog=${NM_MODULE_SYSLOG} stats=${NM_MODULE_STATS} reputation=${NM_MODULE_REPUTATION}"
        return 0
    fi

    # Если задан NM_MODULES или любой NM_ENABLE_* — без вопросов (режим автоматизации).
    if [[ -n "${NM_MODULES+x}" ]] || \
       [[ -n "${NM_ENABLE_AUTH:-}" ]] || [[ -n "${NM_ENABLE_API_AUTH:-}" ]] || \
       [[ -n "${NM_ENABLE_SYSLOG:-}" ]] || [[ -n "${NM_ENABLE_STATS:-}" ]] || \
       [[ -n "${NM_ENABLE_REPUTATION:-}" ]]; then
        _nm_mod_build_compose_profiles
        _nm_mod_log "Модули заданы через окружение (без интерактива)."
        print_modules_summary
        return 0
    fi

    if [[ ! -t 0 ]]; then
        _nm_mod_build_compose_profiles
        _nm_mod_log "Нет TTY — устанавливаем все модули по умолчанию."
        return 0
    fi

    _nm_mod_ensure_ui || true

    local selected=""
    local auth_on=OFF api_on=OFF syslog_on=OFF stats_on=OFF reputation_on=OFF
    [[ "${NM_MODULE_AUTH:-1}" == "1" ]] && auth_on=ON
    [[ "${NM_MODULE_API_AUTH:-1}" == "1" ]] && api_on=ON
    [[ "${NM_MODULE_SYSLOG:-1}" == "1" ]] && syslog_on=ON
    [[ "${NM_MODULE_STATS:-1}" == "1" ]] && stats_on=ON
    [[ "${NM_MODULE_REPUTATION:-1}" == "1" ]] && reputation_on=ON

    if declare -F nm_ui_checklist >/dev/null 2>&1; then
        if selected="$(nm_ui_checklist \
            "Выбор модулей" \
            "Ядро (ClickHouse + Backend + Frontend) ставится всегда.
Отметьте дополнительные модули:" \
            auth "UI-авторизация (логин, роли admin/operator)" "$auth_on" \
            api_auth "Bearer-токен для мутирующих API" "$api_on" \
            syslog "syslog-ng (приём syslog на :514)" "$syslog_on" \
            stats "stats-collector (метрики / страница system)" "$stats_on" \
            reputation "Репутация IP (фиды, /reputation.html, фильтр на карте)" "$reputation_on")"; then
            NM_MODULE_AUTH=0
            NM_MODULE_API_AUTH=0
            NM_MODULE_SYSLOG=0
            NM_MODULE_STATS=0
            NM_MODULE_REPUTATION=0
            _nm_mod_tag_selected "$selected" auth && NM_MODULE_AUTH=1
            _nm_mod_tag_selected "$selected" api_auth && NM_MODULE_API_AUTH=1
            _nm_mod_tag_selected "$selected" syslog && NM_MODULE_SYSLOG=1
            _nm_mod_tag_selected "$selected" stats && NM_MODULE_STATS=1
            _nm_mod_tag_selected "$selected" reputation && NM_MODULE_REPUTATION=1
        else
            _nm_mod_log "Выбор модулей отменён — установка прервана."
            exit 0
        fi
    else
        echo "" >&2
        echo "══════════════════════════════════════════════════════════" >&2
        echo "  Выбор модулей системы" >&2
        echo "══════════════════════════════════════════════════════════" >&2
        echo "" >&2
        echo "  Всегда устанавливаются: ClickHouse, Backend API, Frontend." >&2
        echo "  Остальное можно отключить." >&2
        echo "" >&2

        if _nm_mod_yesno "  Установить UI-авторизацию (логин, роли admin/operator)?" 1; then
            NM_MODULE_AUTH=1
        else
            NM_MODULE_AUTH=0
            echo "    → AUTH_DISABLED=true: вход не требуется, роли отключены." >&2
        fi

        if _nm_mod_yesno "  Защищать мутирующие API Bearer-токеном?" 1; then
            NM_MODULE_API_AUTH=1
        else
            NM_MODULE_API_AUTH=0
            echo "    → API_AUTH_DISABLED=true: ingest/upload открыты без токена (небезопасно в проде)." >&2
        fi

        if _nm_mod_yesno "  Установить syslog-ng (приём syslog на :514)?" 1; then
            NM_MODULE_SYSLOG=1
        else
            NM_MODULE_SYSLOG=0
        fi

        if _nm_mod_yesno "  Установить stats-collector (метрики / страница system)?" 1; then
            NM_MODULE_STATS=1
        else
            NM_MODULE_STATS=0
        fi

        if _nm_mod_yesno "  Включить модуль репутации IP (/reputation.html, фиды, фильтр на карте)?" 1; then
            NM_MODULE_REPUTATION=1
        else
            NM_MODULE_REPUTATION=0
            echo "    → REPUTATION_FETCH_ENABLED=false: модуль репутации полностью отключён." >&2
        fi
    fi

    if [[ "${NM_MODULE_AUTH:-1}" != "1" ]]; then
        _nm_mod_log "UI-авторизация отключена (AUTH_DISABLED=true)."
    fi
    if [[ "${NM_MODULE_API_AUTH:-1}" != "1" ]]; then
        _nm_mod_log "API Bearer-защита отключена (API_AUTH_DISABLED=true)."
    fi
    if [[ "${NM_MODULE_REPUTATION:-1}" != "1" ]]; then
        _nm_mod_log "Модуль репутации IP отключён (REPUTATION_FETCH_ENABLED=false)."
    fi

    _nm_mod_build_compose_profiles
    print_modules_summary
    _nm_mod_log "Модули: auth=${NM_MODULE_AUTH} api_auth=${NM_MODULE_API_AUTH} syslog=${NM_MODULE_SYSLOG} stats=${NM_MODULE_STATS} reputation=${NM_MODULE_REPUTATION}"
}

# Обновляет/добавляет ключи модулей в .env (не затирает остальные строки).
apply_module_selection() {
    local project_dir="${1:-.}"
    local env_file="${project_dir}/.env"
    local modules_json="${project_dir}/install-modules.json"
    local auth_disabled="false"
    local api_auth_disabled="false"
    local reputation_fetch_enabled="true"
    local allow_insecure="0"
    local tmp

    _nm_mod_set_defaults
    _nm_mod_build_compose_profiles

    [[ "${NM_MODULE_AUTH:-1}" == "1" ]] || auth_disabled="true"
    [[ "${NM_MODULE_API_AUTH:-1}" == "1" ]] || api_auth_disabled="true"
    [[ "${NM_MODULE_REPUTATION:-1}" == "1" ]] || reputation_fetch_enabled="false"
    if [[ "$auth_disabled" == "true" || "$api_auth_disabled" == "true" ]]; then
        # Backend ValidateSecurity требует NM_ALLOW_INSECURE при *_DISABLED.
        allow_insecure="1"
    fi

    mkdir -p "$project_dir"
    touch "$env_file"

    tmp="$(mktemp)"
    grep -vE '^[[:space:]]*(# --- Модули \(select_modules\.sh\) ---|NM_MODULE_|AUTH_DISABLED=|API_AUTH_DISABLED=|REPUTATION_FETCH_ENABLED=|NM_ALLOW_INSECURE=|COMPOSE_PROFILES=)' \
        "$env_file" >"$tmp" || true
    # Убрать завершающие пустые строки.
    while [[ -s "$tmp" ]] && [[ -z "$(tail -n1 "$tmp")" ]]; do
        head -n -1 "$tmp" >"${tmp}.2" && mv "${tmp}.2" "$tmp"
    done
    cat >>"$tmp" <<EOF

# --- Модули (select_modules.sh) ---
NM_MODULE_AUTH=${NM_MODULE_AUTH:-1}
NM_MODULE_API_AUTH=${NM_MODULE_API_AUTH:-1}
NM_MODULE_SYSLOG=${NM_MODULE_SYSLOG:-1}
NM_MODULE_STATS=${NM_MODULE_STATS:-1}
NM_MODULE_REPUTATION=${NM_MODULE_REPUTATION:-1}
AUTH_DISABLED=${auth_disabled}
API_AUTH_DISABLED=${api_auth_disabled}
REPUTATION_FETCH_ENABLED=${reputation_fetch_enabled}
NM_ALLOW_INSECURE=${allow_insecure}
COMPOSE_PROFILES=${NM_COMPOSE_PROFILES:-}
EOF
    mv "$tmp" "$env_file"

    cat >"$modules_json" <<EOF
{
  "generated_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "core": ["clickhouse", "backend", "frontend"],
  "modules": {
    "auth": $([ "${NM_MODULE_AUTH:-1}" == "1" ] && echo true || echo false),
    "api_auth": $([ "${NM_MODULE_API_AUTH:-1}" == "1" ] && echo true || echo false),
    "syslog": $([ "${NM_MODULE_SYSLOG:-1}" == "1" ] && echo true || echo false),
    "stats": $([ "${NM_MODULE_STATS:-1}" == "1" ] && echo true || echo false),
    "reputation": $([ "${NM_MODULE_REPUTATION:-1}" == "1" ] && echo true || echo false)
  },
  "compose_profiles": "${NM_COMPOSE_PROFILES:-}",
  "auth_disabled": $([ "$auth_disabled" = "true" ] && echo true || echo false),
  "api_auth_disabled": $([ "$api_auth_disabled" = "true" ] && echo true || echo false),
  "reputation_fetch_enabled": $([ "$reputation_fetch_enabled" = "true" ] && echo true || echo false)
}
EOF

    _nm_mod_log "Записаны модули → ${env_file} и ${modules_json}"
    if [[ "$auth_disabled" == "true" ]]; then
        _nm_mod_log "UI-авторизация отключена (AUTH_DISABLED=true)."
    fi
    if [[ "$api_auth_disabled" == "true" ]]; then
        _nm_mod_log "API Bearer-защита отключена (API_AUTH_DISABLED=true)."
    fi
    if [[ "$reputation_fetch_enabled" == "false" ]]; then
        _nm_mod_log "Модуль репутации IP отключён (REPUTATION_FETCH_ENABLED=false)."
    fi
}

# Короткий запрос «продолжить установку?» перед стартом стека.
confirm_start_stack() {
    if [[ "${NM_AUTO_MODULES:-0}" == "1" ]] || [[ "${NM_FORCE:-0}" == "1" ]] || [[ "${FORCE:-0}" == "1" ]]; then
        return 0
    fi
    if [[ ! -t 0 ]]; then
        return 0
    fi
    _nm_mod_ensure_ui || true
    echo "" >&2
    if _nm_mod_yesno "Запустить стек Docker Compose сейчас?" 1; then
        return 0
    fi
    echo "Установка подготовила файлы, но стек не запущен. Позже: cd ${1:-/opt/network-monitor} && ./start.sh" >&2
    return 1
}

# Интерактивный вопрос про firewall (если ENABLE_* ещё не задан явно в env).
confirm_firewall() {
    # $1 — имя переменной: ENABLE_UFW или ENABLE_FIREWALL
    local var_name="$1"
    local current="${!var_name:-1}"

    # Если уже явно задано снаружи — не спрашиваем.
    if [[ -n "${!var_name+x}" ]] && [[ -n "${!var_name}" ]]; then
        # Переменная экспортирована вызывающим — оставляем как есть.
        # Но при первом запуске скрипта она всегда имеет default из install_*.sh,
        # поэтому проверяем NM_ASK_FIREWALL / TTY отдельно ниже.
        :
    fi

    if [[ "${NM_AUTO_MODULES:-0}" == "1" ]] || [[ ! -t 0 ]]; then
        return 0
    fi

    # Спрашиваем только если пользователь не передал явное значение через окружение
    # до запуска (см. install_*: если ENABLE_UFW задан снаружи, NM_FIREWALL_FROM_ENV=1).
    if [[ "${NM_FIREWALL_FROM_ENV:-0}" == "1" ]]; then
        return 0
    fi

    _nm_mod_ensure_ui || true
    echo "" >&2
    local ports="${HTTP_PORT:-80}"
    local https_on=0
    case "${HTTPS_ENABLED:-}" in
        1|true|TRUE|yes|YES|on|ON|auto) https_on=1 ;;
    esac
    if [[ "$https_on" != "1" ]]; then
        local root="${PROJECT_DIR:-.}"
        if [[ -f "${root}/certs/fullchain.pem" && -f "${root}/certs/privkey.pem" ]]; then
            https_on=1
        fi
    fi
    if [[ "$https_on" == "1" ]]; then
        ports="${ports}, ${HTTPS_PORT:-443}"
    fi
    if _nm_mod_yesno "Настроить правила firewall (порты ${ports}, 514/tcp+udp)?" "${current}"; then
        printf -v "$var_name" '%s' "1"
    else
        printf -v "$var_name" '%s' "0"
        _nm_mod_log "Настройка firewall пропущена."
    fi
    export "${var_name?}"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    confirm_modules
    apply_module_selection "${1:-.}"
    print_modules_summary
fi
