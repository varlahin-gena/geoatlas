# syntax=docker/dockerfile:1
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY index.html vite.config.ts tsconfig.json tsconfig.node.json ./
COPY public ./public
COPY src ./src
RUN npm run build

FROM nginx:1.30.4-alpine
# apk upgrade: Trivy CRITICAL в alpine 3.24 (не только OpenSSL).
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
