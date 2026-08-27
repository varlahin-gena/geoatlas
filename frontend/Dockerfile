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
# libssl3/libcrypto3: apk upgrade закрывает Trivy OpenSSL CVE (3.5.7 → 3.5.8+).
RUN apk upgrade --no-cache libssl3 libcrypto3 && \
    mkdir -p /etc/nginx/includes
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx-app.inc /etc/nginx/includes/app.inc
COPY nginx-security-headers.inc /etc/nginx/includes/security-headers.inc
COPY nginx-proxy-headers.inc /etc/nginx/includes/proxy-headers.inc
COPY nginx-auth-subrequest.inc /etc/nginx/includes/auth-subrequest.inc
COPY docker-entrypoint.sh /docker-entrypoint-nm.sh
RUN chmod +x /docker-entrypoint-nm.sh \
  && rm -f /etc/nginx/conf.d/default.conf
EXPOSE 80 443
ENTRYPOINT ["/bin/sh", "/docker-entrypoint-nm.sh"]
