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

    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    # Baseline CSP: MapLibre/CARTO tiles + blob workers; inline styles for React/uPlot.
    add_header Content-Security-Policy "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'self'; form-action 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https://*.basemaps.cartocdn.com https://*.cartocdn.com; font-src 'self' data:; connect-src 'self' https://basemaps.cartocdn.com https://*.basemaps.cartocdn.com https://*.cartocdn.com; worker-src 'self' blob:; child-src 'self' blob:;" always;

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
        internal;
        client_max_body_size 0;
        proxy_pass http://backend:8080/api/auth/check;
        proxy_pass_request_body off;
        proxy_set_header Content-Length "";
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location = /auth-check-admin {
        internal;
        client_max_body_size 0;
        proxy_pass http://backend:8080/api/auth/check-admin;
        proxy_pass_request_body off;
        proxy_set_header Content-Length "";
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Bearer≥ops / administrator (pipeline ingest, uploads)
    location = /auth-check-ops {
        internal;
        client_max_body_size 0;
        proxy_pass http://backend:8080/api/auth/check-ops;
        proxy_pass_request_body off;
        proxy_set_header Content-Length "";
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location @login_redirect {
        return 302 /login?next=$request_uri;
    }

    location @forbidden_redirect {
        return 302 /;
    }

    location = /health {
        proxy_pass http://backend:8080/health;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Публичные API (без auth_request)
    location = /api/health {
        proxy_pass http://backend:8080/api/health;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location ^~ /api/auth/ {
        proxy_pass http://backend:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Карта / статус индикатора: любой залогиненный (или Bearer)
    location = /api/events {
        auth_request /auth-check;
        proxy_pass http://backend:8080/api/events;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location = /api/events/series {
        auth_request /auth-check;
        proxy_pass http://backend:8080/api/events/series;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location = /api/system/status {
        auth_request /auth-check;
        proxy_pass http://backend:8080/api/system/status;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Личные шаблоны поиска: любой залогиненный (operator / administrator)
    location ^~ /api/me/search-templates {
        auth_request /auth-check;
        proxy_pass http://backend:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Пакетный ingest: до 1 GiB (как backend MaxLogUploadSize)
    location = /api/ingest {
        client_max_body_size 0;
        client_body_timeout 1800s;
        client_body_buffer_size 1m;

        auth_request /auth-check-ops;

        proxy_pass http://backend:8080/api/ingest;
        proxy_http_version 1.1;
        proxy_request_buffering off;
        proxy_buffering off;
        proxy_read_timeout 1800s;
        proxy_send_timeout 1800s;

        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location = /api/ingest/stats {
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080/api/ingest/stats;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Prometheus scrape (Bearer≥ops / administrator) — как ingest/stats.
    location = /metrics {
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080/metrics;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location = /api/geo-ranges/export {
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080/api/geo-ranges/export;
        proxy_http_version 1.1;
        proxy_read_timeout 600s;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # GET list — admin на backend; POST/PUT — ops. Nginx пропускает ≥ops.
    location = /api/geo-ranges {
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080/api/geo-ranges;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Reputation mutations: ops на backend (auth_matrix). GET lists/feeds/catalog —
    # admin; nginx пропускает ≥ops, backend режет GET для чистого ops Bearer.
    # Долгое обновление фидов (backend до 10 мин).
    location = /api/reputation/refresh {
        auth_request /auth-check-ops;

        proxy_pass http://backend:8080;
        proxy_http_version 1.1;
        proxy_read_timeout 1800s;
        proxy_send_timeout 1800s;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location = /api/reputation/feeds {
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location ~ ^/api/reputation/(feeds|lists)/[^/]+$ {
        auth_request /auth-check-ops;
        proxy_pass http://backend:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Lookup на карте: любой залогиненный (tierLogin в auth_matrix)
    location = /api/reputation/lookup {
        auth_request /auth-check;
        proxy_pass http://backend:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Остальной /api/*: administrator или Bearer admin
    location /api/ {
        auth_request /auth-check-admin;

        proxy_pass http://backend:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Streaming-upload: лимит ≈ MaxLogUploadSize (дефолт 1 GiB); 0 = без лимита nginx
    location = /upload-logs {
        client_max_body_size 0;
        client_body_timeout 1800s;
        client_body_buffer_size 1m;

        auth_request /auth-check-ops;

        proxy_pass http://backend:8080/upload-logs;
        proxy_http_version 1.1;
        proxy_request_buffering off;
        proxy_buffering off;
        proxy_read_timeout 1800s;
        proxy_send_timeout 1800s;

        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location = /upload-geo {
        client_max_body_size 0;
        client_body_timeout 1800s;
        client_body_buffer_size 1m;

        auth_request /auth-check-ops;

        proxy_pass http://backend:8080/upload-geo;
        proxy_http_version 1.1;
        proxy_request_buffering off;
        proxy_buffering off;
        proxy_read_timeout 1800s;
        proxy_send_timeout 1800s;

        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location = /upload-reputation {
        client_max_body_size 0;
        client_body_timeout 1800s;
        client_body_buffer_size 1m;

        auth_request /auth-check-ops;

        proxy_pass http://backend:8080/upload-reputation;
        proxy_http_version 1.1;
        proxy_request_buffering off;
        proxy_buffering off;
        proxy_read_timeout 1800s;
        proxy_send_timeout 1800s;

        proxy_set_header Host $http_host;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Cookie $http_cookie;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
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
        try_files $uri =404;
    }

    location ~* \.(css|js|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot|geojson|map)$ {
        expires 1h;
        add_header Cache-Control "public";
        try_files $uri =404;
    }

    # SPA shell (page auth is React Router; API still uses auth_request above).
    # Named fallback avoids matching any exact-location redirects on /index.html.
    location / {
        add_header Cache-Control "no-store" always;
        try_files $uri @spa;
    }

    location @spa {
        add_header Cache-Control "no-store" always;
        rewrite ^ /index.html last;
    }
