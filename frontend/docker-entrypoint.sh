#!/bin/sh
# Frontend entrypoint: UI config.js + nginx default.conf (HTTP or HTTPS).
set -eu

CONF=/etc/nginx/conf.d/default.conf
APP_INC=/etc/nginx/includes/app.inc
CERT="${SSL_CERT_FILE:-/etc/nginx/certs/fullchain.pem}"
KEY="${SSL_KEY_FILE:-/etc/nginx/certs/privkey.pem}"
HTTPS_ENABLED="${HTTPS_ENABLED:-auto}"
HTTP_REDIRECT="${HTTP_REDIRECT:-1}"

truthy() {
    case "${1:-}" in
        1|true|TRUE|yes|YES|on|ON) return 0 ;;
        *) return 1 ;;
    esac
}

falsy() {
    case "${1:-}" in
        0|false|FALSE|no|NO|off|OFF) return 0 ;;
        *) return 1 ;;
    esac
}

certs_ok=0
if [ -f "$CERT" ] && [ -f "$KEY" ]; then
    certs_ok=1
fi

use_https=0
if truthy "$HTTPS_ENABLED"; then
    if [ "$certs_ok" -eq 1 ]; then
        use_https=1
    else
        echo "WARNING: HTTPS_ENABLED but missing cert/key ($CERT / $KEY) — HTTP only" >&2
    fi
elif falsy "$HTTPS_ENABLED"; then
    use_https=0
elif [ "$certs_ok" -eq 1 ]; then
    # auto: enable when PEM files are present
    use_https=1
fi

write_http_server() {
    cat <<EOF
server {
    listen 80;
    server_name _;
    include ${APP_INC};
}
EOF
}

write_https_servers() {
    if truthy "$HTTP_REDIRECT"; then
        cat <<EOF
server {
    listen 80;
    server_name _;
    location = /health {
        access_log off;
        default_type application/json;
        add_header Cache-Control "no-store" always;
        return 200 '{"ok":true,"status":"live"}';
    }
    location / {
        return 301 https://\$host\$request_uri;
    }
}
EOF
    else
        write_http_server
    fi

    cat <<EOF

server {
    listen 443 ssl;
    http2 on;
    server_name _;

    ssl_certificate     ${CERT};
    ssl_certificate_key ${KEY};
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers off;

    include ${APP_INC};
}
EOF
}

if [ ! -f "$APP_INC" ]; then
    echo "ERROR: missing $APP_INC" >&2
    exit 1
fi

if [ "$use_https" -eq 1 ]; then
    echo "nginx: HTTPS enabled (cert=$CERT)"
    write_https_servers >"$CONF"
else
    echo "nginx: HTTP only (place certs in ./certs and set HTTPS_ENABLED=1 or leave auto)"
    write_http_server >"$CONF"
fi

# UI-авторизация через cookie-сессии; API-токен в браузер не отдаём.
printf 'window.NM_CONFIG={};\n' > /tmp/nm-config.js

exec nginx -g 'daemon off;'
