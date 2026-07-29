# Changelog

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/).
Версии — [SemVer](https://semver.org/lang/ru/).

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

[1.1.0]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.1.0
[1.0.0]: https://github.com/varlahin-gena/network_monitor/releases/tag/v1.0.0
