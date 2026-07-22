# context: репозиторий (.) — нужен pkg/chconn
FROM golang:1.24-alpine AS build
WORKDIR /src

COPY pkg/chconn ./pkg/chconn
COPY stats-collector/go.mod stats-collector/go.sum ./stats-collector/

WORKDIR /src/stats-collector
RUN go mod download

COPY stats-collector/ .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/stats-collector .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/stats-collector /app/stats-collector
ENTRYPOINT ["/app/stats-collector"]
