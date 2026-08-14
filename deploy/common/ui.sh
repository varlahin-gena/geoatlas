#!/usr/bin/env bash
# Единый TUI-слой для install/uninstall (whiptail → dialog → текст).
#
# Использование:
#   source deploy/common/ui.sh
#   nm_ui_init
#   nm_ui_msgbox "Заголовок" "Текст"
#   nm_ui_yesno "Заголовок" "Вопрос?" 1   # default 0|1 → return 0 если Yes
#   nm_ui_checklist "Заголовок" "Подсказка" TAG "Desc" ON TAG2 "Desc2" OFF ...
#   nm_ui_radiolist "Заголовок" "Подсказка" TAG "Desc" ON TAG2 "Desc2" OFF ...
#   nm_ui_inputbox "Заголовок" "Подсказка" ["default"]  # stdout = ввод
#   nm_ui_passwordbox "Заголовок" "Подсказка"          # скрытый ввод, stdout = пароль
#   nm_ui_gauge "Заголовок" "Текст" PERCENT
#   nm_ui_run_with_gauge "Заголовок" "Текст" CMD [ARGS...]
#
# Переменные:
#   NM_UI=whiptail|dialog|text   — принудительный бэкенд
#   NM_UI_BACKTITLE              — строка вверху TUI (по умолчанию ГеоАтлас)
#   NEWT_COLORS                  — палитра whiptail (если задана снаружи — не трогаем)
#   NM_UI_DARK=0                 — отключить тёмную тему (вернуть системную)
#
# После nm_ui_init:
#   NM_UI_BACKEND  — выбранный бэкенд
#   NM_UI_AVAILABLE — 1 если интерактивный UI доступен
#
# Коды возврата диалогов:
#   0 — OK / Yes
#   1 — No / Cancel

set -Eeuo pipefail

NM_UI_BACKTITLE="${NM_UI_BACKTITLE:-ГеоАтлас}"
NM_UI_TITLE="${NM_UI_TITLE:-ГеоАтлас}"
NM_UI_BACKEND="${NM_UI_BACKEND:-}"
NM_UI_AVAILABLE="${NM_UI_AVAILABLE:-0}"
NM_UI_HEIGHT="${NM_UI_HEIGHT:-18}"
NM_UI_WIDTH="${NM_UI_WIDTH:-72}"
NM_UI_LIST_HEIGHT="${NM_UI_LIST_HEIGHT:-10}"
NM_UI_GAUGE_HEIGHT="${NM_UI_GAUGE_HEIGHT:-8}"

_nm_ui_log() { echo "[$(date +'%F %T')] [ui] $*" >&2; }

_nm_ui_interactive() {
    [[ -t 0 ]] || [[ -r /dev/tty && -w /dev/tty ]]
}

_nm_ui_pick_backend() {
    local forced="${NM_UI:-}"
    forced="${forced,,}"

    case "$forced" in
        whiptail)
            if command -v whiptail >/dev/null 2>&1; then
                echo "whiptail"
                return
            fi
            _nm_ui_log "NM_UI=whiptail недоступен — запасной вариант."
            ;;
        dialog)
            if command -v dialog >/dev/null 2>&1; then
                echo "dialog"
                return
            fi
            _nm_ui_log "NM_UI=dialog недоступен — запасной вариант."
            ;;
        text)
            echo "text"
            return
            ;;
        yad)
            _nm_ui_log "NM_UI=yad больше не поддерживается — используем whiptail/dialog/text."
            ;;
        "")
            ;;
        *)
            _nm_ui_log "ВНИМАНИЕ: неизвестный NM_UI=${NM_UI} — автовыбор."
            ;;
    esac

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
    if [[ "${NM_UI_DARK:-1}" == "0" ]]; then
        return 0
    fi
    if [[ -z "${NEWT_COLORS:-}" ]]; then
        NEWT_COLORS="$(_nm_ui_default_newt_colors)"
        export NEWT_COLORS
    fi
    case "${NM_UI_BACKEND:-}" in
        dialog)
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
gauge_color = (WHITE,BLACK,ON)
EOF
                export DIALOGRC="$rc"
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
    if _nm_ui_interactive && [[ -r /dev/tty && -w /dev/tty ]]; then
        printf '  [Enter] ' >/dev/tty
        read -r _ </dev/tty || true
    fi
}

_nm_ui_parse_list_items() {
    _NM_UI_TAGS=()
    _NM_UI_DESCS=()
    _NM_UI_ON=()
    while (($# >= 3)); do
        _NM_UI_TAGS+=("$1")
        _NM_UI_DESCS+=("$2")
        _NM_UI_ON+=("$3")
        shift 3
    done
}

_nm_ui_text_checklist() {
    local title="$1"
    local text="$2"
    shift 2
    _nm_ui_parse_list_items "$@"
    echo "" >&2
    echo "  ${title}" >&2
    echo "  ${text}" >&2
    local i selected=()
    for i in "${!_NM_UI_TAGS[@]}"; do
        local def=0
        [[ "${_NM_UI_ON[$i]}" == "ON" ]] && def=1
        local desc="${_NM_UI_TAGS[$i]} — ${_NM_UI_DESCS[$i]}"
        if _nm_ui_text_yesno "  ${desc}" "$def"; then
            selected+=("${_NM_UI_TAGS[$i]}")
        fi
    done
    (IFS=','; echo "${selected[*]-}")
}

_nm_ui_text_radiolist() {
    local title="$1"
    local text="$2"
    shift 2
    _nm_ui_parse_list_items "$@"
    echo "" >&2
    echo "  ${title}" >&2
    echo "  ${text}" >&2
    local i default_idx=0
    for i in "${!_NM_UI_TAGS[@]}"; do
        echo "  [$((i + 1))] ${_NM_UI_TAGS[$i]} — ${_NM_UI_DESCS[$i]}" >&2
        [[ "${_NM_UI_ON[$i]}" == "ON" ]] && default_idx=$((i + 1))
    done
    local answer
    if [[ -r /dev/tty && -w /dev/tty ]]; then
        printf '  Выбор [%s]: ' "$default_idx" >/dev/tty
        read -r answer </dev/tty || answer=""
    else
        printf '  Выбор [%s]: ' "$default_idx" >&2
        read -r answer || answer=""
    fi
    answer="${answer//[[:space:]]/}"
    [[ -z "$answer" ]] && answer="$default_idx"
    if [[ "$answer" =~ ^[0-9]+$ ]] && (( answer >= 1 && answer <= ${#_NM_UI_TAGS[@]} )); then
        echo "${_NM_UI_TAGS[$((answer - 1))]}"
        return 0
    fi
    return 1
}

_nm_ui_text_inputbox() {
    local title="$1"
    local text="$2"
    local def="${3:-}"
    local answer
    echo "" >&2
    echo "  ${title}" >&2
    echo "  ${text}" >&2
    if [[ -r /dev/tty && -w /dev/tty ]]; then
        printf '  [%s]: ' "$def" >/dev/tty
        read -r answer </dev/tty || answer=""
    else
        printf '  [%s]: ' "$def" >&2
        read -r answer || answer=""
    fi
    [[ -z "$answer" ]] && answer="$def"
    echo "$answer"
}

_nm_ui_text_passwordbox() {
    local title="$1"
    local text="$2"
    local answer=""
    echo "" >&2
    echo "  ${title}" >&2
    echo "  ${text}" >&2
    if [[ -r /dev/tty && -w /dev/tty ]]; then
        printf '  пароль: ' >/dev/tty
        read -rs answer </dev/tty || return 1
        printf '\n' >/dev/tty
    else
        printf '  пароль: ' >&2
        read -rs answer || return 1
        printf '\n' >&2
    fi
    printf '%s\n' "$answer"
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
        extra+=(--defaultno)
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
        out="$("$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
            --checklist "$text" "$NM_UI_HEIGHT" "$NM_UI_WIDTH" "$NM_UI_LIST_HEIGHT" \
            "${items[@]}" 2>&1 >/dev/tty)" || rc=$?
    else
        out="$("$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
            --checklist "$text" "$NM_UI_HEIGHT" "$NM_UI_WIDTH" "$NM_UI_LIST_HEIGHT" \
            "${items[@]}" 3>&1 1>&2 2>&3)" || rc=$?
    fi
    (( rc == 0 )) || return 1
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

_nm_ui_tui_inputbox() {
    local title="$1"
    local text="$2"
    local def="${3:-}"
    local cmd out rc=0
    cmd="$(_nm_ui_tui_cmd)"
    if [[ "$cmd" == "dialog" ]]; then
        out="$("$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
            --inputbox "$text" "$NM_UI_HEIGHT" "$NM_UI_WIDTH" "$def" 2>&1 >/dev/tty)" || rc=$?
    else
        out="$("$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
            --inputbox "$text" "$NM_UI_HEIGHT" "$NM_UI_WIDTH" "$def" 3>&1 1>&2 2>&3)" || rc=$?
    fi
    (( rc == 0 )) || return 1
    echo "$out"
}

_nm_ui_tui_passwordbox() {
    local title="$1"
    local text="$2"
    local cmd out rc=0
    cmd="$(_nm_ui_tui_cmd)"
    if [[ "$cmd" == "dialog" ]]; then
        out="$("$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
            --insecure --passwordbox "$text" "$NM_UI_HEIGHT" "$NM_UI_WIDTH" 2>&1 >/dev/tty)" || rc=$?
    else
        out="$("$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
            --passwordbox "$text" "$NM_UI_HEIGHT" "$NM_UI_WIDTH" 3>&1 1>&2 2>&3)" || rc=$?
    fi
    (( rc == 0 )) || return 1
    echo "$out"
}

# Однократный кадр gauge (для ступенчатого прогресса).
# nm_ui_gauge TITLE TEXT PERCENT
nm_ui_gauge() {
    local title="$1"
    local text="$2"
    local pct="${3:-0}"
    [[ "$pct" =~ ^[0-9]+$ ]] || pct=0
    (( pct > 100 )) && pct=100
    (( pct < 0 )) && pct=0

    case "${NM_UI_BACKEND:-text}" in
        whiptail|dialog)
            if [[ "${NM_UI_AVAILABLE:-0}" != "1" ]]; then
                echo "[gauge ${pct}%] ${title}: ${text}" >&2
                return 0
            fi
            local cmd
            cmd="$(_nm_ui_tui_cmd)"
            if [[ "$cmd" == "dialog" ]]; then
                echo "$pct" | "$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
                    --gauge "$text" "$NM_UI_GAUGE_HEIGHT" "$NM_UI_WIDTH" "$pct" 2>&1 >/dev/tty || true
            else
                echo "$pct" | "$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
                    --gauge "$text" "$NM_UI_GAUGE_HEIGHT" "$NM_UI_WIDTH" "$pct" || true
            fi
            ;;
        *)
            echo "[gauge ${pct}%] ${title}: ${text}" >&2
            ;;
    esac
}

# Запуск долгой команды с пульсирующим gauge.
# Лог → tempfile; при ошибке — msgbox с хвостом лога.
# nm_ui_run_with_gauge TITLE TEXT CMD [ARGS...]
nm_ui_run_with_gauge() {
    local title="$1"
    local text="$2"
    shift 2
    local logf pid rc=0 pct=5
    logf="$(mktemp "${TMPDIR:-/tmp}/nm-gauge.XXXXXX")"

    if [[ "${NM_UI_BACKEND:-text}" == "text" ]] || [[ "${NM_UI_AVAILABLE:-0}" != "1" ]]; then
        echo "[gauge] ${title}: ${text}" >&2
        set +e
        "$@" >"$logf" 2>&1
        rc=$?
        set -e
        if (( rc != 0 )); then
            _nm_ui_log "ОШИБКА: ${title} (код выхода ${rc})"
            tail -n 40 "$logf" >&2 || true
            if declare -F nm_ui_msgbox >/dev/null 2>&1; then
                nm_ui_msgbox "Ошибка: ${title}" \
                    "Команда завершилась с кодом ${rc}.

$(tail -n 25 "$logf" 2>/dev/null || true)" || true
            fi
        else
            echo "[gauge 100%] ${title}: готово" >&2
        fi
        rm -f "$logf"
        return "$rc"
    fi

    set +e
    "$@" >"$logf" 2>&1 &
    pid=$!
    set -e

    local cmd
    cmd="$(_nm_ui_tui_cmd)"
    (
        while kill -0 "$pid" 2>/dev/null; do
            echo "$pct"
            pct=$((pct + 4))
            (( pct >= 95 )) && pct=10
            sleep 0.4
        done
        echo 100
        sleep 0.15
    ) | {
        if [[ "$cmd" == "dialog" ]]; then
            "$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
                --gauge "$text" "$NM_UI_GAUGE_HEIGHT" "$NM_UI_WIDTH" 0 2>&1 >/dev/tty || true
        else
            "$cmd" --backtitle "$NM_UI_BACKTITLE" --title "$title" \
                --gauge "$text" "$NM_UI_GAUGE_HEIGHT" "$NM_UI_WIDTH" 0 || true
        fi
    }

    set +e
    wait "$pid"
    rc=$?
    set -e

    if (( rc != 0 )); then
        local tail_txt
        tail_txt="$(tail -n 25 "$logf" 2>/dev/null || true)"
        nm_ui_msgbox "Ошибка: ${title}" \
            "Команда завершилась с кодом ${rc}.

${tail_txt}" || true
        _nm_ui_log "ОШИБКА: ${title} (код выхода ${rc}) — см. лог ${logf}"
        # оставляем лог для отладки при ошибке
        return "$rc"
    fi
    rm -f "$logf"
    return 0
}

# --- public API -------------------------------------------------------------

nm_ui_msgbox() {
    local title="$1"
    local text="$2"
    case "${NM_UI_BACKEND:-text}" in
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

# nm_ui_inputbox TITLE TEXT [default]
# печатает ввод в stdout; return 1 при отмене
nm_ui_inputbox() {
    local title="$1"
    local text="$2"
    local def="${3:-}"
    local out
    case "${NM_UI_BACKEND:-text}" in
        whiptail|dialog)
            if out="$(_nm_ui_tui_inputbox "$title" "$text" "$def")"; then
                echo "$out"
                return 0
            fi
            return 1
            ;;
        *)
            out="$(_nm_ui_text_inputbox "$title" "$text" "$def")"
            echo "$out"
            return 0
            ;;
    esac
}

# nm_ui_passwordbox TITLE TEXT
# скрытый ввод; stdout = пароль; return 1 при отмене. Без значения по умолчанию.
nm_ui_passwordbox() {
    local title="$1"
    local text="$2"
    local out
    case "${NM_UI_BACKEND:-text}" in
        whiptail|dialog)
            if [[ "${NM_UI_AVAILABLE:-0}" != "1" ]]; then
                return 1
            fi
            if out="$(_nm_ui_tui_passwordbox "$title" "$text")"; then
                echo "$out"
                return 0
            fi
            return 1
            ;;
        *)
            if [[ "${NM_UI_AVAILABLE:-0}" != "1" ]]; then
                return 1
            fi
            out="$(_nm_ui_text_passwordbox "$title" "$text")" || return 1
            echo "$out"
            return 0
            ;;
    esac
}
