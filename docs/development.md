# Разработка из git

Прод на сервере ставят из **`geoatlas-X.Y.Z.tar.gz`**, не через `git pull`. Этот документ — для локальной разработки и CI.

Скрипты (`start.sh`, `deploy/…`) рассчитаны на **bash** (Linux, WSL или Git Bash на Windows).

## Быстрый подъём стека

Из корня репозитория:

```bash
./start.sh                 # секреты в .env при отсутствии, compose up --build
DO_BUILD=0 ./start.sh      # без пересборки образов
./stop.sh
```

Нужны Docker Engine 24+ и compose plugin. Профили по умолчанию подтянет `.env` / `start.sh` (часто `syslog,stats` или как в `.env.example`).

Для ослабленной auth в dev: `GA_ALLOW_INSECURE=1` (см. [configuration.md](configuration.md)).

## Go workspace

[`go.work`](../go.work) включает:

- `backend` — модуль `geoatlas`, вход `backend/cmd/geoatlas`
- `pkg/chconn`, `pkg/syslogngstats`
- `stats-collector`

```bash
cd backend
go test ./...
# CI также гоняет race; локально по желанию: go test -race ./...
```

Схема ClickHouse — SoT в `migrate.Ensure*` (не править руками сгенерированный `clickhouse/init.sql` как источник истины):

```bash
cd backend
go generate ./internal/adapter/clickhouse/migrate/...
go generate ./internal/model/...   # action vocab → SQL / backfill helpers
```

## Frontend

```bash
cd frontend
npm install
npm run dev          # http://127.0.0.1:5173, proxy /api → :8080
npm test
npm run build
npm run typecheck
```

Подробнее: [`frontend/README.md`](../frontend/README.md).

## OpenAPI и типы

1. Правите [`openapi.yaml`](../openapi.yaml), при смене контракта поднимаете `info.version`.
2. Цитата `OpenAPI **N**` в [README](../README.md) и [ui.md](ui.md) (CI: `scripts/check-release-contract.sh`).
3. Типы SPA:

```bash
cd frontend
npm run openapi:types    # regen src/api/openapi.d.ts
npm run openapi:check    # drift vs yaml
```

Релизный процесс осей версий: [RELEASING.md](../RELEASING.md).

## Полезные проверки

```bash
bash scripts/check-release-contract.sh
bash scripts/shellcheck.sh          # если установлен shellcheck
bash scripts/check-module-identity.sh
```

Дерево каталогов: [repo-layout.md](repo-layout.md). Архитектура рантайма: [architecture.md](architecture.md).
