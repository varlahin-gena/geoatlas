# context: репозиторий (.) — нужен pkg/chconn и pkg/syslogngstats
FROM golang:1.26.11-alpine@sha256:ae52b5bc6ed045a9ed26ab03dc19327a802a9cc9aa8b27ed57f5c6b60579b0bb AS build
WORKDIR /src

COPY pkg/chconn ./pkg/chconn
COPY pkg/syslogngstats ./pkg/syslogngstats
COPY stats-collector/go.mod stats-collector/go.sum ./stats-collector/

WORKDIR /src/stats-collector
RUN go mod download

COPY stats-collector/ .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/stats-collector .

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a
# libssl3/libcrypto3 и прочие пакеты: полный apk upgrade закрывает Trivy CRITICAL в Alpine.
RUN apk add --no-cache ca-certificates tzdata && \
    apk upgrade --no-cache && \
    addgroup -g 10001 app && \
    adduser -D -u 10001 -G app app
USER app
WORKDIR /app
COPY --from=build /out/stats-collector /app/stats-collector
LABEL org.opencontainers.image.title="geoatlas-stats-collector" \
      org.opencontainers.image.source="https://github.com/varlahin-gena/geoatlas"
ENTRYPOINT ["/app/stats-collector"]
