proxy_http_version 1.1;
proxy_set_header Host $http_host;
proxy_set_header X-Forwarded-Host $http_host;
proxy_set_header Cookie $http_cookie;
proxy_set_header Authorization $http_authorization;
# Double-submit CSRF для /upload-geo и прочих POST через nginx (client → backend).
proxy_set_header X-CSRF-Token $http_x_csrf_token;
proxy_set_header Origin $http_origin;
proxy_set_header Referer $http_referer;
proxy_set_header X-Real-IP $remote_addr;
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
proxy_set_header X-Forwarded-Proto $scheme;
