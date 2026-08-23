#!/usr/bin/env bash
# Пишет install-meta.json: продуктовая версия из пакета.
# Использование:
#   source deploy/common/install_meta.sh
#   ga_write_install_meta /opt/geoatlas
#
# Поля JSON:
#   version  — из VERSION (продуктовая semver)
#   source   — package
#   ref      — vX.Y.Z
#   commit   — пусто (на сервере нет git)
#   display  — строка для UI («v1.4.2»)

_ga_meta_log() { echo "[$(date +'%F %T')] [install-meta] $*"; }

_ga_meta_json_escape() {
    local s="${1:-}"
    s="${s//\\/\\\\}"
    s="${s//\"/\\\"}"
    s="${s//$'\n'/\\n}"
    printf '%s' "$s"
}

_ga_meta_env_get() {
    local file="$1" key="$2" v=""
    [[ -f "$file" ]] || { echo ""; return 0; }
    v="$(grep -E "^[[:space:]]*${key}=" "$file" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
    echo "$v"
}

# $1 — корень проекта
ga_write_install_meta() {
    local root="${1:-.}"
    local out="${root}/install-meta.json"
    local version="unknown" source="package" ref="" display=""
    local env_ref=""

    if [[ -f "${root}/VERSION" ]]; then
        version="$(tr -d '[:space:]' <"${root}/VERSION" || true)"
        [[ -z "$version" ]] && version="unknown"
    fi
    if [[ -f "${root}/.ga-package" ]]; then
        local pkg_ver
        pkg_ver="$(grep -E '^[[:space:]]*version=' "${root}/.ga-package" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        pkg_ver="$(echo "$pkg_ver" | tr -d '[:space:]')"
        if [[ -n "$pkg_ver" ]]; then
            version="$pkg_ver"
        fi
    fi

    env_ref="$(_ga_meta_env_get "${root}/.env" GA_INSTALL_REF)"
    if [[ -n "$env_ref" && "$env_ref" != "package" && "$env_ref" != "main" && "$env_ref" != "master" ]]; then
        ref="$env_ref"
    elif [[ "$version" != "unknown" ]]; then
        ref="v${version}"
    else
        ref="unknown"
    fi

    display="$ref"
    [[ "$display" != v* && "$version" != "unknown" ]] && display="v${version}"

    cat >"$out" <<EOF
{
  "version": "$(_ga_meta_json_escape "$version")",
  "source": "$(_ga_meta_json_escape "$source")",
  "ref": "$(_ga_meta_json_escape "$ref")",
  "commit": "",
  "display": "$(_ga_meta_json_escape "$display")"
}
EOF
    _ga_meta_log "Записано ${out}: display=${display} source=${source} ref=${ref}"
}
