#!/usr/bin/env bash
# Общая логика удаления ГеоАтлас.
# Подключается из deploy/ubuntu/uninstall_ubuntu.sh и deploy/oracle_linux/uninstall_oraclelinux.sh.
#
# Обёртка должна определить nm_remove_firewall_rules() до source.
# Опционально: nm_audit_firewall() для строки аудита firewall.
#
# Переменные окружения:
#   NM_PROJECT_DIR          — каталог проекта (по умолчанию /opt/network-monitor)
#   NM_UNINSTALL_PRESET     — stop | clean | purge (без интерактива)
#   NM_DRY_RUN=1            — только план, без изменений
#   NM_FORCE / FORCE=1      — без подтверждения (CI/Ansible)
#   REMOVE_DOCKER_VOLUMES   — 1 = удалить тома ClickHouse/syslog-ng
#   REMOVE_PROJECT_FILES    — 1 = удалить каталог проекта
#   REMOVE_IMAGES           — 1 = docker compose down --rmi local
#   REMOVE_FIREWALL_RULES   — 1 = вызвать nm_remove_firewall_rules
#
# CLI: --help --dry-run -y|--yes --preset --purge|--clean|--stop
#      --volumes --images --keep-files --no-firewall

set -Eeuo pipefail

NM_PROJECT_DIR="${NM_PROJECT_DIR:-/opt/network-monitor}"
NM_DRY_RUN="${NM_DRY_RUN:-0}"
NM_FORCE="${NM_FORCE:-${FORCE:-0}}"
NM_PRESET="${NM_UNINSTALL_PRESET:-${NM_PRESET:-}}"
NM_WIZARD_USED="${NM_WIZARD_USED:-0}"
REMOVE_DOCKER_VOLUMES="${REMOVE_DOCKER_VOLUMES:-0}"
REMOVE_PROJECT_FILES="${REMOVE_PROJECT_FILES:-1}"
REMOVE_IMAGES="${REMOVE_IMAGES:-0}"
REMOVE_FIREWALL_RULES="${REMOVE_FIREWALL_RULES:-1}"

_nm_log() { echo "[$(date +'%F %T')] $*"; }

_nm_ensure_ui() {
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

_nm_run_gauge_fn() {
    local title="$1" text="$2" fn="$3"
    if declare -F nm_ui_run_with_gauge >/dev/null 2>&1; then
        nm_ui_run_with_gauge "$title" "$text" "$fn"
    else
        "$fn"
    fi
}

_nm_trap_err() {
    _nm_log "ERROR at ${BASH_SOURCE[1]:-?}:${BASH_LINENO[0]:-${LINENO}} (exit code $?)."
}

trap '_nm_trap_err' ERR

_nm_require_root() {
    if [[ $EUID -ne 0 ]]; then
        echo "Please run as root."
        exit 1
    fi
}

_nm_yesno() {
    # $1 — текст; по умолчанию No.
    local prompt="$1"
    if _nm_ensure_ui; then
        nm_ui_yesno "Удаление ГеоАтлас" "$prompt" 0
        return
    fi
    local answer
    read -r -p "$prompt [y/N]: " answer </dev/tty 2>/dev/null || answer=""
    [[ "$answer" =~ ^([yY]|[yY][eE][sS])$ ]]
}

_nm_yesno_default_yes() {
    # $1 — текст; по умолчанию Yes.
    local prompt="$1"
    if _nm_ensure_ui; then
        nm_ui_yesno "Удаление ГеоАтлас" "$prompt" 1
        return
    fi
    local answer
    read -r -p "$prompt [Y/n]: " answer </dev/tty 2>/dev/null || answer=""
    [[ ! "$answer" =~ ^([nN]|[nN][oO])$ ]]
}

_nm_map_legacy_env() {
    # Ubuntu: REMOVE_UFW_RULES → REMOVE_FIREWALL_RULES
    if [[ -n "${REMOVE_UFW_RULES:-}" ]]; then
        REMOVE_FIREWALL_RULES="$REMOVE_UFW_RULES"
    fi
    NM_FORCE="${NM_FORCE:-${FORCE:-0}}"
}

_nm_show_help() {
    cat <<'EOF'
ГеоАтлас — удаление установки

Использование:
  sudo bash deploy/uninstall.sh [опции]
  sudo bash deploy/ubuntu/uninstall_ubuntu.sh [опции]
  sudo bash deploy/oracle_linux/uninstall_oraclelinux.sh [опции]

Опции:
  -h, --help          показать эту справку
  --dry-run           показать план и аудит, ничего не менять
  -y, --yes           не спрашивать подтверждение (FORCE=1)
  --preset PRESET     stop | clean | purge
  --stop              только остановить стек (preset stop)
  --clean             безопасное удаление (preset clean, по умолчанию)
  --purge             полное удаление включая данные (preset purge)
  --volumes           удалить Docker volumes (ClickHouse данные)
  --images            удалить локально собранные образы
  --keep-files        сохранить каталог проекта
  --no-firewall       не трогать правила firewall

Presets:
  stop   — остановить docker compose, всё остальное сохранить
  clean  — stop + удалить файлы проекта + firewall (данные сохраняются)
  purge  — clean + volumes + локальные образы

Переменные окружения (для Ansible/CI):
  FORCE=1  NM_DRY_RUN=1  NM_UNINSTALL_PRESET=purge
  REMOVE_DOCKER_VOLUMES=1  REMOVE_PROJECT_FILES=0  REMOVE_IMAGES=1
  REMOVE_FIREWALL_RULES=0  REMOVE_UFW_RULES=0 (Ubuntu, legacy)

Примеры:
  sudo bash deploy/uninstall.sh
  sudo bash deploy/uninstall.sh --dry-run
  sudo bash deploy/uninstall.sh --purge --yes
  sudo NM_UNINSTALL_PRESET=clean FORCE=1 bash deploy/uninstall.sh
EOF
}

_nm_parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -h|--help)
                _nm_show_help
                exit 0
                ;;
            --dry-run)
                NM_DRY_RUN=1
                shift
                ;;
            -y|--yes)
                NM_FORCE=1
                FORCE=1
                shift
                ;;
            --preset)
                NM_PRESET="${2:-}"
                shift 2
                ;;
            --stop)
                NM_PRESET="stop"
                shift
                ;;
            --clean)
                NM_PRESET="clean"
                shift
                ;;
            --purge)
                NM_PRESET="purge"
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
                _nm_log "Неизвестная опция: $1 (используйте --help)"
                exit 1
                ;;
        esac
    done
}

_nm_apply_preset() {
    local preset="$1"
    NM_PRESET="$preset"
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
            _nm_log "ERROR: неизвестный preset «${preset}». Допустимо: stop, clean, purge."
            exit 1
            ;;
    esac
    _nm_log "Применён preset: ${preset}"
}

_nm_preset_label() {
    case "$1" in
        stop)  echo "только остановить стек" ;;
        clean) echo "безопасное удаление (данные сохраняются)" ;;
        purge) echo "полное удаление (включая ClickHouse данные)" ;;
        *)     echo "$1" ;;
    esac
}

_nm_interactive_wizard() {
    _nm_ensure_ui || true

    if declare -F nm_ui_radiolist >/dev/null 2>&1; then
        local choice
        if ! choice="$(nm_ui_radiolist \
            "Удаление ГеоАтлас" \
            "Выберите режим удаления:" \
            clean "Безопасное удаление — стек, каталог и firewall (данные ClickHouse сохраняются)" ON \
            purge "Полное удаление — включая volumes и локальные образы" OFF \
            stop "Только остановить стек (для обновления/отладки)" OFF \
            custom "Настроить вручную" OFF \
            cancel "Отмена" OFF)"; then
            _nm_log "Отменено пользователем."
            exit 0
        fi
        case "${choice,,}" in
            clean)
                _nm_apply_preset clean
                NM_WIZARD_USED=1
                return 0
                ;;
            purge)
                _nm_apply_preset purge
                NM_WIZARD_USED=1
                return 0
                ;;
            stop)
                _nm_apply_preset stop
                NM_WIZARD_USED=1
                return 0
                ;;
            custom)
                _nm_interactive_custom
                NM_WIZARD_USED=1
                return 0
                ;;
            cancel|q|quit|"")
                _nm_log "Отменено пользователем."
                exit 0
                ;;
            *)
                _nm_log "Неизвестный выбор «${choice}» — применяем clean."
                _nm_apply_preset clean
                NM_WIZARD_USED=1
                return 0
                ;;
        esac
    fi

    echo ""
    echo "══════════════════════════════════════════════════════════"
    echo "  Удаление ГеоАтлас"
    echo "══════════════════════════════════════════════════════════"
    echo ""
    echo "  [1] Безопасное удаление — остановить стек, удалить каталог проекта и firewall"
    echo "      (данные ClickHouse в Docker volumes сохраняются)"
    echo "  [2] Полное удаление (purge) — всё включая ClickHouse и образы"
    echo "  [3] Только остановить стек (для обновления/отладки)"
    echo "  [4] Настроить вручную"
    echo "  [q] Отмена"
    echo ""

    local choice
    while true; do
        read -r -p "Ваш выбор [1]: " choice </dev/tty 2>/dev/null || choice=""
        choice="${choice,,}"
        choice="${choice//[[:space:]]/}"

        case "$choice" in
            ""|1|clean)
                _nm_apply_preset clean
                NM_WIZARD_USED=1
                return 0
                ;;
            2|purge|full)
                _nm_apply_preset purge
                NM_WIZARD_USED=1
                return 0
                ;;
            3|stop)
                _nm_apply_preset stop
                NM_WIZARD_USED=1
                return 0
                ;;
            4|custom|manual)
                _nm_interactive_custom
                NM_WIZARD_USED=1
                return 0
                ;;
            q|quit|cancel|отмена)
                _nm_log "Отменено пользователем."
                exit 0
                ;;
            *)
                echo "  Не понял «${choice}». Enter = [1], или 2/3/4/q." >&2
                ;;
        esac
    done
}

_nm_interactive_custom() {
    _nm_log "Ручная настройка параметров удаления:"
    _nm_ensure_ui || true

    if declare -F nm_ui_checklist >/dev/null 2>&1; then
        local selected=""
        if selected="$(nm_ui_checklist \
            "Параметры удаления" \
            "Отметьте, что удалить:" \
            volumes "Docker volumes (данные ClickHouse) — НЕОБРАТИМО" OFF \
            files "Каталог проекта ${NM_PROJECT_DIR}" ON \
            firewall "Правила firewall (80/514)" ON \
            images "Локально собранные Docker-образы стека" OFF)"; then
            REMOVE_DOCKER_VOLUMES=0
            REMOVE_PROJECT_FILES=0
            REMOVE_FIREWALL_RULES=0
            REMOVE_IMAGES=0
            local list=",${selected},"
            [[ "$list" == *",volumes,"* ]] && REMOVE_DOCKER_VOLUMES=1
            [[ "$list" == *",files,"* ]] && REMOVE_PROJECT_FILES=1
            [[ "$list" == *",firewall,"* ]] && REMOVE_FIREWALL_RULES=1
            [[ "$list" == *",images,"* ]] && REMOVE_IMAGES=1
            return 0
        else
            _nm_log "Отменено пользователем."
            exit 0
        fi
    fi

    echo ""

    if _nm_yesno "Удалить Docker volumes (данные ClickHouse)? НЕОБРАТИМО"; then
        REMOVE_DOCKER_VOLUMES=1
    else
        REMOVE_DOCKER_VOLUMES=0
    fi

    if _nm_yesno_default_yes "Удалить каталог проекта ${NM_PROJECT_DIR}?"; then
        REMOVE_PROJECT_FILES=1
    else
        REMOVE_PROJECT_FILES=0
    fi

    if _nm_yesno_default_yes "Удалить правила firewall (80/514)?"; then
        REMOVE_FIREWALL_RULES=1
    else
        REMOVE_FIREWALL_RULES=0
    fi

    if _nm_yesno "Удалить локально собранные Docker-образы стека?"; then
        REMOVE_IMAGES=1
    else
        REMOVE_IMAGES=0
    fi
}

_nm_format_bytes() {
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

_nm_audit_compose() {
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
        echo "  Контейнеры       : ${running}/${total} running"
        # sed вместо while read — иначе EOF от read (exit 1) ломает set -e
        echo "$ps_out" | tail -n +2 | sed '/./s/^/                     /'
    else
        echo "  Контейнеры       : стек не запущен или не найден"
    fi
}

_nm_audit_volumes() {
    if ! command -v docker >/dev/null 2>&1; then
        echo "  Docker volumes   : docker недоступен"
        return
    fi

    local volumes=()
    mapfile -t volumes <<< "$(docker volume ls -q 2>/dev/null || true)"

    local vol found=0 mount du_size
    for vol in "${volumes[@]}"; do
        [[ -z "$vol" ]] && continue
        case "$vol" in
            *network-monitor*|*network_monitor*|*clickhouse*|*syslog-ng*) ;;
            *) continue ;;
        esac
        found=1
        mount="$(docker volume inspect "$vol" --format '{{.Mountpoint}}' 2>/dev/null || true)"
        if [[ -n "$mount" && -d "$mount" ]]; then
            du_size="$(du -sb "$mount" 2>/dev/null | cut -f1 || echo 0)"
            echo "  Volume           : ${vol} ($(_nm_format_bytes "${du_size:-0}"))"
        else
            echo "  Volume           : ${vol}"
        fi
    done

    if (( found == 0 )); then
        echo "  Docker volumes   : тома проекта не найдены"
    fi
}

_nm_audit_project_dir() {
    local dir="$1"
    if [[ -d "$dir" ]]; then
        local size
        size="$(du -sh "$dir" 2>/dev/null | awk '{print $1}')"
        echo "  Каталог проекта  : ${dir} (${size:-?})"
    else
        echo "  Каталог проекта  : ${dir} (не найден)"
    fi
}

nm_audit_installation() {
    local audit_text=""
    set +e
    audit_text="$(
        set +e
        set +o pipefail
        _nm_audit_project_dir "$NM_PROJECT_DIR"
        _nm_audit_compose "$NM_PROJECT_DIR"
        _nm_audit_volumes
        if declare -F nm_audit_firewall >/dev/null; then
            nm_audit_firewall
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

    if [[ -t 0 && "$NM_FORCE" != "1" && "$NM_DRY_RUN" != "1" ]]; then
        _nm_ensure_ui || true
        if declare -F nm_ui_msgbox >/dev/null 2>&1 && [[ "${NM_UI_BACKEND:-text}" != "text" ]]; then
            local compact
            compact="$(echo "$audit_text" | sed 's/^  //')"
            nm_ui_msgbox "Аудит установки" "$compact" || true
        fi
    fi
}

_nm_print_plan() {
    echo ""
    _nm_log "==================== ПЛАН УДАЛЕНИЯ ===================="
    _nm_log "Каталог проекта       : ${NM_PROJECT_DIR}"
    _nm_log "Остановить стек (down): да"
    if [[ "$REMOVE_DOCKER_VOLUMES" == "1" ]]; then
        _nm_log "ClickHouse данные     : БУДУТ УДАЛЕНЫ (docker volume -v)  <-- НЕОБРАТИМО"
    else
        _nm_log "ClickHouse данные     : сохраняются"
    fi
    if [[ "$REMOVE_IMAGES" == "1" ]]; then
        _nm_log "Локальные образы      : будут удалены (--rmi local)"
    else
        _nm_log "Локальные образы      : сохраняются"
    fi
    if [[ "$REMOVE_FIREWALL_RULES" == "1" ]]; then
        _nm_log "Правила firewall      : будут удалены"
    else
        _nm_log "Правила firewall      : сохраняются"
    fi
    if [[ "$REMOVE_PROJECT_FILES" == "1" ]]; then
        if [[ "$REMOVE_DOCKER_VOLUMES" == "1" ]]; then
            _nm_log "Каталог проекта       : БУДЕТ УДАЛЁН (rm -rf ${NM_PROJECT_DIR})"
        else
            _nm_log "Каталог проекта       : будет удалён (данные ClickHouse в volumes сохранятся)"
        fi
    else
        _nm_log "Каталог проекта       : сохраняется"
    fi
    if [[ -n "$NM_PRESET" ]]; then
        _nm_log "Preset                : ${NM_PRESET} ($(_nm_preset_label "$NM_PRESET"))"
    fi
    if [[ "$NM_DRY_RUN" == "1" ]]; then
        _nm_log "Режим                 : DRY-RUN (изменения не будут применены)"
    fi
    _nm_log "======================================================"
    echo ""
}

_nm_confirm() {
    if [[ "$NM_FORCE" == "1" ]]; then
        _nm_log "FORCE=1 — подтверждение пропущено."
        return 0
    fi

    if [[ ! -t 0 ]]; then
        _nm_log "Нет интерактивного терминала, а FORCE не задан. Прерывание."
        _nm_log "Запустите с --yes или FORCE=1 для неинтерактивного удаления."
        exit 1
    fi

    # Выбор в wizard/custom — уже подтверждение; лишние вопросы не задаём.
    if [[ "${NM_WIZARD_USED}" == "1" ]]; then
        if [[ "$REMOVE_DOCKER_VOLUMES" == "1" ]]; then
            if ! _nm_yesno "Будут удалены данные ClickHouse. Это необратимо. Продолжить?"; then
                _nm_log "Отменено пользователем."
                exit 0
            fi
        fi
        _nm_log "Запуск удаления (preset: ${NM_PRESET:-custom})..."
        return 0
    fi

    if ! _nm_yesno "Продолжить удаление?"; then
        _nm_log "Отменено пользователем."
        exit 0
    fi

    if [[ "$REMOVE_DOCKER_VOLUMES" == "1" ]]; then
        if ! _nm_yesno "Будут удалены данные ClickHouse. Это необратимо. Продолжить?"; then
            _nm_log "Отменено пользователем."
            exit 0
        fi
    fi
}

_nm_step() {
    _nm_log "[$1/3] $2"
}

_nm_stop_stack() {
    if [[ -d "$NM_PROJECT_DIR" ]] && command -v docker >/dev/null 2>&1 && [[ -f "${NM_PROJECT_DIR}/docker-compose.yml" ]]; then
        cd "$NM_PROJECT_DIR"
        _nm_log "Stopping Docker Compose stack..."

        local down_args=(down --remove-orphans)
        if [[ "$REMOVE_DOCKER_VOLUMES" == "1" ]]; then
            _nm_log "WARNING: REMOVE_DOCKER_VOLUMES=1 — ClickHouse data will be DELETED!"
            down_args+=(-v)
        else
            _nm_log "Docker volumes preserved (use --volumes or preset purge to delete)."
        fi

        if [[ "$REMOVE_IMAGES" == "1" ]]; then
            _nm_log "Locally built images will be removed (--rmi local)."
            down_args+=(--rmi local)
        fi

        if [[ "$NM_DRY_RUN" == "1" ]]; then
            _nm_log "DRY-RUN: docker compose ${down_args[*]}"
        else
            docker compose "${down_args[@]}" || true
        fi
        cd /
    else
        _nm_log "Project directory, compose file or docker not found — skipping compose down."
    fi
}

_nm_remove_project_files() {
    if [[ "$REMOVE_PROJECT_FILES" != "1" ]]; then
        _nm_log "Project directory preserved (REMOVE_PROJECT_FILES=0 / --keep-files)."
        return
    fi

    if [[ -d "$NM_PROJECT_DIR" ]]; then
        # Не удалять каталог, пока cwd внутри него
        case "$(pwd -P)" in
            "${NM_PROJECT_DIR}"|${NM_PROJECT_DIR}/*)
                cd / || cd "${TMPDIR:-/tmp}"
                ;;
        esac

        if [[ "$NM_DRY_RUN" == "1" ]]; then
            _nm_log "DRY-RUN: rm -rf ${NM_PROJECT_DIR}"
        else
            _nm_log "Removing project directory: ${NM_PROJECT_DIR}"
            rm -rf "$NM_PROJECT_DIR"
        fi
    else
        _nm_log "Project directory already removed."
    fi
}

_nm_remove_firewall() {
    if [[ "$REMOVE_FIREWALL_RULES" != "1" ]]; then
        _nm_log "Firewall rule removal skipped."
        return
    fi

    if [[ "$NM_DRY_RUN" == "1" ]]; then
        _nm_log "DRY-RUN: nm_remove_firewall_rules"
        return
    fi

    if declare -F nm_remove_firewall_rules >/dev/null; then
        nm_remove_firewall_rules
    else
        _nm_log "WARNING: nm_remove_firewall_rules не определён — пропуск firewall."
    fi
}

_nm_print_summary() {
    _nm_log "Uninstallation completed."
    _nm_log "Docker itself was not removed."
    if [[ "$REMOVE_IMAGES" != "1" ]]; then
        _nm_log "Note: locally built images kept (use --images or preset purge to remove)."
    fi
    if [[ "$REMOVE_DOCKER_VOLUMES" != "1" && "$REMOVE_PROJECT_FILES" == "1" ]]; then
        _nm_log "Note: Docker volumes preserved — данные ClickHouse можно восстановить при повторной установке,"
        _nm_log "      если тома не были удалены вручную."
    fi
    _nm_ensure_ui || true
    if declare -F nm_ui_msgbox >/dev/null 2>&1; then
        local note=""
        if [[ "$REMOVE_DOCKER_VOLUMES" != "1" && "$REMOVE_PROJECT_FILES" == "1" ]]; then
            note=$'\n\nДанные ClickHouse в Docker volumes сохранены.'
        fi
        nm_ui_msgbox "Удаление завершено" \
"ГеоАтлас удалён согласно выбранному режиму.
Docker Engine на хосте не трогали.${note}" || true
    fi
}

nm_run_uninstall() {
    _nm_require_root
    _nm_parse_args "$@"
    _nm_map_legacy_env

    # Preset из env без CLI
    if [[ -z "$NM_PRESET" && -n "${NM_UNINSTALL_PRESET:-}" ]]; then
        NM_PRESET="$NM_UNINSTALL_PRESET"
    fi

    nm_audit_installation

    # dry-run: без меню, показать план по preset (default: clean)
    if [[ "$NM_DRY_RUN" == "1" && -z "$NM_PRESET" ]]; then
        NM_PRESET="${NM_UNINSTALL_PRESET:-clean}"
    fi

    if [[ -n "$NM_PRESET" ]]; then
        _nm_apply_preset "$NM_PRESET"
    elif [[ -t 0 && "$NM_FORCE" != "1" ]]; then
        _nm_interactive_wizard
    else
        # Неинтерактивно без явного preset — поведение как раньше (clean-подобное через defaults env)
        _nm_log "Неинтерактивный режим — используются переменные окружения / defaults."
    fi

    _nm_print_plan

    if [[ "$NM_DRY_RUN" == "1" ]]; then
        _nm_log "DRY-RUN завершён — изменения не применены."
        return 0
    fi

    _nm_confirm

    _nm_ensure_ui || true

    _nm_step 1 "Остановка Docker Compose стека"
    _nm_run_gauge_fn "Остановка стека" "docker compose down…" _nm_stop_stack

    _nm_step 2 "Удаление правил firewall"
    _nm_run_gauge_fn "Firewall" "Удаление правил firewall…" _nm_remove_firewall

    _nm_step 3 "Удаление файлов проекта"
    _nm_run_gauge_fn "Файлы проекта" "Удаление ${NM_PROJECT_DIR}…" _nm_remove_project_files

    _nm_print_summary
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    echo "Этот модуль подключается через deploy/uninstall.sh или OS-обёртки."
    echo "Запуск: sudo bash deploy/uninstall.sh --help"
    exit 1
fi
