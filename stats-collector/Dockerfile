# context: репозиторий (.) — нужен pkg/chconn и pkg/syslogngstats
FROM golang:1.26.9-alpine@sha256:3082400e369fa24d5fc60bca20edab3f6d604e0c5a690ec66b295eff4dd87ade AS build
WORKDIR /src

COPY pkg/chconn ./pkg/chconn
COPY pkg/syslogngstats ./pkg/syslogngstats
COPY stats-collector/go.mod stats-collector/go.sum ./stats-collector/

WORKDIR /src/stats-collector
RUN go mod download

COPY stats-collector/ .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/stats-collector .

FROM alpine:3.23@sha256:85fe1e81d6758c208f3e1eed4338a1997e19d4be002d4dd32d3100c9a8c010a0
# libssl3/libcrypto3 и прочие пакеты: полный apk upgrade закрывает Trivy CRITICAL в Alpine.
RUN apk add --no-cache ca-certificates=20260909-r0 tzdata=2026e-r0 && \
    apk upgrade --no-cache && \
    addgroup -g 10001 app && \
    adduser -D -u 10001 -G app app
USER app
WORKDIR /app
COPY --from=build /out/stats-collector /app/stats-collector
LABEL org.opencontainers.image.title="geoatlas-stats-collector" \
      org.opencontainers.image.source="https://github.com/varlahin-gena/geoatlas"
ENTRYPOINT ["/app/stats-collector"]
