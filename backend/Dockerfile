# ---------- build stage ----------
# context: репозиторий (.) — нужен pkg/chconn и pkg/syslogngstats
FROM golang:1.25.14-alpine AS build

WORKDIR /src

COPY pkg/chconn ./pkg/chconn
COPY pkg/syslogngstats ./pkg/syslogngstats
COPY backend/go.mod backend/go.sum ./backend/

WORKDIR /src/backend
RUN go mod download

COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/geoatlas ./cmd/geoatlas

# ---------- runtime stage ----------
FROM alpine:3.23
# uid/gid 101 = clickhouse/clickhouse-server — общий том clickhouse-backups
# (иначе auth *.tgz и prune падают с permission denied).
# libssl3/libcrypto3 и прочие пакеты: полный apk upgrade закрывает Trivy CRITICAL в Alpine.
RUN apk add --no-cache ca-certificates=20260611-r0 tzdata=2026c-r0 && \
    apk upgrade --no-cache && \
    addgroup -g 101 app && \
    adduser -D -u 101 -G app app && \
    mkdir -p /app/data && chown -R app:app /app
USER app
WORKDIR /app
COPY --from=build /out/geoatlas /app/geoatlas
EXPOSE 8080 1514
LABEL org.opencontainers.image.title="geoatlas-backend" \
      org.opencontainers.image.source="https://github.com/varlahin-gena/geoatlas"
HEALTHCHECK --interval=10s --timeout=5s --start-period=20s --retries=12 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/live || exit 1
ENTRYPOINT ["/app/geoatlas"]
