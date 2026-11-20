# ГеоАтлас

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/15404/badge)](https://www.bestpractices.dev/projects/15404)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/varlahin-gena/geoatlas/badge)](https://securityscorecards.dev/viewer/?uri=github.com/varlahin-gena/geoatlas)
[![CodeQL](https://github.com/varlahin-gena/geoatlas/actions/workflows/codeql.yml/badge.svg)](https://github.com/varlahin-gena/geoatlas/actions/workflows/codeql.yml)
[![Image scan](https://github.com/varlahin-gena/geoatlas/actions/workflows/image-scan.yml/badge.svg)](https://github.com/varlahin-gena/geoatlas/actions/workflows/image-scan.yml)
[![CI](https://github.com/varlahin-gena/geoatlas/actions/workflows/ci.yml/badge.svg)](https://github.com/varlahin-gena/geoatlas/actions/workflows/ci.yml)
[![golangci-lint](https://img.shields.io/github/actions/workflow/status/varlahin-gena/geoatlas/golangci-lint.yml?branch=main&label=golangci-lint&logo=go&logoColor=white)](https://github.com/varlahin-gena/geoatlas/actions/workflows/golangci-lint.yml)
[![coverage](https://img.shields.io/badge/coverage-%E2%89%A580%25-brightgreen)](.github/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/varlahin-gena/geoatlas?filename=backend/go.mod&logo=go&logoColor=white&label=Go)](backend/go.mod)
[![Node](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fraw.githubusercontent.com%2Fvarlahin-gena%2Fgeoatlas%2Fmain%2Ffrontend%2Fpackage.json&query=%24.engines.node&label=Node&logo=nodedotjs&logoColor=white&color=339933)](frontend/package.json)
[![OpenAPI](https://img.shields.io/badge/dynamic/yaml?url=https%3A%2F%2Fraw.githubusercontent.com%2Fvarlahin-gena%2Fgeoatlas%2Fmain%2Fopenapi.yaml&query=%24.info.version&label=OpenAPI&logo=openapiinitiative&logoColor=white&color=6BA539)](openapi.yaml)
[![Docs](https://img.shields.io/badge/docs-docs%2F-blue)](docs/README.md)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/varlahin-gena/geoatlas)](https://github.com/varlahin-gena/geoatlas/releases)

ГеоАтлас — визуализатор трафика МСЭ: принимает syslog, парсит логи вендоров, кладёт события в ClickHouse и рисует связи на 2D/3D карте. Польза — быстрее понять географию и характер потоков: репутация IP, аномалии (сканы, beaconing, lateral) и разбор алерта живут рядом с картой, а не в отдельных инструментах.

- **Парсеры МСЭ.** UserGate, FortiGate, Cisco ASA, Cisco FTD (Firepower) и формат событий SIEM KUMA; несетевые строки осознанно пропускаются, а не засоряют `parse_errors`.
- **Карта и глобус.** Связи на 2D-карте и 3D-глобусе; группировка узлов по IP, `/24`, городу или стране.
- **Аномалии рядом с картой.** Сканирование портов и обход подсети, резкий рост блокировок и трафика, beaconing, обход внутренней сети, новая страна, контакт с отмеченным адресом; разбор алерта и охоты без отдельного SIEM.
- **Репутация IP.** Offline-списки и URL-фиды: фильтр и подсветка дуг на карте.
- **Один пакет.** `geoatlas-X.Y.Z.tar.gz` для Ubuntu 20.04+ и Oracle Linux 8+ / RHEL; syslog `:514`, ClickHouse, React SPA на одном хосте (IPv4-only, доставка at-most-once).

**Актуальный релиз:** [v2.8.3](https://github.com/varlahin-gena/geoatlas/releases/tag/v2.8.3) (`VERSION` = 2.8.3, pre-release). Лимиты appliance — [архитектура](docs/architecture.md).

![Карта сетевых связей в ГеоАтлас](docs/images/map-overview.png)

Дуги из точки МСЭ по миру: **зелёные** — разрешено (`allowed`), **красные** — заблокировано (`blocked`).

## Содержание

- [Быстрый старт](#быстрый-старт)
- [Возможности](#возможности)
- [Документация](#документация)
- [Лицензия](#лицензия)
- [Безопасность](#безопасность)
- [Структура репозитория](#структура-репозитория)

## Быстрый старт

Нужен Linux-сервер (Ubuntu 20.04+ или Oracle Linux 8+ / RHEL-совместимый), Docker и остальные необходимые пакеты для работы системы поставит установщик, порты **80** (UI) и **514** tcp/udp (syslog).

```bash
VER=2.8.3

cd /tmp
curl -fLO "https://github.com/varlahin-gena/geoatlas/releases/download/v${VER}/geoatlas-${VER}.tar.gz"
curl -fLO "https://github.com/varlahin-gena/geoatlas/releases/download/v${VER}/geoatlas-${VER}.tar.gz.sha256"
sha256sum -c "geoatlas-${VER}.tar.gz.sha256"
tar -xzf "geoatlas-${VER}.tar.gz"
cd "geoatlas-${VER}"
```

```bash
sudo ./deploy/install.sh
```

Установщик сам определяет ОС (Ubuntu/Debian или Oracle Linux / RHEL / Rocky / Alma), ставит **полный стек** и спросит доступ к сайту (HTTP/HTTPS), режим производительности и пароль admin (мин. 10 символов, буква и цифра). После запуска:

1. Откройте `http://<IP_сервера>/` (при HTTPS — `https://…`).
2. Войдите как **admin** (роль `administrator`).
3. Направьте syslog МСЭ на `:514` — [syslog.md](docs/syslog.md); загрузите GeoIP — [geoip.md](docs/geoip.md).

Подробности, удаление и обновление — [docs/install.md](docs/install.md).

## Возможности

- **Приём логов** — syslog TCP/UDP `:514` и доп. порты коннекторов (UI **Система → Коннекторы**)
- **Авторизация** — учётка `admin` по умолчанию; роли `administrator` / `operator` / `dashboard`
- **HTTPS** — опционально на nginx со своими PEM (редирект HTTP→HTTPS)
- **GeoIP** — загрузка и правка базы из UI
- **ClickHouse** — хранение, настраиваемый TTL, резервное копирование
- **Расследование** — охоты (`/hunts`), разбор алерта (`/investigate`), конструктор поиска и шаблоны запросов на карте
- **Операции** — тест парсеров, журнал ошибок парсинга, страница системного мониторинга, логи контейнеров (`/dozzle/`)
- **Защита API** — доступ только через nginx, лимиты запросов, повторный вход, ротация токенов, HSTS при HTTPS

## Документация

| Раздел | Описание |
|--------|----------|
| [Архитектура](docs/architecture.md) | Сервисы, поток данных, product limits, Ingest SLO, TTL |
| [Установка](docs/install.md) | Требования, пакет, Ubuntu / Oracle Linux, удаление |
| [Syslog](docs/syslog.md) | Подключение МСЭ / SIEM |
| [Конфигурация](docs/configuration.md) | `.env`, секреты, модули |
| [Обслуживание](docs/operations.md) | Запуск, профили, retention, бэкапы, логи, обновление |
| [GeoIP](docs/geoip.md) | CSV, загрузка с сервера, 502/OOM |
| [UI и HTTP API](docs/ui.md) | Страницы SPA, роли, OpenAPI **1.23.0** |
| [Репутация](docs/reputation.md) | Фиды и offline-списки |
| [Аномалии](docs/anomalies.md) | Типы (`port_scan`, `byte_surge`, `beaconing`, `lateral_fanout`, …), ack/assign, разбор |
| [Разработка](docs/development.md) | Локальный стек из git |
| [CONTRIBUTING.md](CONTRIBUTING.md) | PR, DCO, coding standards, тесты |
| [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) | Кодекс поведения |
| [GOVERNANCE.md](GOVERNANCE.md) | Роли, решения, access continuity |
| [Roadmap](docs/roadmap.md) | Намерения на 12 месяцев |
| [Требования безопасности](docs/security-requirements.md) | Чего ждать / не ждать от продукта |
| [Assurance case](docs/assurance-case.md) | Threat model и аргумент защищённости |
| [Структура репо](docs/repo-layout.md) | Дерево каталогов |
| [RELEASING.md](RELEASING.md) | Версии продукта / OpenAPI / схемы CH |
| [SECURITY.md](SECURITY.md) | Сообщение об уязвимостях |
| [CHANGELOG.md](CHANGELOG.md) | История релизов |
| [OpenSSF Passing](docs/openssf-best-practices.md) | Шпаргалка Passing |
| [OpenSSF Silver](docs/openssf-silver.md) | Шпаргалка Silver |

Контракт REST: [`openapi.yaml`](openapi.yaml) (документ OpenAPI **1.23.0**).

## Лицензия

ГеоАтлас распространяется под [Apache License 2.0](LICENSE). Продукт бесплатный и общедоступный; релизы публикуются на GitHub.

Баги, вопросы и заказ доработок — через [Issues](https://github.com/varlahin-gena/geoatlas/issues). Как прислать патч — [CONTRIBUTING.md](CONTRIBUTING.md).

## Безопасность

Уязвимости — только приватно, см. [SECURITY.md](SECURITY.md) (подтверждение в течение 5 рабочих дней). Обычные баги — в Issues.

Кратко для прод-установки:

- Секреты только через `./start.sh`; пароль admin — минимум 10 символов (буква и цифра), одноразовый файл `.admin_password_once`
- UI за HTTPS при доступе из сети; не публиковать ClickHouse `8123`/`9000` и backend `8080`/`1514`
- Backend только за nginx (`GA_REQUIRE_PROXY=1`); syslog `:514` без TLS — ограничьте источником МСЭ
- Пакет только с GitHub Releases: проверяйте `.sha256` до установки / `./update.sh`

Полный checklist и threat model — [security-requirements.md](docs/security-requirements.md), [assurance-case.md](docs/assurance-case.md), [configuration.md](docs/configuration.md).

## Структура репозитория

| Путь | Назначение |
|------|------------|
| `backend/` | Go API, парсеры, GeoIP, ingest |
| `frontend/` | React SPA → nginx |
| `clickhouse/` | конфиг и скрипты CH |
| `deploy/` | единый установщик + OS-бэкенды Ubuntu / Oracle Linux |
| `docs/` | документация оператора и разработчика |
| `docs/images/` | скрины для README |
| `scripts/` | CI, pack-release, бэкапы |
| `openapi.yaml` | контракт HTTP API (OpenAPI **1.23.0**) |

Полное дерево: [docs/repo-layout.md](docs/repo-layout.md).
