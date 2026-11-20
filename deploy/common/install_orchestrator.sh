#!/usr/bin/env bash
# Общий оркестратор установки ГеоАтлас.
# Требует заранее загруженный OS-бэкенд (см. deploy/*/os_backend.sh) и:
#   PROJECT_DIR, GA_DEPLOY_DIR
# Вызов: ga_install_main "$@"

: "${PROJECT_DIR:=/opt/geoatlas}"

log() { echo "[$(date +'%F %T')] $*"; }

trap 'log "ОШИБКА на строке ${LINENO} (код выхода $?). Смотрите лог выше / docker compose logs."' ERR

_ga_banner_text() {
    local src_line steps
    if [[ -n "${GA_INSTALL_PACKAGE:-}" ]]; then
        src_line="Пакет установки: ${GA_INSTALL_PACKAGE}"
    else
        src_line="Пакет установки: текущий каталог"
    fi
    if declare -F ga_os_banner_steps >/dev/null 2>&1; then
        steps="$(ga_os_banner_steps)"
    else
        steps="$(cat <<EOF
Шаги:
  1. Подготовка системы
  2. Наложение пакета
  3. Доступ к сайту
  4. Режим производительности
  5. Пароль администратора
  6. Сетевой экран
  7. Запуск
EOF
)"
    fi
    cat <<EOF
Каталог: ${PROJECT_DIR}
${src_line}

${steps}
EOF
}

print_banner() {
    local text label
    text="$(_ga_banner_text)"
    label="${GA_OS_LABEL:-Linux}"
    echo ""
    echo "══════════════════════════════════════════════════════════"
    echo "  ГеоАтлас — установка (${label})"
    echo "══════════════════════════════════════════════════════════"
    echo "$text" | sed 's/^/  /'
    echo "══════════════════════════════════════════════════════════"
    echo ""
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
    local name="$1"
    local candidates=(
        "${PROJECT_DIR}/deploy/common/${name}"
        "${GA_DEPLOY_DIR}/common/${name}"
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
    if [[ "${GA_AUTO_MODULES:-0}" == "1" ]] || [[ ! -t 0 ]]; then
        return 0
    fi
    _ensure_dark_newt
    _source_ui || true
    if declare -F ga_ui_msgbox >/dev/null 2>&1; then
        local body
        if declare -F ga_os_welcome_lines >/dev/null 2>&1; then
            body="$(ga_os_welcome_lines)"
        else
            body="Дальше: подготовка системы, пакет, доступ к сайту,
режим работы, пароль администратора, сетевой экран и запуск."
        fi
        ga_ui_msgbox "Установка ГеоАтлас" \
"Добро пожаловать в мастер установки.

Каталог: ${PROJECT_DIR}

${body}" || true
    fi
}

require_root() {
    if [[ $EUID -ne 0 ]]; then
        echo "Запустите от имени root."
        exit 1
    fi
}

log_detected_os() {
    if [[ -f /etc/os-release ]]; then
        # shellcheck disable=SC1091
        . /etc/os-release
        log "Обнаружена ОС: ${PRETTY_NAME:-unknown} (family=${GA_OS_FAMILY:-unknown})"
        if [[ -n "${VERSION_ID:-}" ]]; then
            log "Мажорная версия: ${VERSION_ID%%.*}"
        fi
    fi
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
        "${GA_DEPLOY_DIR}/common/select_source.sh"
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
    log "select_source.sh недоступен. Запустите ./deploy/install.sh из распакованного пакета."
    exit 1
}

source_module_helper() {
    if _ga_source_common select_modules.sh; then
        return 0
    fi
    return 1
}

ga_enable_full_stack() {
    if source_module_helper && declare -F ga_enable_all_modules >/dev/null 2>&1; then
        ga_enable_all_modules
        return 0
    fi
    GA_MODULE_AUTH=1
    GA_MODULE_API_AUTH=1
    GA_MODULE_SYSLOG=1
    GA_MODULE_STATS=1
    GA_MODULE_REPUTATION=1
    GA_MODULE_DOZZLE=1
    GA_COMPOSE_PROFILES="syslog,stats,dozzle"
    export GA_MODULE_AUTH GA_MODULE_API_AUTH GA_MODULE_SYSLOG GA_MODULE_STATS
    export GA_MODULE_REPUTATION GA_MODULE_DOZZLE GA_COMPOSE_PROFILES
    log "Модули: полный стек (fallback без select_modules.sh)."
}

configure_ui_access() {
    local helper="${PROJECT_DIR}/deploy/common/select_ui_access.sh"
    if [[ ! -f "$helper" ]]; then
        helper="${GA_DEPLOY_DIR}/common/select_ui_access.sh"
    fi
    if [[ -f "$helper" ]]; then
        # shellcheck source=deploy/common/select_ui_access.sh
        source "$helper"
        confirm_ui_access "$PROJECT_DIR"
        return 0
    fi
    # Fallback: раздельные селекторы
    local https_h="${PROJECT_DIR}/deploy/common/select_https.sh"
    [[ -f "$https_h" ]] || https_h="${GA_DEPLOY_DIR}/common/select_https.sh"
    if [[ -f "$https_h" ]]; then
        export GA_SKIP_HTTPS_HTTP_CHAIN=1 GA_INSTALL_QUIET_DIALOGS=1
        # shellcheck source=/dev/null
        source "$https_h"
        confirm_https "$PROJECT_DIR"
    fi
    local port_h="${PROJECT_DIR}/deploy/common/select_http_port.sh"
    [[ -f "$port_h" ]] || port_h="${GA_DEPLOY_DIR}/common/select_http_port.sh"
    if [[ -f "$port_h" ]]; then
        # shellcheck source=/dev/null
        source "$port_h"
        confirm_http_port
    else
        HTTP_PORT="${HTTP_PORT:-80}"
        export HTTP_PORT GA_HTTP_PORT_CONFIRMED=1
    fi
}

configure_resources_select() {
    local detector="${PROJECT_DIR}/deploy/common/detect_resources.sh"
    if [[ ! -f "$detector" ]]; then
        detector="${GA_DEPLOY_DIR}/common/detect_resources.sh"
    fi
    if [[ ! -f "$detector" ]]; then
        log "Детектор ресурсов не найден — профиль по умолчанию medium."
        GA_SELECTED_PROFILE="${GA_SELECTED_PROFILE:-medium}"
        export GA_SELECTED_PROFILE
        return 0
    fi
    # shellcheck source=deploy/common/detect_resources.sh
    source "$detector"
    export GA_INSTALL_QUIET_DIALOGS=1
    select_resource_profile "$PROJECT_DIR"
}

confirm_admin_password_step() {
    # Auth всегда включён в полном стеке; skip если уже задан или auth выкл. через CI override.
    if [[ "${GA_MODULE_AUTH:-1}" != "1" ]]; then
        log "UI-авторизация отключена — шаг пароля пропущен."
        return 0
    fi
    if [[ -n "${AUTH_ADMIN_PASSWORD:-}" ]]; then
        log "Пароль администратора уже задан в окружении."
        return 0
    fi
    if [[ -f "${PROJECT_DIR}/.env" ]] \
        && grep -qE '^[[:space:]]*AUTH_ADMIN_PASSWORD=.+' "${PROJECT_DIR}/.env" 2>/dev/null; then
        log "Пароль администратора уже есть в .env."
        return 0
    fi

    if ! _ga_source_common admin_auth.sh; then
        log "admin_auth.sh не найден — пароль будет сгенерирован при записи профиля."
        return 0
    fi
    _source_ui || true

    if [[ "${GA_AUTO_MODULES:-0}" == "1" ]] || [[ ! -t 0 ]] \
        || ! declare -F ga_ui_passwordbox >/dev/null 2>&1; then
        log "Нет интерактива — пароль администратора сгенерирует финальная запись конфигурации."
        return 0
    fi

    # Не через $(...): иначе whiptail msgbox рисует в pipe и «зависает» после подтверждения.
    GA_ADMIN_PASSWORD_RESULT=""
    if ! ga_prompt_admin_password "${AUTH_ADMIN_USER:-admin}"; then
        log "Пароль администратора не задан — установка прервана."
        exit 1
    fi
    if [[ -z "${GA_ADMIN_PASSWORD_RESULT:-}" ]]; then
        log "Пароль администратора пуст — установка прервана."
        exit 1
    fi
    AUTH_ADMIN_USER="${AUTH_ADMIN_USER:-admin}"
    AUTH_ADMIN_PASSWORD="$GA_ADMIN_PASSWORD_RESULT"
    AUTH_ADMIN_MUST_RESET=0
    GA_ADMIN_PASSWORD_PRINT=0
    export AUTH_ADMIN_USER AUTH_ADMIN_PASSWORD AUTH_ADMIN_MUST_RESET GA_ADMIN_PASSWORD_PRINT
    unset GA_ADMIN_PASSWORD_RESULT
    log "Пароль администратора принят."
}

ga_finalize_install_env() {
    cd "$PROJECT_DIR"
    export GA_SKIP_ADMIN_PROMPT=1

    local detector="${PROJECT_DIR}/deploy/common/detect_resources.sh"
    if [[ ! -f "$detector" ]]; then
        detector="${GA_DEPLOY_DIR}/common/detect_resources.sh"
    fi
    if [[ -f "$detector" ]]; then
        # shellcheck source=deploy/common/detect_resources.sh
        source "$detector"
        if declare -F commit_resource_profile >/dev/null 2>&1; then
            commit_resource_profile "$PROJECT_DIR"
        else
            apply_resource_profile "$PROJECT_DIR"
        fi
    else
        log "detect_resources.sh не найден — .env профиля не записан."
    fi

    if source_module_helper && declare -F apply_module_selection >/dev/null 2>&1; then
        apply_module_selection "$PROJECT_DIR"
    fi

    if declare -F apply_install_source >/dev/null 2>&1; then
        apply_install_source "$PROJECT_DIR"
    elif [[ -f "${PROJECT_DIR}/deploy/common/select_source.sh" ]]; then
        # shellcheck source=deploy/common/select_source.sh
        source "${PROJECT_DIR}/deploy/common/select_source.sh"
        apply_install_source "$PROJECT_DIR"
    fi

    if _ga_source_common install_meta.sh && declare -F ga_write_install_meta >/dev/null 2>&1; then
        ga_write_install_meta "$PROJECT_DIR"
    fi
}

print_install_summary() {
    local https_line="выкл. (HTTP)"
    local fw_line="не настраивался"
    local var_name="${GA_OS_FIREWALL_VAR:-ENABLE_UFW}"
    local profile_label="${GA_SELECTED_PROFILE:-—}"

    case "${HTTPS_ENABLED:-0}" in
        1|true|TRUE|yes|YES|on|ON)
            https_line="вкл., порт ${HTTPS_PORT:-443}"
            ;;
        auto)
            https_line="авто (если есть сертификаты), порт ${HTTPS_PORT:-443}"
            ;;
    esac
    if [[ "${!var_name:-0}" == "1" ]]; then
        fw_line="открыть порты"
    else
        fw_line="пропущен"
    fi
    if declare -F profile_label_ru >/dev/null 2>&1 && [[ -n "${GA_SELECTED_PROFILE:-}" ]]; then
        profile_label="$(profile_label_ru "${GA_SELECTED_PROFILE}")"
    fi

    local summary
    summary="Каталог: ${PROJECT_DIR}
Стек: полный (сайт, логи, метрики, репутация, журналы контейнеров)
HTTP-порт: ${HTTP_PORT:-80}
HTTPS: ${https_line}
Режим: ${profile_label}
Сетевой экран: ${fw_line}
Администратор: ${AUTH_ADMIN_USER:-admin}"

    if [[ "${GA_ADMIN_PASSWORD_PRINT:-0}" == "1" ]]; then
        summary="${summary}
Пароль: будет в файле ${PROJECT_DIR}/.admin_password_once после запуска"
    fi

    echo ""
    echo "══════════════════════════════════════════════════════════"
    echo "  Итог установки"
    echo "══════════════════════════════════════════════════════════"
    echo "$summary" | sed 's/^/  /'
    echo "══════════════════════════════════════════════════════════"
    echo ""

    if [[ "${GA_AUTO_MODULES:-0}" != "1" ]] && [[ -t 0 ]] \
        && declare -F ga_ui_msgbox >/dev/null 2>&1 \
        && [[ "${GA_UI_BACKEND:-text}" != "text" ]]; then
        ga_ui_msgbox "Итог установки" "$summary" || true
    fi
}

_chmod_install_scripts() {
    local f extra
    log "Выставление прав на исполнение..."
    for f in start.sh stop.sh update.sh \
             scripts/tune-resources.sh \
             scripts/pack-release.sh \
             scripts/backup-clickhouse.sh \
             scripts/restore-clickhouse.sh \
             clickhouse/backfill_edges_agg.sh \
             clickhouse/reset_data.sh \
             deploy/uninstall.sh \
             deploy/install.sh \
             deploy/common/detect_resources.sh \
             deploy/common/select_modules.sh \
             deploy/common/select_source.sh \
             deploy/common/apply_package.sh \
             deploy/common/select_http_port.sh \
             deploy/common/select_https.sh \
             deploy/common/select_ui_access.sh \
             deploy/common/firewall_helpers.sh \
             deploy/common/ui.sh \
             deploy/common/load_images.sh \
             deploy/common/admin_auth.sh \
             deploy/common/uninstall.sh \
             deploy/common/install_orchestrator.sh \
             deploy/common/install_meta.sh; do
        if [[ -f "$f" ]]; then
            chmod +x "$f"
        fi
    done
    if declare -F ga_os_chmod_extra >/dev/null 2>&1; then
        while IFS= read -r extra; do
            [[ -z "$extra" ]] && continue
            if [[ -f "$extra" ]]; then
                chmod +x "$extra"
            fi
        done < <(ga_os_chmod_extra)
    fi
}

prepare_project() {
    cd "$PROJECT_DIR"

    [[ -f docker-compose.yml ]] || { log "docker-compose.yml не найден после наложения пакета."; exit 1; }

    _chmod_install_scripts

    log "Включение полного стека модулей..."
    ga_enable_full_stack

    log "Настройка доступа к веб-интерфейсу..."
    configure_ui_access

    log "Анализ ресурсов и выбор режима производительности..."
    configure_resources_select

    log "Пароль администратора..."
    confirm_admin_password_step

    log "Запись конфигурации..."
    ga_finalize_install_env

    print_install_summary
}

_firewall_enabled() {
    local var_name="${GA_OS_FIREWALL_VAR:-ENABLE_UFW}"
    [[ "${!var_name:-0}" == "1" ]]
}

ask_firewall() {
    local var_name="${GA_OS_FIREWALL_VAR:-ENABLE_UFW}"

    if source_module_helper && declare -F confirm_firewall >/dev/null 2>&1; then
        confirm_firewall "$var_name"
    elif [[ "${GA_FIREWALL_FROM_ENV:-0}" == "1" ]] || [[ "${GA_AUTO_MODULES:-0}" == "1" ]] || [[ ! -t 0 ]]; then
        :
    else
        printf -v "$var_name" '%s' "1"
        export "${var_name?}"
    fi

    # Ubuntu: отдельно предложить включить экран (правила уже будут добавлены).
    if [[ "${GA_OS_FAMILY:-}" == "debian" ]] && [[ "${ENABLE_UFW:-0}" == "1" ]]; then
        if [[ "${UFW_AUTO_ENABLE:-0}" == "1" ]]; then
            return 0
        fi
        if [[ "${GA_FIREWALL_FROM_ENV:-0}" == "1" ]] || [[ "${GA_AUTO_MODULES:-0}" == "1" ]] || [[ ! -t 0 ]]; then
            return 0
        fi
        _source_ui || true
        if declare -F ga_ui_yesno >/dev/null 2>&1; then
            if ga_ui_yesno "Сетевой экран" \
                "Порты будут открыты в правилах.
Включить сетевой экран сейчас?
(Если экран уже управляется отдельно — можно ответить «Нет».)" 0; then
                UFW_AUTO_ENABLE=1
                export UFW_AUTO_ENABLE
            fi
        fi
    fi
}

start_stack() {
    cd "$PROJECT_DIR"

    if source_module_helper && declare -F confirm_start_stack >/dev/null 2>&1; then
        if ! confirm_start_stack "$PROJECT_DIR"; then
            log "Запуск системы пропущен пользователем."
            return 0
        fi
    fi

    log "Запуск через ./start.sh ..."
    ./start.sh
}

_ga_require_os_backend() {
    local missing=0
    local fn
    for fn in ga_os_install_packages ga_os_install_docker ga_os_configure_firewall; do
        if ! declare -F "$fn" >/dev/null 2>&1; then
            log "OS-бэкенд не загружен: нет ${fn}"
            missing=1
        fi
    done
    if [[ -z "${GA_OS_FAMILY:-}" || -z "${GA_OS_FIREWALL_VAR:-}" ]]; then
        log "OS-бэкенд не загружен: нет GA_OS_FAMILY / GA_OS_FIREWALL_VAR"
        missing=1
    fi
    if [[ "$missing" -ne 0 ]]; then
        exit 1
    fi
}

ga_install_main() {
    _ga_require_os_backend
    require_root
    log_detected_os
    _ensure_dark_newt
    _source_ui || true
    print_banner
    _welcome_dialog

    if declare -F ga_os_before_docker >/dev/null 2>&1; then
        if [[ "${GA_OS_FAMILY}" == "rhel" ]]; then
            _ga_run_gauge_fn "Подготовка" "Освобождение места под Docker…" ga_os_before_docker
        else
            ga_os_before_docker
        fi
    fi

    _ga_run_gauge_fn "Пакеты" "${GA_OS_PACKAGES_GAUGE_TEXT:-Установка зависимостей…}" ga_os_install_packages
    _source_ui || true
    _ga_run_gauge_fn "Docker" "Установка Docker…" ga_os_install_docker

    if declare -F ga_os_after_docker >/dev/null 2>&1; then
        if [[ "${GA_OS_FAMILY}" == "rhel" ]]; then
            _ga_run_gauge_fn "Безопасность" "Настройка политики безопасности…" ga_os_after_docker
        else
            ga_os_after_docker
        fi
    fi

    choose_install_source
    _ga_run_gauge_fn "Пакет" "Установка файлов ГеоАтлас…" apply_install_package
    _source_ui || true
    prepare_project
    ask_firewall
    if _firewall_enabled; then
        _ga_run_gauge_fn "Сетевой экран" "${GA_OS_FIREWALL_GAUGE_TEXT:-Открытие портов…}" ga_os_configure_firewall
    else
        ga_os_configure_firewall
    fi
    start_stack
}
