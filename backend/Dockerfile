# ---------- build stage ----------
# context: репозиторий (.) — нужен pkg/chconn и pkg/syslogngstats
FROM golang:1.26.11-alpine@sha256:ae52b5bc6ed045a9ed26ab03dc19327a802a9cc9aa8b27ed57f5c6b60579b0bb AS build

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
FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a
# uid/gid 101 = clickhouse/clickhouse-server — общий том clickhouse-backups
# (иначе auth *.tgz и prune падают с permission denied).
# libssl3/libcrypto3 и прочие пакеты: полный apk upgrade закрывает Trivy CRITICAL в Alpine.
RUN apk add --no-cache ca-certificates tzdata && \
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
