#!/usr/bin/env bash
# Источник установки — только локальный пакет (tar.gz или распакованный каталог).
# Использование:
#   source deploy/common/select_source.sh
#   confirm_install_source   # выставляет GA_INSTALL_PACKAGE, BRANCH, GA_INSTALL_SOURCE=package
#
# Env:
#   GA_INSTALL_PACKAGE=/path/to/geoatlas-*.tar.gz  — архив или каталог
#   GA_INSTALL_PACKAGE_SHA256=<hex>                — проверить сумму (опционально)
#   GA_UI=whiptail|dialog|text

set -Eeuo pipefail

_ga_src_log() { echo "[$(date +'%F %T')] [source] $*"; }

_ga_src_ensure_ui() {
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

_ga_src_load_apply_package() {
    if declare -F ga_apply_install_payload >/dev/null 2>&1; then
        return 0
    fi
    local dir helper
    dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
    helper="${dir}/apply_package.sh"
    if [[ -f "$helper" ]]; then
        # shellcheck source=deploy/common/apply_package.sh
        source "$helper"
        return 0
    fi
    return 1
}

# Корень распакованного пакета: deploy/common → ../..
_ga_src_bundled_tree() {
    local dir root
    dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
    root="$(cd -- "${dir}/../.." && pwd)"
    if declare -F ga_package_looks_like_tree >/dev/null 2>&1 \
        && ga_package_looks_like_tree "$root"; then
        echo "$root"
        return 0
    fi
    return 1
}

_ga_src_set_package_ref() {
    local ver=""
    GA_INSTALL_SOURCE="package"
    GA_INSTALL_IS_TAG=1
    _ga_src_load_apply_package || true
    if declare -F ga_package_read_version >/dev/null 2>&1; then
        ver="$(ga_package_read_version "${GA_INSTALL_PACKAGE}" || true)"
    fi
    if [[ -n "$ver" ]]; then
        BRANCH="v${ver}"
    else
        BRANCH="${BRANCH:-package}"
        GA_INSTALL_IS_TAG=0
    fi
    _ga_src_log "Источник: пакет ${GA_INSTALL_PACKAGE} (ref=${BRANCH})"
}

# Выставляет: BRANCH, GA_INSTALL_SOURCE=package, GA_INSTALL_IS_TAG, GA_INSTALL_PACKAGE
confirm_install_source() {
    local pkg_path="" bundled=""

    _ga_src_load_apply_package || true

    if [[ -z "${GA_INSTALL_PACKAGE:-}" ]]; then
        bundled="$(_ga_src_bundled_tree || true)"
        if [[ -n "$bundled" ]]; then
            GA_INSTALL_PACKAGE="$bundled"
        fi
    fi

    if [[ -n "${GA_INSTALL_PACKAGE:-}" ]]; then
        if [[ ! -f "${GA_INSTALL_PACKAGE}" && ! -d "${GA_INSTALL_PACKAGE}" ]]; then
            _ga_src_log "Пакет не найден: ${GA_INSTALL_PACKAGE}"
            exit 1
        fi
        _ga_src_set_package_ref
        export BRANCH GA_INSTALL_SOURCE GA_INSTALL_IS_TAG GA_INSTALL_PACKAGE
        return 0
    fi

    _ga_src_ensure_ui || true
    if declare -F ga_ui_inputbox >/dev/null 2>&1 && { [[ -t 0 ]] || [[ -r /dev/tty ]]; }; then
        pkg_path="$(ga_ui_inputbox "Локальный пакет" \
            "Путь к geoatlas-*.tar.gz или распакованному каталогу:" \
            "" || true)"
    elif [[ -t 0 ]]; then
        echo -n "Путь к пакету (.tar.gz или каталог): " >&2
        read -r pkg_path
    fi
    pkg_path="${pkg_path:-}"
    if [[ -z "$pkg_path" || ( ! -f "$pkg_path" && ! -d "$pkg_path" ) ]]; then
        _ga_src_log "Нужен локальный пакет. Скачайте geoatlas-X.Y.Z.tar.gz, распакуйте и запустите install из каталога пакета, либо задайте GA_INSTALL_PACKAGE."
        exit 1
    fi
    GA_INSTALL_PACKAGE="$pkg_path"
    _ga_src_set_package_ref
    export BRANCH GA_INSTALL_SOURCE GA_INSTALL_IS_TAG GA_INSTALL_PACKAGE
}

# Записать GA_INSTALL_SOURCE / BRANCH в .env (для install-meta / UI).
apply_install_source() {
    local project_dir="${1:-.}"
    local env_file="${project_dir}/.env"
    local src="${GA_INSTALL_SOURCE:-package}"
    local ref="${BRANCH:-package}"
    local tmp

    mkdir -p "$project_dir"
    touch "$env_file"
    tmp="$(mktemp)"
    grep -vE '^[[:space:]]*(# --- Install source \(select_source\.sh\) ---|GA_INSTALL_SOURCE=|GA_INSTALL_REF=|GA_INSTALL_IS_TAG=)' \
        "$env_file" >"$tmp" || true
    while [[ -s "$tmp" ]] && [[ -z "$(tail -n1 "$tmp")" ]]; do
        head -n -1 "$tmp" >"${tmp}.2" && mv "${tmp}.2" "$tmp"
    done
    cat >>"$tmp" <<EOF

# --- Install source (select_source.sh) ---
GA_INSTALL_SOURCE=${src}
GA_INSTALL_REF=${ref}
GA_INSTALL_IS_TAG=${GA_INSTALL_IS_TAG:-0}
EOF
    mv "$tmp" "$env_file"
    _ga_src_log "Записан GA_INSTALL_SOURCE=${src} REF=${ref} → ${env_file}"
}

_ga_src_load_apply_package || true
