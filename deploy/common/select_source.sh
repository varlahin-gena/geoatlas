#!/usr/bin/env bash
# Источник установки — только локальный пакет (tar.gz или распакованный каталог).
# Использование:
#   source deploy/common/select_source.sh
#   confirm_install_source   # выставляет NM_INSTALL_PACKAGE, BRANCH, NM_INSTALL_SOURCE=package
#
# Env:
#   NM_INSTALL_PACKAGE=/path/to/geoatlas-*.tar.gz  — архив или каталог
#   NM_INSTALL_PACKAGE_SHA256=<hex>                — проверить сумму (опционально)
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

_nm_src_load_apply_package() {
    if declare -F nm_apply_install_payload >/dev/null 2>&1; then
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
_nm_src_bundled_tree() {
    local dir root
    dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
    root="$(cd -- "${dir}/../.." && pwd)"
    if declare -F nm_package_looks_like_tree >/dev/null 2>&1 \
        && nm_package_looks_like_tree "$root"; then
        echo "$root"
        return 0
    fi
    return 1
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
    _nm_src_log "Источник: пакет ${NM_INSTALL_PACKAGE} (ref=${BRANCH})"
}

# Выставляет: BRANCH, NM_INSTALL_SOURCE=package, NM_INSTALL_IS_TAG, NM_INSTALL_PACKAGE
confirm_install_source() {
    local pkg_path="" bundled=""

    _nm_src_load_apply_package || true

    if [[ -z "${NM_INSTALL_PACKAGE:-}" ]]; then
        bundled="$(_nm_src_bundled_tree || true)"
        if [[ -n "$bundled" ]]; then
            NM_INSTALL_PACKAGE="$bundled"
        fi
    fi

    if [[ -n "${NM_INSTALL_PACKAGE:-}" ]]; then
        if [[ ! -f "${NM_INSTALL_PACKAGE}" && ! -d "${NM_INSTALL_PACKAGE}" ]]; then
            _nm_src_log "Пакет не найден: ${NM_INSTALL_PACKAGE}"
            exit 1
        fi
        _nm_src_set_package_ref
        export BRANCH NM_INSTALL_SOURCE NM_INSTALL_IS_TAG NM_INSTALL_PACKAGE
        return 0
    fi

    _nm_src_ensure_ui || true
    if declare -F nm_ui_inputbox >/dev/null 2>&1 && { [[ -t 0 ]] || [[ -r /dev/tty ]]; }; then
        pkg_path="$(nm_ui_inputbox "Локальный пакет" \
            "Путь к geoatlas-*.tar.gz или распакованному каталогу:" \
            "" || true)"
    elif [[ -t 0 ]]; then
        echo -n "Путь к пакету (.tar.gz или каталог): " >&2
        read -r pkg_path
    fi
    pkg_path="${pkg_path:-}"
    if [[ -z "$pkg_path" || ( ! -f "$pkg_path" && ! -d "$pkg_path" ) ]]; then
        _nm_src_log "Нужен локальный пакет. Скачайте geoatlas-X.Y.Z.tar.gz, распакуйте и запустите install из каталога пакета, либо задайте NM_INSTALL_PACKAGE."
        exit 1
    fi
    NM_INSTALL_PACKAGE="$pkg_path"
    _nm_src_set_package_ref
    export BRANCH NM_INSTALL_SOURCE NM_INSTALL_IS_TAG NM_INSTALL_PACKAGE
}

# Записать NM_INSTALL_SOURCE / BRANCH в .env (для install-meta / UI).
apply_install_source() {
    local project_dir="${1:-.}"
    local env_file="${project_dir}/.env"
    local src="${NM_INSTALL_SOURCE:-package}"
    local ref="${BRANCH:-package}"
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

_nm_src_load_apply_package || true
