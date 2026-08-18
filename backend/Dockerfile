# ---------- build stage ----------
# context: репозиторий (.) — нужен pkg/chconn и pkg/syslogngstats
FROM golang:1.25.13-alpine AS build

WORKDIR /src

COPY pkg/chconn ./pkg/chconn
COPY pkg/syslogngstats ./pkg/syslogngstats
COPY backend/go.mod backend/go.sum ./backend/

WORKDIR /src/backend
RUN go mod download

COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/network-monitor ./cmd/network-monitor

# ---------- runtime stage ----------
FROM alpine:3.23
# uid/gid 101 = clickhouse/clickhouse-server — общий том clickhouse-backups
# (иначе auth *.tgz и prune падают с permission denied).
RUN apk add --no-cache ca-certificates=20260611-r0 tzdata=2026c-r0 && \
    addgroup -g 101 app && \
    adduser -D -u 101 -G app app && \
    mkdir -p /app/data && chown -R app:app /app
USER app
WORKDIR /app
COPY --from=build /out/network-monitor /app/network-monitor
EXPOSE 8080 1514
ENTRYPOINT ["/app/network-monitor"]
