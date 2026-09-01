# ГеоАтлас

Визуализатор сетевых взаимодействий на базе логов межсетевых экранов (МСЭ).
Принимает syslog с файрволов или из SIEM, парсит события, обогащает их геоданными
(IP → страна/координаты) и строит карту сетевых связей в веб-интерфейсе.

**Актуальный релиз:** [v2.4.0](https://github.com/varlahin-gena/geoatlas/releases/tag/v2.4.0) (`VERSION` = 2.4.0) · ОС: Ubuntu 20.04+, Oracle Linux 8+ / RHEL-совместимые · пакет: `geoatlas-X.Y.Z.tar.gz` в [Releases](https://github.com/varlahin-gena/geoatlas/releases).

Appliance: **IPv4-only**, один хост / один процесс backend, доставка syslog **at-most-once** (при переполнении очереди возможны drops). Подробнее — [архитектура](docs/architecture.md).

## Возможности

- Приём логов по **syslog** (TCP/UDP, порт 514)
- Ручная загрузка логов через веб-интерфейс
- Парсинг **UserGate, FortiGate, Cisco ASA, Cisco FTD (FirePower), Cowrie (honeypot)** и универсальный фолбэк (Generic KV)
- **Осознанный пропуск** событий: распознанные, но несетевые строки (например, часть `cowrie.*`) не попадают в `parse_errors`
- **Авторизация по умолчанию**: одна учётка по умолчанию `admin`; роли `administrator` / `operator` / `dashboard`; управление УЗ в UI.
- Веб-интерфейс — **React SPA**
- Опциональный **HTTPS** на nginx со своими PEM-сертификатами (редирект HTTP→HTTPS)
- Загрузка и правка **GeoIP-базы**
- **Пошаговая установка**
- **Один установочный пакет** `geoatlas-X.Y.Z.tar.gz` для Ubuntu и Oracle Linux / RHEL; обновление через архив на сервере (`./update.sh`)
- **Toast-уведомления**
- **Репутация IP**: offline-списки и URL-фиды (`/reputation`), каталог публичных источников, фильтр и подсветка дуг на карте
- Хранение и аналитика в **ClickHouse**
- **Резервное копирование ClickHouse**
- **Настраиваемый TTL (retention)**
- **Построение связей** на 2D карте и 3D глобусе
- **Движок аномалий**: port/horizontal scan, всплеск блокировок, **всплеск объёма** (`byte_surge`), **beaconing**, **lateral movement** (`lateral_fanout`), новая страна, репутационный peer; панель «Связи» на `/anomalies`
- **Разбор алерта** (`/investigate?alert=<fingerprint>`): workspace с peers, ack/assign, CSV, шаблон поиска, deep-link на карту
- **Аномалии на карте** (баннер со ссылкой на список)
- **Конструктор поиска** на карте (гибридный query builder) и **личные шаблоны** запросов; у администратора — просмотр всех шаблонов
- **Группировка узлов**: по IP / по подсети `/24` / **по городу (по умолчанию)** / по стране
- **Тест парсеров** в браузере: статусы parsed / skipped / error, гео-обогащение, пресеты по вендорам
- **Журнал ошибок парсинга**: поиск, выборочное и полное удаление, передача строк в «Тест парсеров»
- **Страница системного мониторинга** (Обзор / Pipeline / Безопасность / Графики / Резервное копирование)
- **Логи контейнеров**: realtime stdout стека в UI `/dozzle/` для administrator

## Быстрый старт

1. Установите пакет с [GitHub Releases](https://github.com/varlahin-gena/geoatlas/releases) — см. [установку](docs/install.md).
2. Откройте `http://<IP_сервера>/` (при HTTPS — `https://<IP_сервера>/`).
3. Первый вход — учётка **admin** (роль administrator). Пароля по умолчанию нет: пошаговая установка спрашивает пароль (дважды, мин. 8 символов).
4. Направьте syslog МСЭ на `:514` — [syslog.md](docs/syslog.md); загрузите GeoIP — [geoip.md](docs/geoip.md).

Подробности по ОС, модулям, HTTPS и удалению — в [docs/install.md](docs/install.md).

## Документация

| Раздел | Описание |
|--------|----------|
| [Архитектура](docs/architecture.md) | Сервисы, поток данных, product limits, Ingest SLO, TTL |
| [Установка](docs/install.md) | Требования, пакет, Ubuntu / Oracle Linux, удаление |
| [Syslog](docs/syslog.md) | Подключение МСЭ / SIEM |
| [Конфигурация](docs/configuration.md) | `.env`, секреты, модули |
| [Обслуживание](docs/operations.md) | Запуск, профили, retention, бэкапы, логи, обновление |
| [GeoIP](docs/geoip.md) | CSV, загрузка с сервера, 502/OOM |
| [UI и HTTP API](docs/ui.md) | Страницы SPA, роли, OpenAPI **1.16.0** |
| [Репутация](docs/reputation.md) | Фиды и offline-списки |
| [Аномалии](docs/anomalies.md) | Типы (в т.ч. byte_surge / beaconing / lateral), ack/assign, разбор |
| [Разработка](docs/development.md) | Локальный стек из git |
| [Структура репо](docs/repo-layout.md) | Дерево каталогов |
| [RELEASING.md](RELEASING.md) | Версии продукта / OpenAPI / схемы CH |
| [SECURITY.md](SECURITY.md) | Сообщение об уязвимостях |
| [CHANGELOG.md](CHANGELOG.md) | История релизов |

Контракт REST: [`openapi.yaml`](openapi.yaml) (документ OpenAPI **1.16.0**).

## Лицензия

ГеоАтлас распространяется под [Apache License 2.0](LICENSE). Продукт бесплатный и общедоступный; релизы публикуются на GitHub.

Баги, вопросы и заказ доработок — через [Issues](https://github.com/varlahin-gena/geoatlas/issues).

## Безопасность

Уязвимости — только приватно, см. [SECURITY.md](SECURITY.md) (подтверждение в течение 5 рабочих дней). Обычные баги — в Issues.

Краткий checklist для прод-установки:

- Секреты только через `./start.sh` (`API_AUTH_TOKEN`, `SESSION_SECRET`, `INGEST_SHARED_SECRET`, `CLICKHOUSE_PASSWORD`)
- Пароли УЗ: минимум 10 символов, буква и цифра, не из common-list
- Одноразовый пароль admin: `.admin_password_once` (600), не stdout
- UI за HTTPS при доступе из сети; не публиковать ClickHouse `8123`/`9000` и backend `1514`
- ClickHouse `default`: networks loopback+RFC1918 (не `::/0`)
- Syslog `:514` **без TLS/auth** — ограничьте источником МСЭ (`GA_SYSLOG_ALLOW_FROM` / Security Group / firewall)
- Ingest внутри docker: маркер с `INGEST_SHARED_SECRET` + `INGEST_ALLOW_FROM=syslog-ng` (HTTP upload API — без маркера)
- Reputation URL-фиды: только публичные IPv4-хосты (private/metadata блокируются)
- Установка и обновление: только `geoatlas-X.Y.Z.tar.gz` с GitHub Releases; проверяйте `.sha256` до `./update.sh`

## Структура репозитория

| Путь | Назначение |
|------|------------|
| `backend/` | Go API, парсеры, GeoIP, ingest |
| `frontend/` | React SPA → nginx |
| `clickhouse/` | конфиг и скрипты CH |
| `deploy/` | установщики Ubuntu / Oracle Linux |
| `docs/` | документация оператора и разработчика |
| `scripts/` | CI, pack-release, бэкапы |
| `openapi.yaml` | контракт HTTP API (OpenAPI **1.16.0**) |

Полное дерево: [docs/repo-layout.md](docs/repo-layout.md).
