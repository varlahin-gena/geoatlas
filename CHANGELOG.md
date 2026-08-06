# Changelog

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/).
Версии — [SemVer](https://semver.org/lang/ru/).

## [Unreleased]

### Added
- HTTPS на nginx со своими PEM: `certs/fullchain.pem` + `certs/privkey.pem`, `HTTPS_ENABLED` / `HTTPS_PORT` / `HTTP_REDIRECT`, `docker-compose.https.yml`
- Entrypoint frontend генерирует `default.conf` (HTTP или HTTPS + редирект); общий `nginx-app.inc`
- `deploy/common/compose.sh` (`nm_compose`) — start/stop/tune подключают HTTPS-override при наличии сертификатов
- Firewall (UFW/firewalld) открывает HTTPS-порт при включённом TLS
- Установщик (Ubuntu / Oracle Linux): интерактивный шаг HTTPS (`select_https.sh`) в пошаговом режиме и в «Сделай мне хорошо»; env-overrides `NM_HTTPS_*` / `NM_SSL_*` / `NM_CERTS_DIR`
- Fix: вопрос HTTPS не пропускается после whiptail (`/dev/tty` + цепочка из `select_http_port.sh`)
- UI: версия в меню пользователя (`main` / тег релиза) — `install-meta.json`, `GET /api/system/version`
- Fix: установщик не пишет `.env` до `git clone` (непустой `/opt/network-monitor` ломал клон → «docker-compose.yml not found»)

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

[1.1.4]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.1.4
[1.1.3]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.1.3
[1.1.2]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.1.2
[1.1.1]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.1.1
[1.1.0]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.1.0
[1.0.0]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.0.0
