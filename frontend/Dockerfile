# syntax=docker/dockerfile:1
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json* ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY index.html vite.config.ts tsconfig.json tsconfig.node.json ./
COPY public ./public
COPY src ./src
RUN npm run build

FROM nginx:1.27-alpine
RUN mkdir -p /etc/nginx/includes
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx-app.inc /etc/nginx/includes/app.inc
COPY docker-entrypoint.sh /docker-entrypoint-nm.sh
RUN chmod +x /docker-entrypoint-nm.sh \
  && rm -f /etc/nginx/conf.d/default.conf
EXPOSE 80 443
ENTRYPOINT ["/bin/sh", "/docker-entrypoint-nm.sh"]
