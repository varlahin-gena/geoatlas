#!/usr/bin/env bash
# Выбор источника установки: последний GitHub Release, ветка main или локальный пакет.
# Использование:
#   source deploy/common/select_source.sh
#   confirm_install_source   # выставляет BRANCH, NM_INSTALL_SOURCE, NM_INSTALL_IS_TAG
#
# Env (CI / без TTY):
#   NM_INSTALL_SOURCE=release|main|package   — без вопросов
#   NM_INSTALL_PACKAGE=/path/to/geoatlas-*.tar.gz  — локальный пакет (каталог тоже)
#   BRANCH=v1.1.0|main|...           — явный ref (без вопросов), если задан до скрипта
#   NM_AUTO_MODULES=1                — как release (стабильный), если source не задан
#   REPO_URL                         — URL git-репозитория
#   NM_UI=whiptail|dialog|text

set -Eeuo pipefail

_nm_src_log() { echo "[$(date +'%F %T')] [source] $*"; }

_nm_src_ensure_ui() {
    if ! declare -F nm_ui_radiolist >/dev/null 2>&1; then
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

# Подтянуть apply_package.sh (рядом или с GitHub, если select_source сcurlили в /tmp).
_nm_src_load_apply_package() {
    if declare -F nm_apply_install_payload >/dev/null 2>&1; then
        return 0
    fi
    local dir helper tmp owner
    dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
    helper="${dir}/apply_package.sh"
    if [[ -f "$helper" ]]; then
        # shellcheck source=deploy/common/apply_package.sh
        source "$helper"
        return 0
    fi
    if ! command -v curl >/dev/null 2>&1; then
        return 1
    fi
    owner="$(nm_git_owner_repo "${REPO_URL:-}")"
    tmp="$(mktemp)"
    if curl -fsSL --connect-timeout 10 --max-time 30 \
        "https://raw.githubusercontent.com/${owner}/main/deploy/common/apply_package.sh" \
        -o "$tmp"; then
        # shellcheck source=/dev/null
        source "$tmp"
        rm -f "$tmp"
        return 0
    fi
    rm -f "$tmp"
    return 1
}

# owner/repo из REPO_URL (github).
nm_git_owner_repo() {
    local u="${1:-${REPO_URL:-}}"
    u="${u%.git}"
    u="${u%/}"
    if [[ "$u" =~ github\.com[:/]([^/]+)/([^/]+)$ ]]; then
        echo "${BASH_REMATCH[1]}/${BASH_REMATCH[2]}"
        return 0
    fi
    echo "varlahin-gena/network_monitor"
}

# Последний semver-подобный тег (git ls-remote; fallback — GitHub API).
nm_latest_release_tag() {
    local repo_url="${1:-${REPO_URL:-}}"
    local tag=""

    if command -v git >/dev/null 2>&1 && [[ -n "$repo_url" ]]; then
        tag="$(git ls-remote --tags --refs "$repo_url" 2>/dev/null \
            | awk '{print $2}' \
            | sed 's#refs/tags/##' \
            | grep -E '^v?[0-9]' \
            | sort -V \
            | tail -n1 || true)"
    fi

    if [[ -z "$tag" ]]; then
        local api owner
        owner="$(nm_git_owner_repo "$repo_url")"
        api="https://api.github.com/repos/${owner}/releases/latest"
        if command -v curl >/dev/null 2>&1; then
            tag="$(curl -fsSL --connect-timeout 10 --max-time 30 "$api" 2>/dev/null \
                | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
                | head -n1 || true)"
        fi
    fi

    [[ -n "$tag" ]] || return 1
    echo "$tag"
}

_nm_src_set_package_ref() {
    local ver=""
    NM_INSTALL_SOURCE="package"
    NM_INSTALL_IS_TAG=1
    _nm_src_load_apply_package || true
    if declare -F nm_package_read_version >/dev/null 2>&1; then
        ver="$(nm_package_read_version "${NM_INSTALL_PACKAGE}" || true)"
    fi
    if [[ -n "$ver" ]]; then
        BRANCH="v${ver}"
    else
        BRANCH="${BRANCH:-package}"
        NM_INSTALL_IS_TAG=0
    fi
    _nm_src_log "Источник: локальный пакет ${NM_INSTALL_PACKAGE} (ref=${BRANCH})"
}

# Выставляет:
#   BRANCH, NM_INSTALL_SOURCE (release|main|package), NM_INSTALL_IS_TAG (0|1)
#   NM_INSTALL_PACKAGE — если выбран локальный пакет
confirm_install_source() {
    local source_choice=""
    local release_tag=""
    local release_label
    local pkg_path=""

    _nm_src_load_apply_package || true

    # Явный путь к пакету — не спрашиваем (даже если задан BRANCH).
    if [[ -n "${NM_INSTALL_PACKAGE:-}" ]]; then
        _nm_src_set_package_ref
        export BRANCH NM_INSTALL_SOURCE NM_INSTALL_IS_TAG NM_INSTALL_PACKAGE
        return 0
    fi

    # Явный BRANCH снаружи (не дефолт скрипта) — не спрашиваем.
    if [[ "${NM_BRANCH_FROM_ENV:-0}" == "1" ]]; then
        NM_INSTALL_SOURCE="${NM_INSTALL_SOURCE:-custom}"
        NM_INSTALL_IS_TAG=0
        if [[ "${BRANCH}" != "main" && "${BRANCH}" != "master" ]]; then
            # Похоже на тег — пометим; clone всё равно сделает fetch --tags.
            NM_INSTALL_IS_TAG=1
        fi
        _nm_src_log "Источник задан через BRANCH=${BRANCH} (без интерактива)."
        export BRANCH NM_INSTALL_SOURCE NM_INSTALL_IS_TAG
        return 0
    fi

    if [[ -n "${NM_INSTALL_SOURCE:-}" ]]; then
        source_choice="${NM_INSTALL_SOURCE,,}"
    elif [[ "${NM_AUTO_MODULES:-0}" == "1" ]] || [[ ! -t 0 ]]; then
        # Авто/без TTY: стабильный релиз (с откатом на main).
        source_choice="release"
        _nm_src_log "Нет TTY / NM_AUTO_MODULES — пробуем последний релиз."
    else
        release_tag="$(nm_latest_release_tag "${REPO_URL:-}" || true)"
        if [[ -n "$release_tag" ]]; then
            release_label="Релиз ${release_tag}"
        else
            release_label="Релиз (или main)"
        fi

        _nm_src_ensure_ui || true
        if declare -F nm_ui_radiolist >/dev/null 2>&1; then
            if ! source_choice="$(nm_ui_radiolist \
                "Источник установки" \
                "Откуда взять код?" \
                release "$release_label" ON \
                main "Ветка main" OFF \
                package "Локальный пакет .tar.gz" OFF)"; then
                _nm_src_log "Установка отменена пользователем."
                exit 0
            fi
        else
            source_choice="release"
            _nm_src_log "UI недоступен — источник release."
        fi
    fi

    case "${source_choice}" in
        package|pkg|local|tarball)
            _nm_src_ensure_ui || true
            if declare -F nm_ui_inputbox >/dev/null 2>&1 && { [[ -t 0 ]] || [[ -r /dev/tty ]]; }; then
                pkg_path="$(nm_ui_inputbox "Локальный пакет" \
                    "Путь к geoatlas-*.tar.gz или распакованному каталогу:" \
                    "${NM_INSTALL_PACKAGE:-}" || true)"
            elif [[ -t 0 ]]; then
                echo -n "Путь к пакету (.tar.gz или каталог): " >&2
                read -r pkg_path
            fi
            pkg_path="${pkg_path:-}"
            if [[ -z "$pkg_path" || ( ! -f "$pkg_path" && ! -d "$pkg_path" ) ]]; then
                _nm_src_log "Пакет не найден: «${pkg_path}». Задайте NM_INSTALL_PACKAGE."
                exit 1
            fi
            NM_INSTALL_PACKAGE="$pkg_path"
            _nm_src_set_package_ref
            ;;
        release|rel|stable)
            NM_INSTALL_SOURCE="release"
            if [[ -z "$release_tag" ]]; then
                release_tag="$(nm_latest_release_tag "${REPO_URL:-}" || true)"
            fi
            if [[ -n "$release_tag" ]]; then
                BRANCH="$release_tag"
                NM_INSTALL_IS_TAG=1
                _nm_src_log "Выбран последний релиз: ${BRANCH}"
            else
                BRANCH="main"
                NM_INSTALL_IS_TAG=0
                NM_INSTALL_SOURCE="main"
                _nm_src_log "ВНИМАНИЕ: теги релизов не найдены — устанавливаем main."
            fi
            ;;
        main|latest|dev|*)
            NM_INSTALL_SOURCE="main"
            BRANCH="main"
            NM_INSTALL_IS_TAG=0
            _nm_src_log "Выбрана ветка main (последние изменения)."
            ;;
    esac
    export BRANCH NM_INSTALL_SOURCE NM_INSTALL_IS_TAG NM_INSTALL_PACKAGE
}

# Записать NM_INSTALL_SOURCE / BRANCH в .env (для install-meta / UI).
apply_install_source() {
    local project_dir="${1:-.}"
    local env_file="${project_dir}/.env"
    local src="${NM_INSTALL_SOURCE:-main}"
    local ref="${BRANCH:-main}"
    local tmp

    mkdir -p "$project_dir"
    touch "$env_file"
    tmp="$(mktemp)"
    grep -vE '^[[:space:]]*(# --- Install source \(select_source\.sh\) ---|NM_INSTALL_SOURCE=|NM_INSTALL_REF=|NM_INSTALL_IS_TAG=)' \
        "$env_file" >"$tmp" || true
    while [[ -s "$tmp" ]] && [[ -z "$(tail -n1 "$tmp")" ]]; do
        head -n -1 "$tmp" >"${tmp}.2" && mv "${tmp}.2" "$tmp"
    done
    cat >>"$tmp" <<EOF

# --- Install source (select_source.sh) ---
NM_INSTALL_SOURCE=${src}
NM_INSTALL_REF=${ref}
NM_INSTALL_IS_TAG=${NM_INSTALL_IS_TAG:-0}
EOF
    mv "$tmp" "$env_file"
    _nm_src_log "Записан NM_INSTALL_SOURCE=${src} REF=${ref} → ${env_file}"
}

# Клонирование / обновление с учётом тега или ветки.
# Требует: PROJECT_DIR, REPO_URL, BRANCH, NM_INSTALL_IS_TAG
nm_clone_or_update_repo() {
    local project_dir="${1:-${PROJECT_DIR:-/opt/network-monitor}}"
    local repo_url="${2:-${REPO_URL:?REPO_URL required}}"
    local ref="${3:-${BRANCH:-main}}"
    local is_tag="${4:-${NM_INSTALL_IS_TAG:-0}}"
    local tmp_clone saved_env=""

    mkdir -p "$(dirname -- "$project_dir")"

    if [[ -d "${project_dir}/.git" ]]; then
        _nm_src_log "Проект уже есть — обновляем (${ref})..."
        cd "$project_dir"

        if ! git diff --quiet || ! git diff --cached --quiet; then
            _nm_src_log "Локальные изменения — stash перед обновлением."
            git stash push -u -m "install-$(date +%s)" || true
        fi

        if [[ "$is_tag" == "1" ]]; then
            git fetch origin --tags --force
            git checkout --force "$ref"
        else
            git fetch origin
            git checkout "$ref"
            git pull --ff-only origin "$ref"
        fi
        return 0
    fi

    # Каталог без .git (например, только .env от прерванной установки) —
    # git clone в непустой путь падает. Клонируем во временный каталог.
    if [[ -d "$project_dir" ]] && [[ -n "$(ls -A "$project_dir" 2>/dev/null || true)" ]]; then
        _nm_src_log "Каталог ${project_dir} не пуст и без .git — клонируем во временный каталог…"
        if [[ -f "${project_dir}/.env" ]]; then
            saved_env="$(mktemp)"
            cp -a "${project_dir}/.env" "$saved_env"
        fi
        tmp_clone="$(mktemp -d "${project_dir}.clone.XXXXXX")"
        if [[ "$is_tag" == "1" ]]; then
            git clone --branch "$ref" "$repo_url" "$tmp_clone"
        else
            git clone -b "$ref" "$repo_url" "$tmp_clone"
        fi
        # Заменяем содержимое целевого каталога результатом клона.
        find "$project_dir" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
        # shellcheck disable=SC2086
        shopt -s dotglob nullglob
        mv "$tmp_clone"/* "$project_dir"/
        shopt -u dotglob nullglob
        rmdir "$tmp_clone" 2>/dev/null || rm -rf "$tmp_clone"
        if [[ -n "$saved_env" && -f "$saved_env" ]]; then
            # Сохраняем прежний .env только если в клоне его нет (его и не должно быть в git).
            cp -a "$saved_env" "${project_dir}/.env"
            rm -f "$saved_env"
        fi
        cd "$project_dir"
        _nm_src_log "Клонирование завершено → ${project_dir}"
        return 0
    fi

    _nm_src_log "Клонирование ${repo_url} @ ${ref}..."
    if [[ "$is_tag" == "1" ]]; then
        git clone --branch "$ref" "$repo_url" "$project_dir"
    else
        git clone -b "$ref" "$repo_url" "$project_dir"
    fi
    cd "$project_dir"
}

_nm_src_load_apply_package || true
