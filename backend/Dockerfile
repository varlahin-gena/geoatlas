# ---------- build stage ----------
# context: репозиторий (.) — нужен pkg/chconn
FROM golang:1.24-alpine AS build

WORKDIR /src

COPY pkg/chconn ./pkg/chconn
COPY backend/go.mod backend/go.sum ./backend/

WORKDIR /src/backend
RUN go mod download || true

COPY backend/ .

RUN set -e; \
    build() { \
        CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
            -o /out/network-monitor ./cmd/network-monitor; \
    }; \
    if ! build; then \
        echo ">>> Первая сборка не удалась, вероятно go.mod рассинхронизирован."; \
        echo ">>> Запускаю 'go mod tidy' (fallback) и повторяю сборку..."; \
        go mod tidy; \
        build; \
    fi

# ---------- runtime stage ----------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 app && \
    mkdir -p /app/data && chown -R app:app /app
USER app
WORKDIR /app
COPY --from=build /out/network-monitor /app/network-monitor
VOLUME ["/app/data"]
EXPOSE 8080 1514
ENTRYPOINT ["/app/network-monitor"]
