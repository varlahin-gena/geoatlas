# Changelog

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/).
Версии — [SemVer](https://semver.org/lang/ru/).

## [Unreleased]

## [2.7.8] — 2026-11-18

Редактируемые правила парсеров CEF/KV; устойчивая выгрузка большой базы GeoIP; полировка аномалий и мониторинга.

### Added
- **Парсеры**: UserGate / FortiGate / Generic живут в ClickHouse (`parser_rules`) с hot reload; страница `/parsers` — CRUD, dry-run, enable/priority. Cisco ASA / FTD остаются встроенными.
- **Тест парсеров**: нормализованные поля показываются колонками результата.

### Changed
- **Аномалии**: «Связи» — обычная кнопка в строке (без меню «Ещё»); период peers согласован с прыжком на карту.
- **Конструктор поиска**: «НЕ» — читаемый toggle-chip.
- **Мониторинг**: operator-facing формулировки вместо API-жаргона.

### Fixed
- **GeoIP**: выгрузка CSV стримится из ClickHouse и скачивается браузером — базы на миллионы диапазонов больше не рвут соединение (`Failed to fetch`).

### Notes
- OpenAPI API doc version: **1.23.0**
- Продуктовая версия: **2.7.8**
- Схема ClickHouse: таблица `parser_rules` (Ensure* / `init.sql`).
- Достаточно `update.sh` 2.7.8 (recreate **backend** + **frontend**).

## [2.7.7] — 2026-11-17

Интервал опроса репутации из UI без рестарта; HIG-полировка интерфейса.

### Added
- **Репутация**: на `/reputation` можно сменить интервал опроса URL-фидов (1–168 ч, по умолчанию 6); `GET`/`PUT /api/reputation/settings` → `reputation_settings.json`, без рестарта.

### Changed
- **UI (HIG)**: контраст active nav / badges в light, типографика ≥12px, таблицы без обрезанных типов, operator-facing copy без API-жаргона, loading stuck → retry (~5 с), компактные empty charts на мониторинге.

### Notes
- OpenAPI API doc version: **1.22.0**
- Продуктовая версия: **2.7.7**
- Схема ClickHouse: без изменений Ensure*.
- Достаточно `update.sh` 2.7.7 (recreate **backend** + **frontend**).

## [2.7.6] — 2026-11-16

Патч: sheet исключения больше не сжимается до 360px из‑за стилей admin search-box.

### Fixed
- **Движок аномалий**: `body.page-admin .search-box { max-width: 360px }` больше не режет sheet исключения — окно снова до ~1120px.

### Notes
- OpenAPI API doc version: **1.21.0**
- Продуктовая версия: **2.7.6**
- Схема ClickHouse: без изменений Ensure*.
- Достаточно `update.sh` 2.7.6 (recreate **frontend**).

## [2.7.5] — 2026-11-16

Патч: sheet исключения на движке аномалий без горизонтальной прокрутки.

### Fixed
- **Движок аномалий**: ширина sheet исключения до ~1120px / ширины родителя; убран forced `min-width` строки; на узком sheet условие переносится через container query.

### Notes
- OpenAPI API doc version: **1.21.0**
- Продуктовая версия: **2.7.5**
- Схема ClickHouse: без изменений Ensure*.
- Достаточно `update.sh` 2.7.5 (recreate **frontend**).

## [2.7.4] — 2026-11-16

Патч: сетка конструктора исключений на движке аномалий больше не теряет поле и «НЕ».

### Fixed
- **Конструктор запросов**: в sheet исключения колонки поля и «НЕ» не схлопываются; кнопка «Закрыть» остаётся справа в шапке.

### Notes
- OpenAPI API doc version: **1.21.0**
- Продуктовая версия: **2.7.4**
- Схема ClickHouse: без изменений Ensure*.
- Достаточно `update.sh` 2.7.4 (recreate **backend** + **frontend**).

## [2.7.3] — 2026-11-16

Патч: проверки аномалий ближе к разбору аналитика, исключение по запросу карты, профили не превышают RAM хоста, журнал аномалий возвращается вместе с трафиком.

### Added
- **Движок аномалий**: у каждой проверки своё исключение тем же конструктором, что поиск на карте. Совпавший трафик не входит только в эту проверку.

### Changed
- **Аномалии**: сканирование портов считает отказы; обход хостов предприятия вместо обхода подсети; репутация, новая страна, beacon, всплеск трафика и охота срабатывают, когда есть что смотреть.
- **Навигация**: пункт `/anomalies/engine` — **«Движок аномалий»**.
- **Профили**: рекомендация — меньший из классов по CPU и RAM. `tiny` — 1.5 ГиБ ClickHouse, 768 МиБ backend, 384 МиБ syslog-ng. Байтовая очередь ingest от 64 МиБ до 1 ГиБ. `xlarge` — от 64 ГиБ RAM.
- **Движок аномалий**: секции «Расписание» и «Чувствительность»; IP без обхода хостов у карточки lateral fanout; Save только при несохранённых правках.

### Fixed
- **`/ready`**: остаётся 503, пока нет схемы events. Обычные HTTP-соединения больше не держатся 35 минут. Скан аномалий записывает проверки, которые не уложились в бюджет тика.
- **Бэкап**: `backup-clickhouse.sh` / `restore-clickhouse.sh` возвращают журнал аномалий вместе с трафиком.
- **Движок аномалий**: «Конструктор запроса» для исключений больше не ломает админ-topbar; стили конструктора общие для карты и настроек.

### Notes
- OpenAPI API doc version: **1.21.0**
- Продуктовая версия: **2.7.3**
- Схема ClickHouse: без изменений Ensure*.
- Достаточно `update.sh` 2.7.3 (recreate **backend** + **frontend**). Уже стоящий сервер не пересчитывает профиль сам: `./scripts/tune-resources.sh`.

## [2.7.2] — 2026-11-14

Патч: понятные названия аномалий в UI, фикс полей в группе конструктора запросов, CSV load-test без локали RU.

### Changed
- **Аномалии**: человекочитаемые типы и заголовки (без жаргона «движок» / «веер» / «beaconing» в UI); пункт меню **«Настройки»** вместо «Движок».
- **scripts/load-test-connectors.ps1**: CSV soak/threshold через `InvariantCulture`; параметр `-ThresholdLog`.
- **docs/presentations**: HTML/PNG графики EPS (soak / threshold).

### Fixed
- **Конструктор запросов**: поле «Значение» внутри группы условий больше не схлопывается (гибкая CSS Grid + класс `search-builder-value`).

### Notes
- OpenAPI API doc version: **1.21.0**
- Продуктовая версия: **2.7.2**
- Схема ClickHouse: без изменений Ensure*.
- Достаточно `update.sh` 2.7.2 (recreate **backend** + **frontend**).

## [2.7.1] — 2026-11-13

Патч: демо аномалий на боевой, пиры в разборе по окну алерта, кнопка «Закрыть», фикс PNG-экспорта карты.

### Added
- **Демо аномалий**: `scripts/emulate-map-load.ps1` — `-Recipes` (default `port_scan,lateral_fanout`), `-AnomalyOnly`; seed-скрипты карты (`seed-demo-map.*`).
- **Карта**: экспорт PNG (MapLibre + deck.gl).
- **docs/presentations**: материалы meetup + скрипт сборки pptx.

### Changed
- **Аномалии**: видимая кнопка **«Закрыть»** (ack) на `/anomalies` и `/investigate`; статус «Закрыт» / фильтр «С закрытыми».
- **Разбор**: пиры запрашиваются по абсолютному окну алерта (`from`/`to`), а не «последний 1 ч от сейчас».
- **Архитектура**: обновлена схема docker-сети и таблица «кто с кем говорит».

### Fixed
- **lateral_fanout** в демо: src/dst только в gray enterprise nets (`10.0` / `10.25` / `10.27` / `10.28` / `10.30`) — раньше src `10.68.*` часто вне `enterprise_nets`.
- **Пиры пустые**: scan map period `15m` → `2h`; подсказка про отсутствие гео у src/dst.
- **Экспорт PNG карты**: MapLibre v6 — `canvasContextAttributes.preserveDrawingBuffer` (top-level флаг игнорировался → чёрная подложка); ожидание кадра `render` и композит map+overlay по размеру canvas MapLibre.

### Notes
- OpenAPI API doc version: **1.21.0**
- Продуктовая версия: **2.7.1**
- Схема ClickHouse: без изменений Ensure*.
- Достаточно `update.sh` 2.7.1 (recreate **backend** + **frontend**).

## [2.7.0] — 2026-11-12

Минор: стабильный пакет после train 2.6.1–2.6.15 — multi-port syslog connectors, dismiss system alerts, HIG admin UI, Pipeline/syslog metrics.

### Added
- **Syslog connectors** (multi-port): CRUD в UI `/connectors`, Apply → `syslog-ng.d/zz_connectors.conf` + reload; порты `5514-5529`; per-connector метрики на `/system` → Pipeline; `traffic_logs.connector_id`; API `/api/connectors*`.
- **System alerts dismiss**: скрытие warn-алертов на `/system` (`POST/DELETE /api/system/alerts/ack`); error-алерты скрыть нельзя.
- **Пользователи**: включение/выключение УЗ (`POST /api/users/{username}/enabled`); нельзя выключить себя и последнего активного администратора.
- **Коннекторы**: «Изменить» (имя/порт/протоколы без пересоздания) и «Перезапуск»; статус ВКЛ/ВЫКЛ по `enabled` + `last_event_at`.
- **scripts/load-test-connectors.ps1**: режим `-Soak` для длительной нагрузки.

### Changed
- **Admin UI (Apple HIG)**: table density, quieter actions, progressive disclosure, page commands из шапки в контент, More-меню на Users/Connectors, форма «Добавить фид» в модалке.
- **Карта**: минимальный период **1h**; окна часов читают `traffic_edges_hourly`; быстрее поиск (debounce, IP/port fast-path, один in-flight GetMap).
- **stats-collector**: CPU как доля хоста (шкала Dozzle); память `usage − inactive_file`; тексты алертов EPS vs CPU разделены.
- **Dozzle**: **v11.4.2**; Pipeline: колонка Backend EPS и корректный scrape syslog-ng (prometheus + legacy).

### Fixed
- **syslog-ng stats**: `stats-with-legacy(yes)`, attribution `input/output_events` и `dst_network` / delivered; UI больше не подставляет backend EPS при пустом scrape.
- **Коннекторы Apply**: `volume-perms` на `./syslog-ng.d` (uid 101).
- **SPA**: retry/timeout lazy-chunks и проверки сессии; клипы меню и лишний x-scroll таблиц.

### Notes
- OpenAPI API doc version: **1.21.0**
- Продуктовая версия: **2.7.0**
- Схема ClickHouse: `EnsureTrafficLogsConnectorID` (`connector_id`).
- Обновление с 2.6.0: `update.sh` 2.7.0 (recreate **backend** + **frontend** + **syslog-ng**; при профиле dozzle — recreate `dozzle`; stats-collector при смене метрик CPU/mem).

## [2.6.0] — 2026-11-03

Минор: профиль и сессии, карта «город—страна», HIG triage UX, hardening образов и Scorecard на main.

### Added
- Страница **/profile**: смена пароля, тема UI и группировка дуг карты; prefs на сервере (`users.json`); `PATCH /api/auth/me/preferences`.
- Session registry: активные UI-сессии, отзыв одной / всех (`/api/auth/sessions*`); политика lockout логина; сводка **Безопасность** (`GET /api/system/security`).
- Карта: группировка **Город — страна** (`group_by=city_country`); «Скрыть связи внутри страны» в этом режиме.

### Changed
- Аномалии / навигация / мониторинг / логин / карта: HIG-полировка (inline triage, без лишней вкладки «Разбор», chrome карты в «Отображение», светлая/тёмная тема, Dozzle **v11.4.0**).
- Релизы: подписи как `.sigstore` для Scorecard Signed-Releases; контракт CI coverage ≥80% и promote-gates после слияния train в main.

### Security
- Trivy Image scan: сброс CRITICAL по compose-образам (#148).
- syslog-ng: `pyasn1` **0.6.5**, pin `oauthlib`; runtime digests Alpine/Node, nginx / Go build pins; frontend overrides (minimatch / undici / zod и др.).

### Fixed
- Глобус: длинные дуги (например RU↔AU) больше не отфильтровываются.
- CI: golangci/gofmt, knip, MapLibre typing, SPA smoke после удаления upload с карты.

### Notes
- OpenAPI API doc version: **1.19.0** (prefs, sessions, security summary, login-policy, city_country)
- Продуктовая версия: **2.6.0**
- Схема ClickHouse (ga_schema_version): без изменений Ensure*.

## [2.5.0] — 2026-09-11

Минор: redesign SPA (HIG), command palette, усиление edge/auth, пороги аномалий в UI и интервал обновления карты.

### Added
- **Command palette** (`Ctrl/⌘K` или `/`): быстрый переход по страницам, команды карты и параметры (тема, плотность).
- **Интервал обновления карты**: пресеты 30 с / 1 мин / 5 мин (по умолчанию 5 мин), сохранение в `localStorage`.
- **Живой счётчик сетей предприятия** на карте (из ClickHouse, без устаревшего scan-cache).
- **Re-auth паролем** для чувствительных cookie-действий: logout-all, УЗ (reset password), API-токены (create / rotate / delete), загрузка TLS PEM.
- **Ротация именованных API-токенов**: `POST /api/tokens/{id}/rotate` (новый secret один раз; id/name/scope/expires сохраняются).
- **Proxy gate** (`GA_REQUIRE_PROXY`, в compose по умолчанию `1`): прямой доступ к backend `:8080` режется — только nginx / loopback / Bearer.
- **API threat protection**: SpikeArrest (`GA_API_RATE_LIMIT_RPS` / `GA_API_RATE_BURST`), injection/path guard, жёсткие JSON-лимиты, security headers на API; HSTS и снятие `Server` на nginx.
- **Настройки порогов детекторов** на `/anomalies/engine` с группированной справкой; overrides в `anomaly_settings.json`, сброс к профилю без рестарта.
- Страница **404** в SPA; секции навигации **Рабочее место / Разбор / Система / Данные / Доступ**; ссылки Access в Settings nav.

### Changed
- SPA: токены Geist, иконки Phosphor, finish-states / skeleton, z-index токены карты, общий sidebar CSS, polish collapsed-sidebar.
- Окно обучения аномалий: `beaconing` (как `new_country_dst`) **не** срабатывает, пока `learning=true`.
- Beaconing-детектор ускорен, таймаут увеличен; фикс cast IP для beaconing / `horizontal_scan`.
- Geo CSV export стримится; scan-запросы всегда с верхней границей.
- Trusted proxies: DNS `frontend` резолвится заранее и с retry (proxy-gate не режет UI при позднем DNS).
- Compose v5: совместимость `pids_limit` с `deploy.resources.limits`; proxy-gate health probes; syslog-ng read-only persist path.
- Backend: разбиение httpapi routes / anomaly detectors, `jsonfile.Store`, `jobscheduler.Loop`, буферы колонок CH, парсеры FortiGate/UserGate/FTD без лишних map-аллокаций (perf).

### Fixed
- Дуги карты за пределами viewport; раскладка search bar на `/hunts`.
- TLS cert upload UX; клики нижней навигации в свёрнутом sidebar.
- Nil-receiver / batch leak / in-flight gauge leak в ingest path.
- CI: flaky npm audit (retry + timeout); Image scan (Alpine/CH apt, Go toolchain, `golang.org/x/crypto`); Dependabot vitest/js-yaml.

### Security
- Re-auth + token rotate + proxy gate + rate limit + strict request schema (`additionalProperties: false`).
- Nginx security headers (HSTS без `preload`, CSP/frame/referrer и снятие `Server`).
- Runtime image upgrades и bump crypto для SSH DoS CVE в Image scan.

### Notes
- OpenAPI API doc version: **1.17.0** (reauth / rotate / строгие request schemas)
- Продуктовая версия: **2.5.0**
- Cookie-клиенты чувствительных мутаций обязаны передавать `current_password`; Bearer может опустить reauth.
- Схема ClickHouse (`ga_schema_version`): без изменений Ensure*.

## [2.0.0] — 2026-08-23

### Added
- **Anomaly Engine v1**: журнал аномалий на карте — `port_scan`, `horizontal_scan`, `blocked_surge`, `new_country_dst`, `rep_new_peer`; `GET /api/anomalies`, `/summary`, `/status`, `POST /ack`; фоновый скан каждые 5 мин; полоска и панель на карте. `ANOMALY_ENABLED` (по умолчанию true). OpenAPI **1.12.0**.
- Аномалии: назначение на УЗ (`POST /api/anomalies/{fp}/assign`, таблица `anomaly_assignments`); в списке поля `assigned_to` / `ack_by`; `GET /api/users/directory` для выбора исполнителя. При закрытии УЗ закрывшего проставляется, если исполнитель ещё пуст. OpenAPI **1.13.0**.
- Карта: баннер активного алерта («На карте») и кнопка **Live-карта** — сброс фильтров к live-виду.
- CI: сборка Docker-образов backend / frontend / stats-collector (`scripts/ci-docker-build.sh`); frontend — дым `/health` после `docker build`
- CI: Trivy image scan для `balabit/syslog-ng:4.11.0` (вход `:514`, тот же тег что в compose)
- CI: `govulncheck` по backend / stats-collector / pkg/chconn / pkg/syslogngstats (`scripts/ci-govulncheck.sh`)
- Релиз: SBOM установочного пакета — CycloneDX + SPDX (`scripts/ci-sbom.sh`, Syft v1.51.0) в Assets вместе с `geoatlas-X.Y.Z.tar.gz`

### Fixed
- Карта: полоска аномалий и подсказка о лимите связей не перекрывают друг друга — общая колонка сверху
- Карта: легенда «Статус трафика» не перекрывает панель аномалий — оба блока в правой колонке
- CI: frontend image smoke — nginx стартует без compose DNS (`--add-host backend:127.0.0.1`); раньше `host not found in upstream "backend"`
- CI / образы: Go **1.25.13** (stdlib `net/url`, `crypto/tls`, `net/http`, `encoding/asn1`); `setup-go` `check-latest: true`, `golang:1.25.13-alpine`

### Changed
- OpenAPI **1.14.0**: JSON schemas for users/tokens/events.series/geo/reputation/system history·backups·tls (frontend type-safety).
- Frontend: API modules prefer OpenAPI-generated types (`ApiJson` supports 200/201); Map viz chrome hook + `MapChromeProvider`; System stats/charts hooks; Playwright `system.spec.ts`.
- CI: GitHub Actions запинены по commit SHA (`checkout`, `setup-go`, `setup-node`, CodeQL, golangci-lint, buildx) — как Trivy/Hadolint/Syft
- CI: `concurrency: cancel-in-progress`; на PR Playwright / ClickHouse / fuzz / docker — только если изменились их деревья (`dorny/paths-filter`); push в `main`/`develop` гоняет все job'ы
- CI: Hadolint валит job на правилах severity `error` (`failure-threshold: error`); warning/info по-прежнему в Security tab
- Установка и обновление на сервере только из локального `geoatlas-X.Y.Z.tar.gz`: нет `git clone` / `git pull`, установщик не ставит пакет `git`, нет `--download` и curl-установки одним скриптом с GitHub. `install-meta.json` берёт версию из пакета, не из git.
- ClickHouse image: `clickhouse/clickhouse-server:25.8.29.51` → `25.8.30.16` в compose, image scan и CI integration, чтобы подтянуть Ubuntu package security fixes для контейнерного образа.

### Breaking
- Полный rebrand **network-monitor → geoatlas**: Go-модуль `geoatlas`, бинарь `geoatlas`, каталог `/opt/geoatlas`, `COMPOSE_PROJECT_NAME=geoatlas`, env `GA_*`, cookie `ga_session` / `ga_csrf`, ingest-маркеры `@@ga/…`, бэкапы `ga-*`, пакет `.ga-package`. Обратной совместимости с `network_monitor`, `NM_*`, `@@nm/`, `nm-*` бэкапами и `/opt/network-monitor` нет — только чистая установка.

### Notes
- OpenAPI API doc version: **1.14.0**
- Продуктовая версия: **2.0.0**
- GitHub: репозиторий `varlahin-gena/geoatlas`

## [1.0.0] — 2026-07-23

Первый стабильный релиз ГеоАтлас (geoatlas).

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
- `ValidateSecurity` блокирует insecure placeholders без `GA_ALLOW_INSECURE=1`

### Notes
- OpenAPI API doc version: **1.2.0**
- Продуктовая версия (этот файл / git tag): **1.0.0**

[2.7.8]: https://github.com/varlahin-gena/geoatlas/releases/tag/v2.7.8
[2.7.7]: https://github.com/varlahin-gena/geoatlas/releases/tag/v2.7.7
[2.7.6]: https://github.com/varlahin-gena/geoatlas/releases/tag/v2.7.6
[2.7.5]: https://github.com/varlahin-gena/geoatlas/releases/tag/v2.7.5
[2.7.4]: https://github.com/varlahin-gena/geoatlas/releases/tag/v2.7.4
[2.7.3]: https://github.com/varlahin-gena/geoatlas/releases/tag/v2.7.3
[2.7.2]: https://github.com/varlahin-gena/geoatlas/releases/tag/v2.7.2
[2.7.1]: https://github.com/varlahin-gena/geoatlas/releases/tag/v2.7.1
[2.7.0]: https://github.com/varlahin-gena/geoatlas/releases/tag/v2.7.0
[2.6.0]: https://github.com/varlahin-gena/geoatlas/releases/tag/v2.6.0

[2.5.0]: https://github.com/varlahin-gena/geoatlas/releases/tag/v2.5.0
[2.0.0]: https://github.com/varlahin-gena/geoatlas/releases/tag/v2.0.0
[1.0.0]: https://github.com/varlahin-gena/geoatlas/releases/tag/v1.0.0
