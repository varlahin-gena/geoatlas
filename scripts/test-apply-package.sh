#!/usr/bin/env bash
# Дымовой тест pack-release + наложение пакета (без Docker).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "test-apply-package FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

[[ -f deploy/common/apply_package.sh ]] || fail "нет apply_package.sh"
# shellcheck source=deploy/common/apply_package.sh
source deploy/common/apply_package.sh

bash scripts/pack-release.sh
pkg="$(ls -1 dist/geoatlas-*.tar.gz | head -n1)"
[[ -n "$pkg" && -f "$pkg" ]] || fail "tarball не собран"
[[ -f "${pkg}.sha256" ]] || fail "нет .sha256"
ok "packed $(basename "$pkg")"

ver="$(nm_package_read_version "$pkg" || true)"
[[ -n "$ver" ]] || fail "не прочитали VERSION из tar.gz"
ok "package version $ver"

dest="$(mktemp -d "${TMPDIR:-/tmp}/nm-apply-dest.XXXXXX")"
echo 'SECRET=keep-me' >"${dest}/.env"
mkdir -p "${dest}/certs"
echo 'old-pem' >"${dest}/certs/fullchain.pem"
echo 'stale' >"${dest}/VERSION"
echo 'should-go' >"${dest}/extra-local.txt"

nm_apply_install_payload "$pkg" "$dest"

grep -q 'SECRET=keep-me' "${dest}/.env" || fail ".env не сохранился"
[[ -f "${dest}/start.sh" ]] || fail "нет start.sh"
[[ -f "${dest}/docker-compose.yml" ]] || fail "нет docker-compose.yml"
[[ -f "${dest}/update.sh" ]] || fail "нет update.sh"
[[ -f "${dest}/.nm-package" ]] || fail "нет .nm-package"
[[ -f "${dest}/certs/fullchain.pem" ]] || fail "нет certs/fullchain.pem"
grep -q 'old-pem' "${dest}/certs/fullchain.pem" || fail "PEM перезаписан пакетом"
[[ ! -e "${dest}/extra-local.txt" ]] || fail "лишний файл не удалён при наложении"
got="$(tr -d '[:space:]' <"${dest}/VERSION")"
[[ "$got" == "$ver" ]] || fail "VERSION dest=${got} package=${ver}"
ok "tarball apply preserves runtime files"

# Каталог вместо tar.gz
stage="$(mktemp -d "${TMPDIR:-/tmp}/nm-apply-dir.XXXXXX")"
tar -xzf "$pkg" -C "$stage"
root="$(nm_package_find_root "$stage")"
[[ -n "$root" ]] || fail "find_root"
dest2="$(mktemp -d "${TMPDIR:-/tmp}/nm-apply-dest2.XXXXXX")"
echo 'FROMDIR=1' >"${dest2}/.env"
nm_apply_install_payload "$root" "$dest2"
grep -q 'FROMDIR=1' "${dest2}/.env" || fail "dir apply потерял .env"
[[ -f "${dest2}/start.sh" ]] || fail "dir apply нет start.sh"
ok "directory apply"

dest3="$(mktemp -d "${TMPDIR:-/tmp}/nm-apply-dest3.XXXXXX")"
export NM_INSTALL_PACKAGE="$pkg"
nm_fetch_project "$dest3"
[[ -f "${dest3}/start.sh" ]] || fail "nm_fetch_project не наложил пакет"
ok "nm_fetch_project from NM_INSTALL_PACKAGE"

# shellcheck source=deploy/common/select_source.sh
source deploy/common/select_source.sh
unset NM_INSTALL_PACKAGE BRANCH NM_INSTALL_SOURCE NM_INSTALL_IS_TAG
export NM_INSTALL_PACKAGE="$pkg"
confirm_install_source
[[ "${NM_INSTALL_SOURCE}" == "package" ]] || fail "confirm_install_source source=${NM_INSTALL_SOURCE}"
[[ -n "${BRANCH:-}" ]] || fail "confirm_install_source не выставил BRANCH"
ok "confirm_install_source package"

# Path traversal
evil="$(mktemp -d "${TMPDIR:-/tmp}/nm-evil.XXXXXX")"
mkdir -p "${evil}/safe"
echo x >"${evil}/safe/file"
# GNU tar: --absolute-names не нужен; кладём '..' в имя.
if tar -czf "${evil}/bad.tar.gz" -C "$evil" --transform='s,safe,../outside,' safe/file 2>/dev/null; then
    if nm_package_reject_unsafe_tar "${evil}/bad.tar.gz"; then
        fail "ожидали отказ на архиве с '..'"
    fi
    ok "reject unsafe tar"
else
    ok "skip unsafe-tar (tar --transform недоступен)"
fi

rm -rf "$dest" "$dest2" "$dest3" "$stage" "$evil"
echo "test-apply-package: all checks passed"
