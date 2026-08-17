# context: репозиторий (.) — нужен pkg/chconn и pkg/syslogngstats
FROM golang:1.25-alpine AS build
WORKDIR /src

COPY pkg/chconn ./pkg/chconn
COPY pkg/syslogngstats ./pkg/syslogngstats
COPY stats-collector/go.mod stats-collector/go.sum ./stats-collector/

WORKDIR /src/stats-collector
RUN go mod download

COPY stats-collector/ .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/stats-collector .

FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 10001 app && \
    adduser -D -u 10001 -G app app
USER app
WORKDIR /app
COPY --from=build /out/stats-collector /app/stats-collector
ENTRYPOINT ["/app/stats-collector"]
