# context: репозиторий (.) — нужен pkg/chconn и pkg/syslogngstats
FROM golang:1.26.10-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308 AS build
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
RUN apk add --no-cache ca-certificates=20261020-r0 tzdata=2026e-r0 && \
    apk upgrade --no-cache && \
    addgroup -g 10001 app && \
    adduser -D -u 10001 -G app app
USER app
WORKDIR /app
COPY --from=build /out/stats-collector /app/stats-collector
LABEL org.opencontainers.image.title="geoatlas-stats-collector" \
      org.opencontainers.image.source="https://github.com/varlahin-gena/geoatlas"
ENTRYPOINT ["/app/stats-collector"]
