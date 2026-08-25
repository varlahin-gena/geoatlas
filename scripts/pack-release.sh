#!/usr/bin/env bash
# Собрать установочный tar.gz из рабочего дерева (без .git / gitignored).
#   bash scripts/pack-release.sh [--with-images] [выходной-каталог]
# Пишет:
#   dist/geoatlas-<VERSION>.tar.gz
#   dist/geoatlas-<VERSION>.tar.gz.sha256
#
# --with-images: собрать/стянуть runtime Docker-образы и положить в images/
#   (нужен docker; пакет для сервера без docker build / Docker Hub).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "pack-release FAIL: $*" >&2; exit 1; }

WITH_IMAGES=0
OUT_DIR=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --with-images)
            WITH_IMAGES=1
            shift
            ;;
        -h|--help)
            cat <<'EOF'
Сборка установочного пакета geoatlas-X.Y.Z.tar.gz

  bash scripts/pack-release.sh [--with-images] [выходной-каталог]

  --with-images   включить Docker-образы (images/) для офлайн-установки
  выходной-каталог  по умолчанию dist/
EOF
            exit 0
            ;;
        -*)
            fail "неизвестная опция: $1"
            ;;
        *)
            if [[ -n "$OUT_DIR" ]]; then
                fail "лишний аргумент: $1"
            fi
            OUT_DIR="$1"
            shift
            ;;
    esac
done

[[ -f VERSION ]] || fail "нет VERSION"
[[ -f docker-compose.yml ]] || fail "нет docker-compose.yml"
[[ -f start.sh ]] || fail "нет start.sh"
command -v git >/dev/null 2>&1 || fail "нужен git"
command -v tar >/dev/null 2>&1 || fail "нужен tar"

VERSION="$(tr -d '[:space:]' < VERSION)"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "VERSION не X.Y.Z: ${VERSION:-<empty>}"

OUT_DIR="${OUT_DIR:-${ROOT}/dist}"
PREFIX="geoatlas-${VERSION}"
TARBALL_NAME="${PREFIX}.tar.gz"
mkdir -p "$OUT_DIR"
OUT_DIR="$(cd "$OUT_DIR" && pwd)"
OUT="${OUT_DIR}/${TARBALL_NAME}"

IMAGES_SRC=""
if [[ "$WITH_IMAGES" == "1" ]]; then
    command -v docker >/dev/null 2>&1 || fail "--with-images требует docker"
    IMAGES_SRC="${OUT_DIR}/image-bundle/images"
    bash "${ROOT}/scripts/export-images.sh" "$IMAGES_SRC"
    [[ -f "${IMAGES_SRC}/manifest.txt" ]] || fail "export-images не создал manifest.txt"
fi

stage="$(mktemp -d "${TMPDIR:-/tmp}/ga-pack.XXXXXX")"
cleanup() { rm -rf "$stage"; }
trap cleanup EXIT

mkdir -p "${stage}/${PREFIX}"

# HEAD (tracked) одним архивом — быстро; поверх — грязное дерево и untracked.
git archive HEAD | tar -x -C "${stage}/${PREFIX}"
while IFS= read -r -d '' f; do
    [[ -n "$f" ]] || continue
    [[ -e "$f" ]] || continue
    mkdir -p "${stage}/${PREFIX}/$(dirname -- "$f")"
    cp -a "$f" "${stage}/${PREFIX}/${f}"
done < <({ git diff -z --name-only HEAD; git ls-files -z --others --exclude-standard; })

[[ -f "${stage}/${PREFIX}/start.sh" ]] || fail "в staging нет start.sh"

IMAGES_META=0
if [[ -n "$IMAGES_SRC" ]]; then
    mkdir -p "${stage}/${PREFIX}/images"
    cp -a "${IMAGES_SRC}/." "${stage}/${PREFIX}/images/"
    [[ -f "${stage}/${PREFIX}/images/manifest.txt" ]] || fail "images/manifest.txt не скопирован"
    IMAGES_META=1
fi

cat >"${stage}/${PREFIX}/.ga-package" <<EOF
product=geoatlas
version=${VERSION}
format=1
images=${IMAGES_META}
EOF

# Права на ops-скрипты внутри архива (git может хранить 0644).
while IFS= read -r -d '' f; do
    chmod +x "$f"
done < <(find "${stage}/${PREFIX}" \( -name '*.sh' -o -name 'update.sh' -o -name 'start.sh' -o -name 'stop.sh' \) -print0)

tar -czf "$OUT" -C "$stage" "$PREFIX"

if command -v sha256sum >/dev/null 2>&1; then
    ( cd "$OUT_DIR" && sha256sum "$TARBALL_NAME" >"${TARBALL_NAME}.sha256" )
elif command -v shasum >/dev/null 2>&1; then
    ( cd "$OUT_DIR" && shasum -a 256 "$TARBALL_NAME" >"${TARBALL_NAME}.sha256" )
else
    echo "pack-release WARN: нет sha256sum — checksum не записан" >&2
fi

echo "pack-release: ${OUT}"
if [[ -f "${OUT}.sha256" ]]; then
    echo "pack-release: ${OUT}.sha256"
fi
if [[ "$IMAGES_META" == "1" ]]; then
    echo "pack-release: images=1 (офлайн docker load)"
fi
