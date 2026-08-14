# Shared server{} body for HTTP and HTTPS. Included by frontend entrypoint.
# No listen / ssl_* here — those belong in generated default.conf.

    root /usr/share/nginx/html;
    index index.html;

    client_max_body_size 32m;
    client_body_timeout 60s;
    send_timeout 60s;

    proxy_read_timeout 600s;
    proxy_send_timeout 600s;

    gzip on;
    gzip_comp_level 5;
    gzip_min_length 1024;
    gzip_vary on;
    gzip_proxied any;
    gzip_types
        application/json
        application/javascript
        text/css
        text/plain
        application/geo+json
        image/svg+xml;

    include /etc/nginx/includes/security-headers.inc;

    location ~ /\. {
        deny all;
        access_log off;
        log_not_found off;
    }

    # Токен для мутирующих запросов UI (пишется entrypoint'ом контейнера).
    location = /config.js {
        alias /tmp/nm-config.js;
        default_type application/javascript;
        add_header Cache-Control "no-store" always;
        include /etc/nginx/includes/security-headers.inc;
    }

    location ~* \.(sql|zip|tar|tar\.gz|tgz|bak|backup|env|ini|conf|inc|sh)$ {
        deny all;
        access_log off;
        log_not_found off;
    }

    # nginx auth_request → backend (Cookie + Authorization для Bearer)
    # client_max_body_size 0 обязателен: лимит проверяется и на subrequest,
    # иначе upload > server max (32m) даёт 413 в auth → клиенту 500.
    location = /auth-check {
        include /etc/nginx/includes/auth-subrequest.inc;
        proxy_pass http://backend:8080/api/auth/check;
    }

    location = /auth-check-admin {
        include /etc/nginx/includes/auth-subrequest.inc;
        proxy_pass http://backend:8080/api/auth/check-admin;
    }

    location = /auth-check-ops {
        include /etc/nginx/includes/auth-subrequest.inc;
        proxy_pass http://backend:8080/api/auth/check-ops;
    }

    location @login_redirect {
        return 302 /login?next=$request_uri;
    }

    location @forbidden_redirect {
        return 302 /;
    }

    location = /health {
        proxy_pass http://backend:8080/health;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    location = /api/health {
        proxy_pass http://backend:8080/api/health;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    location ^~ /api/auth/ {
        proxy_pass http://backend:8080;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    # Карта / статус индикатора: любой залогиненный (или Bearer)
    location = /api/events {
        auth_request /auth-check;
        proxy_pass http://backend:8080/api/events;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    location = /api/events/series {
        auth_request /auth-check;
        proxy_pass http://backend:8080/api/events/series;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    location = /api/system/status {
        auth_request /auth-check;
        proxy_pass http://backend:8080/api/system/status;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    # Личные шаблоны поиска: любой залогиненный (operator / administrator)
    location ^~ /api/me/search-templates {
        auth_request /auth-check;
        proxy_pass http://backend:8080;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    # Пакетный ingest: до 1 GiB (как backend MaxLogUploadSize)
    location = /api/ingest {
        client_max_body_size 0;
        client_body_timeout 1800s;
        client_body_buffer_size 1m;
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080/api/ingest;
        proxy_request_buffering off;
        proxy_buffering off;
        proxy_read_timeout 1800s;
        proxy_send_timeout 1800s;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    location = /api/ingest/stats {
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080/api/ingest/stats;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    location = /metrics {
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080/metrics;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    location = /api/geo-ranges/export {
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080/api/geo-ranges/export;
        proxy_read_timeout 600s;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    # GET list — admin на backend; POST/PUT — ops. Nginx пропускает ≥ops.
    location = /api/geo-ranges {
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080/api/geo-ranges;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    # Reputation mutations: ops на backend (auth_matrix). GET lists/feeds/catalog —
    # admin; nginx пропускает ≥ops, backend режет GET для чистого ops Bearer.
    location = /api/reputation/refresh {
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080;
        proxy_read_timeout 1800s;
        proxy_send_timeout 1800s;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    location = /api/reputation/feeds {
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    location ~ ^/api/reputation/(feeds|lists)/[^/]+$ {
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    # Lookup на карте: любой залогиненный (tierLogin в auth_matrix)
    location = /api/reputation/lookup {
        auth_request /auth-check;
        proxy_pass http://backend:8080;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    # Остальной /api/*: administrator или Bearer admin
    location /api/ {
        auth_request /auth-check-admin;
        proxy_pass http://backend:8080;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    # Streaming-upload: лимит ≈ MaxLogUploadSize (дефолт 1 GiB); 0 = без лимита nginx
    location = /upload-logs {
        client_max_body_size 0;
        client_body_timeout 1800s;
        client_body_buffer_size 1m;
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080/upload-logs;
        proxy_request_buffering off;
        proxy_buffering off;
        proxy_read_timeout 1800s;
        proxy_send_timeout 1800s;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    location = /upload-geo {
        client_max_body_size 0;
        client_body_timeout 1800s;
        client_body_buffer_size 1m;
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080/upload-geo;
        proxy_request_buffering off;
        proxy_buffering off;
        proxy_read_timeout 1800s;
        proxy_send_timeout 1800s;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    location = /upload-reputation {
        client_max_body_size 0;
        client_body_timeout 1800s;
        client_body_buffer_size 1m;
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080/upload-reputation;
        proxy_request_buffering off;
        proxy_buffering off;
        proxy_read_timeout 1800s;
        proxy_send_timeout 1800s;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    # Legacy HTML bookmarks → SPA routes.
    # IMPORTANT: do NOT redirect /index.html here — SPA try_files falls back to
    # /index.html internally; an external 302 to / would loop (ERR_TOO_MANY_REDIRECTS).
    location = /login.html { return 302 /login$is_args$args; }
    location = /system.html { return 302 /system$is_args$args; }
    location = /parser-test.html { return 302 /parser-test$is_args$args; }
    location = /parse-errors.html { return 302 /parse-errors$is_args$args; }
    location = /geo-missing.html { return 302 /geo-missing$is_args$args; }
    location = /geo-ranges.html { return 302 /geo-ranges$is_args$args; }
    location = /reputation.html { return 302 /reputation$is_args$args; }
    location = /users.html { return 302 /users$is_args$args; }
    location = /api-tokens.html { return 302 /api-tokens$is_args$args; }
    location = /change-password.html { return 302 /change-password$is_args$args; }

    # Hashed Vite assets
    location ^~ /assets/ {
        expires 7d;
        add_header Cache-Control "public, immutable";
        include /etc/nginx/includes/security-headers.inc;
        try_files $uri =404;
    }

    location ~* \.(css|js|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot|geojson|map)$ {
        expires 1h;
        add_header Cache-Control "public";
        include /etc/nginx/includes/security-headers.inc;
        try_files $uri =404;
    }

    # SPA shell (page auth is React Router; API still uses auth_request above).
    # Named fallback avoids matching any exact-location redirects on /index.html.
    location / {
        add_header Cache-Control "no-store" always;
        include /etc/nginx/includes/security-headers.inc;
        try_files $uri @spa;
    }

    location @spa {
        add_header Cache-Control "no-store" always;
        include /etc/nginx/includes/security-headers.inc;
        rewrite ^ /index.html last;
    }
