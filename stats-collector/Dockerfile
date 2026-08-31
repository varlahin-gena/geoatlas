# context: репозиторий (.) — нужен pkg/chconn и pkg/syslogngstats
FROM golang:1.25.14-alpine AS build
WORKDIR /src

COPY pkg/chconn ./pkg/chconn
COPY pkg/syslogngstats ./pkg/syslogngstats
COPY stats-collector/go.mod stats-collector/go.sum ./stats-collector/

WORKDIR /src/stats-collector
RUN go mod download

COPY stats-collector/ .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/stats-collector .

FROM alpine:3.23
# libssl3/libcrypto3: apk upgrade закрывает Trivy OpenSSL CVE (3.5.7 → 3.5.8+).
RUN apk add --no-cache ca-certificates=20260611-r0 tzdata=2026c-r0 && \
    apk upgrade --no-cache libssl3 libcrypto3 && \
    addgroup -g 10001 app && \
    adduser -D -u 10001 -G app app
USER app
WORKDIR /app
COPY --from=build /out/stats-collector /app/stats-collector
ENTRYPOINT ["/app/stats-collector"]
