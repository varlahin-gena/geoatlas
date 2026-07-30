#!/usr/bin/env bash
# Выбор источника установки: последний GitHub Release или ветка main (latest).
# Использование:
#   source deploy/common/select_source.sh
#   confirm_install_source   # выставляет BRANCH, NM_INSTALL_SOURCE, NM_INSTALL_IS_TAG
#
# Env (CI / без TTY):
#   NM_INSTALL_SOURCE=release|main   — без вопросов
#   BRANCH=v1.1.0|main|...           — явный ref (без вопросов), если задан до скрипта
#   NM_AUTO_MODULES=1                — как release (стабильный), если source не задан
#   REPO_URL                         — URL git-репозитория
#   NM_UI=yad|whiptail|dialog|text

set -Eeuo pipefail

_nm_src_log() { echo "[$(date +'%F %T')] [source] $*"; }

_nm_src_ensure_ui() {
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

_nm_src_text_radiolist() {
    # stdout: release|main
    local def="${1:-release}"
    echo "" >&2
    echo "══════════════════════════════════════════════════════════" >&2
    echo "  Источник установки" >&2
    echo "══════════════════════════════════════════════════════════" >&2
    echo "  [release] Последний релиз (стабильный тег)" >&2
    echo "  [main]    Весь проект с последними изменениями (ветка main)" >&2
    echo "" >&2
    local answer hint
    if [[ "$def" == "main" ]]; then
        hint="main/release"
    else
        hint="release/main"
    fi
    if [[ -r /dev/tty && -w /dev/tty ]]; then
        printf 'Ваш выбор [%s]: ' "$def" >/dev/tty
        read -r answer </dev/tty || answer=""
    else
        printf 'Ваш выбор [%s]: ' "$def" >&2
        read -r answer || answer=""
    fi
    answer="${answer,,}"
    answer="${answer//[[:space:]]/}"
    [[ -z "$answer" ]] && answer="$def"
    case "$answer" in
        release|rel|r|stable|стаб*) echo "release" ;;
        main|latest|dev|m|ветк*) echo "main" ;;
        *)
            echo "  Введите release или main." >&2
            _nm_src_text_radiolist "$def"
            ;;
    esac
}

# Выставляет:
#   BRANCH, NM_INSTALL_SOURCE (release|main), NM_INSTALL_IS_TAG (0|1)
confirm_install_source() {
    local source_choice=""
    local release_tag=""
    local release_label

    # Явный BRANCH снаружи (не дефолт скрипта) — не спрашиваем.
    if [[ "${NM_BRANCH_FROM_ENV:-0}" == "1" ]]; then
        NM_INSTALL_SOURCE="${NM_INSTALL_SOURCE:-custom}"
        NM_INSTALL_IS_TAG=0
        if [[ "${BRANCH}" != "main" && "${BRANCH}" != "master" ]]; then
            # Похоже на тег — пометим; clone всё равно сделает fetch --tags.
            NM_INSTALL_IS_TAG=1
        fi
        _nm_src_log "Источник задан через BRANCH=${BRANCH} (без интерактива)."
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
            release_label="Последний релиз ${release_tag} (стабильный)"
        else
            release_label="Последний релиз (тег не найден — будет main)"
        fi

        _nm_src_ensure_ui || true
        if declare -F nm_ui_radiolist >/dev/null 2>&1; then
            source_choice="$(nm_ui_radiolist \
                "Источник установки" \
                "Что установить с GitHub?" \
                release "$release_label" ON \
                main "Ветка main — все последние изменения" OFF)" || source_choice="release"
        elif command -v whiptail >/dev/null 2>&1; then
            source_choice="$(whiptail --backtitle "ГеоАтлас" --title "Источник установки" \
                --radiolist "Что установить с GitHub?" 14 72 2 \
                release "$release_label" ON \
                main "Ветка main — все последние изменения" OFF \
                3>&1 1>&2 2>&3)" || source_choice="release"
            source_choice="${source_choice//\"/}"
            source_choice="${source_choice%%[[:space:]]*}"
        else
            source_choice="$(_nm_src_text_radiolist release)"
        fi
    fi

    case "${source_choice}" in
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
                _nm_src_log "WARNING: теги релизов не найдены — устанавливаем main."
            fi
            ;;
        main|latest|dev|*)
            NM_INSTALL_SOURCE="main"
            BRANCH="main"
            NM_INSTALL_IS_TAG=0
            _nm_src_log "Выбрана ветка main (последние изменения)."
            ;;
    esac
    export BRANCH NM_INSTALL_SOURCE NM_INSTALL_IS_TAG
}

# Клонирование / обновление с учётом тега или ветки.
# Требует: PROJECT_DIR, REPO_URL, BRANCH, NM_INSTALL_IS_TAG
nm_clone_or_update_repo() {
    local project_dir="${1:-${PROJECT_DIR:-/opt/network-monitor}}"
    local repo_url="${2:-${REPO_URL:?REPO_URL required}}"
    local ref="${3:-${BRANCH:-main}}"
    local is_tag="${4:-${NM_INSTALL_IS_TAG:-0}}"

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
    else
        _nm_src_log "Клонирование ${repo_url} @ ${ref}..."
        if [[ "$is_tag" == "1" ]]; then
            git clone --branch "$ref" "$repo_url" "$project_dir"
        else
            git clone -b "$ref" "$repo_url" "$project_dir"
        fi
        cd "$project_dir"
    fi
}
