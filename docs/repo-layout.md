# Структура репозитория

```
geoatlas/
├── go.work                           # workspace: backend + stats-collector + pkg/chconn + pkg/syslogngstats
├── backend/                          # Go: API, парсеры, geoip, ingest
│   ├── cmd/geoatlas/main.go          # единственный вход (модуль geoatlas)
│   ├── Dockerfile
│   └── internal/
│       ├── adapter/
│       │   ├── httpapi/              # HTTP delivery (handlers, middleware, cookies/CSRF)
│       │   ├── clickhouse/           # pool/retention/maintenance + domain stores
│       │   │   ├── ingeststore/      # INSERT traffic_logs / parse_errors
│       │   │   ├── perrorstore/      # list/delete parse_errors
│       │   │   ├── trafficstore/     # map/events scans
│       │   │   ├── anomalystore/     # anomaly detection SQL (port scan, byte_surge, …)
│       │   │   ├── geostore/         # GeoIP ranges + ReloadableGeoIndex
│       │   │   ├── repstore/         # reputation ranges + index
│       │   │   ├── sysstore/         # system metrics
│       │   │   ├── backupstore/      # BACKUP/RESTORE
│       │   │   ├── aggstate/         # Prefer* / статус edges agg
│       │   │   ├── migrate/          # Ensure* / DDL / backfill (SoT схемы)
│       │   │   ├── query/            # Scan* / settings
│       │   │   └── sqlclause/        # общие SQL-выражения
│       │   ├── geojob/               # сериализация reload GeoIP + backfill
│       │   ├── parseradapter/        # parser port
│       │   ├── geoipcodec/           # GeoIP CSV/CIDR helpers
│       │   ├── bootstrapadapter/     # Ensure*/Backfill* для usecase/bootstrap
│       │   ├── retentionfile/        # JSON-store TTL (`retention.json`)
│       │   └── systemlive/           # live ingest / syslog-ng stats / profile adapters
│       ├── usecase/                  # application use cases + ports (bootstrap, retention, anomaly, …)
│       ├── auth/                     # users / sessions / roles
│       ├── config/                   # конфигурация из env
│       ├── geoip/                    # импорт CSV и in-memory индекс
│       ├── ingest/                   # syslog TCP, очередь, batch INSERT
│       ├── installprofile/           # чтение install-profile.json
│       ├── logging/                  # slog setup
│       ├── model/                    # доменные структуры; action vocab SoT (go generate → backfill sh + schema SQL)
│       ├── mapagg/                   # агрегация рёбер/узлов для карты
│       └── parser/                   # парсеры вендоров
├── pkg/
│   ├── chconn/                       # общий ClickHouse connect
│   └── syslogngstats/                # CSV/Prometheus parser stats-exporter
├── clickhouse/
│   ├── config.d/override.xml
│   ├── config.d/backups.xml          # disk `backups` для BACKUP/RESTORE
│   ├── users.d/query_limits.xml
│   ├── init.sql                      # generated: cold bootstrap базовых таблиц (SoT — migrate.Ensure*)
│   ├── migrate_*.sql                 # generated: ops-fallback (не runtime SoT)
│   ├── backfill_edges_agg.sh         # то же для BLOCKED=…
│   └── reset_data.sql / reset_data.sh
├── scripts/
│   ├── check-release-contract.sh     # CI: VERSION / CHANGELOG / OpenAPI
│   ├── check-module-identity.sh      # CI: один модуль geoatlas, cmd/geoatlas
│   ├── ci-docker-build.sh            # CI: docker build backend / frontend / stats
│   ├── ci-govulncheck.sh             # CI: govulncheck по Go-модулям
│   ├── ci-sbom.sh                    # релиз: CycloneDX + SPDX из geoatlas-*.tar.gz
│   ├── pack-release.sh               # dist/geoatlas-X.Y.Z.tar.gz (+ sha256); --with-images
│   ├── export-images.sh              # docker build/pull + save → images/ для офлайн-пакета
│   ├── test-apply-package.sh         # CI: pack + наложение пакета
│   ├── test-compose-stop-env.sh      # CI: заглушки для compose down без секретов
│   ├── shellcheck.sh                 # CI: start/stop/update, scripts/, deploy/
│   ├── backup-clickhouse.sh          # native BACKUP → том clickhouse-backups
│   └── restore-clickhouse.sh         # RESTORE + optional auth tarball
├── docs/                             # установка, ops, архитектура, GeoIP, UI/API
├── certs/                            # PEM (fullchain/privkey); README + .gitkeep; ключи в .gitignore
├── deploy/
│   ├── uninstall.sh
│   ├── common/                       # detect_resources, select_modules, apply_package, ui, compose.sh, …
│   ├── ubuntu/
│   └── oracle_linux/
├── docker-compose.yml
├── docker-compose.https.yml          # публикация :443 при HTTPS
├── frontend/                         # React+Vite SPA → nginx image
│   ├── package.json / package-lock.json / vite.config.ts
│   ├── Dockerfile                    # multi-stage: npm ci + vite build → nginx
│   ├── index.html                    # Vite entry (#root)
│   ├── public/                       # favicon, logo, countries.geojson
│   ├── src/                          # React pages (Map, Anomalies, Investigate, System, admin…)
│   ├── nginx.conf
│   ├── nginx-app.inc                 # SPA try_files @spa + auth_request API
│   ├── docker-entrypoint.sh          # config.js + HTTP/HTTPS default.conf
├── stats-collector/                  # Go: системные метрики
│   ├── main.go
│   └── internal/
│       ├── config/
│       └── collector/
├── openapi.yaml                      # контракт HTTP API (OpenAPI 1.15.0)
├── LICENSE / NOTICE                  # Apache License 2.0
├── SECURITY.md                       # как сообщать об уязвимостях
├── VERSION / CHANGELOG.md / RELEASING.md
├── .github/workflows/ci.yml          # тесты + docker build образов compose
├── .github/workflows/release-tag.yml # тег v*: Release + tar.gz + SBOM
├── start.sh / stop.sh / update.sh    # update.sh — наложение geoatlas-*.tar.gz
├── syslog-ng/Dockerfile              # 4.12.0 + apt upgrade + Python venv patches
├── syslog-ng.conf
└── syslog-ng.d/                      # 00-keep.conf + zz_profile.conf.example; zz_profile.conf генерируется
```

Локальная разработка: [development.md](development.md).
