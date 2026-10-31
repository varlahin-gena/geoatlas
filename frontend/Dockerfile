# syntax=docker/dockerfile:1
FROM node:22-alpine@sha256:fe4f6d74e20370acb04ad889077d14d0f64a9c8ea6a42bc60cddf1e844699914 AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY index.html vite.config.ts tsconfig.json tsconfig.node.json ./
COPY public ./public
COPY src ./src
RUN npm run build

FROM nginx:1.30.6-alpine@sha256:0e52bd054126cdc040b1276c0af347aeb02a41b9cfe305d6094767ac30e97949
# apk upgrade: Trivy CRITICAL в alpine 3.24 (не только OpenSSL).
# nginx 1.30.6: CVE-2026-15176 / 93479 / 97205 (+ medium) fixed vs 1.30.5.
RUN apk upgrade --no-cache && \
    mkdir -p /etc/nginx/includes
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx-app.inc /etc/nginx/includes/app.inc
COPY nginx-security-headers-common.inc /etc/nginx/includes/security-headers-common.inc
COPY nginx-security-headers-ui.inc /etc/nginx/includes/security-headers-ui.inc
COPY nginx-security-headers-hsts.inc /etc/nginx/includes/security-headers-hsts.inc
COPY nginx-security-headers-edge.inc /etc/nginx/includes/security-headers-edge.inc
COPY nginx-security-headers.inc /etc/nginx/includes/security-headers.inc
COPY nginx-proxy-headers.inc /etc/nginx/includes/proxy-headers.inc
COPY nginx-auth-subrequest.inc /etc/nginx/includes/auth-subrequest.inc
COPY nginx-server-tokens.conf /etc/nginx/conf.d/00-server-tokens.conf
COPY docker-entrypoint.sh /docker-entrypoint-nm.sh
RUN chmod +x /docker-entrypoint-nm.sh \
  && rm -f /etc/nginx/conf.d/default.conf
EXPOSE 80 443
LABEL org.opencontainers.image.title="geoatlas-frontend" \
      org.opencontainers.image.source="https://github.com/varlahin-gena/geoatlas"
HEALTHCHECK --interval=10s --timeout=5s --start-period=15s --retries=12 \
    CMD wget -q -O /dev/null http://127.0.0.1/health || exit 1
ENTRYPOINT ["/bin/sh", "/docker-entrypoint-nm.sh"]
