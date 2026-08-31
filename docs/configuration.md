# Конфигурация (переменные окружения)

Основной файл на сервере: `/opt/geoatlas/.env` (шаблон в репозитории — [`.env.example`](../.env.example)).
Не копируйте пример «как есть» в прод и не коммитьте заполненный `.env`.

Секреты и модули обычно выставляет установщик / `./start.sh`. После установки правьте только то, что понимаете; часть значений переживает [`update.sh`](operations.md#обновление-системы).

## Что генерируется автоматически

| Переменная | Кто создаёт | Назначение |
|------------|-------------|------------|
| `API_AUTH_TOKEN` | `./start.sh` | Env Bearer со scope **admin** |
| `SESSION_SECRET` | `./start.sh` | Подпись cookie-сессии |
| `AUTH_ADMIN_PASSWORD` | `./start.sh` / установщик | Пароль seed-УЗ admin (+ `AUTH_ADMIN_MUST_RESET`) |
| `CLICKHOUSE_PASSWORD` | `./start.sh` | Пароль пользователя ClickHouse |
| `INGEST_SHARED_SECRET` | `./start.sh` | Токен маркера syslog-ng → backend `:1514` |
| `API_OPS_TOKEN` | установщик / `detect_resources.sh` | Env Bearer scope **ops** для sidecars (stats-collector); **не** admin |

Одноразовый пароль admin пишется в `.admin_password_once` (режим `600`), не в stdout.

## Что переживает обновление пакета

`update.sh` **не затирает**: `.env`, `.admin_password_once`, `docker-compose.override.yml`, `install-profile.json`, `install-modules.json`, `certs/`, `syslog-ng.d/zz_profile.conf`, `syslog-ng.d/zz_ingest_auth.conf`, `clickhouse/users.d/zz_install_limits.xml`. Тома Docker (данные CH, `/app/data`) не трогаются.

## Порты и HTTPS

| Переменная | По умолчанию | Назначение |
|------------|--------------|------------|
| `HTTP_PORT` | `80` | Хостовый порт UI (HTTP / редирект) |
| `HTTPS_ENABLED` | `auto` | `1` — вкл.; `0` — выкл.; `auto` — вкл. если есть оба PEM в `certs/` |
| `HTTPS_PORT` | `443` | Хостовый порт TLS |
| `HTTP_REDIRECT` | `1` | Редирект HTTP→HTTPS |

Подробнее: [HTTPS](install.md#https-свои-сертификаты), `certs/README.md`.

## Модули и Compose-профили

| Переменная | По умолчанию | Эффект |
|------------|--------------|--------|
| `COMPOSE_PROFILES` | `syslog,stats,dozzle` (пример) | Какие optional-сервисы поднимать; без dozzle — уберите его из списка |
| `GA_MODULE_AUTH` | `1` | UI-логин (`AUTH_DISABLED=false`) |
| `GA_MODULE_API_AUTH` | `1` | Bearer на мутирующих API |
| `GA_MODULE_SYSLOG` | `1` | Профиль `syslog` (:514) |
| `GA_MODULE_STATS` | `1` | Профиль `stats` |
| `GA_MODULE_REPUTATION` | `1` | `REPUTATION_FETCH_ENABLED=true`; `0` → модуль выкл. |
| `GA_MODULE_DOZZLE` | `1` | Профиль `dozzle` (`/dozzle/`) |
| `DOZZLE_HOSTNAME` | `geoatlas` | Имя в UI Dozzle |
| `SYSLOG_STATS_URL` | задаёт установщик | Scrape stats-exporter syslog-ng |

Репутация — **не** compose-профиль, а флаг backend. Выбор модулей при установке: [install.md](install.md).

## Авторизация

| Переменная | По умолчанию | Назначение |
|------------|--------------|------------|
| `AUTH_DISABLED` | `false` | Выкл. UI-логин (только с `GA_ALLOW_INSECURE=1`) |
| `API_AUTH_DISABLED` | `false` | Открыть мутирующие API без Bearer |
| `GA_ALLOW_INSECURE` | `0` | Разрешить ослабленную auth (dev) |
| `AUTH_ADMIN_USER` | `admin` | Seed administrator |
| `SESSION_TTL_HOURS` | `12` | TTL cookie для admin/operator (не dashboard) |
| `API_AUTH_PREVIOUS_TOKEN` / `API_OPS_PREVIOUS_TOKEN` | пусто | Ротация env-токенов |
| `GA_TRUSTED_PROXIES` | `frontend` | Доверенные hop для X-Real-IP (login throttle) |

Роли, cookie и scopes: [ui.md](ui.md#роли-и-доступ).

## Ingest и syslog

| Переменная | По умолчанию | Назначение |
|------------|--------------|------------|
| `INGEST_LISTEN_ADDR` | `:1514` | Слушатель backend (внутри docker) |
| `INGEST_ALLOW_FROM` | `syslog-ng` | Allowlist peer для ingest TCP |
| `GA_SYSLOG_ALLOW_FROM` | пусто (= любой) | CIDR/IP источника для хостового FW на `:514` |
| `INGEST_WORKERS` / batch / queue / flush | из профиля | Параметры пайплайна (`docker-compose.override.yml`) |

Onboarding МСЭ: [syslog.md](syslog.md). Лимиты ёмкости: [архитектура — Ingest SLO](architecture.md#ingest-slo).

## GeoIP

| Переменная | По умолчанию | Назначение |
|------------|--------------|------------|
| `GEOIP_SNAPSHOT_FILE` | рядом с users (`geo_index.snap`) | `off` / `0` / `-` — без snapshot на диск |
| `GEOIP_UPLOAD_MAX_BYTES` | из install-profile | Лимит тела `/upload-geo` |
| `GEOIP_UPLOAD_MAX_RANGES` | из install-profile | Лимит числа диапазонов |

Практика загрузки: [geoip.md](geoip.md).

## Репутация и аномалии

| Переменная | По умолчанию | Назначение |
|------------|--------------|------------|
| `REPUTATION_FETCH_ENABLED` | `true` | Полный выключатель модуля |
| `REPUTATION_FETCH_INTERVAL` | `6h` | Период опроса URL-фидов (не короче ~1m) |
| `REPUTATION_FEEDS` / `REPUTATION_FEEDS_FILE` | seed / `/app/data/…` | Список фидов |
| `ANOMALY_ENABLED` | `true` | Движок аномалий |
| `ANOMALY_SCAN_INTERVAL` | `5m` | Фоновый скан |
| `ANOMALY_INCLUDE_PRIVATE` | `false` | Учитывать RFC1918 и т.п. |
| `ANOMALY_LEARNING_DAYS` | `3` | Окно обучения |
| `ANOMALY_SUPPRESS_HOURS` | `24` | Подавление после срабатывания |

Подробнее: [reputation.md](reputation.md), [anomalies.md](anomalies.md).

## Прочее

| Переменная | По умолчанию | Назначение |
|------------|--------------|------------|
| `LISTEN_ADDR` | `:8080` | HTTP API внутри контейнера backend |
| `LOG_LEVEL` / `LOG_FORMAT` | `info` / `text` | slog |
| `CLICKHOUSE_*` | host `clickhouse`, port `9000`, user `default` | Подключение к CH |
| `BACKUP_*` | вкл., keep 7, disk backups | Native BACKUP (см. operations) |
| `RETENTION_FILE` | `/app/data/retention.json` | TTL таблиц |
| `CH_MAX_THREADS` | из профиля | Потолок потоков тяжёлых запросов карты |

Лимиты CPU/RAM контейнеров и буферы syslog-ng задаёт [профиль производительности](operations.md#профили-производительности), не ручной правкой compose без нужды.
