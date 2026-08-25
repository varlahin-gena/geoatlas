#!/usr/bin/env bash
# Загрузка Docker-образов из каталога images/ установочного пакета.
# Использование:
#   source deploy/common/load_images.sh
#   ga_load_release_images /opt/geoatlas
#
# Ожидает images/manifest.txt (строки: image_tag filename sha256 image_id).
# Без manifest — no-op (dev / git checkout).

_ga_img_log() { echo "[$(date +'%F %T')] [images] $*" >&2; }

# true, если в project_dir есть бандл образов релиза.
ga_release_images_present() {
    local project_dir="${1:-.}"
    [[ -f "${project_dir}/images/manifest.txt" ]]
}

# Загрузить все образы из images/. rc=0 даже если бандла нет.
ga_load_release_images() {
    local project_dir="${1:-.}"
    local manifest="${project_dir}/images/manifest.txt"
    local img_dir="${project_dir}/images"
    local tag file sum id path got loaded=0

    if [[ ! -f "$manifest" ]]; then
        return 0
    fi

    if ! command -v docker >/dev/null 2>&1; then
        _ga_img_log "ОШИБКА: docker не найден — нельзя загрузить images/"
        return 1
    fi

    _ga_img_log "загрузка образов из ${img_dir}…"
    while IFS= read -r line || [[ -n "$line" ]]; do
        [[ -n "$line" ]] || continue
        [[ "$line" =~ ^# ]] && continue
        # tag file sha256 [image_id]
        tag="$(awk '{print $1}' <<<"$line")"
        file="$(awk '{print $2}' <<<"$line")"
        sum="$(awk '{print $3}' <<<"$line")"
        [[ -n "$tag" && -n "$file" ]] || continue
        path="${img_dir}/${file}"
        if [[ ! -f "$path" ]]; then
            _ga_img_log "ОШИБКА: нет файла ${path} (образ ${tag})"
            return 1
        fi
        if [[ -n "$sum" && "$sum" != "unknown" ]]; then
            if command -v sha256sum >/dev/null 2>&1; then
                got="$(sha256sum "$path" | awk '{print $1}')"
                if [[ "${got,,}" != "${sum,,}" ]]; then
                    _ga_img_log "ОШИБКА: SHA-256 ${file} не совпал"
                    return 1
                fi
            elif command -v shasum >/dev/null 2>&1; then
                got="$(shasum -a 256 "$path" | awk '{print $1}')"
                if [[ "${got,,}" != "${sum,,}" ]]; then
                    _ga_img_log "ОШИБКА: SHA-256 ${file} не совпал"
                    return 1
                fi
            fi
        fi
        _ga_img_log "docker load -i ${file} (${tag})"
        if ! docker load -i "$path"; then
            _ga_img_log "ОШИБКА: docker load не удался для ${file}"
            return 1
        fi
        loaded=$((loaded + 1))
    done <"$manifest"

    _ga_img_log "загружено образов: ${loaded}"
    return 0
}
