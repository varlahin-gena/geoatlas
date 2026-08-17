#!/usr/bin/env bash
# Пишет install-meta.json: product version + git ref (main / vX.Y.Z).
# Использование:
#   source deploy/common/install_meta.sh
#   nm_write_install_meta /opt/network-monitor
#
# Поля JSON:
#   version  — из VERSION (продуктовая semver)
#   source   — release | main | git | package
#   ref      — тег (v1.1.4) или ветка (main)
#   commit   — короткий SHA (если есть git)
#   display  — строка для UI («v1.1.4» или «main»)

_nm_meta_log() { echo "[$(date +'%F %T')] [install-meta] $*"; }

_nm_meta_json_escape() {
    local s="${1:-}"
    s="${s//\\/\\\\}"
    s="${s//\"/\\\"}"
    s="${s//$'\n'/\\n}"
    printf '%s' "$s"
}

# $1 — корень проекта
nm_write_install_meta() {
    local root="${1:-.}"
    local out="${root}/install-meta.json"
    local version="unknown" source="unknown" ref="" commit="" display=""
    local env_src="" tag=""

    if [[ -f "${root}/VERSION" ]]; then
        version="$(tr -d '[:space:]' <"${root}/VERSION" || true)"
        [[ -z "$version" ]] && version="unknown"
    fi

    if [[ -f "${root}/.env" ]]; then
        env_src="$(grep -E '^[[:space:]]*NM_INSTALL_SOURCE=' "${root}/.env" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        env_src="${env_src,,}"
        local env_ref
        env_ref="$(grep -E '^[[:space:]]*NM_INSTALL_REF=' "${root}/.env" 2>/dev/null | tail -n1 | cut -d= -f2- || true)"
        if [[ -z "$ref" && -n "$env_ref" ]]; then
            ref="$env_ref"
        fi
    fi

    if command -v git >/dev/null 2>&1 && [[ -d "${root}/.git" || -f "${root}/.git" ]]; then
        commit="$(git -C "$root" rev-parse --short HEAD 2>/dev/null || true)"
        if tag="$(git -C "$root" describe --exact-match --tags HEAD 2>/dev/null)"; then
            ref="$tag"
            source="release"
        else
            ref="$(git -C "$root" rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
            if [[ -z "$ref" || "$ref" == "HEAD" ]]; then
                ref="${commit:-unknown}"
                source="git"
            elif [[ "$ref" == "main" || "$ref" == "master" ]]; then
                source="main"
            else
                source="git"
            fi
        fi
    fi

    # Env от установщика имеет приоритет для source, если git не дал exact tag.
    if [[ "$source" != "release" ]]; then
        case "$env_src" in
            release)
                source="release"
                if [[ -z "$ref" || "$ref" == "main" || "$ref" == "master" ]]; then
                    if [[ "$version" != "unknown" ]]; then
                        ref="v${version}"
                    fi
                fi
                ;;
            package)
                source="package"
                if [[ -z "$ref" || "$ref" == "main" || "$ref" == "master" || "$ref" == "package" ]]; then
                    if [[ "$version" != "unknown" ]]; then
                        ref="v${version}"
                    fi
                fi
                ;;
            main)
                source="main"
                [[ -z "$ref" ]] && ref="main"
                ;;
        esac
    fi

    if [[ -z "$ref" ]]; then
        if [[ "$version" != "unknown" ]]; then
            ref="v${version}"
            [[ "$source" == "unknown" ]] && source="release"
        else
            ref="unknown"
        fi
    fi

    case "$source" in
        release)
            display="$ref"
            [[ "$display" != v* && "$version" != "unknown" ]] && display="v${version}"
            ;;
        package)
            display="$ref"
            [[ "$display" != v* && "$version" != "unknown" ]] && display="v${version}"
            ;;
        main)
            display="main"
            ;;
        *)
            display="$ref"
            ;;
    esac

    cat >"$out" <<EOF
{
  "version": "$(_nm_meta_json_escape "$version")",
  "source": "$(_nm_meta_json_escape "$source")",
  "ref": "$(_nm_meta_json_escape "$ref")",
  "commit": "$(_nm_meta_json_escape "$commit")",
  "display": "$(_nm_meta_json_escape "$display")"
}
EOF
    _nm_meta_log "Записано ${out}: display=${display} source=${source} ref=${ref}"
}
