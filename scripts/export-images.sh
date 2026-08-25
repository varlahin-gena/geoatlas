#!/usr/bin/env bash
# Сборка / pull образов runtime и docker save в каталог images/ для офлайн-пакета.
#   bash scripts/export-images.sh [выходной-каталог]
# По умолчанию: dist/image-bundle/images
#
# Пишет:
#   <out>/manifest.txt
#   <out>/*.tar
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "export-images FAIL: $*" >&2; exit 1; }
log() { echo "[export-images] $*"; }

command -v docker >/dev/null 2>&1 || fail "нужен docker"
docker info >/dev/null 2>&1 || fail "docker daemon не запущен"

[[ -f docker-compose.yml ]] || fail "нет docker-compose.yml"
[[ -f backend/Dockerfile ]] || fail "нет backend/Dockerfile"
[[ -f frontend/Dockerfile ]] || fail "нет frontend/Dockerfile"
[[ -f stats-collector/Dockerfile ]] || fail "нет stats-collector/Dockerfile"
[[ -f syslog-ng/Dockerfile ]] || fail "нет syslog-ng/Dockerfile"

OUT="${1:-${ROOT}/dist/image-bundle/images}"
mkdir -p "$OUT"
OUT="$(cd "$OUT" && pwd)"
rm -f "${OUT}"/*.tar "${OUT}/manifest.txt" 2>/dev/null || true

# tag -> safe filename stem (no / or :)
_ga_img_stem() {
    local tag="$1"
    echo "${tag}" | tr '/:' '__'
}

_ga_file_sha256() {
    local f="$1"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$f" | awk '{print $1}'
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$f" | awk '{print $1}'
    else
        echo "unknown"
    fi
}

_ga_image_id() {
    docker image inspect -f '{{.Id}}' "$1" 2>/dev/null || echo ""
}

build_or_pull() {
    local mode="$1" tag="$2"
    shift 2
    case "$mode" in
        build)
            log "build ${tag}"
            docker build -t "$tag" "$@"
            ;;
        pull)
            log "pull ${tag}"
            docker pull "$tag"
            ;;
        *)
            fail "unknown mode: ${mode}"
            ;;
    esac
}

save_one() {
    local tag="$1"
    local stem file sum id
    stem="$(_ga_img_stem "$tag")"
    file="${stem}.tar"
    log "save ${tag} → ${file}"
    docker save -o "${OUT}/${file}" "$tag"
    sum="$(_ga_file_sha256 "${OUT}/${file}")"
    id="$(_ga_image_id "$tag")"
    # manifest: image_tag filename sha256 image_id
    printf '%s %s %s %s\n' "$tag" "$file" "$sum" "$id" >>"${OUT}/manifest.txt"
}

# App images (compose tags).
build_or_pull build geoatlas-backend:latest -f backend/Dockerfile .
build_or_pull build geoatlas-stats-collector:latest -f stats-collector/Dockerfile .
build_or_pull build geoatlas-frontend:latest -f frontend/Dockerfile frontend
build_or_pull build geoatlas-syslog-ng:latest -f syslog-ng/Dockerfile syslog-ng

# Third-party runtime images from docker-compose.yml.
build_or_pull pull clickhouse/clickhouse-server:25.8.30.16
build_or_pull pull alpine:3.23

: >"${OUT}/manifest.txt"
save_one geoatlas-backend:latest
save_one geoatlas-stats-collector:latest
save_one geoatlas-frontend:latest
save_one geoatlas-syslog-ng:latest
save_one clickhouse/clickhouse-server:25.8.30.16
save_one alpine:3.23

log "готово: ${OUT}/manifest.txt"
wc -l <"${OUT}/manifest.txt" | awk '{print "[export-images] образов: " $1}'
du -sh "$OUT" | awk '{print "[export-images] размер: " $1}'
