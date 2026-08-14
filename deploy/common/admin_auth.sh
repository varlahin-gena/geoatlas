#!/usr/bin/env bash
# Seed-пароль администратора: генерация, проверка, TUI-запрос.
# Source only — не запускать как скрипт.

nm_rand_hex() {
    local nbytes="${1:-12}"
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -hex "$nbytes"
    else
        head -c "$nbytes" /dev/urandom | od -An -tx1 | tr -d ' \n'
    fi
}

nm_password_is_weak() {
    local user="${1:-}" pass="${2:-}"
    user="${user,,}"
    pass="${pass,,}"
    [[ -z "$pass" ]] && return 1
    [[ "$pass" == "$user" ]] && return 0
    case "$pass" in
        admin|operator|password|changeme|123456) return 0 ;;
    esac
    return 1
}

# Пароль для неквотированного .env: без пробелов и символов, ломающих compose.
nm_validate_admin_password() {
    local user="$1" pass="$2"
    local n=${#pass}
    if (( n < 8 )); then
        echo "Пароль короче 8 символов." >&2
        return 1
    fi
    if (( n > 128 )); then
        echo "Пароль слишком длинный (макс. 128)." >&2
        return 1
    fi
    if [[ "$pass" =~ [[:space:]] ]]; then
        echo "Пароль не должен содержать пробелы и переводы строк." >&2
        return 1
    fi
    if [[ "$pass" =~ [\'\"#\$\`\\] ]]; then
        echo "Пароль не должен содержать кавычки, \$, #, backtick или обратный слэш (ограничение .env)." >&2
        return 1
    fi
    if nm_password_is_weak "$user" "$pass"; then
        echo "Пароль слишком простой (не совпадайте с логином; не admin/password/changeme)." >&2
        return 1
    fi
    return 0
}

# Спросить пароль дважды. stdout = пароль. return 1 при отмене.
nm_prompt_admin_password() {
    local user="${1:-admin}"
    local a b
    if ! declare -F nm_ui_passwordbox >/dev/null 2>&1; then
        echo "TUI passwordbox недоступен." >&2
        return 1
    fi
    while true; do
        if ! a="$(nm_ui_passwordbox "Пароль администратора" \
            "Учётка ${user} (роль administrator).
Минимум 8 символов, без пробелов и кавычек.
Другие пользователи — после входа, в разделе «Пользователи».")"; then
            return 1
        fi
        if ! nm_validate_admin_password "$user" "$a"; then
            if declare -F nm_ui_msgbox >/dev/null 2>&1; then
                nm_ui_msgbox "Пароль" "Пароль не принят. Повторите ввод." || true
            fi
            continue
        fi
        if ! b="$(nm_ui_passwordbox "Подтверждение пароля" \
            "Повторите пароль для ${user}.")"; then
            return 1
        fi
        if [[ "$a" != "$b" ]]; then
            if declare -F nm_ui_msgbox >/dev/null 2>&1; then
                nm_ui_msgbox "Пароль" "Пароли не совпали." || true
            fi
            continue
        fi
        printf '%s\n' "$a"
        return 0
    done
}
