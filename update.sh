#!/usr/bin/env bash
# Обновление ГеоАтлас из локального пакета (tar.gz или распакованный каталог).
# Тома Docker / .env / сертификаты / профиль сохраняются.
#
#   sudo ./update.sh /path/to/geoatlas-1.4.2.tar.gz
#   tar -xzf geoatlas-1.4.2.tar.gz && sudo ./geoatlas-1.4.2/update.sh
#
# Опции:
#   --package PATH     пакет (.tar.gz или каталог)
#   --project-dir DIR  каталог установки (по умолчанию /opt/geoatlas)
#   --no-stop          не вызывать ./stop.sh
#   --no-start         не вызывать ./start.sh после наложения
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"

PROJECT_DIR="${GA_PROJECT_DIR:-/opt/geoatlas}"
DO_STOP=1
DO_START=1
PACKAGE="${GA_INSTALL_PACKAGE:-}"

log() { echo "[$(date +'%F %T')] [update] $*"; }

trap 'log "ОШИБКА на строке ${LINENO} (код выхода $?)."' ERR

usage() {
    cat <<'EOF'
Обновление ГеоАтлас из установочного пакета.

  sudo ./update.sh /path/to/geoatlas-X.Y.Z.tar.gz
  sudo ./update.sh --package /path/to/geoatlas-X.Y.Z

Опции:
  --package PATH      tar.gz или распакованный каталог
  --project-dir DIR   каталог установки (по умолчанию /opt/geoatlas)
  --no-stop           не останавливать стек
  --no-start          не запускать ./start.sh
  -h, --help          эта справка

Первичная установка из пакета — через deploy/ubuntu/install_ubuntu.sh
(или install_oraclelinux.sh) из распакованного geoatlas-X.Y.Z.
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
    if [[ -f "$apply" ]]; then
        # shellcheck source=deploy/common/apply_package.sh
        source "$apply"
    fi
    if [[ -f "$src" ]]; then
        # shellcheck source=deploy/common/select_source.sh
        source "$src"
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
                shift 2
                ;;
            --project-dir)
                [[ $# -ge 2 ]] || { echo "--project-dir требует путь" >&2; exit 1; }
                PROJECT_DIR="$2"
                shift 2
                ;;
            --no-stop)
                DO_STOP=0
                shift
                ;;
            --no-start)
                DO_START=0
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
    log "Остановка стека…"
    if [[ -f "$compose" ]]; then
        if (
            # shellcheck source=deploy/common/compose.sh
            source "$compose"
            ga_compose "$dir" down --remove-orphans
        ); then
            return 0
        fi
        log "ВНИМАНИЕ: docker compose down не удался — пробуем установленный stop.sh."
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
    log "ВНИМАНИЕ: стек может остаться запущенным — наложение пакета продолжается (start.sh пересоздаст контейнеры)."
    return 0
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
    echo "Укажите пакет (.tar.gz или каталог). См. --help." >&2
    exit 1
}

main() {
    parse_args "$@"
    require_root

    _source_helpers "$SCRIPT_DIR"
    if ! declare -F ga_apply_install_payload >/dev/null 2>&1; then
        echo "Не найден deploy/common/apply_package.sh рядом с update.sh." >&2
        exit 1
    fi

    resolve_default_package

    if [[ ! -f "${PROJECT_DIR}/docker-compose.yml" ]]; then
        echo "В ${PROJECT_DIR} нет установленной системы (нет docker-compose.yml)." >&2
        echo "Первичная установка из пакета:" >&2
        echo "  tar -xzf geoatlas-X.Y.Z.tar.gz && cd geoatlas-X.Y.Z" >&2
        echo "  sudo ./deploy/ubuntu/install_ubuntu.sh" >&2
        exit 1
    fi

    local payload="$PACKAGE"
    local ver=""

    ver="$(ga_package_read_version "$payload" || true)"
    if [[ -n "$ver" ]]; then
        log "Версия пакета: ${ver}"
    fi

    if [[ "$DO_STOP" == "1" ]]; then
        ga_update_stop_stack "$PROJECT_DIR"
    fi

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

    if [[ "$DO_START" == "1" ]]; then
        log "Запуск ./start.sh (пересборка образов)…"
        "${PROJECT_DIR}/start.sh"
    else
        log "Старт пропущен (--no-start). Запустите ${PROJECT_DIR}/start.sh"
    fi

    log "Обновление завершено."
}

main "$@"
