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
        alias /tmp/ga-config.js;
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
        access_log off;
        default_type application/json;
        add_header Cache-Control "no-store" always;
        return 200 '{"ok":true,"status":"live"}';
    }

    location = /live {
        proxy_pass http://backend:8080/live;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    location = /api/live {
        proxy_pass http://backend:8080/api/live;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    location = /ready {
        proxy_pass http://backend:8080/ready;
        include /etc/nginx/includes/proxy-headers.inc;
    }

    location = /api/ready {
        proxy_pass http://backend:8080/api/ready;
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

    # Dozzle (профиль compose dozzle): realtime logs + container actions.
    # Variable upstream + Docker DNS — frontend стартует и без контейнера dozzle.
    location = /dozzle {
        return 302 /dozzle/;
    }

    location ^~ /dozzle/ {
        auth_request /auth-check-admin;
        error_page 401 = @login_redirect;
        error_page 403 = @forbidden_redirect;

        resolver 127.0.0.11 valid=10s ipv6=off;
        set $dozzle_upstream dozzle:8080;
        proxy_pass http://$dozzle_upstream;

        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Cookie $http_cookie;
        # Без gzip от upstream — иначе sub_filter не видит HTML.
        proxy_set_header Accept-Encoding "";

        # SSE: без буфера; длинные таймауты для стрима логов.
        proxy_buffering off;
        proxy_cache off;
        chunked_transfer_encoding off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        gzip off;

        # Кнопка «назад в ГеоАтлас» (Dozzle не умеет кастомный chrome).
        # Инжект в </head>: у Dozzle <body> часто с class/атрибутами.
        sub_filter_once on;
        sub_filter_types text/html;
        sub_filter '</head>'
            '<style id="ga-dozzle-back-css">#ga-dozzle-back{position:fixed;z-index:2147483646;top:0;left:0;right:0;height:36px;display:flex;align-items:center;gap:8px;padding:0 14px;box-sizing:border-box;background:#14161a;border-bottom:1px solid #2c313a;font:500 13px/1.2 ui-sans-serif,system-ui,-apple-system,Segoe UI,sans-serif}#ga-dozzle-back a{color:#e8eaed;text-decoration:none}#ga-dozzle-back a:hover{color:#fff;text-decoration:underline}html.ga-dozzle-back-pad body,body.ga-dozzle-back-pad{padding-top:36px!important}</style><script id="ga-dozzle-back-js">(function(){function m(){if(document.getElementById("ga-dozzle-back"))return;var n=document.createElement("nav");n.id="ga-dozzle-back";n.setAttribute("aria-label","Навигация ГеоАтлас");var a=document.createElement("a");a.href="/system";a.textContent="\u2190 \u0413\u0435\u043e\u0410\u0442\u043b\u0430\u0441";n.appendChild(a);document.documentElement.classList.add("ga-dozzle-back-pad");if(document.body){document.body.classList.add("ga-dozzle-back-pad");document.body.insertBefore(n,document.body.firstChild)}}if(document.body)m();else document.addEventListener("DOMContentLoaded",m)})();</script></head>';
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

    # Hashed Vite assets. Nested deny: ^~ /assets/ skips regex locations.
    location ^~ /assets/ {
        location ~* \.map$ {
            return 404;
        }
        expires 7d;
        add_header Cache-Control "public, immutable";
        include /etc/nginx/includes/security-headers.inc;
        try_files $uri =404;
    }

    # Vite source maps are not published (build sourcemap off unless GA_SOURCEMAP=1).
    location ~* \.map$ {
        return 404;
    }

    location ~* \.(css|js|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot|geojson)$ {
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
