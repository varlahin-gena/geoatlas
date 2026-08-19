# Changelog

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/).
Версии — [SemVer](https://semver.org/lang/ru/).

## [Unreleased]

### Added
- **Anomaly Engine v1**: журнал аномалий на карте — `port_scan`, `horizontal_scan`, `blocked_surge`, `new_country_dst`, `rep_new_dst`; `GET /api/anomalies`, `/summary`, `/status`, `POST /ack`; фоновый скан каждые 5 мин; полоска и панель на карте. `ANOMALY_ENABLED` (по умолчанию true). OpenAPI **1.12.0**.
- CI: сборка Docker-образов backend / frontend / stats-collector (`scripts/ci-docker-build.sh`); frontend — дым `/health` после `docker build`
- CI: Trivy image scan для `balabit/syslog-ng:4.11.0` (вход `:514`, тот же тег что в compose)
- CI: `govulncheck` по backend / stats-collector / pkg/chconn / pkg/syslogngstats (`scripts/ci-govulncheck.sh`)
- Релиз: SBOM установочного пакета — CycloneDX + SPDX (`scripts/ci-sbom.sh`, Syft v1.51.0) в Assets вместе с `geoatlas-X.Y.Z.tar.gz`

### Fixed
- Карта: легенда «Статус трафика» не перекрывает панель аномалий — оба блока в правой колонке
- CI: frontend image smoke — nginx стартует без compose DNS (`--add-host backend:127.0.0.1`); раньше `host not found in upstream "backend"`
- CI / образы: Go **1.25.13** (stdlib `net/url`, `crypto/tls`, `net/http`, `encoding/asn1`); `setup-go` `check-latest: true`, `golang:1.25.13-alpine`

### Changed
- CI: GitHub Actions запинены по commit SHA (`checkout`, `setup-go`, `setup-node`, CodeQL, golangci-lint, buildx) — как Trivy/Hadolint/Syft
- CI: `concurrency: cancel-in-progress`; на PR Playwright / ClickHouse / fuzz / docker — только если изменились их деревья (`dorny/paths-filter`); push в `main`/`develop` гоняет все job'ы
- CI: Hadolint валит job на правилах severity `error` (`failure-threshold: error`); warning/info по-прежнему в Security tab
- Установка и обновление на сервере только из локального `geoatlas-X.Y.Z.tar.gz`: нет `git clone` / `git pull`, установщик не ставит пакет `git`, нет `--download` и curl-установки одним скриптом с GitHub. `install-meta.json` берёт версию из пакета, не из git.
- ClickHouse image: `clickhouse/clickhouse-server:25.8.29.51` → `25.8.30.16` в compose, image scan и CI integration, чтобы подтянуть Ubuntu package security fixes для контейнерного образа.

## [1.4.2] — 2026-08-18

Патч-релиз: старт из `/opt/network_monitor` не создаёт пустые тома и не конфликтует с контейнерами проекта `network-monitor`.

### Fixed
- `nm_compose`: если контейнеры `clickhouse` / `nm-volume-perms` уже принадлежат проекту `network-monitor`, а каталог — `/opt/network_monitor`, берём `COMPOSE_PROJECT_NAME` с существующих контейнеров и при необходимости пишем его в `.env` (иначе `up` создаёт пустые тома и падает на занятом имени).

### Notes
- OpenAPI API doc version: **1.11.0** (без изменений)
- Продуктовая версия: **1.4.2**
- После обновления: скачать `geoatlas-1.4.2.tar.gz` (+ `.sha256`) с GitHub Release и `sudo /opt/network-monitor/update.sh ./geoatlas-1.4.2.tar.gz` (если каталог другой: `--project-dir /opt/network_monitor`). Health: `/api/ready`.
- Не удаляйте тома `network-monitor_*`. Пустые `network_monitor_*`, если они уже появились при неудачном `up`, можно убрать после того, как стек снова работает на `network-monitor_*`.
- 1 коммит с `v1.4.1`

## [1.4.1] — 2026-08-18

Патч-релиз: `update.sh` не падает на `docker compose down`, если в `.env` пустой `CLICKHOUSE_PASSWORD`.

### Fixed
- `update.sh` / `stop.sh`: `docker compose down` больше не падает на пустом `CLICKHOUSE_PASSWORD` (и других `${VAR:?}`). Compose всё равно интерполирует YAML при остановке; для `down`/`stop` подставляются заглушки только в процессе, `.env` не меняется. `./start.sh` по-прежнему fail-closed. `update.sh` останавливает стек через `compose.sh` из пакета (установленный `stop.sh` ещё старый) и не прерывает наложение, если стоп неполный.

### Notes
- OpenAPI API doc version: **1.11.0** (без изменений)
- Продуктовая версия: **1.4.1**
- После обновления: скачать `geoatlas-1.4.1.tar.gz` (+ `.sha256`) с GitHub Release и `sudo /opt/network-monitor/update.sh ./geoatlas-1.4.1.tar.gz` (если каталог другой: `--project-dir /opt/network_monitor`). Health: `/api/ready`.
- Не экспортируйте фиктивный `CLICKHOUSE_PASSWORD` на весь `update.sh` — `./start.sh` подхватит его вместо `.env`.
- 1 коммит с `v1.4.0`

## [1.4.0] — 2026-08-18

Минорный релиз: обновление из установочного пакета без `git pull`, OpenAPI **1.11.0**, hardening и bootstrap схемы ClickHouse из `EnsureBaseSchema`.

### Added
- Обновление из локального пакета: `geoatlas-1.4.0.tar.gz` в GitHub Release (один архив для Ubuntu и Oracle Linux / RHEL), `./update.sh` на сервере (без `git pull`); установщик умеет `NM_INSTALL_PACKAGE`
- Compact GeoIP snapshot на `/app/data/geo_index.snap`: карта после рестарта не ждёт полный скан `geo_ranges`; сверка stamp с ClickHouse, файл не входит в auth-tarball
- Лицензия **Apache License 2.0** (`LICENSE`, `NOTICE`); продукт бесплатный, доработки через GitHub Issues
- Политика уязвимостей: [`SECURITY.md`](SECURITY.md) (приватный GitHub advisory, ответ за 5 рабочих дней)
- Контракт релизов: `scripts/check-release-contract.sh` (CI) — VERSION, CHANGELOG Notes и OpenAPI не разъезжаются
- CI: путь «лог → карта» (`TestIntegrationMapPathLogToEvents`: upload-geo + parser samples → `/api/events`)
- CI: `scripts/shellcheck.sh` (`-S error`) на `start.sh` / `stop.sh` / `update.sh` / `scripts/` / `deploy/`
- Карта: period / group / filter / q / country в query string; 401 вне `/api/auth` → `/login?next=`
- Playwright: фикстура карты + URL-параметры + редирект по 401
- `bootstrap.RunStartup` unit-тесты; `trafficstore` — geo-path не падает в raw scan
- Playwright GeoIP wizard (Escape) + в CI `vite preview`, не только `vite dev`

### Changed
- Документация: один `geoatlas-X.Y.Z.tar.gz` для Ubuntu и Oracle Linux; `update.sh` сохраняет runtime-файлы; CI на тег сам клеит архив к GitHub Release
- Образы: `nginx:1.27-alpine` → `1.30.4-alpine` (stable; 1.27 EOL), runtime/`volume-perms` `alpine:3.20` → `3.23` (3.20 EOL). Без `apk upgrade`.
- Toolchain: Go 1.24 → 1.25 (`go.mod` / `go.work` / CI `1.25.x` / `golang:1.25-alpine`); `golang.org/x/crypto` v0.31.0 → v0.55.0 (bcrypt). `clickhouse-go` пока 2.17.1.
- ClickHouse LTS patch: `clickhouse/clickhouse-server:25.8.28.1` → `25.8.29.51` в compose, CI integration и image scan.
- ClickHouse Go stack: `clickhouse-go` `v2.17.1` → `v2.48.0` и совместимые indirect зависимости (`ch-go`, `orb`, `lz4`, `otel`, `compress`).
- Schema SoT: базовые таблицы в `migrate.EnsureBaseSchema`; `clickhouse/init.sql` и `migrate_*.sql` генерируются из того же DDL (`go generate ./internal/adapter/clickhouse/migrate/...`). Пустой том без init.sql больше не оставляет `traffic_logs` несозданным.
- JSON control plane: запись users/tokens/retention/feeds/templates/backup-schedule через общий `fileatomic` (tmp + fsync + rename)
- HTTP API doc **1.5.0 → 1.11.0**:
  - **1.7.0**: `GET /metrics` (Prometheus, Bearer≥ops), `POST /api/auth/logout-all` в спецификации, ingest SLO в `/api/system/stats`
  - **1.8.0**: `/api/events` — серверные `filter` / `limit` / `country` / `q`; live `pipeline.syslogng` в `/api/system/stats`
  - **1.9.0**: `/api/events` — AST-поиск `q` и фильтр репутации `rep_cat`/`rep_list`/`rep_side` до LIMIT; `reputation_facets`; схема `SystemStats`; SPA-типы из `openapi-typescript`
  - **1.10.0**: `GET /live` и `GET /ready` (+ `/api/*`); `/health` и `/api/health` — liveness без ClickHouse (503 по CH только на `/ready`)
  - **1.11.0**: схема `AuthUser` на login/me/geo-wizard-dismiss; nested `SystemStats` / `SystemStatus` / `SystemVersion`; SPA `apiGet`/`apiPost` по generated paths
- SPA: `uplot` 1.6.30 → 1.6.32

### Security
- Reputation feeds: SSRF dial pin — `SecureHTTPClient` connects only via `tcp4` to public IPv4 resolved at dial time (DNS rebinding / AAAA residual)
- Ingest: `INGEST_SHARED_SECRET` applies to `:1514` markers only; HTTP upload (`transport=http`) exempt after API auth
- ClickHouse `default` user networks: loopback + RFC1918 (not `::/0`) — defense if 8123/9000 ever published
- Bootstrap admin password: written to `.admin_password_once` (mode 600), not printed to stdout / full-auto dialog
- Password policy: минимум 10 символов, буква+цифра, blocklist common; UI + `admin_auth.sh` синхронизированы
- CSP: `style-src-elem 'self'` + `style-src-attr 'unsafe-inline'` (React style={}), без inline `<style>` injection
- Reputation feeds: IPv4-only SSRF guards (block private/metadata; safe redirects) on AddFeed и fetch
- Ingest `:1514`: `INGEST_SHARED_SECRET` в маркере `@@nm/{udp|tcp}/<token>/@@` + peer allowlist `INGEST_ALLOW_FROM` (дефолт `syslog-ng`); генерирует `./start.sh`
- Публичный `/ready` без queue/drops/`last_error` (детали — `/api/ingest/stats`)
- CSRF: Origin с literal IP только при совпадении с Host/X-Forwarded-Host
- Login throttle: `X-Real-IP` только от trusted proxies (`NM_TRUSTED_PROXIES`, дефолт `frontend`)
- Seed только **admin**: установщик спрашивает пароль; full-auto / `./start.sh` без TTY берут `AUTH_ADMIN_PASSWORD` или генерируют одноразовый. Operator с завода не создаётся (UI `/users`). Нет литерала `admin`/`admin`.
- nginx: CSP/XFO/HSTS на HTML-шелл (include security-headers в location с Cache-Control)
- Full-auto больше не выключает host firewall: allowlist UI + `:514`; `NM_DISABLE_HOST_FIREWALL=1` — старое поведение; `NM_SYSLOG_ALLOW_FROM` сужает syslog
- `CLICKHOUSE_PASSWORD` генерируется в `./start.sh` / `detect_resources.sh`; compose fail-closed; default user `from_env`
- Мастер GeoIP: abort polling при закрытии, `apiFetchRaw` (401 → session expired), Escape / focus trap / клик по backdrop
- nginx не отдаёт `*.map` (в т.ч. `/assets/`); Vite sourcemap только при `NM_SOURCEMAP=1`
- Dependabot: ignore minor/major на экосистему (группа `patch` иначе открывает отдельные minor); Docker без minor/major; Actions без major; Trivy fs в CI + weekly scan образов; GitHub Release из CHANGELOG на тег `v*`

### Fixed
- CI: `aquasecurity/trivy-action` pinned на v0.36.0 (тег `0.28.0` снят после инцидента 2026); `setup-node@v5`
- CI: `GOWORK=off` — bump `go 1.25` в одном модуле не валит остальные через `go.work`
- Integration map-path: `stubGeoJobs` больше не паникует на нулевом счётчике при `POST /upload-geo`
- SPA: 401 вне `/api/auth/*` сбрасывает сессию и ведёт на `/login?next=`
- Карта: period/group/filter/q/country в query string
- Глобус: camera в ref, deck-слои только при смене cull-ячейки 0.25° (не каждый кадр auto-rotate)
- Frontend `GET /health` — локальный 200 без прокси на backend; при HTTPS `:80` `/health` не уходит в 301

### Notes
- OpenAPI API doc version: **1.11.0**
- Продуктовая версия: **1.4.0**
- После обновления: скачать `geoatlas-1.4.0.tar.gz` (+ `.sha256`) с GitHub Release и `sudo /opt/network-monitor/update.sh ./geoatlas-1.4.0.tar.gz` (тома Docker, `.env`, certs на месте). Health: `/api/ready`.
- В `.env` появится `CLICKHOUSE_PASSWORD`, если его ещё не было; ClickHouse перезапустится с паролем (данные на месте). `clickhouse-client` внутри контейнера: `sh -c 'clickhouse-client --password "$CLICKHOUSE_PASSWORD" -q "SELECT 1"'`.
- Пустой том ClickHouse: базовые таблицы создаёт `migrate.EnsureBaseSchema` (`init.sql` generated).
- Сгенерированный admin-пароль: `.admin_password_once` (не stdout); удалите после входа.
- Пример ключей `.env` без секретов: [`.env.example`](.env.example)
- 64 коммита с `v1.3.1`

## [1.3.1] — 2026-08-12

Патч-релиз: экономнее память GeoIP, лучше наблюдаемость индекса и чище empty-state карты.

### Changed
- **GeoIP index in RAM**: compact snapshot со словарями строк вместо хранения полного `[]GeoRange`; потоковая сборка snapshot из ClickHouse и CSV upload-path без лишней нормализации/копий
- `/system`: backend health показывает размер GeoIP-индекса в памяти и число ranges; предупреждения в GeoIP upload flow смягчены с учётом нового memory profile

### Fixed
- Карта: баннер пустой GeoIP больше не дублирует центральный `no_geo` overlay, перечитывает статус GeoIP при возврате на страницу и не конфликтует с легендой/подсказками сверху

### Notes
- OpenAPI API doc version: **1.5.0** (без изменений)
- Продуктовая версия: **1.3.1**
- После обновления: `git fetch && git checkout v1.3.1` (или `main`), `./start.sh`
- 4 коммита с `v1.3.0`

## [1.3.0] — 2026-08-12

Минорный релиз: first-run мастер GeoIP, hardening SPA/auth и русскоязычные сообщения установщика.

### Added
- **First-run GeoIP wizard** на карте: шаги «почему пусто / загрузка / готово»; CSV с `dry_run` preview, curl-snippet для больших файлов, ссылка на `/geo-missing`; polling до появления диапазонов; dismiss в `users.json` (`POST /api/auth/geo-wizard-dismiss`) и баннер/кнопка в сайдбаре
- `index_ready` в `GET /api/geo-ranges` (пока async Reload индекса при старте)
- Empty overlay карты различает «нет событий» / «нет координат (GeoIP)» / «всё отфильтровано»

### Changed
- Карта/System: MapLibre вынесен в `useMapLibreController`, сайдбар/топбар на grouped props; System на `AdminLayout` + scoped CSS; общий `usePolling`
- Роли UI: `deriveIsAdmin` / `roles.ts` вместо хрупкого сравнения строк
- Установщик и uninstall (Ubuntu / Oracle Linux / common / `start.sh` / `stop.sh`): пользовательские сообщения на русском

### Fixed
- Sparkline XSS на карте; nginx `auth_request` / auth matrix для reputation; жёстче контракт OpenAPI MapLine
- Smoke/контракты SPA расширены под wizard и auth edge

### Notes
- OpenAPI API doc version: **1.5.0** (`geo-wizard-dismiss`, `geo_wizard_dismissed`, `index_ready`)
- Продуктовая версия: **1.3.0**
- После обновления: `git fetch && git checkout v1.3.0` (или `main`), `./start.sh`
- 3 коммита с `v1.2.1`

## [1.2.1] — 2026-08-11

Патч после 1.2.0: порядок шагов установщика (HTTPS → HTTP-порт), uninstall под тома 1.2.0, удаление мёртвого кода.

### Fixed
- Установщик: вопрос HTTPS идёт **до** выбора HTTP-порта (цепочка `select_https` → `confirm_http_port`); full-auto согласован
- Uninstall: cleanup firewall/томов по `NM_PROJECT_DIR` и составу volumes 1.2.0 (в т.ч. `clickhouse-backups`)
- Install: `chmod` на backup/ops-скрипты при установке

### Changed
- Удалены мёртвый backend/GeoIP/reputation код, one-shot CH migrate SQL и дублирующие frontend-ассеты (иконки/GeoJSON вне SPA `public/`)

### Notes
- OpenAPI API doc version: **1.4.0** (без изменений)
- Продуктовая версия: **1.2.1**
- После обновления: `git fetch && git checkout v1.2.1` (или `main`), `./start.sh`
- 2 коммита с `v1.2.0`

## [1.2.0] — 2026-08-11

Минорный релиз: React SPA, HTTPS, резервное копирование ClickHouse, лимиты GeoIP upload и hardening Docker.

### Added
- **Frontend SPA**: React + TypeScript + Vite; clean routes (`/`, `/system`, …); legacy `*.html` редиректятся; unit-тесты карты (vitest) и `scripts/frontend-smoke.sh`
- Опциональный **HTTPS** на nginx: свои PEM (`certs/`), `HTTPS_ENABLED` / `HTTPS_PORT` / `HTTP_REDIRECT`, `docker-compose.https.yml`, entrypoint генерирует `default.conf`; `deploy/common/compose.sh` подключает override
- Установщик (Ubuntu / Oracle Linux): интерактивный шаг HTTPS (`select_https.sh`) в пошаговом и full-auto; env `NM_HTTPS_*` / `NM_SSL_*` / `NM_CERTS_DIR`
- **Резервное копирование ClickHouse**: native `BACKUP`/`RESTORE` на том `clickhouse-backups`; CLI `scripts/backup-clickhouse.sh` / `restore-clickhouse.sh`; UI `/system` → «Резервное копирование» (список, создать, расписание, keep, edges/auth)
- Неразрушающее **Подключить** бэкапа через shadow-таблицы `nm_bak_*` (карта Live / Бэкап); Отключить / Удалить; маркер источника `вручную` / `по расписанию`
- Расписание бэкапов: ежедневно `hour:minute` + IANA timezone, имена с локальным временем и UTC-offset; timestamps в UI в timezone расписания
- **GeoIP**: лимиты размера/числа ranges из install-profile (`GEOIP_UPLOAD_MAX_*`); early **409** при опасном full-replace поверх крупного индекса; **Очистить базу** на `/geo-ranges` + загрузка CSV там же
- `GET /api/system/version` + строка версии (`main` / тег) в меню пользователя (`install-meta.json`)
- Ingest drops / queue bytes / circuit breaker явнее на `/system` (Pipeline); в README — product limits (IPv4-only, single-host control plane)
- Action vocabulary ClickHouse: SoT в `model` → `go generate` в ops SQL

### Changed
- Docker hardening: root `.dockerignore`, hermetic backend build, non-root `stats-collector`, `cap_drop: ALL`, frontend healthcheck + tmpfs; frontend `cap_add: NET_BIND_SERVICE` + `CHOWN`/`SETUID`/`SETGID` для nginx
- Compose **fail-closed** на секретах: `API_AUTH_TOKEN` / `SESSION_SECRET` / seed-пароли требуют `.env` (`:?`); предпочтителен `./start.sh`
- Усиление хранения ClickHouse (IPv4 migrate, TTL/agg правки); MapPage / SystemPage разбиты на hooks и вкладки
- Firewall (UFW/firewalld) открывает HTTPS-порт при TLS

### Fixed
- Вопрос HTTPS после whiptail не пропускается; установщик не пишет `.env` до `git clone` (непустой `/opt/network-monitor`)
- Scheduled backup: пропуск дня после failed/queued-only run; timezone UI; permission denied на `*.auth.tgz` (общий uid 101 / `volume-perms`)
- Multi-table `BACKUP` SQL под ClickHouse 25; gosec/contextcheck в backup и action-vocab путях
- Карта: race wipe basemap, globe GeoJSON fills, defaults/sidebar/globe visibility после SPA-миграции
- Подписи байт на графиках `/system` (storage / buffer)

### Notes
- OpenAPI API doc version: **1.4.0** (backups, backup-schedule, geo-ranges/clear, system/version)
- Продуктовая версия: **1.2.0**
- После обновления: `git fetch && git checkout v1.2.0` (или `main`), `./start.sh` (или `docker compose up -d --build`); при HTTPS — PEM в `certs/` и `docker-compose.https.yml` через `./start.sh`
- 59 коммитов с `v1.1.4`

## [1.1.4] — 2026-08-06

Патч: смена пароля и UI на порту **8080** (full-auto) — CSRF и доступность после установки.

### Fixed
- CSRF: `csrf origin rejected` при UI на нестандартном порту / по IP за nginx (`Host` без порта или `backend:8080`); литеральный IP в Origin и `X-Forwarded-Host`
- Установщик «Сделай мне хорошо»: если firewall не выключился — fallback `allow` на HTTP-порт; `NM_FULL_AUTO` всегда фиксирует **8080**; после старта — проверка `/login.html`, URL и учётки

### Changed
- nginx: `Host` / `X-Forwarded-Host` = `$http_host` (с портом клиента)
- `start.sh`: явно печатает login URL и default `admin` / `admin`

### Notes
- OpenAPI API doc version: **1.3.0** (без изменений)
- Продуктовая версия: **1.1.4**
- После обновления: `git fetch && git checkout v1.1.4` (или `main`), `docker compose up -d --build backend` и `docker compose up -d frontend`
- 3 коммита с `v1.1.3`

## [1.1.3] — 2026-08-05

Патч: установщик «Сделай мне хорошо», устойчивее GeoIP/старт backend, UX уведомлений и загрузки CSV.

### Added
- Установщик: режим **«Сделай мне хорошо»** (`NM_FULL_AUTO=1` / `--full-auto` / пункт TUI) — релиз, все модули, порт **8080**, автопрофиль, firewall OFF, старт стека
- UI: toast’ы без автоскрытия, крестик закрытия; незакрытые уведомления переживают смену страниц (`sessionStorage`)

### Fixed
- Cancel в TUI установщика больше не продолжает установку (источник / порт / модули / профиль / режим)
- Короткий radiolist (UTF-8) — рамка whiptail не ломается на длинных подписях
- Старт backend: загрузка GeoIP/reputation-индекса **асинхронно** — после OOM/рестарта страницы не получают nginx **500** на auth, пока индекс ещё поднимается
- Upload GeoIP/логов: понятное предупреждение при уходе со страницы (`Failed to fetch`); `beforeunload` только на время POST, не на ожидание индекса

### Changed
- Документация: загрузка большого GeoIP с сервера (`curl`), OOM/502 при повторной заливке поверх индекса в RAM, не уходить со страницы во время browser-upload

### Notes
- OpenAPI API doc version: **1.3.0** (без изменений)
- Продуктовая версия: **1.1.3**
- После обновления: `git pull`, `docker compose up -d --build backend` (frontend с тома — Ctrl+F5)
- 9 коммитов с `v1.1.2`

## [1.1.2] — 2026-08-05

Патч: карта снова читает daily geo-агрегаты; stats-collector корректно снимает MemoryTracking.

### Fixed
- Geo edges daily scan: `ORDER BY` по `src_ip`/`dst_ip` на таблицах `traffic_edges_{city|country}_daily` (там `src_key`/`dst_key`) — CH code 47 и постоянный fallback на cold `traffic_logs`
- stats-collector: `system.metrics.MemoryTracking` (Int64) сканится через `toUInt64(value)` — без ошибки `converting Int64 to *uint64`

### Notes
- OpenAPI API doc version: **1.3.0** (без изменений)
- Продуктовая версия: **1.1.2**
- После обновления: пересобрать/перезапустить `backend` и `stats-collector`
- 3 коммита с `v1.1.1`

## [1.1.1] — 2026-07-31

Патч: снижение CPU ClickHouse при обновлении карты, единое админ-меню, доработки установщика и репутации.

### Fixed
- Geo edges MV/backfill: alias-shadowing `trimBoth` + `anyState(src_city) AS src_city` (CH code 43) — pre-agg снова создаётся, карта не уходит в cold-скан `traffic_logs`
- Пики CPU при смене периода/группировки: `AbortController` + debounce для `/api/events` (и series); без лишнего raw fallback после успешного geo-скана
- `/api/events` 500 и panic карты при неготовых geo-агрегатах / отключённой репутации
- Пункт «API-токены» пропадал из статичных сайдбаров карты/мониторинга — навигация из единого `PAGE_NAV`
- Репутация: тосты refresh, подсчёт ranges, скрытие уже активных пресетов каталога; убраны битые/устаревшие фиды (`et_block_official`, `cruzit_web_attacks`)

### Changed
- `CH_MAX_THREADS` / `max_threads` в профиле установки (по умолчанию 2; ~½ ядер CH, потолок 4) — тяжёлые GROUP BY не утилизируют все ядра
- Установщик: профили только `tiny`…`xlarge` (без «оставить docker-compose.yml»); выбор HTTP-порта; источник install (релиз / `main`); опциональный модуль reputation; whiptail/dialog TUI
- Управление пользователями скрыто при отключённом UI-auth

### Added
- Дефолтные reputation seeds в установочном каталоге; объединение URL-фидов и загруженных списков в одной таблице UI

### Notes
- OpenAPI API doc version: **1.3.0** (без изменений)
- Продуктовая версия: **1.1.1**
- После обновления: `git pull`, перезапуск backend (пересоздаст geo MV, schema v2); для `max_threads` в users.d — `./scripts/tune-resources.sh` или `CH_MAX_THREADS` в override
- 22 коммита с `v1.1.0`

## [1.1.0] — 2026-07-29

Минорный релиз: репутация IP, поиск по карте, MapLibre/глобус, ускорение загрузки карты и усиление ingest.

### Added
- **Репутация IP**: offline-списки и URL-фиды (`/reputation.html`), каталог публичных источников, форматы `netset`/`plain`/`spamhaus_json`/`csv_ip`, расписание refresh, ручная загрузка CSV
- Фильтр репутации на карте (по спискам/категориям), опциональная подсветка дуг; приватные IP не помечаются
- **Конструктор поиска** на карте (гибридный query builder) с группировкой условий
- **Личные шаблоны поиска** (`/api/me/search-templates`): сохранение/загрузка для любого залогиненного; у администратора — просмотр всех шаблонов с группировкой по автору
- Карта на **MapLibre** + deck.gl: 2D-обзор мира и **3D-глобус** (проекция Globe), авто-вращение
- Heatmap стран во всех режимах группировки (на глобусе отключён ради производительности)
- Gzip-сжатие ответов API; дневные geo-агрегаты для day-пресетов периода
- Circuit breaker ingest при недоступности ClickHouse, лимиты байт очереди, метрики `queue_bytes` / `buffer_drops_total` / `circuit_open` в stats и алерты на `/system.html`
- Типизированные domain errors (`apperr`) → стабильные HTTP-коды (409 conflict, 400 validation и т.п.)
- `GET /api/events/series` для sparkline/серий по странам

### Changed
- Быстрый первый кадр карты: локальный basemap, параллельный init, отложенные слои, неблокирующий UI загрузки
- Убраны hub–spoke раскладка дуг и hex-density overlay; полный mesh + viewport-fit zoom
- Дефолтные reputation-фиды: раздельные источники FireHOL; из каталога убраны deprecated SSLBL IP-blacklist
- Composition root backend разбит на wire/lifecycle файлы; map fallback перенесён в repository layer
- Строгая валидация критичных env при старте (`ValidateConfig`)

### Fixed
- Дуги на глобусе «сквозь» планету, обрезка за верх viewport, бесконечный refresh после regroup
- Подгонка zoom карты/глобуса под viewport; центрирование pill загрузки данных
- Soak/watch-скрипты: видимость failed POST, `queue_bytes`, корректный pipefail
- CI: contextcheck / staticcheck (inherited contexts, nil Context в тестах)

### Security
- Allowlist имён таблиц в SQL health-агрегации ingest
- Операторы имеют доступ только к **своим** шаблонам поиска; админский список — отдельно

### Notes
- OpenAPI API doc version: **1.3.0**
- Продуктовая версия (этот файл / git tag): **1.1.0**
- 50 коммитов с `v1.0.0` (2026-07-23)

## [1.0.0] — 2026-07-23

Первый стабильный релиз ГеоАтлас (network_monitor).

### Added
- Именованные API-токены со scopes `read` | `ops` | `admin` (`/api/tokens`, UI `/api-tokens.html`, файл `API_TOKENS_FILE`)
- Ротация env Bearer через `API_AUTH_PREVIOUS_TOKEN`
- `last_drop_at` и edge-triggered логи при queue drops
- Адаптивный drain ingest при shutdown (отдельный бюджет от HTTP)
- Классификация retry ClickHouse по `code: N` + permanent code map
- `geojob.Store` / `MaintenanceStore` — geojob не импортирует `migrate`
- ClickHouse SQL/DDL SoT в `adapter/clickhouse` (пакет `storage` удалён)
- CI coverage smoke (≥40% на критичных пакетах)

### Security
- CSRF, cookie-сессии, роли administrator/operator
- `ValidateSecurity` блокирует insecure placeholders без `NM_ALLOW_INSECURE=1`

### Notes
- OpenAPI API doc version: **1.2.0**
- Продуктовая версия (этот файл / git tag): **1.0.0**

[1.4.2]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.4.2
[1.4.1]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.4.1
[1.4.0]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.4.0
[1.3.1]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.3.1
[1.3.0]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.3.0
[1.2.1]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.2.1
[1.2.0]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.2.0
[1.1.4]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.1.4
[1.1.3]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.1.3
[1.1.2]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.1.2
[1.1.1]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.1.1
[1.1.0]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.1.0
[1.0.0]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.0.0
