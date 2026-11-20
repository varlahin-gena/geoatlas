#!/usr/bin/env bash
# Обновление ГеоАтлас из локального пакета (tar.gz или распакованный каталог).
# Тома Docker / .env / сертификаты / профиль сохраняются.
#
#   sudo ./update.sh /path/to/geoatlas-2.8.1.tar.gz
#   tar -xzf geoatlas-2.8.1.tar.gz && sudo ./geoatlas-2.8.1/update.sh
#
# Опции:
#   --package PATH     пакет (.tar.gz или каталог)
#   --project-dir DIR  каталог установки (по умолчанию /opt/geoatlas)
#   --no-stop          не останавливать систему до обновления
#   --no-start         не запускать ./start.sh после наложения
#   -y, --yes          без TUI (FORCE=1)
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"

PROJECT_DIR="${GA_PROJECT_DIR:-/opt/geoatlas}"
DO_STOP=1
DO_START=1
PACKAGE="${GA_INSTALL_PACKAGE:-}"
GA_FORCE="${GA_FORCE:-${FORCE:-0}}"
GA_WIZARD_USED=0
PACKAGE_FROM_CLI=0
STOP_FROM_CLI=0
START_FROM_CLI=0

log() { echo "[$(date +'%F %T')] [update] $*"; }

trap 'log "ОШИБКА на строке ${LINENO} (код выхода $?)."' ERR

usage() {
    cat <<'EOF'
Обновление ГеоАтлас из установочного пакета.

  sudo ./update.sh /path/to/geoatlas-X.Y.Z.tar.gz
  sudo ./update.sh --package /path/to/geoatlas-X.Y.Z

В интерактивном режиме: приветствие, аудит версий, выбор пакета (если не указан),
план (остановить / запустить), подтверждение. Данные и .env сохраняются.

Опции:
  --package PATH      tar.gz или распакованный каталог
  --project-dir DIR   каталог установки (по умолчанию /opt/geoatlas)
  --no-stop           не останавливать систему до обновления
  --no-start          не запускать ./start.sh после наложения
  -y, --yes           без подтверждений (FORCE=1), для CI/скриптов
  -h, --help          эта справка

Переменные: FORCE=1  GA_PROJECT_DIR  GA_INSTALL_PACKAGE  GA_UI

Первичная установка из пакета — через deploy/install.sh
из распакованного geoatlas-X.Y.Z.
EOF
}

require_root() {
    if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
        echo "Запустите от root: sudo $0 $*" >&2
        exit 1
    fi
}

_source_helpers() {
    local dir="$1"
    local apply="${dir}/deploy/common/apply_package.sh"
    local src="${dir}/deploy/common/select_source.sh"
    local ui="${dir}/deploy/common/ui.sh"
    if [[ -f "$apply" ]]; then
        # shellcheck source=deploy/common/apply_package.sh
        source "$apply"
    fi
    if [[ -f "$src" ]]; then
        # shellcheck source=deploy/common/select_source.sh
        source "$src"
    fi
    if [[ -f "$ui" ]]; then
        # shellcheck source=deploy/common/ui.sh
        source "$ui"
    fi
}

_ga_ensure_ui() {
    if ! declare -F ga_ui_init >/dev/null 2>&1; then
        return 1
    fi
    if [[ -z "${GA_UI_BACKEND:-}" ]]; then
        ga_ui_init
    fi
    return 0
}

_ga_want_wizard() {
    [[ "$GA_FORCE" != "1" ]] || return 1
    [[ -t 0 ]] || [[ -r /dev/tty && -w /dev/tty ]] || return 1
    return 0
}

_ga_run_gauge_fn() {
    local title="$1" text="$2" fn="$3"
    if declare -F ga_ui_run_with_gauge >/dev/null 2>&1; then
        ga_ui_run_with_gauge "$title" "$text" "$fn"
    else
        "$fn"
    fi
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -h|--help)
                usage
                exit 0
                ;;
            --package)
                [[ $# -ge 2 ]] || { echo "--package требует путь" >&2; exit 1; }
                PACKAGE="$2"
                PACKAGE_FROM_CLI=1
                shift 2
                ;;
            --project-dir)
                [[ $# -ge 2 ]] || { echo "--project-dir требует путь" >&2; exit 1; }
                PROJECT_DIR="$2"
                shift 2
                ;;
            --no-stop)
                DO_STOP=0
                STOP_FROM_CLI=1
                shift
                ;;
            --no-start)
                DO_START=0
                START_FROM_CLI=1
                shift
                ;;
            -y|--yes)
                GA_FORCE=1
                FORCE=1
                shift
                ;;
            --)
                shift
                break
                ;;
            -*)
                echo "Неизвестная опция: $1" >&2
                usage >&2
                exit 1
                ;;
            *)
                if [[ -n "$PACKAGE" ]]; then
                    echo "Лишний аргумент: $1" >&2
                    exit 1
                fi
                PACKAGE="$1"
                PACKAGE_FROM_CLI=1
                shift
                ;;
        esac
    done
}

# Остановка до наложения пакета. Берём compose.sh из *этого* архива:
# установленный stop.sh ещё старый и падает на пустом CLICKHOUSE_PASSWORD.
# Заглушки только в subshell — иначе docker compose up подхватит их вместо .env.
ga_update_stop_stack() {
    local dir="$1"
    local compose="${SCRIPT_DIR}/deploy/common/compose.sh"
    log "Остановка системы…"
    if [[ -f "$compose" ]]; then
        if (
            # shellcheck source=deploy/common/compose.sh
            source "$compose"
            ga_compose "$dir" down --remove-orphans
        ); then
            return 0
        fi
        log "ВНИМАНИЕ: остановка через compose не удалась — пробуем установленный stop.sh."
    fi
    if [[ -x "${dir}/stop.sh" ]]; then
        if (
            # shellcheck source=deploy/common/compose.sh
            [[ -f "$compose" ]] && source "$compose"
            if declare -F _ga_compose_fill_stop_placeholders >/dev/null 2>&1; then
                _ga_compose_fill_stop_placeholders "$dir"
            fi
            "${dir}/stop.sh"
        ); then
            return 0
        fi
        log "ВНИМАНИЕ: ${dir}/stop.sh не удался."
    fi
    log "ВНИМАНИЕ: система может остаться запущенной — наложение пакета продолжается (start.sh пересоздаст контейнеры)."
    return 0
}

_ga_read_installed_version() {
    local dir="$1"
    local ver=""
    if [[ -f "${dir}/VERSION" ]]; then
        ver="$(tr -d '[:space:]' <"${dir}/VERSION" || true)"
    fi
    if [[ -z "$ver" && -f "${dir}/install-meta.json" ]]; then
        ver="$(grep -oE '"version"[[:space:]]*:[[:space:]]*"[^"]+"' "${dir}/install-meta.json" 2>/dev/null \
            | head -1 \
            | sed -E 's/.*"([^"]+)"[[:space:]]*$/\1/' || true)"
    fi
    echo "${ver:-unknown}"
}

resolve_default_package() {
    if [[ -n "$PACKAGE" ]]; then
        return 0
    fi
    # Распакованный пакет: ./geoatlas-X.Y.Z/update.sh без аргументов.
    # Не брать сам каталог установки (/opt/...) — иначе «обновление» будет no-op.
    if declare -F ga_package_looks_like_tree >/dev/null 2>&1 \
        && ga_package_looks_like_tree "$SCRIPT_DIR"; then
        if ! _ga_pkg_same_dir "$SCRIPT_DIR" "$PROJECT_DIR"; then
            PACKAGE="$SCRIPT_DIR"
            log "Пакет не указан — используем каталог скрипта (${SCRIPT_DIR})."
            return 0
        fi
    fi
    return 1
}

_ga_resolve_package_interactive() {
    if [[ -n "$PACKAGE" ]]; then
        return 0
    fi
    if resolve_default_package; then
        return 0
    fi
    if ! _ga_want_wizard || ! declare -F ga_ui_inputbox >/dev/null 2>&1; then
        echo "Укажите пакет (.tar.gz или каталог). См. --help." >&2
        exit 1
    fi
    local path=""
    if ! path="$(ga_ui_inputbox "Обновление ГеоАтлас" \
        "Путь к пакету (.tar.gz или распакованный каталог):" \
        "")"; then
        log "Отменено пользователем."
        exit 0
    fi
    path="${path#"${path%%[![:space:]]*}"}"
    path="${path%"${path##*[![:space:]]}"}"
    if [[ -z "$path" ]]; then
        echo "Пакет не указан." >&2
        exit 1
    fi
    PACKAGE="$path"
}

_ga_require_installed() {
    if [[ ! -f "${PROJECT_DIR}/docker-compose.yml" ]]; then
        echo "В ${PROJECT_DIR} нет установленной системы (нет docker-compose.yml)." >&2
        echo "Первичная установка из пакета:" >&2
        echo "  tar -xzf geoatlas-X.Y.Z.tar.gz && cd geoatlas-X.Y.Z" >&2
        echo "  sudo ./deploy/install.sh" >&2
        exit 1
    fi
}

_ga_welcome() {
    _ga_ensure_ui || true
    if ! declare -F ga_ui_msgbox >/dev/null 2>&1; then
        return 0
    fi
    ga_ui_msgbox "Обновление ГеоАтлас" \
"Будет обновлён код системы из пакета.

Данные ClickHouse, учётки, .env и сертификаты сохраняются.
Настройки доступа и пароль администратора не меняются." || true
}

_ga_audit_text() {
    local current="$1" pkg_ver="$2"
    local compose_ok="нет"
    [[ -f "${PROJECT_DIR}/docker-compose.yml" ]] && compose_ok="да"
    cat <<EOF
Каталог установки : ${PROJECT_DIR}
Установка найдена : ${compose_ok}
Текущая версия    : ${current}
Версия пакета     : ${pkg_ver:-?}
Пакет             : ${PACKAGE}
EOF
}

_ga_show_audit() {
    local current="$1" pkg_ver="$2"
    local text
    text="$(_ga_audit_text "$current" "$pkg_ver")"
    echo ""
    echo "══════════════════════════════════════════════════════════"
    echo "  Аудит перед обновлением"
    echo "══════════════════════════════════════════════════════════"
    echo "$text" | sed 's/^/  /'
    echo "══════════════════════════════════════════════════════════"
    echo ""
    _ga_ensure_ui || true
    if declare -F ga_ui_msgbox >/dev/null 2>&1 && [[ "${GA_UI_BACKEND:-text}" != "text" ]]; then
        ga_ui_msgbox "Аудит перед обновлением" "$text" || true
    fi
}

_ga_plan_stop_start() {
    if [[ "$STOP_FROM_CLI" == "1" || "$START_FROM_CLI" == "1" ]]; then
        return 0
    fi
    if ! _ga_want_wizard; then
        return 0
    fi
    _ga_ensure_ui || true
    GA_WIZARD_USED=1

    if declare -F ga_ui_checklist >/dev/null 2>&1; then
        local stop_def="ON" start_def="ON" selected=""
        [[ "$DO_STOP" == "1" ]] || stop_def="OFF"
        [[ "$DO_START" == "1" ]] || start_def="OFF"
        if selected="$(ga_ui_checklist \
            "План обновления" \
            "Отметьте шаги:" \
            stop "Остановить систему перед обновлением" "$stop_def" \
            start "Запустить систему после обновления" "$start_def")"; then
            DO_STOP=0
            DO_START=0
            local list=",${selected},"
            [[ "$list" == *",stop,"* ]] && DO_STOP=1
            [[ "$list" == *",start,"* ]] && DO_START=1
            return 0
        else
            log "Отменено пользователем."
            exit 0
        fi
    fi

    if declare -F ga_ui_yesno >/dev/null 2>&1; then
        if ga_ui_yesno "Обновление ГеоАтлас" "Остановить систему перед обновлением?" 1; then
            DO_STOP=1
        else
            DO_STOP=0
        fi
        if ga_ui_yesno "Обновление ГеоАтлас" "Запустить систему после обновления?" 1; then
            DO_START=1
        else
            DO_START=0
        fi
    fi
}

_ga_confirm() {
    local current="$1" pkg_ver="$2"
    if [[ "$GA_FORCE" == "1" ]]; then
        log "FORCE=1 — подтверждение пропущено."
        return 0
    fi
    if ! _ga_want_wizard; then
        return 0
    fi
    _ga_ensure_ui || true

    local stop_l="нет" start_l="нет"
    [[ "$DO_STOP" == "1" ]] && stop_l="да"
    [[ "$DO_START" == "1" ]] && start_l="да"

    local prompt
    prompt="Обновить ${PROJECT_DIR}?
Сейчас: ${current} → пакет: ${pkg_ver:-?}
Остановить до обновления: ${stop_l}
Запустить после: ${start_l}

Данные и .env сохраняются. Продолжить?"

    if declare -F ga_ui_yesno >/dev/null 2>&1; then
        if ! ga_ui_yesno "Подтверждение обновления" "$prompt" 1; then
            log "Отменено пользователем."
            exit 0
        fi
    else
        local answer
        read -r -p "Продолжить обновление? [Y/n]: " answer </dev/tty 2>/dev/null || answer=""
        if [[ "$answer" =~ ^([nN]|[nN][oO])$ ]]; then
            log "Отменено пользователем."
            exit 0
        fi
    fi
}

_ga_step_stop() {
    ga_update_stop_stack "$PROJECT_DIR"
}

GA_UPDATE_PAYLOAD=""
GA_UPDATE_VER=""

_ga_step_apply() {
    local payload="${1:-$GA_UPDATE_PAYLOAD}"
    local ver="${2:-$GA_UPDATE_VER}"
    ga_apply_install_payload "$payload" "$PROJECT_DIR"

    export GA_INSTALL_SOURCE=package
    if [[ -n "$ver" ]]; then
        export BRANCH="v${ver}"
        export GA_INSTALL_IS_TAG=1
    else
        export BRANCH="${BRANCH:-package}"
        export GA_INSTALL_IS_TAG=0
    fi

    if declare -F apply_install_source >/dev/null 2>&1; then
        apply_install_source "$PROJECT_DIR"
    fi

    if [[ -x "${PROJECT_DIR}/start.sh" ]]; then
        chmod +x "${PROJECT_DIR}/start.sh" "${PROJECT_DIR}/stop.sh" "${PROJECT_DIR}/update.sh" 2>/dev/null || true
    fi
}

_ga_step_apply_gauge() {
    _ga_step_apply "$GA_UPDATE_PAYLOAD" "$GA_UPDATE_VER"
}

_ga_step_load_images() {
    if [[ -f "${PROJECT_DIR}/deploy/common/load_images.sh" ]]; then
        # shellcheck source=/dev/null
        source "${PROJECT_DIR}/deploy/common/load_images.sh"
    elif [[ -f "${SCRIPT_DIR}/deploy/common/load_images.sh" ]]; then
        # shellcheck source=/dev/null
        source "${SCRIPT_DIR}/deploy/common/load_images.sh"
    fi
}

_ga_step_start() {
    _ga_step_load_images
    log "Запуск системы…"
    "${PROJECT_DIR}/start.sh"
}

_ga_step_images_only() {
    _ga_step_load_images
    if declare -F ga_load_release_images >/dev/null 2>&1 \
        && declare -F ga_release_images_present >/dev/null 2>&1 \
        && ga_release_images_present "$PROJECT_DIR"; then
        ga_load_release_images "$PROJECT_DIR"
    fi
    log "Старт пропущен. Запустите ${PROJECT_DIR}/start.sh"
}

_ga_print_summary() {
    local pkg_ver="$1"
    local new_ver
    new_ver="$(_ga_read_installed_version "$PROJECT_DIR")"
    log "Обновление завершено."
    [[ -n "$pkg_ver" ]] && log "Версия пакета: ${pkg_ver}"
    log "Установленная версия: ${new_ver}"

    _ga_ensure_ui || true
    if declare -F ga_ui_msgbox >/dev/null 2>&1; then
        local start_note="Система запущена."
        [[ "$DO_START" == "1" ]] || start_note="Запуск пропущен — выполните ${PROJECT_DIR}/start.sh"
        ga_ui_msgbox "Обновление завершено" \
"ГеоАтлас обновлён.
Версия: ${new_ver}
Каталог: ${PROJECT_DIR}

Данные и .env сохранены.
${start_note}" || true
    fi
}

main() {
    parse_args "$@"
    require_root
    GA_FORCE="${GA_FORCE:-${FORCE:-0}}"

    _source_helpers "$SCRIPT_DIR"
    if ! declare -F ga_apply_install_payload >/dev/null 2>&1; then
        echo "Не найден deploy/common/apply_package.sh рядом с update.sh." >&2
        exit 1
    fi

    _ga_require_installed

    if _ga_want_wizard; then
        _ga_ensure_ui || true
        _ga_welcome
    fi

    if [[ "$PACKAGE_FROM_CLI" == "1" ]]; then
        :
    elif ! resolve_default_package; then
        _ga_resolve_package_interactive
    fi

    if [[ -z "$PACKAGE" ]]; then
        echo "Укажите пакет (.tar.gz или каталог). См. --help." >&2
        exit 1
    fi
    if [[ ! -e "$PACKAGE" ]]; then
        echo "Пакет не найден: ${PACKAGE}" >&2
        exit 1
    fi

    local payload="$PACKAGE"
    local ver=""
    local current=""

    ver="$(ga_package_read_version "$payload" || true)"
    current="$(_ga_read_installed_version "$PROJECT_DIR")"
    if [[ -n "$ver" ]]; then
        log "Версия пакета: ${ver}"
    fi
    log "Текущая версия: ${current}"

    if _ga_want_wizard; then
        _ga_show_audit "$current" "${ver:-?}"
        _ga_plan_stop_start
        _ga_confirm "$current" "${ver:-?}"
    fi

    if [[ "$DO_STOP" == "1" ]]; then
        _ga_run_gauge_fn "Остановка" "Остановка системы…" _ga_step_stop
    else
        log "Остановка пропущена (--no-stop)."
    fi

    GA_UPDATE_PAYLOAD="$payload"
    GA_UPDATE_VER="$ver"
    _ga_run_gauge_fn "Наложение пакета" "Обновление файлов системы…" _ga_step_apply_gauge

    if [[ "$DO_START" == "1" ]]; then
        _ga_run_gauge_fn "Запуск" "Запуск системы…" _ga_step_start
    else
        _ga_step_images_only
    fi

    _ga_print_summary "$ver"
}

main "$@"
