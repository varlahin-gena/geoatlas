#!/usr/bin/env bash
# Единый UI-слой для install/uninstall (YAD → whiptail/dialog → текст).
#
# Использование:
#   source deploy/common/ui.sh
#   nm_ui_init
#   nm_ui_msgbox "Заголовок" "Текст"
#   nm_ui_yesno "Заголовок" "Вопрос?" 1   # default 0|1 → return 0 если Yes
#   nm_ui_checklist "Заголовок" "Подсказка" TAG "Desc" ON TAG2 "Desc2" OFF ...
#   nm_ui_radiolist "Заголовок" "Подсказка" TAG "Desc" ON TAG2 "Desc2" OFF ...
#
# Переменные:
#   NM_UI=yad|whiptail|dialog|text   — принудительный бэкенд
#   NM_UI_BACKTITLE                  — строка вверху TUI (по умолчанию ГеоАтлас)
#   NM_UI_TITLE                      — короткое имя для YAD window title
#   NEWT_COLORS                      — палитра whiptail (если задана снаружи — не трогаем)
#   NM_UI_DARK=0                     — отключить тёмную тему (вернуть системную)
#
# После nm_ui_init:
#   NM_UI_BACKEND  — выбранный бэкенд
#   NM_UI_AVAILABLE — 1 если интерактивный UI доступен (не text-only без TTY)
#
# Коды возврата диалогов:
#   0 — OK / Yes
#   1 — No / Cancel (для yesno: No; для остальных — отмена)
#   2 — ESC / abort (трактуем как cancel)

set -Eeuo pipefail

NM_UI_BACKTITLE="${NM_UI_BACKTITLE:-ГеоАтлас}"
NM_UI_TITLE="${NM_UI_TITLE:-ГеоАтлас}"
NM_UI_BACKEND="${NM_UI_BACKEND:-}"
NM_UI_AVAILABLE="${NM_UI_AVAILABLE:-0}"
NM_UI_HEIGHT="${NM_UI_HEIGHT:-18}"
NM_UI_WIDTH="${NM_UI_WIDTH:-72}"
NM_UI_LIST_HEIGHT="${NM_UI_LIST_HEIGHT:-10}"

_nm_ui_log() { echo "[$(date +'%F %T')] [ui] $*" >&2; }

_nm_ui_has_display() {
    [[ -n "${DISPLAY:-}" || -n "${WAYLAND_DISPLAY:-}" ]]
}

_nm_ui_interactive() {
    [[ -t 0 ]] || [[ -r /dev/tty && -w /dev/tty ]]
}

_nm_ui_pick_backend() {
    local forced="${NM_UI:-}"
    forced="${forced,,}"

    case "$forced" in
        yad|whiptail|dialog|text)
            if [[ "$forced" == "yad" ]]; then
                if command -v yad >/dev/null 2>&1 && _nm_ui_has_display; then
                    echo "yad"
                    return
                fi
                _nm_ui_log "NM_UI=yad недоступен — fallback."
            elif [[ "$forced" == "whiptail" ]]; then
                if command -v whiptail >/dev/null 2>&1; then
                    echo "whiptail"
                    return
                fi
                _nm_ui_log "NM_UI=whiptail недоступен — fallback."
            elif [[ "$forced" == "dialog" ]]; then
                if command -v dialog >/dev/null 2>&1; then
                    echo "dialog"
                    return
                fi
                _nm_ui_log "NM_UI=dialog недоступен — fallback."
            else
                echo "text"
                return
            fi
            ;;
        "")
            ;;
        *)
            _nm_ui_log "WARNING: неизвестный NM_UI=${NM_UI} — автовыбор."
            ;;
    esac

    if _nm_ui_has_display && command -v yad >/dev/null 2>&1; then
        echo "yad"
        return
    fi
    if command -v whiptail >/dev/null 2>&1; then
        echo "whiptail"
        return
    fi
    if command -v dialog >/dev/null 2>&1; then
        echo "dialog"
        return
    fi
    echo "text"
}

# Тёмная палитра вместо розового/magenta default newt.
# Формат: element=fg,bg  (цвета newt: black, red, green, brown, blue, magenta, cyan, lightgray, …)
_nm_ui_default_newt_colors() {
    cat <<'EOF'
root=white,black
border=brightcyan,black
window=white,black
shadow=black,black
title=brightcyan,black
button=black,brightcyan
actbutton=brightcyan,black
compactbutton=white,black
checkbox=white,black
actcheckbox=black,brightcyan
entry=white,black
label=white,black
listbox=white,black
actlistbox=brightcyan,black
sellistbox=black,white
actsellistbox=black,brightcyan
textbox=white,black
acttextbox=black,brightcyan
helpline=black,lightgray
roottext=lightgray,black
emptyscale=black,lightgray
disabledentry=black,lightgray
scale=white,black
EOF
}

_nm_ui_apply_theme() {
    # Не перетираем пользовательскую NEWT_COLORS; NM_UI_DARK=0 — оставить системную.
    if [[ "${NM_UI_DARK:-1}" == "0" ]]; then
        return 0
    fi
    # Whiptail/newt: ставим всегда (и для ранних вызовов, и когда backend=whiptail).
    if [[ -z "${NEWT_COLORS:-}" ]]; then
        NEWT_COLORS="$(_nm_ui_default_newt_colors)"
        export NEWT_COLORS
    fi
    case "${NM_UI_BACKEND:-}" in
        dialog)
            # dialog читает DIALOGRC; задаём временный тёмный rc, если свой не указан.
            if [[ -z "${DIALOGRC:-}" ]]; then
                local rc="${TMPDIR:-/tmp}/nm-dialogrc.$$"
                cat >"$rc" <<'EOF'
use_shadow = OFF
screen_color = (WHITE,BLACK,ON)
shadow_color = (BLACK,BLACK,OFF)
dialog_color = (WHITE,BLACK,OFF)
title_color = (CYAN,BLACK,ON)
border_color = (CYAN,BLACK,ON)
button_active_color = (BLACK,CYAN,ON)
button_inactive_color = (WHITE,BLACK,OFF)
button_key_active_color = (BLACK,CYAN,ON)
button_key_inactive_color = (CYAN,BLACK,ON)
button_label_active_color = (BLACK,CYAN,ON)
button_label_inactive_color = (WHITE,BLACK,ON)
inputbox_color = (WHITE,BLACK,OFF)
inputbox_border_color = (CYAN,BLACK,ON)
searchbox_color = (WHITE,BLACK,OFF)
searchbox_title_color = (CYAN,BLACK,ON)
searchbox_border_color = (CYAN,BLACK,ON)
position_indicator_color = (CYAN,BLACK,ON)
menubox_color = (WHITE,BLACK,OFF)
menubox_border_color = (CYAN,BLACK,ON)
item_color = (WHITE,BLACK,OFF)
item_selected_color = (BLACK,CYAN,ON)
tag_color = (CYAN,BLACK,ON)
tag_selected_color = (BLACK,CYAN,ON)
tag_key_color = (CYAN,BLACK,ON)
tag_key_selected_color = (BLACK,CYAN,ON)
check_color = (WHITE,BLACK,OFF)
check_selected_color = (BLACK,CYAN,ON)
uarrow_color = (CYAN,BLACK,ON)
darrow_color = (CYAN,BLACK,ON)
itemhelp_color = (BLACK,LIGHTGRAY,OFF)
form_active_text_color = (WHITE,BLACK,ON)
form_text_color = (WHITE,BLACK,OFF)
EOF
                export DIALOGRC="$rc"
            fi
            ;;
        yad)
            # Предпочитаем тёмную GTK-тему, если пользователь не задал свою.
            if [[ -z "${GTK_THEME:-}" ]]; then
                export GTK_THEME="${NM_UI_GTK_THEME:-Adwaita:dark}"
            fi
            ;;
    esac
}

nm_ui_init() {
    NM_UI_BACKEND="$(_nm_ui_pick_backend)"
    if [[ "$NM_UI_BACKEND" != "text" ]] && _nm_ui_interactive; then
        NM_UI_AVAILABLE=1
    elif [[ "$NM_UI_BACKEND" == "text" ]] && _nm_ui_interactive; then
        NM_UI_AVAILABLE=1
    else
        NM_UI_AVAILABLE=0
        NM_UI_BACKEND="text"
    fi
    _nm_ui_apply_theme
    _nm_ui_log "backend=${NM_UI_BACKEND} available=${NM_UI_AVAILABLE}"
}

# --- text helpers -----------------------------------------------------------

_nm_ui_text_yesno() {
    # $1 prompt, $2 default 0|1 → 0 if yes
    local prompt="$1"
    local def="${2:-1}"
    local hint answer
    if [[ "$def" == "1" ]]; then
        hint="Y/n"
    else
        hint="y/N"
    fi
    if [[ -r /dev/tty && -w /dev/tty ]]; then
        printf '%s [%s]: ' "$prompt" "$hint" >/dev/tty
        read -r answer </dev/tty || answer=""
    else
        printf '%s [%s]: ' "$prompt" "$hint" >&2
        read -r answer || answer=""
    fi
    answer="${answer,,}"
    answer="${answer//[[:space:]]/}"
    if [[ -z "$answer" ]]; then
        [[ "$def" == "1" ]]
        return
    fi
    case "$answer" in
        y|yes|д|да|1) return 0 ;;
        n|no|н|нет|0) return 1 ;;
        *)
            echo "  Введите y или n." >/dev/tty 2>/dev/null || echo "  Введите y или n." >&2
            _nm_ui_text_yesno "$prompt" "$def"
            ;;
    esac
}

_nm_ui_text_msgbox() {
    local title="$1"
    local text="$2"
    echo "" >&2
    echo "══════════════════════════════════════════════════════════" >&2
    echo "  ${title}" >&2
    echo "══════════════════════════════════════════════════════════" >&2
    # shellcheck disable=SC2001
    echo "$text" | sed 's/^/  /' >&2
    echo "══════════════════════════════════════════════════════════" >&2
    echo "" >&2
    if [[ -r /dev/tty && -w /dev/tty ]]; then
        printf 'Нажмите Enter для продолжения… ' >/dev/tty
        read -r _ </dev/tty || true
    fi
}

# Парсинг пар tag/desc/status для checklist/radiolist.
# Аргументы после title+text: TAG "Desc" ON|OFF ...
# Результат в массивах _NM_UI_TAGS _NM_UI_DESCS _NM_UI_ON
_nm_ui_parse_list_items() {
    _NM_UI_TAGS=()
    _NM_UI_DESCS=()
    _NM_UI_ON=()
    while [[ $# -ge 3 ]]; do
        _NM_UI_TAGS+=("$1")
        _NM_UI_DESCS+=("$2")
        local st="${3^^}"
        if [[ "$st" == "ON" || "$st" == "1" || "$st" == "TRUE" ]]; then
            _NM_UI_ON+=("ON")
        else
            _NM_UI_ON+=("OFF")
        fi
        shift 3
    done
}

_nm_ui_text_checklist() {
    local title="$1"
    local text="$2"
    shift 2
    _nm_ui_parse_list_items "$@"
    echo "" >&2
    echo "══════════════════════════════════════════════════════════" >&2
    echo "  ${title}" >&2
    echo "══════════════════════════════════════════════════════════" >&2
    # shellcheck disable=SC2001
    echo "$text" | sed 's/^/  /' >&2
    echo "" >&2

    local i tag desc def ans
    local -a selected=()
    for i in "${!_NM_UI_TAGS[@]}"; do
        tag="${_NM_UI_TAGS[$i]}"
        desc="${_NM_UI_DESCS[$i]}"
        def=0
        [[ "${_NM_UI_ON[$i]}" == "ON" ]] && def=1
        if _nm_ui_text_yesno "  ${desc}" "$def"; then
            selected+=("$tag")
        fi
    done
    (IFS=','; echo "${selected[*]-}")
}

_nm_ui_text_radiolist() {
    local title="$1"
    local text="$2"
    shift 2
    _nm_ui_parse_list_items "$@"
    local default_tag=""
    local i
    for i in "${!_NM_UI_TAGS[@]}"; do
        if [[ "${_NM_UI_ON[$i]}" == "ON" ]]; then
            default_tag="${_NM_UI_TAGS[$i]}"
            break
        fi
    done
    [[ -z "$default_tag" && ${#_NM_UI_TAGS[@]} -gt 0 ]] && default_tag="${_NM_UI_TAGS[0]}"

    echo "" >&2
    echo "══════════════════════════════════════════════════════════" >&2
    echo "  ${title}" >&2
    echo "══════════════════════════════════════════════════════════" >&2
    # shellcheck disable=SC2001
    echo "$text" | sed 's/^/  /' >&2
    echo "" >&2
    for i in "${!_NM_UI_TAGS[@]}"; do
        local mark=""
        [[ "${_NM_UI_TAGS[$i]}" == "$default_tag" ]] && mark="  ← по умолчанию"
        printf "  [%s] %s%s\n" "${_NM_UI_TAGS[$i]}" "${_NM_UI_DESCS[$i]}" "$mark" >&2
    done
    echo "" >&2

    local answer
    while true; do
        if [[ -r /dev/tty && -w /dev/tty ]]; then
            printf 'Ваш выбор [%s]: ' "$default_tag" >/dev/tty
            read -r answer </dev/tty || answer=""
        else
            printf 'Ваш выбор [%s]: ' "$default_tag" >&2
            read -r answer || answer=""
        fi
        answer="${answer,,}"
        answer="${answer//[[:space:]]/}"
        [[ -z "$answer" ]] && answer="$default_tag"

        for i in "${!_NM_UI_TAGS[@]}"; do
            if [[ "${_NM_UI_TAGS[$i],,}" == "$answer" ]]; then
                echo "${_NM_UI_TAGS[$i]}"
                return 0
            fi
        done
        # также принять индекс 1..N
        if [[ "$answer" =~ ^[0-9]+$ ]] && (( answer >= 1 && answer <= ${#_NM_UI_TAGS[@]} )); then
            echo "${_NM_UI_TAGS[$((answer - 1))]}"
            return 0
        fi
        if [[ "$answer" == "q" || "$answer" == "quit" || "$answer" == "отмена" ]]; then
            return 1
        fi
        echo "  Не понял «${answer}». Введите тег из списка или q для отмены." >&2
    done
}

# --- YAD --------------------------------------------------------------------

_nm_ui_yad_msgbox() {
    local title="$1"
    local text="$2"
    yad --title="${NM_UI_TITLE}" \
        --window-icon=dialog-information \
        --center --width=520 \
        --button="OK:0" \
        --text="<b>${title}</b>\n\n${text}" \
        --text-align=left 2>/dev/null
}

_nm_ui_yad_yesno() {
    local title="$1"
    local text="$2"
    local def="${3:-1}"
    local btn_yes="Да:0"
    local btn_no="Нет:1"
    if [[ "$def" == "1" ]]; then
        yad --title="${NM_UI_TITLE}" --center --width=480 \
            --button="$btn_yes" --button="$btn_no" \
            --text="<b>${title}</b>\n\n${text}" 2>/dev/null
    else
        yad --title="${NM_UI_TITLE}" --center --width=480 \
            --button="$btn_no" --button="$btn_yes" \
            --text="<b>${title}</b>\n\n${text}" 2>/dev/null
        local rc=$?
        # при default=No кнопки переставлены: первая = Нет(1), вторая = Да(0)
        # yad возвращает код кнопки; при Esc — 252
        return "$rc"
    fi
}

_nm_ui_yad_checklist() {
    local title="$1"
    local text="$2"
    shift 2
    _nm_ui_parse_list_items "$@"
    local -a args=()
    local i
    for i in "${!_NM_UI_TAGS[@]}"; do
        if [[ "${_NM_UI_ON[$i]}" == "ON" ]]; then
            args+=(TRUE)
        else
            args+=(FALSE)
        fi
        args+=("${_NM_UI_TAGS[$i]}" "${_NM_UI_DESCS[$i]}")
    done
    local out
    out="$(yad --title="${NM_UI_TITLE}" --center --width=560 --height=360 \
        --list --checklist --multiple \
        --text="<b>${title}</b>\n${text}" \
        --column="Вкл" --column="Код" --column="Описание" \
        --button="OK:0" --button="Отмена:1" \
        "${args[@]}" 2>/dev/null)" || return 1
    # YAD checklist: строки "TRUE|tag|desc" через \n, поля через |
    local line tag
    local -a selected=()
    while IFS= read -r line; do
        [[ -z "$line" ]] && continue
        # формат: TRUE|tag|desc  или TRUE|tag
        tag="$(echo "$line" | awk -F'|' '{print $2}')"
        [[ -n "$tag" ]] && selected+=("$tag")
    done <<<"$out"
    (IFS=','; echo "${selected[*]-}")
}

_nm_ui_yad_radiolist() {
    local title="$1"
    local text="$2"
    shift 2
    _nm_ui_parse_list_items "$@"
    local -a args=()
    local i
    for i in "${!_NM_UI_TAGS[@]}"; do
        if [[ "${_NM_UI_ON[$i]}" == "ON" ]]; then
            args+=(TRUE)
        else
            args+=(FALSE)
        fi
        args+=("${_NM_UI_TAGS[$i]}" "${_NM_UI_DESCS[$i]}")
    done
    local out
    out="$(yad --title="${NM_UI_TITLE}" --center --width=560 --height=360 \
        --list --radiolist \
        --text="<b>${title}</b>\n${text}" \
        --column="Выбор" --column="Код" --column="Описание" \
        --button="OK:0" --button="Отмена:1" \
        "${args[@]}" 2>/dev/null)" || return 1
    # TRUE|tag|desc
    echo "$out" | awk -F'|' '{print $2}' | head -n1
}

# --- whiptail / dialog ------------------------------------------------------

_nm_ui_tui_cmd() {
    if [[ "$NM_UI_BACKEND" == "dialog" ]]; then
        echo "dialog"
    else
        echo "whiptail"
    fi
}

_nm_ui_tui_msgbox() {
    local title="$1"
    local text="$2"
    local cmd
    cmd="$(_nm_ui_tui_cmd)"
    if [[ "$cmd" == "dialog" ]]; then
        "$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
            --msgbox "$text" "$NM_UI_HEIGHT" "$NM_UI_WIDTH" 2>&1 >/dev/tty
    else
        "$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
            --msgbox "$text" "$NM_UI_HEIGHT" "$NM_UI_WIDTH"
    fi
}

_nm_ui_tui_yesno() {
    local title="$1"
    local text="$2"
    local def="${3:-1}"
    local cmd
    cmd="$(_nm_ui_tui_cmd)"
    local -a extra=()
    if [[ "$def" != "1" ]]; then
        if [[ "$cmd" == "whiptail" ]]; then
            extra+=(--defaultno)
        else
            extra+=(--defaultno)
        fi
    fi
    if [[ "$cmd" == "dialog" ]]; then
        "$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
            "${extra[@]}" --yesno "$text" "$NM_UI_HEIGHT" "$NM_UI_WIDTH" 2>&1 >/dev/tty
    else
        "$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
            "${extra[@]}" --yesno "$text" "$NM_UI_HEIGHT" "$NM_UI_WIDTH"
    fi
}

_nm_ui_tui_checklist() {
    local title="$1"
    local text="$2"
    shift 2
    _nm_ui_parse_list_items "$@"
    local cmd
    cmd="$(_nm_ui_tui_cmd)"
    local -a items=()
    local i
    for i in "${!_NM_UI_TAGS[@]}"; do
        items+=("${_NM_UI_TAGS[$i]}" "${_NM_UI_DESCS[$i]}" "${_NM_UI_ON[$i]}")
    done
    local out rc=0
    if [[ "$cmd" == "dialog" ]]; then
        # dialog пишет выбор в stderr
        out="$("$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
            --checklist "$text" "$NM_UI_HEIGHT" "$NM_UI_WIDTH" "$NM_UI_LIST_HEIGHT" \
            "${items[@]}" 2>&1 >/dev/tty)" || rc=$?
    else
        out="$("$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
            --checklist "$text" "$NM_UI_HEIGHT" "$NM_UI_WIDTH" "$NM_UI_LIST_HEIGHT" \
            "${items[@]}" 3>&1 1>&2 2>&3)" || rc=$?
    fi
    (( rc == 0 )) || return 1
    # whiptail: "tag1" "tag2"  или tag1 tag2
    out="${out//\"/}"
    # shellcheck disable=SC2086
    local -a selected=($out)
    (IFS=','; echo "${selected[*]-}")
}

_nm_ui_tui_radiolist() {
    local title="$1"
    local text="$2"
    shift 2
    _nm_ui_parse_list_items "$@"
    local cmd
    cmd="$(_nm_ui_tui_cmd)"
    local -a items=()
    local i
    for i in "${!_NM_UI_TAGS[@]}"; do
        items+=("${_NM_UI_TAGS[$i]}" "${_NM_UI_DESCS[$i]}" "${_NM_UI_ON[$i]}")
    done
    local out rc=0
    if [[ "$cmd" == "dialog" ]]; then
        out="$("$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
            --radiolist "$text" "$NM_UI_HEIGHT" "$NM_UI_WIDTH" "$NM_UI_LIST_HEIGHT" \
            "${items[@]}" 2>&1 >/dev/tty)" || rc=$?
    else
        out="$("$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
            --radiolist "$text" "$NM_UI_HEIGHT" "$NM_UI_WIDTH" "$NM_UI_LIST_HEIGHT" \
            "${items[@]}" 3>&1 1>&2 2>&3)" || rc=$?
    fi
    (( rc == 0 )) || return 1
    out="${out//\"/}"
    out="${out%%[[:space:]]*}"
    echo "$out"
}

# --- public API -------------------------------------------------------------

nm_ui_msgbox() {
    local title="$1"
    local text="$2"
    case "${NM_UI_BACKEND:-text}" in
        yad)
            _nm_ui_yad_msgbox "$title" "$text" || _nm_ui_text_msgbox "$title" "$text"
            ;;
        whiptail|dialog)
            _nm_ui_tui_msgbox "$title" "$text" || _nm_ui_text_msgbox "$title" "$text"
            ;;
        *)
            _nm_ui_text_msgbox "$title" "$text"
            ;;
    esac
}

# nm_ui_yesno TITLE TEXT [default 0|1]
# return 0 = Yes, 1 = No
nm_ui_yesno() {
    local title="$1"
    local text="$2"
    local def="${3:-1}"
    case "${NM_UI_BACKEND:-text}" in
        yad)
            if _nm_ui_yad_yesno "$title" "$text" "$def"; then
                return 0
            fi
            local rc=$?
            # Esc / close → treat as No
            if (( rc == 252 || rc > 1 )); then
                return 1
            fi
            return 1
            ;;
        whiptail|dialog)
            if _nm_ui_tui_yesno "$title" "$text" "$def"; then
                return 0
            fi
            return 1
            ;;
        *)
            _nm_ui_text_yesno "${title}: ${text}" "$def"
            ;;
    esac
}

# nm_ui_checklist TITLE TEXT TAG DESC ON|OFF ...
# печатает выбранные теги через запятую в stdout; return 1 при отмене
nm_ui_checklist() {
    local title="$1"
    local text="$2"
    shift 2
    local out
    case "${NM_UI_BACKEND:-text}" in
        yad)
            if out="$(_nm_ui_yad_checklist "$title" "$text" "$@")"; then
                echo "$out"
                return 0
            fi
            # fallback на text при сбое yad
            if out="$(_nm_ui_text_checklist "$title" "$text" "$@")"; then
                echo "$out"
                return 0
            fi
            return 1
            ;;
        whiptail|dialog)
            if out="$(_nm_ui_tui_checklist "$title" "$text" "$@")"; then
                echo "$out"
                return 0
            fi
            return 1
            ;;
        *)
            out="$(_nm_ui_text_checklist "$title" "$text" "$@")"
            echo "$out"
            return 0
            ;;
    esac
}

# nm_ui_radiolist TITLE TEXT TAG DESC ON|OFF ...
# печатает выбранный тег в stdout; return 1 при отмене
nm_ui_radiolist() {
    local title="$1"
    local text="$2"
    shift 2
    local out
    case "${NM_UI_BACKEND:-text}" in
        yad)
            if out="$(_nm_ui_yad_radiolist "$title" "$text" "$@")"; then
                echo "$out"
                return 0
            fi
            if out="$(_nm_ui_text_radiolist "$title" "$text" "$@")"; then
                echo "$out"
                return 0
            fi
            return 1
            ;;
        whiptail|dialog)
            if out="$(_nm_ui_tui_radiolist "$title" "$text" "$@")"; then
                echo "$out"
                return 0
            fi
            return 1
            ;;
        *)
            if out="$(_nm_ui_text_radiolist "$title" "$text" "$@")"; then
                echo "$out"
                return 0
            fi
            return 1
            ;;
    esac
}

# Инициализация по умолчанию при source (можно переопределить вызовом nm_ui_init).
if [[ -z "${NM_UI_BACKEND:-}" ]]; then
    nm_ui_init
fi
