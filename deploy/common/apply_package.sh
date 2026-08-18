#!/usr/bin/env bash
# Распаковка / наложение установочного пакета ГеоАтлас на каталог проекта.
# Использование:
#   source deploy/common/apply_package.sh
#   nm_apply_install_payload /path/to/geoatlas-1.4.2.tar.gz /opt/network-monitor
#   nm_fetch_project /opt/network-monitor
#
# Env:
#   NM_INSTALL_PACKAGE           — tar.gz или распакованный каталог
#   NM_INSTALL_PACKAGE_SHA256    — ожидаемый hex SHA-256 (опционально)

set -Eeuo pipefail

_nm_pkg_log() { echo "[$(date +'%F %T')] [package] $*"; }

nm_package_preserve_relpaths() {
    cat <<'EOF'
.env
docker-compose.override.yml
.admin_password_once
install-profile.json
install-modules.json
clickhouse/users.d/zz_install_limits.xml
syslog-ng.d/zz_profile.conf
syslog-ng.d/zz_ingest_auth.conf
EOF
}

# Корень дерева в распакованном архиве (префикс geoatlas-X.Y.Z или GitHub archive).
nm_package_find_root() {
    local dir="${1:-}"
    local d
    [[ -n "$dir" && -d "$dir" ]] || return 1
    if [[ -f "${dir}/VERSION" && -f "${dir}/docker-compose.yml" && -f "${dir}/start.sh" ]]; then
        echo "$dir"
        return 0
    fi
    for d in "$dir"/*; do
        if [[ -d "$d" && -f "${d}/VERSION" && -f "${d}/docker-compose.yml" && -f "${d}/start.sh" ]]; then
            echo "$d"
            return 0
        fi
    done
    return 1
}

nm_package_looks_like_tree() {
    local dir="${1:-}"
    [[ -d "$dir" && -f "${dir}/VERSION" && -f "${dir}/docker-compose.yml" && -f "${dir}/start.sh" ]]
}

# VERSION из tar.gz или каталога. stdout = X.Y.Z (без v).
nm_package_read_version() {
    local payload="${1:-}"
    local root verfile ver=""

    if [[ -d "$payload" ]]; then
        root="$(nm_package_find_root "$payload" || true)"
        [[ -n "$root" && -f "${root}/VERSION" ]] || return 1
        ver="$(tr -d '[:space:]' <"${root}/VERSION" || true)"
        [[ -n "$ver" ]] || return 1
        echo "$ver"
        return 0
    fi

    [[ -f "$payload" ]] || return 1
    verfile="$(tar -tzf "$payload" 2>/dev/null | grep -E '(^|/)VERSION$' | head -n1 || true)"
    [[ -n "$verfile" ]] || return 1
    ver="$(tar -xOf "$payload" "$verfile" 2>/dev/null | tr -d '[:space:]' || true)"
    [[ -n "$ver" ]] || return 1
    echo "$ver"
}

nm_package_reject_unsafe_tar() {
    local tarball="${1:-}"
    local names
    names="$(tar -tzf "$tarball" 2>/dev/null || true)"
    [[ -n "$names" ]] || return 1
    if echo "$names" | grep -qE '(^|/)\.\.(/|$)' ; then
        _nm_pkg_log "ОТКАЗ: архив содержит путь с '..'."
        return 1
    fi
    if echo "$names" | grep -qE '^/' ; then
        _nm_pkg_log "ОТКАЗ: архив содержит абсолютные пути."
        return 1
    fi
    return 0
}

nm_package_verify_sha256() {
    local tarball="${1:-}"
    local expect="${NM_INSTALL_PACKAGE_SHA256:-}"
    local side got=""
    local sumfile=""

    [[ -f "$tarball" ]] || return 0

    if [[ -n "$expect" ]]; then
        expect="${expect,,}"
        expect="${expect// /}"
        if command -v sha256sum >/dev/null 2>&1; then
            got="$(sha256sum "$tarball" | awk '{print $1}')"
        elif command -v shasum >/dev/null 2>&1; then
            got="$(shasum -a 256 "$tarball" | awk '{print $1}')"
        else
            _nm_pkg_log "Нет sha256sum — пропускаем проверку NM_INSTALL_PACKAGE_SHA256."
            return 0
        fi
        got="${got,,}"
        if [[ "$got" != "$expect" ]]; then
            _nm_pkg_log "SHA-256 не совпал (ожидали ${expect}, получили ${got})."
            return 1
        fi
        _nm_pkg_log "SHA-256 пакета совпал."
        return 0
    fi

    side="${tarball}.sha256"
    if [[ -f "$side" ]]; then
        sumfile="$side"
    fi
    [[ -n "$sumfile" ]] || return 0

    expect="$(awk '{print $1; exit}' "$sumfile" || true)"
    expect="${expect,,}"
    if [[ ! "$expect" =~ ^[0-9a-f]{64}$ ]]; then
        _nm_pkg_log "Некорректный ${sumfile} — пропускаем проверку SHA-256."
        return 0
    fi
    if command -v sha256sum >/dev/null 2>&1; then
        got="$(sha256sum "$tarball" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
        got="$(shasum -a 256 "$tarball" | awk '{print $1}')"
    else
        _nm_pkg_log "Нет sha256sum — файл ${sumfile} не проверен."
        return 0
    fi
    got="${got,,}"
    if [[ "$got" != "$expect" ]]; then
        _nm_pkg_log "SHA-256 не совпал с ${sumfile}."
        return 1
    fi
    _nm_pkg_log "SHA-256 пакета совпал (${sumfile})."
    return 0
}

_nm_pkg_backup_runtime() {
    local dest="$1" keep="$2"
    local rel
    mkdir -p "$keep"
    while IFS= read -r rel; do
        [[ -n "$rel" ]] || continue
        if [[ -e "${dest}/${rel}" || -L "${dest}/${rel}" ]]; then
            mkdir -p "${keep}/$(dirname -- "$rel")"
            cp -a "${dest}/${rel}" "${keep}/${rel}"
        fi
    done < <(nm_package_preserve_relpaths)
    if [[ -d "${dest}/certs" ]]; then
        mkdir -p "${keep}/certs"
        cp -a "${dest}/certs/." "${keep}/certs/"
    fi
}

_nm_pkg_restore_runtime() {
    local dest="$1" keep="$2"
    local rel
    while IFS= read -r rel; do
        [[ -n "$rel" ]] || continue
        if [[ -e "${keep}/${rel}" || -L "${keep}/${rel}" ]]; then
            mkdir -p "${dest}/$(dirname -- "$rel")"
            cp -a "${keep}/${rel}" "${dest}/${rel}"
        fi
    done < <(nm_package_preserve_relpaths)
    if [[ -d "${keep}/certs" ]]; then
        mkdir -p "${dest}/certs"
        cp -a "${keep}/certs/." "${dest}/certs/"
    fi
}

_nm_pkg_same_dir() {
    local a="$1" b="$2"
    local pa pb
    [[ -d "$a" && -d "$b" ]] || return 1
    pa="$(cd -- "$a" && pwd)"
    pb="$(cd -- "$b" && pwd)"
    [[ "$pa" == "$pb" ]]
}

# Наложить дерево пакета на dest, сохранив runtime-файлы (.env, certs, профили).
nm_apply_package_tree() {
    local src="${1:-}"
    local dest="${2:-}"
    local tmp keep staging

    [[ -n "$src" && -d "$src" ]] || { _nm_pkg_log "Нет дерева пакета: ${src}"; return 1; }
    [[ -n "$dest" ]] || { _nm_pkg_log "Не задан каталог установки."; return 1; }
    if ! nm_package_looks_like_tree "$src"; then
        _nm_pkg_log "В ${src} нет VERSION / docker-compose.yml / start.sh."
        return 1
    fi

    mkdir -p "$dest"

    if _nm_pkg_same_dir "$src" "$dest"; then
        _nm_pkg_log "Пакет уже лежит в каталоге установки (${dest}) — копирование не нужно."
        return 0
    fi

    tmp="$(mktemp -d "${TMPDIR:-/tmp}/nm-pkg-apply.XXXXXX")"
    staging="${tmp}/staging"
    keep="${tmp}/keep"
    mkdir -p "$staging" "$keep"

    # Сначала полная копия нового дерева — dest ещё не трогаем.
    if ! cp -a "${src}/." "${staging}/"; then
        rm -rf "$tmp"
        _nm_pkg_log "Не удалось скопировать пакет во временный каталог."
        return 1
    fi
    _nm_pkg_backup_runtime "$dest" "$keep"

    _nm_pkg_log "Заменяем содержимое ${dest} деревом пакета (тома Docker не трогаем)…"
    if [[ -n "$(ls -A "$dest" 2>/dev/null || true)" ]]; then
        find "$dest" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
    fi
    if ! cp -a "${staging}/." "${dest}/"; then
        _nm_pkg_log "Копирование в ${dest} не удалось — пробуем восстановить runtime-файлы."
        _nm_pkg_restore_runtime "$dest" "$keep"
        rm -rf "$tmp"
        return 1
    fi
    _nm_pkg_restore_runtime "$dest" "$keep"
    rm -rf "$tmp"

    if [[ ! -f "${dest}/docker-compose.yml" || ! -f "${dest}/start.sh" ]]; then
        _nm_pkg_log "После наложения нет docker-compose.yml или start.sh."
        return 1
    fi
    _nm_pkg_log "Пакет наложен → ${dest}"
    return 0
}

# payload: .tar.gz / .tgz или каталог. dest: каталог установки.
nm_apply_install_payload() {
    local payload="${1:-}"
    local dest="${2:-}"
    local tmp="" root="" abs

    [[ -n "$payload" ]] || { _nm_pkg_log "Не задан пакет (NM_INSTALL_PACKAGE)."; return 1; }
    [[ -n "$dest" ]] || { _nm_pkg_log "Не задан каталог установки."; return 1; }

    if [[ -d "$payload" ]]; then
        abs="$(cd -- "$payload" && pwd)"
        root="$(nm_package_find_root "$abs" || true)"
        [[ -n "$root" ]] || { _nm_pkg_log "Каталог ${payload} не похож на пакет ГеоАтлас."; return 1; }
        nm_apply_package_tree "$root" "$dest"
        return
    fi

    [[ -f "$payload" ]] || { _nm_pkg_log "Файл пакета не найден: ${payload}"; return 1; }
    case "$payload" in
        *.tar.gz|*.tgz) ;;
        *)
            _nm_pkg_log "Ожидался .tar.gz (или каталог), получено: ${payload}"
            return 1
            ;;
    esac

    nm_package_reject_unsafe_tar "$payload" || return 1
    nm_package_verify_sha256 "$payload" || return 1

    tmp="$(mktemp -d "${TMPDIR:-/tmp}/nm-pkg-extract.XXXXXX")"
    _nm_pkg_log "Распаковка $(basename -- "$payload")…"
    if ! tar -xzf "$payload" -C "$tmp"; then
        rm -rf "$tmp"
        _nm_pkg_log "Не удалось распаковать ${payload}"
        return 1
    fi
    root="$(nm_package_find_root "$tmp" || true)"
    if [[ -z "$root" ]]; then
        rm -rf "$tmp"
        _nm_pkg_log "В архиве нет VERSION / docker-compose.yml / start.sh."
        return 1
    fi
    if ! nm_apply_package_tree "$root" "$dest"; then
        rm -rf "$tmp"
        return 1
    fi
    rm -rf "$tmp"
    return 0
}

# Наложить NM_INSTALL_PACKAGE (или $2) в каталог установки.
nm_fetch_project() {
    local project_dir="${1:-${PROJECT_DIR:-/opt/network-monitor}}"
    local payload="${2:-${NM_INSTALL_PACKAGE:-}}"

    if [[ -z "$payload" ]]; then
        _nm_pkg_log "Не задан пакет (NM_INSTALL_PACKAGE). Скачайте geoatlas-X.Y.Z.tar.gz на сервер."
        return 1
    fi
    nm_apply_install_payload "$payload" "$project_dir"
}
