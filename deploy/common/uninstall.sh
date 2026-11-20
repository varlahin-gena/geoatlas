#!/usr/bin/env bash
# Общая логика удаления ГеоАтлас.
# Подключается из deploy/uninstall.sh после OS-бэкенда (os_backend.sh).
#
# OS-бэкенд должен определить ga_remove_firewall_rules() / ga_os_remove_firewall_rules().
# Опционально: ga_audit_firewall() / ga_os_audit_firewall() для строки аудита.
#
# Переменные окружения:
#   GA_PROJECT_DIR          — каталог проекта (по умолчанию /opt/geoatlas)
#   GA_UNINSTALL_PRESET     — stop | clean | purge (без интерактива)
#   GA_DRY_RUN=1            — только план, без изменений
#   GA_FORCE / FORCE=1      — без подтверждения (CI/Ansible)
#   REMOVE_DOCKER_VOLUMES   — 1 = удалить тома ClickHouse / backups / auth-users / syslog-ng
#   REMOVE_PROJECT_FILES    — 1 = удалить каталог проекта
#   REMOVE_IMAGES           — 1 = docker compose down --rmi local
#   REMOVE_FIREWALL_RULES   — 1 = вызвать ga_remove_firewall_rules (HTTP/HTTPS/514)
#
# CLI: --help --dry-run -y|--yes --preset --purge|--clean|--stop
#      --volumes --images --keep-files --no-firewall
# TUI: только clean | purge (stop/custom — через CLI).

set -Eeuo pipefail

# GA_PROJECT_DIR — канон; PROJECT_DIR принимаем как legacy alias.
if [[ -z "${GA_PROJECT_DIR:-}" && -n "${PROJECT_DIR:-}" ]]; then
    GA_PROJECT_DIR="$PROJECT_DIR"
fi
GA_PROJECT_DIR="${GA_PROJECT_DIR:-/opt/geoatlas}"
PROJECT_DIR="${PROJECT_DIR:-$GA_PROJECT_DIR}"
GA_DRY_RUN="${GA_DRY_RUN:-0}"
GA_FORCE="${GA_FORCE:-${FORCE:-0}}"
GA_PRESET="${GA_UNINSTALL_PRESET:-${GA_PRESET:-}}"
GA_WIZARD_USED="${GA_WIZARD_USED:-0}"
REMOVE_DOCKER_VOLUMES="${REMOVE_DOCKER_VOLUMES:-0}"
REMOVE_PROJECT_FILES="${REMOVE_PROJECT_FILES:-1}"
REMOVE_IMAGES="${REMOVE_IMAGES:-0}"
REMOVE_FIREWALL_RULES="${REMOVE_FIREWALL_RULES:-1}"

_ga_log() { echo "[$(date +'%F %T')] $*"; }

_ga_ensure_ui() {
    if ! declare -F ga_ui_yesno >/dev/null 2>&1; then
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

_ga_run_gauge_fn() {
    local title="$1" text="$2" fn="$3"
    if declare -F ga_ui_run_with_gauge >/dev/null 2>&1; then
        ga_ui_run_with_gauge "$title" "$text" "$fn"
    else
        "$fn"
    fi
}

_ga_trap_err() {
    _ga_log "ОШИБКА в ${BASH_SOURCE[1]:-?}:${BASH_LINENO[0]:-${LINENO}} (код выхода $?)."
}

trap '_ga_trap_err' ERR

_ga_require_root() {
    if [[ $EUID -ne 0 ]]; then
        echo "Запустите от имени root."
        exit 1
    fi
}

_ga_yesno() {
    # $1 — текст; по умолчанию No.
    local prompt="$1"
    if _ga_ensure_ui; then
        ga_ui_yesno "Удаление ГеоАтлас" "$prompt" 0
        return
    fi
    local answer
    read -r -p "$prompt [y/N]: " answer </dev/tty 2>/dev/null || answer=""
    [[ "$answer" =~ ^([yY]|[yY][eE][sS])$ ]]
}

_ga_yesno_default_yes() {
    # $1 — текст; по умолчанию Yes.
    local prompt="$1"
    if _ga_ensure_ui; then
        ga_ui_yesno "Удаление ГеоАтлас" "$prompt" 1
        return
    fi
    local answer
    read -r -p "$prompt [Y/n]: " answer </dev/tty 2>/dev/null || answer=""
    [[ ! "$answer" =~ ^([nN]|[nN][oO])$ ]]
}

_ga_map_legacy_env() {
    # Ubuntu: REMOVE_UFW_RULES → REMOVE_FIREWALL_RULES
    if [[ -n "${REMOVE_UFW_RULES:-}" ]]; then
        REMOVE_FIREWALL_RULES="$REMOVE_UFW_RULES"
    fi
    GA_FORCE="${GA_FORCE:-${FORCE:-0}}"
}

_ga_show_help() {
    cat <<'EOF'
ГеоАтлас — удаление установки

Использование:
  sudo bash deploy/uninstall.sh [опции]

  GA_FORCE_OS_FAMILY=debian|rhel — принудительно выбрать семейство ОС

Опции:
  -h, --help          показать эту справку
  --dry-run           показать план и аудит, ничего не менять
  -y, --yes           не спрашивать подтверждение (FORCE=1)
  --preset PRESET     stop | clean | purge
  --stop              только остановить систему (preset stop)
  --clean             безопасное удаление (preset clean, по умолчанию)
  --purge             полное удаление включая данные (preset purge)
  --volumes           удалить данные ClickHouse, бэкапы и учётки
  --images            удалить образы контейнеров системы
  --keep-files        сохранить каталог проекта
  --no-firewall       не трогать правила сетевого экрана

Пресеты:
  stop   — остановить систему, всё остальное сохранить
  clean  — stop + удалить файлы проекта + сетевой экран (данные сохраняются)
  purge  — clean + данные (CH/бэкапы/учётки) + образы контейнеров

В интерактивном меню доступны только clean и purge; stop — через --stop.

Переменные окружения (для Ansible/CI):
  FORCE=1  GA_DRY_RUN=1  GA_UNINSTALL_PRESET=purge
  REMOVE_DOCKER_VOLUMES=1  REMOVE_PROJECT_FILES=0  REMOVE_IMAGES=1
  REMOVE_FIREWALL_RULES=0  REMOVE_UFW_RULES=0 (Ubuntu, legacy)

Примеры:
  sudo bash deploy/uninstall.sh
  sudo bash deploy/uninstall.sh --dry-run
  sudo bash deploy/uninstall.sh --purge --yes
  sudo GA_UNINSTALL_PRESET=clean FORCE=1 bash deploy/uninstall.sh
EOF
}

_ga_parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -h|--help)
                _ga_show_help
                exit 0
                ;;
            --dry-run)
                GA_DRY_RUN=1
                shift
                ;;
            -y|--yes)
                GA_FORCE=1
                FORCE=1
                shift
                ;;
            --preset)
                GA_PRESET="${2:-}"
                shift 2
                ;;
            --stop)
                GA_PRESET="stop"
                shift
                ;;
            --clean)
                GA_PRESET="clean"
                shift
                ;;
            --purge)
                GA_PRESET="purge"
                shift
                ;;
            --volumes)
                REMOVE_DOCKER_VOLUMES=1
                shift
                ;;
            --images)
                REMOVE_IMAGES=1
                shift
                ;;
            --keep-files)
                REMOVE_PROJECT_FILES=0
                shift
                ;;
            --no-firewall)
                REMOVE_FIREWALL_RULES=0
                shift
                ;;
            *)
                _ga_log "Неизвестная опция: $1 (используйте --help)"
                exit 1
                ;;
        esac
    done
}

_ga_apply_preset() {
    local preset="$1"
    GA_PRESET="$preset"
    case "$preset" in
        stop)
            REMOVE_DOCKER_VOLUMES=0
            REMOVE_PROJECT_FILES=0
            REMOVE_IMAGES=0
            REMOVE_FIREWALL_RULES=0
            ;;
        clean)
            REMOVE_DOCKER_VOLUMES=0
            REMOVE_PROJECT_FILES=1
            REMOVE_IMAGES=0
            REMOVE_FIREWALL_RULES=1
            ;;
        purge)
            REMOVE_DOCKER_VOLUMES=1
            REMOVE_PROJECT_FILES=1
            REMOVE_IMAGES=1
            REMOVE_FIREWALL_RULES=1
            ;;
        *)
            _ga_log "ОШИБКА: неизвестный preset «${preset}». Допустимо: stop, clean, purge."
            exit 1
            ;;
    esac
    _ga_log "Применён preset: ${preset}"
}

_ga_preset_label() {
    case "$1" in
        stop)  echo "только остановить систему" ;;
        clean) echo "безопасное удаление (данные сохраняются)" ;;
        purge) echo "полное удаление (данные ClickHouse, бэкапы, учётки)" ;;
        *)     echo "$1" ;;
    esac
}

_ga_interactive_wizard() {
    _ga_ensure_ui || true

    if declare -F ga_ui_radiolist >/dev/null 2>&1; then
        local choice
        if ! choice="$(ga_ui_radiolist \
            "Удаление ГеоАтлас" \
            "Выберите режим удаления:" \
            clean "Безопасное удаление — остановить систему и убрать файлы; данные сохраняются" ON \
            purge "Полное удаление — плюс данные ClickHouse, бэкапы, учётки и образы" OFF \
            cancel "Отмена" OFF)"; then
            _ga_log "Отменено пользователем."
            exit 0
        fi
        case "${choice,,}" in
            clean)
                _ga_apply_preset clean
                GA_WIZARD_USED=1
                return 0
                ;;
            purge)
                _ga_apply_preset purge
                GA_WIZARD_USED=1
                return 0
                ;;
            cancel|q|quit|"")
                _ga_log "Отменено пользователем."
                exit 0
                ;;
            *)
                _ga_log "Неизвестный выбор «${choice}» — применяем clean."
                _ga_apply_preset clean
                GA_WIZARD_USED=1
                return 0
                ;;
        esac
    fi

    echo ""
    echo "══════════════════════════════════════════════════════════"
    echo "  Удаление ГеоАтлас"
    echo "══════════════════════════════════════════════════════════"
    echo ""
    echo "  [1] Безопасное удаление — остановить систему, убрать файлы и правила экрана"
    echo "      (данные ClickHouse, бэкапы и учётки сохраняются)"
    echo "  [2] Полное удаление — всё включая данные и образы контейнеров"
    echo "  [q] Отмена"
    echo ""

    local choice
    while true; do
        read -r -p "Ваш выбор [1]: " choice </dev/tty 2>/dev/null || choice=""
        choice="${choice,,}"
        choice="${choice//[[:space:]]/}"

        case "$choice" in
            ""|1|clean)
                _ga_apply_preset clean
                GA_WIZARD_USED=1
                return 0
                ;;
            2|purge|full)
                _ga_apply_preset purge
                GA_WIZARD_USED=1
                return 0
                ;;
            q|quit|cancel|отмена)
                _ga_log "Отменено пользователем."
                exit 0
                ;;
            *)
                echo "  Не понял «${choice}». Enter = [1], либо 2/q." >&2
                ;;
        esac
    done
}
_ga_format_bytes() {
    local bytes="$1"
    if (( bytes >= 1073741824 )); then
        awk -v b="$bytes" 'BEGIN { printf "%.1f GiB", b / 1073741824 }'
    elif (( bytes >= 1048576 )); then
        awk -v b="$bytes" 'BEGIN { printf "%.1f MiB", b / 1048576 }'
    elif (( bytes > 0 )); then
        awk -v b="$bytes" 'BEGIN { printf "%.0f KiB", b / 1024 }'
    else
        echo "—"
    fi
}

_ga_audit_compose() {
    local dir="$1"
    if [[ ! -f "${dir}/docker-compose.yml" ]] || ! command -v docker >/dev/null 2>&1; then
        echo "  Docker Compose   : не найден или docker недоступен"
        return
    fi

    local ps_out running total
    ps_out="$(cd "$dir" && docker compose ps -a --format 'table {{.Name}}\t{{.Status}}' 2>/dev/null || true)"
    if [[ -n "$ps_out" ]]; then
        running="$(echo "$ps_out" | tail -n +2 | grep -ci 'up' 2>/dev/null || true)"
        total="$(echo "$ps_out" | tail -n +2 | grep -c . 2>/dev/null || true)"
        echo "  Контейнеры       : ${running}/${total} запущено"
        # sed вместо while read — иначе EOF от read (exit 1) ломает set -e
        echo "$ps_out" | tail -n +2 | sed '/./s/^/                     /'
    else
        echo "  Контейнеры       : стек не запущен или не найден"
    fi
}

_ga_audit_volumes() {
    if ! command -v docker >/dev/null 2>&1; then
        echo "  Данные           : Docker недоступен"
        return
    fi

    local volumes=()
    mapfile -t volumes <<< "$(docker volume ls -q 2>/dev/null || true)"

    local vol found=0 mount du_size
    for vol in "${volumes[@]}"; do
        [[ -z "$vol" ]] && continue
        case "$vol" in
            *geoatlas*|*clickhouse*|*syslog-ng*|*auth-users*) ;;
            *) continue ;;
        esac
        found=1
        mount="$(docker volume inspect "$vol" --format '{{.Mountpoint}}' 2>/dev/null || true)"
        if [[ -n "$mount" && -d "$mount" ]]; then
            du_size="$(du -sb "$mount" 2>/dev/null | cut -f1 || echo 0)"
            echo "  Данные           : ${vol} ($(_ga_format_bytes "${du_size:-0}"))"
        else
            echo "  Данные           : ${vol}"
        fi
    done

    if (( found == 0 )); then
        echo "  Данные           : тома проекта не найдены"
    fi
}

_ga_audit_project_dir() {
    local dir="$1"
    if [[ -d "$dir" ]]; then
        local size
        size="$(du -sh "$dir" 2>/dev/null | awk '{print $1}')"
        echo "  Каталог проекта  : ${dir} (${size:-?})"
    else
        echo "  Каталог проекта  : ${dir} (не найден)"
    fi
}

ga_audit_installation() {
    local audit_text=""
    set +e
    audit_text="$(
        set +e
        set +o pipefail
        _ga_audit_project_dir "$GA_PROJECT_DIR"
        _ga_audit_compose "$GA_PROJECT_DIR"
        _ga_audit_volumes
        if declare -F ga_os_audit_firewall >/dev/null; then
            ga_os_audit_firewall
        elif declare -F ga_audit_firewall >/dev/null; then
            ga_audit_firewall
        fi
    )"
    set -e
    set -o pipefail

    echo ""
    echo "══════════════════════════════════════════════════════════"
    echo "  Аудит установки"
    echo "══════════════════════════════════════════════════════════"
    echo "$audit_text"
    echo "══════════════════════════════════════════════════════════"
    echo ""

    if [[ -t 0 && "$GA_FORCE" != "1" && "$GA_DRY_RUN" != "1" ]]; then
        _ga_ensure_ui || true
        if declare -F ga_ui_msgbox >/dev/null 2>&1 && [[ "${GA_UI_BACKEND:-text}" != "text" ]]; then
            local compact
            compact="$(echo "$audit_text" | sed 's/^  //')"
            ga_ui_msgbox "Аудит установки" "$compact" || true
        fi
    fi
}

_ga_print_plan() {
    echo ""
    _ga_log "==================== ПЛАН УДАЛЕНИЯ ===================="
    _ga_log "Каталог проекта       : ${GA_PROJECT_DIR}"
    _ga_log "Остановить систему    : да"
    if [[ "$REMOVE_DOCKER_VOLUMES" == "1" ]]; then
        _ga_log "Данные                : БУДУТ УДАЛЕНЫ (ClickHouse, бэкапы, учётки)  <-- НЕОБРАТИМО"
    else
        _ga_log "Данные                : сохраняются (ClickHouse, бэкапы, учётки)"
    fi
    if [[ "$REMOVE_IMAGES" == "1" ]]; then
        _ga_log "Образы контейнеров    : будут удалены"
    else
        _ga_log "Образы контейнеров    : сохраняются"
    fi
    if [[ "$REMOVE_FIREWALL_RULES" == "1" ]]; then
        _ga_log "Правила сетевого экрана: будут удалены"
    else
        _ga_log "Правила сетевого экрана: сохраняются"
    fi
    if [[ "$REMOVE_PROJECT_FILES" == "1" ]]; then
        if [[ "$REMOVE_DOCKER_VOLUMES" == "1" ]]; then
            _ga_log "Каталог проекта       : БУДЕТ УДАЛЁН (${GA_PROJECT_DIR})"
        else
            _ga_log "Каталог проекта       : будет удалён (данные ClickHouse/бэкапы/учётки сохранятся)"
        fi
    else
        _ga_log "Каталог проекта       : сохраняется"
    fi
    if [[ -n "$GA_PRESET" ]]; then
        _ga_log "Режим                 : ${GA_PRESET} ($(_ga_preset_label "$GA_PRESET"))"
    fi
    if [[ "$GA_DRY_RUN" == "1" ]]; then
        _ga_log "Проверка              : DRY-RUN (изменения не будут применены)"
    fi
    _ga_log "======================================================"
    echo ""
}

_ga_confirm() {
    if [[ "$GA_FORCE" == "1" ]]; then
        _ga_log "FORCE=1 — подтверждение пропущено."
        return 0
    fi

    if [[ ! -t 0 ]]; then
        _ga_log "Нет интерактивного терминала, а FORCE не задан. Прерывание."
        _ga_log "Запустите с --yes или FORCE=1 для неинтерактивного удаления."
        exit 1
    fi

    # Выбор в wizard — уже подтверждение; лишние вопросы не задаём.
    if [[ "${GA_WIZARD_USED}" == "1" ]]; then
        if [[ "$REMOVE_DOCKER_VOLUMES" == "1" ]]; then
            if ! _ga_yesno "Будут удалены данные ClickHouse, бэкапы и учётки. Это необратимо. Продолжить?"; then
                _ga_log "Отменено пользователем."
                exit 0
            fi
        fi
        _ga_log "Запуск удаления (режим: ${GA_PRESET:-clean})..."
        return 0
    fi

    if ! _ga_yesno "Продолжить удаление?"; then
        _ga_log "Отменено пользователем."
        exit 0
    fi

    if [[ "$REMOVE_DOCKER_VOLUMES" == "1" ]]; then
        if ! _ga_yesno "Будут удалены данные ClickHouse, бэкапы и учётки. Это необратимо. Продолжить?"; then
            _ga_log "Отменено пользователем."
            exit 0
        fi
    fi
}

_ga_step() {
    _ga_log "[$1/3] $2"
}

_ga_stop_stack() {
    if [[ -d "$GA_PROJECT_DIR" ]] && command -v docker >/dev/null 2>&1 && [[ -f "${GA_PROJECT_DIR}/docker-compose.yml" ]]; then
        cd "$GA_PROJECT_DIR"
        _ga_log "Остановка системы..."

        local down_args=(down --remove-orphans)
        if [[ "$REMOVE_DOCKER_VOLUMES" == "1" ]]; then
            _ga_log "ВНИМАНИЕ: данные ClickHouse, бэкапы и учётки будут УДАЛЕНЫ!"
            down_args+=(-v)
        else
            _ga_log "Данные сохранены (удалить: --volumes или --purge)."
        fi

        if [[ "$REMOVE_IMAGES" == "1" ]]; then
            _ga_log "Образы контейнеров системы будут удалены."
            down_args+=(--rmi local)
        fi

        if [[ "$GA_DRY_RUN" == "1" ]]; then
            _ga_log "DRY-RUN: docker compose ${down_args[*]}"
        else
            local compose_helper="${GA_PROJECT_DIR}/deploy/common/compose.sh"
            if [[ -f "$compose_helper" ]]; then
                # shellcheck source=deploy/common/compose.sh
                source "$compose_helper"
                ga_compose "$GA_PROJECT_DIR" "${down_args[@]}" || true
            else
                docker compose "${down_args[@]}" || true
            fi
        fi
        cd /
    else
        _ga_log "Каталог проекта или Docker не найдены — пропуск остановки."
    fi
}

_ga_remove_project_files() {
    if [[ "$REMOVE_PROJECT_FILES" != "1" ]]; then
        _ga_log "Каталог проекта сохранён (REMOVE_PROJECT_FILES=0 / --keep-files)."
        return
    fi

    if [[ -d "$GA_PROJECT_DIR" ]]; then
        # Не удалять каталог, пока cwd внутри него
        case "$(pwd -P)" in
            "${GA_PROJECT_DIR}"|${GA_PROJECT_DIR}/*)
                cd / || cd "${TMPDIR:-/tmp}"
                ;;
        esac

        if [[ "$GA_DRY_RUN" == "1" ]]; then
            _ga_log "DRY-RUN: rm -rf ${GA_PROJECT_DIR}"
        else
            _ga_log "Удаление каталога проекта: ${GA_PROJECT_DIR}"
            rm -rf "$GA_PROJECT_DIR"
        fi
    else
        _ga_log "Каталог проекта уже удалён."
    fi
}

_ga_remove_firewall() {
    if [[ "$REMOVE_FIREWALL_RULES" != "1" ]]; then
        _ga_log "Снятие правил сетевого экрана пропущено."
        return
    fi

    if [[ "$GA_DRY_RUN" == "1" ]]; then
        _ga_log "DRY-RUN: ga_remove_firewall_rules"
        return
    fi

    if declare -F ga_os_remove_firewall_rules >/dev/null; then
        ga_os_remove_firewall_rules
    elif declare -F ga_remove_firewall_rules >/dev/null; then
        ga_remove_firewall_rules
    else
        _ga_log "ВНИМАНИЕ: хук сетевого экрана не определён — пропуск."
    fi
}

_ga_print_summary() {
    _ga_log "Удаление завершено."
    _ga_log "Docker Engine на хосте не удалялся."
    if [[ "$REMOVE_IMAGES" != "1" ]]; then
        _ga_log "Примечание: образы контейнеров сохранены (удалить: --images или --purge)."
    fi
    if [[ "$REMOVE_DOCKER_VOLUMES" != "1" && "$REMOVE_PROJECT_FILES" == "1" ]]; then
        _ga_log "Примечание: данные сохранены — ClickHouse, бэкапы и учётки можно восстановить при повторной установке,"
        _ga_log "      если тома не были удалены вручную."
    fi
    _ga_ensure_ui || true
    if declare -F ga_ui_msgbox >/dev/null 2>&1; then
        local note=""
        if [[ "$REMOVE_DOCKER_VOLUMES" != "1" && "$REMOVE_PROJECT_FILES" == "1" ]]; then
            note=$'\n\nДанные ClickHouse, бэкапы и учётки сохранены.'
        elif [[ "$REMOVE_DOCKER_VOLUMES" == "1" ]]; then
            note=$'\n\nДанные и образы удалены согласно выбранному режиму.'
        fi
        ga_ui_msgbox "Удаление завершено" \
"ГеоАтлас удалён.
Docker Engine на хосте не трогали.${note}" || true
    fi
}

ga_run_uninstall() {
    _ga_require_root
    _ga_parse_args "$@"
    _ga_map_legacy_env

    # Preset из env без CLI
    if [[ -z "$GA_PRESET" && -n "${GA_UNINSTALL_PRESET:-}" ]]; then
        GA_PRESET="$GA_UNINSTALL_PRESET"
    fi

    ga_audit_installation

    # dry-run: без меню, показать план по preset (default: clean)
    if [[ "$GA_DRY_RUN" == "1" && -z "$GA_PRESET" ]]; then
        GA_PRESET="${GA_UNINSTALL_PRESET:-clean}"
    fi

    if [[ -n "$GA_PRESET" ]]; then
        _ga_apply_preset "$GA_PRESET"
    elif [[ -t 0 && "$GA_FORCE" != "1" ]]; then
        _ga_interactive_wizard
    else
        # Неинтерактивно без явного preset — поведение как раньше (clean-подобное через defaults env)
        _ga_log "Неинтерактивный режим — используются переменные окружения / defaults."
    fi

    _ga_print_plan

    if [[ "$GA_DRY_RUN" == "1" ]]; then
        _ga_log "DRY-RUN завершён — изменения не применены."
        return 0
    fi

    _ga_confirm

    _ga_ensure_ui || true

    _ga_step 1 "Остановка системы"
    _ga_run_gauge_fn "Остановка" "Остановка системы…" _ga_stop_stack

    _ga_step 2 "Сетевой экран"
    _ga_run_gauge_fn "Сетевой экран" "Снятие правил сетевого экрана…" _ga_remove_firewall

    _ga_step 3 "Файлы проекта"
    _ga_run_gauge_fn "Файлы проекта" "Удаление ${GA_PROJECT_DIR}…" _ga_remove_project_files

    _ga_print_summary
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    echo "Этот модуль подключается через deploy/uninstall.sh."
    echo "Запуск: sudo bash deploy/uninstall.sh --help"
    exit 1
fi
