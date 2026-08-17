# ГеоАтлас

Визуализатор сетевых взаимодействий на базе логов межсетевых экранов (МСЭ).
Принимает syslog с файрволов или из SIEM, парсит события, обогащает их геоданными
(IP → страна/координаты) и строит карту сетевых связей в веб-интерфейсе.

---

## Содержание

- [Возможности](#возможности)
- [Архитектура](#архитектура)
- [Требования](#требования)
  - [HTTPS (свои сертификаты)](#https-свои-сертификаты)
- [Быстрый старт](#быстрый-старт)
- [Установка](#установка)
  - [Установочный пакет](#установочный-пакет)
  - [Ubuntu (автоматическая)](#ubuntu-автоматическая)
  - [Oracle Linux / RHEL (автоматическая)](#oracle-linux--rhel-автоматическая)
  - [Ручная установка](#ручная-установка)
- [Удаление](#удаление)
  - [Быстрый старт (автоопределение ОС)](#быстрый-старт-автоопределение-ос)
  - [Ubuntu / Debian](#ubuntu--debian)
  - [Oracle Linux / RHEL](#oracle-linux--rhel)
  - [Остановка без удаления проекта](#остановка-без-удаления-проекта)
- [Запуск и остановка](#запуск-и-остановка)
- [Обслуживание](#обслуживание)
  - [Профили производительности](#профили-производительности)
  - [Срок хранения (TTL / retention)](#срок-хранения-ttl--retention)
  - [Резервное копирование ClickHouse](#резервное-копирование-clickhouse)
  - [Очистка данных ClickHouse](#очистка-данных-clickhouse)
  - [Мониторинг ingest](#мониторинг-ingest)
  - [Логи и диагностика](#логи-и-диагностика)
  - [Обновление системы](#обновление-системы)
  - [Пересборка образов](#пересборка-образов)
- [GeoIP](#geoip)
- [Веб-интерфейс](#веб-интерфейс)
  - [HTTP API](#http-api)
- [Лицензия](#лицензия)
- [Безопасность](#безопасность)
- [Структура репозитория](#структура-репозитория)

---

## Возможности

- Приём логов по **syslog** (TCP/UDP, порт 514)
- Ручная загрузка логов через веб-интерфейс
- Парсинг **UserGate, FortiGate, Cisco ASA, Cisco FTD (FirePower), Cowrie (honeypot)** и универсальный фолбэк (Generic KV)
- **Осознанный пропуск** событий: распознанные, но несетевые строки (например, часть `cowrie.*`) не попадают в `parse_errors`
- **Авторизация по умолчанию**: одна seed-учётка `admin`; роли `administrator` / `operator` (operator — через UI `/users`); cookie-сессии, CSRF, Bearer `API_AUTH_TOKEN` (+ `API_AUTH_PREVIOUS_TOKEN`); именованные API-токены со scopes `read`/`ops`/`admin` (UI `/api-tokens`); управление УЗ в UI
- Веб-интерфейс — **React SPA** (Vite + TypeScript); clean paths, legacy `*.html` редиректятся
- Опциональный **HTTPS** на nginx со своими PEM-сертификатами (редирект HTTP→HTTPS)
- Загрузка и правка **GeoIP-базы** (CSV SIEM KUMA, `/geo-ranges`, IP без координат на `/geo-missing`); лимиты upload по профилю, очистка базы, большие CSV — с сервера через `curl` (см. [GeoIP](#geoip))
- Установка **«Сделай мне хорошо»** (`NM_FULL_AUTO` / `--full-auto`): релиз, все модули, порт 8080, автопрофиль, firewall allowlist (UI + :514)
- **Один установочный пакет** `geoatlas-X.Y.Z.tar.gz` для Ubuntu и Oracle Linux / RHEL; обновление без `git pull` через `./update.sh`
- Toast-уведомления без автоскрытия (крестик), сохраняются при смене страниц до ручного закрытия
- **Репутация IP** (модуль опционален при установке): offline-списки и URL-фиды (`/reputation`), каталог публичных источников, фильтр и подсветка дуг на карте; приватные IP не помечаются
- Хранение и аналитика в **ClickHouse**; дневные geo-агрегаты для пресетов `1d+` (city/country)
- **Резервное копирование ClickHouse**: расписание из UI, CLI backup/restore, неразрушающее «Подключить» (shadow `nm_bak_*`) для просмотра на карте
- **Настраиваемый TTL (retention)** таблиц из UI `/system`
- Построение связей на карте (2D MapLibre) и глобусе (3D MapLibre Globe); на карту попадают только узлы/рёбра с координатами
- Полный mesh дуг + viewport-fit zoom; heatmap стран (на глобусе отключён) + sparkline по клику на страну; экспорт PNG; светлая/тёмная тема
- Фильтрация: все / разрешённые / заблокированные (на клиенте); опционально «один цвет линий»
- **Конструктор поиска** на карте (гибридный query builder) и **личные шаблоны** запросов; у администратора — просмотр всех шаблонов
- Группировка узлов: по IP / по подсети `/24` / **по городу (по умолчанию)** / по стране; при отсутствии города — фолбэк на центр страны
- **Тест парсеров** в браузере: статусы parsed / skipped / error, гео-обогащение, пресеты по вендорам
- **Журнал ошибок парсинга**: поиск, выборочное и полное удаление, передача строк в «Тест парсеров»
- Страница системного мониторинга (Обзор / Pipeline / Безопасность / Графики / **Резервное копирование**): метрики контейнеров, пайплайна (в т.ч. **UDP/TCP EPS**, drops, circuit breaker), **форма TTL**, неуспешные логины, хранилище, профиль установки, **индикатор ёмкости**, алёрты; ручной maintenance backfill агрегатов
- Индикатор здоровья системы на главной странице (ссылка на `/system`); **версия установки** (`main` / тег) в меню пользователя
- Docker: fail-closed секреты в compose, hardened контейнеры (`cap_drop: ALL`); запуск через `./start.sh`
- Контракт HTTP API: [`openapi.yaml`](openapi.yaml) (OpenAPI **1.11.0**)

---

## Архитектура

Система состоит из **пяти сервисов**, оркеструемых через Docker Compose.
Ядро всегда поднимается: **clickhouse + backend + frontend**.
`syslog-ng` и `stats-collector` включаются через compose-профили (`COMPOSE_PROFILES=syslog,stats` — по умолчанию в `start.sh`, если в `.env` ещё нет).

```
МСЭ (UserGate, FortiGate, …) или SIEM
      │ syslog (514/tcp, 514/udp)          [профиль syslog]
      ▼
 ┌───────────┐   TCP :1514 (@@nm/udp/@@, @@nm/tcp/@@)                      ┌───────────┐
 │ syslog-ng │ ──────────────────────────────────────────────────────────► │ backend   │
 └───────────┘                                                             │ (Go)      │
                                                                           │ parser +  │
 пользователь ── login / upload-logs / upload-geo ───►                     │ geoip +   │
                                                                           │ ingest +  │
                                                                           │ aggregator│
 ┌────────────────┐                                                        └────┬──────┘
 │ stats-collector│ ──────────────────────────────────────────────────────────► │ ClickHouse
 └────────────────┘   system_metrics   [профиль stats]                          │ (25.8)
                                                                          ┌─────▼───────┐
 браузер ────────── :80 / :443 ───────► frontend (nginx) ── /api/* ─────► │ traffic_*   │
                                      auth_request                        │ geo_ranges  │
                                                                          │ parse_errors│
                                                                          └─────────────┘
```

### Сервисы

| Сервис            | Контейнер          | Профиль     | Назначение                                              |
|-------------------|--------------------|-------------|---------------------------------------------------------|
| `frontend`        | `frontend`         | *(ядро)*    | Веб-интерфейс, nginx, auth_request, прокси `/api/*`     |
| `backend`         | `backend`          | *(ядро)*    | Парсинг, GeoIP, ingest, агрегация, HTTP API, auth       |
| `clickhouse`      | `clickhouse`       | *(ядро)*    | Хранилище и аналитика (только внутренняя docker-сеть)   |
| `syslog-ng`       | `syslog-ng`        | `syslog`    | Приём syslog от МСЭ, буферизация, передача в backend    |
| `stats-collector` | `stats-collector`  | `stats`     | Сбор системных метрик контейнеров в ClickHouse          |

### Поток данных

1. **Syslog**: МСЭ отправляет события на `<IP_сервера>:514` (TCP или UDP).
2. **syslog-ng** принимает сообщения и пересылает их по TCP на `backend:1514` с маркерами транспорта (`@@nm/udp/@@` / `@@nm/tcp/@@`).
3. **backend** снимает маркеры транспорта, парсит строки, обогащает GeoIP, пишет в ClickHouse; при старте `EnsureBaseSchema` / `Ensure*` создаёт базовые таблицы и агрегаты (`traffic_edges_*`, geo), применяет TTL из `retention.json` и при необходимости делает backfill. **Delivery contract — at-most-once / best-effort:** полная очередь ingest **не блокирует** TCP — лишние строки дропаются (`dropped_total`); при outage ClickHouse insert circuit ставит dequeue на паузу (очередь растёт → admission drops), а потери из processor-буфера учитываются отдельно (`buffer_drops_total`). Перед backend **syslog-ng уже буферизует** (см. ниже). Алерты и live-метрики drops — на `/system`.
4. **frontend** отдаёт статику и проксирует API-запросы на backend.
5. **stats-collector** каждые 30 секунд собирает метрики CPU/RAM контейнеров и состояние пайплайна (включая разбивку UDP/TCP).

**Буфер syslog-ng (включён по умолчанию):** в `syslog-ng.conf` у назначений UDP/TCP стоят `disk-buffer` (`reliable(no)`). Окно в RAM — `flow-control-window-size` (число сообщений, не байты: в 4.11 `mem-buf-size` с `reliable(no)` игнорируется), диск — `capacity-bytes`. Размеры **fifo / window / disk** задаёт профиль (`syslog-ng.d/zz_profile.conf`, пишет `start.sh` или `tune-resources.sh`). Без файла действуют безопасные дефолты под compose 1 GiB. Это сглаживает краткие пики/рестарты backend; после буфера доставка в backend всё равно at-most-once. Потери **до** backend видны на стадии Syslog-NG (`/system`, `pipeline.syslogng.dropped_total` / `queued`); drops очереди ingest — `/api/ingest/stats`.

syslog-ng **4.11** (`balabit/syslog-ng:4.11.0`): healthcheck `syslog-ng-ctl stats`, внутренний stats-exporter `:9577` (не публикуется). Backend скрейпит live (`SYSLOG_STATS_URL`), stats-collector пишет историю в `system_metrics`.

### Product limits (appliance)

| Ограничение | Суть |
|-------------|------|
| **IPv4-only** | Success path GeoIP / карта / lookup — IPv4. IPv6 не обогащается и не строится на карте. |
| **Single-host control plane** | Учётки, API-токены, retention, reputation feeds, schedules — JSON на `/app/data`. Backend берёт exclusive lock (`/app/data/.nm_backend.lock`); второй процесс на том же томе не стартует. Обход только для стендов: `NM_ALLOW_MULTI_INSTANCE=1` (небезопасно при shared volume). |

### Ingest SLO (at-most-once)

Доставка — **best-effort / at-most-once** (очередь может дропать). Product SLO описывает, когда потери — инцидент (алёрты на `/system`, поле `ingest_slo` в `/api/system/stats`, метрики Prometheus):

| Сигнал | Warn | Critical (error) |
|--------|------|------------------|
| Queue depth / byte budget | ≥ 75% capacity | ≥ 90% |
| Processor buffer lines | > 10k | > 100k |
| Admission / buffer / syslog-ng drops/s | любое > 0 | ≥ 100/s |
| Insert circuit open | да (warn) | (эскалация через queue/drops) |
| Pipeline lag (при ненулевом rate) | > 60s | > 300s |
| EPS vs install profile max | > 105% | > 125% |

Runbook кратко: warn drops → проверить стадию Syslog-NG (queued/dropped) / syslog-ng buffer / profile / CH; critical → `tune-resources.sh` или снизить входной EPS; circuit open → здоровье ClickHouse.

### Хранение данных (TTL)

Дефолты ниже, менять можно в UI (`/system` → Pipeline → «Срок хранения»).

| Таблица / объект                    | Срок по умолчанию | Источник дефолта                             |
|-------------------------------------|-------------------|----------------------------------------------|
| `traffic_logs`                      | 30 дней           | EnsureBaseSchema / retention `traffic_logs_days`   |
| `traffic_edges_daily` (+ MV)        | 30 дней           | `EnsureEdgesAgg` / retention `edges_days`    |
| `traffic_edges_city_daily` (+ MV)   | 30 дней           | `EnsureGeoEdgesAgg` / retention `edges_days` |
| `traffic_edges_country_daily` (+ MV)| 30 дней           | `EnsureGeoEdgesAgg` / retention `edges_days` |
| `parse_errors`                      | 7 дней            | EnsureBaseSchema / retention `parse_errors_days`   |
| `system_metrics`                    | 7 дней            | EnsureBaseSchema / retention `system_metrics_days` |
| `geo_ranges`                        | без TTL           | EnsureBaseSchema (не настраивается)                |
| `nm_schema_version`                 | без TTL           | `Ensure*` (метаданные схемы)                 |

Допустимый диапазон настраиваемых дней: **1…730**. На `traffic_logs` / `parse_errors` / `system_metrics` / `traffic_edges_*` включён `ttl_only_drop_parts` при **дневных** партициях: истечение удаляет дневную партицию целиком (без дорогих row-level TTL merges). Уменьшение TTL удалит старые партиции при следующем TTL merge/drop в ClickHouse.

Системные логи ClickHouse (`trace_log`, `text_log`, `metric_log`, …) по умолчанию **отключены** (`clickhouse/config.d/z_system_logs.xml`): на малых объёмах данных они иначе раздуваются сильнее `traffic_logs` и держат высокий idle CPU. `query_log` остаётся включённым. Образ ClickHouse в compose: **`clickhouse/clickhouse-server:25.8.29.51`**.

---

## Требования

### Аппаратные (минимум / рекомендуется)

| Ресурс   | Минимум              | Рекомендуется         |
|----------|----------------------|-----------------------|
| CPU      | 2 ядра               | 4–8 ядер              |
| RAM      | 4 GiB                | 8–16 GiB              |
| Диск     | 20 GiB свободно      | 50+ GiB (логи, CH)    |

При установке скрипт автоматически определяет ресурсы и предлагает профиль производительности (см. [Профили производительности](#профили-производительности)).

### Программные

- **ОС**: Linux (Ubuntu 20.04+, Oracle Linux 8+, Rocky/Alma/RHEL/CentOS)
- **Docker Engine** 24+ с плагином **docker compose**
- Открытые порты **80** (UI HTTP), при HTTPS — ещё **443**, и **514/tcp**, **514/udp** (syslog)
- Доступ к `/proc` и `/sys/fs/cgroup` хоста (нужен `stats-collector`)

### Сетевые порты

| Порт        | Протокол | Назначение              | Доступ снаружи |
|-------------|----------|-------------------------|----------------|
| 80          | TCP      | Веб-интерфейс (HTTP)    | Да             |
| 443         | TCP      | Веб-интерфейс (HTTPS)   | Опционально    |
| 514         | TCP/UDP  | Syslog от МСЭ           | Да             |
| 8080        | TCP      | Backend API             | Нет (docker)   |
| 1514        | TCP      | Ingest от syslog-ng     | Нет (docker)   |
| 8123 / 9000 | TCP      | ClickHouse HTTP/native  | Нет (docker)   |

### HTTPS (свои сертификаты)

1. Положите PEM в `certs/fullchain.pem` и `certs/privkey.pem` (см. `certs/README.md`).
2. Переменные в `.env`:

| Переменная     | По умолчанию | Назначение |
|----------------|--------------|------------|
| `HTTPS_ENABLED`| `auto`       | `1`/`true` — вкл.; `0` — выкл.; `auto` — вкл. если есть оба PEM |
| `HTTPS_PORT`   | `443`        | Хостовый порт TLS |
| `HTTP_REDIRECT`| `1`          | Редирект HTTP→HTTPS |
| `HTTP_PORT`    | `80`         | HTTP (и редирект) |

```env
HTTPS_ENABLED=1
HTTPS_PORT=443
HTTP_PORT=80
HTTP_REDIRECT=1
```

`HTTPS_ENABLED=auto` (дефолт) тоже включит TLS, если оба файла на месте.

Установщик (Ubuntu / Oracle Linux) спрашивает HTTPS и в пошаговом режиме, и в «Сделай мне хорошо» (при TTY). Без вопросов / CI:

| Env | Назначение |
|-----|------------|
| `NM_HTTPS_ENABLED` / `HTTPS_ENABLED` | `1` / `0` / `auto` |
| `NM_HTTPS_PORT` / `HTTPS_PORT` | порт TLS (по умолчанию 443) |
| `NM_SSL_CERT_SRC` + `NM_SSL_KEY_SRC` | пути к PEM для копирования в `certs/` |
| `NM_CERTS_DIR` | каталог с `fullchain.pem`+`privkey.pem` (или `cert.pem`+`key.pem`) |

3. Запуск через `./start.sh` / `./stop.sh` (или `deploy/common/compose.sh`) — подхватывает `docker-compose.https.yml` и публикует `:443`. Голый `docker compose` без `-f docker-compose.https.yml` порт `:443` не опубликует.
4. UI: `https://<host>/`. HTTP при `HTTP_REDIRECT=1` уходит на HTTPS.

Ключи не коммитятся (`.gitignore`). Syslog на `:514` по-прежнему без TLS.

---

## Быстрый старт

После установки (см. ниже) система доступна по адресу:

```
http://<IP_сервера>/
```

При HTTPS (сертификаты в `certs/` + `HTTPS_ENABLED`): `https://<IP_сервера>/`.

По умолчанию включена UI-авторизация. Первый вход — учётка **admin** (роль administrator). Пароля по умолчанию нет:

- пошаговая установка спрашивает пароль (дважды, мин. 8 символов);
- `--full-auto` / нет TTY / голый `./start.sh` — берут `AUTH_ADMIN_PASSWORD` из окружения или генерируют одноразовый и пишут его в `.admin_password_once` (права 600; удалите после входа).

Если пароль сгенерирован, при входе его нужно сменить (`must_reset_password`). Operator с завода не создаётся — заведите в UI `/users`.
`./start.sh` при необходимости генерирует `API_AUTH_TOKEN`, `SESSION_SECRET`, пароль admin и `CLICKHOUSE_PASSWORD` в `.env` (ключи без значений — [`.env.example`](.env.example)).
Голый `docker compose up` без `AUTH_ADMIN_PASSWORD` / `CLICKHOUSE_PASSWORD` в `.env` не стартует (fail-closed).

---

## Установка

Каталог установки по умолчанию: **`/opt/network-monitor`**.

### Установочный пакет

Один архив **`geoatlas-X.Y.Z.tar.gz`** (плюс `.sha256`) с GitHub Release — это исходники стека, **оба** установщика (`deploy/ubuntu/…` и `deploy/oracle_linux/…`) и `update.sh`. Это не `.deb`/`.rpm`: пакет Docker и файрвол хоста по-прежнему ставит OS-скрипт.

| Задача | Что запускать | Пакет |
|--------|----------------|-------|
| Первая установка, Ubuntu | `deploy/ubuntu/install_ubuntu.sh` | тот же tar.gz (или скачать релиз с GitHub) |
| Первая установка, Oracle Linux / RHEL | `deploy/oracle_linux/install_oraclelinux.sh` | тот же tar.gz |
| Обновление уже стоящей системы | `/opt/network-monitor/update.sh` | тот же tar.gz |

Источник в TUI установщика: **релиз** (скачать tar.gz с GitHub), **`main`** (git) или **локальный пакет**. Режим `release` без локального файла сам берёт `geoatlas-X.Y.Z.tar.gz` с Releases; если asset ещё нет — `git clone` тега.

Локально собрать архив (для проверки, не вместо CI на теге): `bash scripts/pack-release.sh` → `dist/geoatlas-<VERSION>.tar.gz`.

### Ubuntu (автоматическая)

Скрипт устанавливает Docker, **спрашивает источник** (последний релиз, ветка `main` или локальный пакет `.tar.gz` — тот же архив, что и для Oracle Linux), получает исходники, **интерактивно предлагает модули** и профиль производительности, настраивает UFW и запускает стек.

Диалоги установки и удаления — **TUI** (`whiptail` → `dialog` → текст). Долгие шаги (apt/Docker/исходники) показывают **gauge**.

```bash
# Скачать скрипт (или скопировать из репозитория)
curl -fsSL -o install_ubuntu.sh \
  https://raw.githubusercontent.com/varlahin-gena/network_monitor/main/deploy/ubuntu/install_ubuntu.sh
chmod +x install_ubuntu.sh

# Запуск от root
sudo ./install_ubuntu.sh
```

**«Сделай мне хорошо» (полный авто):** без вопросов ставит последний релиз, все модули, порт **8080**, автопрофиль по ресурсам, **включает** host firewall с allowlist портов (UI и при syslog `:514/tcp+udp`) и запускает стек. `:514` без TLS/auth — ограничьте источником МСЭ или `NM_SYSLOG_ALLOW_FROM=CIDR`. Выключить firewall как раньше: `NM_DISABLE_HOST_FIREWALL=1`. HTTPS при TTY спрашивается первым среди сетевых шагов (или задайте `NM_HTTPS_ENABLED` / сертификаты через env).

```bash
sudo NM_FULL_AUTO=1 ./install_ubuntu.sh
# или
sudo ./install_ubuntu.sh --full-auto
```

После установки UI: `http://<host>:8080`. В интерактивном TUI тот же режим — первый пункт radiolist «Сделай мне хорошо».

**Что делает скрипт:**

1. Обновляет списки пакетов
2. Устанавливает `curl`, `git`, `ufw`, `whiptail` (опционально `dialog`)
3. Устанавливает Docker Engine и compose plugin (если ещё нет)
4. Спрашивает источник: **последний GitHub Release**, ветка **`main`** или **локальный пакет** (`.tar.gz`)
5. Скачивает пакет релиза / клонирует репозиторий / накладывает локальный архив в `/opt/network-monitor`
6. Спрашивает, какие модули ставить (checklist: авторизация, API-токен, syslog-ng, stats-collector, репутация IP)
7. Спрашивает **HTTPS** (свои PEM; можно оставить только HTTP)
8. Спрашивает **порт(ы)**: при HTTPS — порт TLS, затем HTTP (редирект); при HTTP-only — порт UI (80 / 8080 или свой)
9. Запускает детектор ресурсов и предлагает профиль
10. Настраивает UFW (HTTP, при HTTPS — TLS-порт, и при необходимости 514)
11. Вызывает `./start.sh` (можно отказаться на последнем шаге)

**TUI / неинтерактивный режим:**

| Переменная | Назначение |
|------------|------------|
| `NM_UI=whiptail\|dialog\|text` | принудительный бэкенд диалогов |
| `NM_FULL_AUTO=1` / `--full-auto` | «Сделай мне хорошо»: релиз, все модули, порт 8080, автопрофиль, firewall allowlist, старт стека |
| `NM_DISABLE_HOST_FIREWALL=1` | full-auto: выключить UFW/firewalld (как раньше; небезопасно на публичном IP) |
| `NM_SYSLOG_ALLOW_FROM` | CIDR/IP: открыть `:514` только с этого источника (UFW/firewalld) |
| `AUTH_ADMIN_PASSWORD` | пароль admin для full-auto / без TTY; иначе установщик спросит или сгенерирует |
| `NM_AUTO_MODULES=1` | без вопросов: модули по умолчанию, порт 80, стабильный релиз |
| `NM_INSTALL_PACKAGE` | путь к `.tar.gz` или распакованному каталогу (источник `package`) |
| нет TTY (CI/pipe) | то же, что авто-режим; gauge пишет прогресс в лог |

**Источник кода (интерактивно или через env):**

| Режим | Env | Что ставится |
|-------|-----|--------------|
| Последний релиз (по умолчанию в UI) | `NM_INSTALL_SOURCE=release` | пакет `geoatlas-X.Y.Z.tar.gz` с GitHub Releases; если asset ещё нет — `git clone` тега |
| Ветка main | `NM_INSTALL_SOURCE=main` | `main` — последние изменения (`git clone` / `git pull`) |
| Локальный пакет | `NM_INSTALL_SOURCE=package` + `NM_INSTALL_PACKAGE=/path/to/geoatlas-X.Y.Z.tar.gz` | архив или распакованный каталог, без git |
| Явный ref | `BRANCH=v1.1.4` | указанная ветка/тег без вопроса |

`NM_INSTALL_PREFER_GIT=1` — для `release` не скачивать tar.gz, а клонировать тег. `NM_INSTALL_PACKAGE_SHA256=<hex>` — проверить контрольную сумму пакета.

**Порт UI:** `HTTP_PORT=8080` (или `NM_HTTP_PORT`) — без вопроса; в compose: `${HTTP_PORT:-80}:80`. HTTPS спрашивается **до** HTTP-порта (или через env: `NM_HTTPS_ENABLED`, `NM_HTTPS_PORT`, `NM_SSL_CERT_SRC` / `NM_SSL_KEY_SRC`, `NM_CERTS_DIR`; см. [HTTPS](#https-свои-сертификаты)).

**Выбор модулей (интерактивно или через env):**

| Модуль           | По умолчанию | Что даёт                                                |
|------------------|--------------|---------------------------------------------------------|
| UI-авторизация   | вкл.         | логин, роли admin/operator (`AUTH_DISABLED` при отказе) |
| API Bearer-токен | вкл.         | защита мутирующих API (`API_AUTH_DISABLED` при отказе)  |
| syslog-ng        | вкл.         | приём syslog на `:514` (Compose profile `syslog`)       |
| stats-collector  | вкл.         | метрики / `/system` (Compose profile `stats`)       |
| Репутация IP     | вкл.         | модуль целиком; при отказе `REPUTATION_FETCH_ENABLED=false` (API/UI/фиды выкл.) |

Ядро (ClickHouse + Backend + Frontend) ставится всегда.

---

### Oracle Linux / RHEL (автоматическая)

Поддерживаются Oracle Linux, RHEL, Rocky Linux, AlmaLinux, CentOS. Установочный **`geoatlas-X.Y.Z.tar.gz` тот же**, что для Ubuntu (см. [Установочный пакет](#установочный-пакет)).

```bash
curl -fsSL -o install_oraclelinux.sh \
  https://raw.githubusercontent.com/varlahin-gena/network_monitor/main/deploy/oracle_linux/install_oraclelinux.sh
chmod +x install_oraclelinux.sh

sudo ./install_oraclelinux.sh
```

Полный авто (как на Ubuntu): `sudo NM_FULL_AUTO=1 ./install_oraclelinux.sh` или `sudo ./install_oraclelinux.sh --full-auto` → UI на порту **8080**, firewalld включён (порты UI + :514).

**Что делает скрипт:**

1. Удаляет конфликтующие пакеты (`podman`, `buildah`, `runc`) при необходимости
2. Устанавливает Docker CE из официального репозитория
3. Настраивает SELinux (`container_manage_cgroup`) или переводит в permissive (опционально)
4. Спрашивает источник (релиз / `main` / локальный пакет), получает исходники, предлагает модули, HTTPS, HTTP-порт, профиль, настраивает firewalld и запускает стек

---

### Ручная установка

Если автоматические скрипты не подходят:

```bash
# 1. Установить Docker и compose plugin (см. docs.docker.com)

# 2a. Из пакета релиза (без git; Ubuntu и Oracle Linux одинаково)
#     tar -xzf geoatlas-X.Y.Z.tar.gz && sudo mkdir -p /opt
#     sudo cp -a geoatlas-X.Y.Z /opt/network-monitor
# 2b. Или клонировать репозиторий
sudo mkdir -p /opt
sudo git clone -b main https://github.com/varlahin-gena/network_monitor.git /opt/network-monitor
cd /opt/network-monitor

# 3. Права на скрипты
chmod +x start.sh stop.sh update.sh scripts/tune-resources.sh \
  deploy/common/detect_resources.sh deploy/common/select_modules.sh deploy/common/ui.sh \
  deploy/common/admin_auth.sh

# 4. (Рекомендуется) Выбрать модули и профиль производительности
./deploy/common/select_modules.sh .
./scripts/tune-resources.sh
# или неинтерактивно:
# NM_ENABLE_AUTH=0 NM_AUTO_MODULES=1 ./deploy/common/select_modules.sh .
# NM_AUTO_PROFILE=1 ./deploy/common/detect_resources.sh .
# 5. Запуск
./start.sh
```

Открыть порты вручную:

```bash
# Ubuntu (UFW)
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp   # если HTTPS
sudo ufw allow 514/tcp
sudo ufw allow 514/udp

# Oracle Linux / RHEL (firewalld)
sudo firewall-cmd --permanent --add-port=80/tcp
sudo firewall-cmd --permanent --add-port=443/tcp   # если HTTPS
sudo firewall-cmd --permanent --add-port=514/tcp
sudo firewall-cmd --permanent --add-port=514/udp
sudo firewall-cmd --reload
```

---

## Удаление

### Быстрый старт (автоопределение ОС)

```bash
cd /opt/network-monitor   # или каталог, откуда запускали скрипт
sudo bash deploy/uninstall.sh
```

Скрипт покажет **аудит** (контейнеры, volumes, размер каталога) и предложит **интерактивное меню** (whiptail / dialog):
1. Безопасное удаление — stop + файлы + firewall, данные ClickHouse сохраняются
2. Полное удаление (purge) — включая volumes и образы
3. Только остановить стек
4. Настроить вручную

Долгие шаги (compose down, rm, firewall) идут через **gauge**. Бэкенд диалогов тот же, что при установке (`NM_UI`).

### Ubuntu / Debian

```bash
sudo bash deploy/ubuntu/uninstall_ubuntu.sh
```

### Oracle Linux / RHEL

```bash
sudo bash deploy/oracle_linux/uninstall_oraclelinux.sh
```

### Остановка без удаления проекта

```bash
cd /opt/network-monitor
./stop.sh
```

---

## Запуск и остановка

```bash
cd /opt/network-monitor

# Запуск (с пересборкой образов)
./start.sh

# Запуск без пересборки
DO_BUILD=0 ./start.sh

# Остановка (данные сохраняются)
./stop.sh

# Обновление из пакета (тома Docker и .env не трогает) — см. ниже
# sudo ./update.sh /path/to/geoatlas-X.Y.Z.tar.gz
```

**Прямые команды Docker Compose:**

> Предпочтительно `./start.sh` (генерирует секреты в `.env`, подключает HTTPS-override).
> Голый `docker compose up` без `.env` с `API_AUTH_TOKEN` / `SESSION_SECRET` / `CLICKHOUSE_PASSWORD` / паролями — ошибка подстановки.
> Без `-f docker-compose.https.yml` порт `:443` не публикуется.

```bash
cd /opt/network-monitor

# Статус контейнеров
docker compose ps

# Запуск в фоне (нужен заполненный .env)
docker compose up -d

# Запуск с пересборкой
docker compose up -d --build

# Остановка
docker compose down

# Остановка с удалением томов
docker compose down -v

# Перезапуск одного сервиса
docker compose restart backend
docker compose restart clickhouse
```

---

## Обслуживание

### Профили производительности

При установке скрипт `deploy/common/detect_resources.sh` анализирует CPU, RAM и диск, затем генерирует:

| Файл                                       | Назначение                                              |
|--------------------------------------------|---------------------------------------------------------|
| `docker-compose.override.yml`              | Лимиты CPU/RAM контейнеров, параметры ingest, `CH_MAX_THREADS` |
| `.env`                                     | Переменные для compose (`SYSLOG_STATS_URL` при модуле syslog) |
| `clickhouse/users.d/zz_install_limits.xml` | Лимиты памяти и `max_threads` запросов ClickHouse       |
| `syslog-ng.d/zz_profile.conf`              | `@define` буферов syslog-ng (fifo / window / disk)      |
| `install-profile.json`                     | Сводка профиля (отображается в UI «Система»)            |

**Буферы syslog-ng по профилю** (на каждое из двух назначений UDP/TCP; `reliable(no)`; TCP `max-connections(64)`, `log-iw-size` ≥ 6400):

| Профиль  | RAM контейнера | fifo / window (сообщений) | disk-buf | UDP rcvbuf |
|----------|----------------|---------------------------|----------|------------|
| `tiny`   | 512 MiB        | 10 000                    | 256 MiB  | 16 MiB     |
| `small`  | 768 MiB        | 25 000                    | 512 MiB  | 32 MiB     |
| `medium` | 1 GiB          | 50 000                    | 1 GiB    | 64 MiB     |
| `large`  | 2 GiB          | 100 000                   | 2 GiB    | 128 MiB    |
| `xlarge` | 4 GiB          | 200 000                   | 4 GiB    | 128 MiB    |

Запрошенные `so-rcvbuf` / `so-sndbuf` применятся только если хост позволяет (`net.core.rmem_max` / `wmem_max`). Иначе в логе syslog-ng будет `The kernel refused to set the receive/send buffer` и останется ~416 KiB — на приём это не влияет, на пиках UDP возможны потери в сокете. Пример (не в compose по умолчанию):

```bash
sysctl -w net.core.rmem_max=134217728
sysctl -w net.core.wmem_max=16777216
```

**Доступные профили:**

| Профиль  | Сервер (ориентир)       | ClickHouse    | Backend       | Workers | EPS (событий/с) |
|----------|-------------------------|---------------|---------------|---------|-----------------|
| `tiny`   | ≤2 CPU / ≤4 GiB RAM     | 2 GiB         | 1 GiB         | 1       | 500 – 2 000     |
| `small`  | ≤4 CPU / ≤8 GiB RAM     | 3 GiB / 3 CPU | 2 GiB / 2 CPU | 2       | 5 000 – 12 000  |
| `medium` | ≤8 CPU / ≤16 GiB RAM    | 6 GiB         | 4 GiB         | 4       | 10 000 – 25 000 |
| `large`  | ≤16 CPU / ≤32 GiB RAM   | 12 GiB        | 8 GiB         | 8       | 25 000 – 80 000 |
| `xlarge` | >16 CPU / >32 GiB RAM   | 24 GiB        | 16 GiB        | 12      | 80 000 – 200 000|

Профиль также задаёт **`CH_MAX_THREADS`** (~половина ядер ClickHouse, потолок 4; по умолчанию 2): лимит потоков на тяжёлые `GROUP BY` карты, чтобы refresh не утилизировал все ядра. Варианты в установщике — только `tiny`…`xlarge`.

**Пересчёт профиля после изменения ресурсов сервера:**

```bash
cd /opt/network-monitor

# Интерактивный выбор
./scripts/tune-resources.sh

# Автоматически — рекомендованный профиль
NM_AUTO_PROFILE=1 ./scripts/tune-resources.sh

# Принудительно задать профиль
NM_FORCE_PROFILE=large ./scripts/tune-resources.sh

```

Скрипт `tune-resources.sh` автоматически перезапускает стек, если он уже запущен.

Просмотр текущего профиля:

```bash
cat /opt/network-monitor/install-profile.json | jq .
# или через API:
TOKEN="$(grep -E '^API_AUTH_TOKEN=' .env | cut -d= -f2-)"
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1/api/system/install-profile | jq .
```

---

### Срок хранения (TTL / retention)

TTL таблиц ClickHouse настраивается без правки сгенерированного `init.sql`. Настройки хранятся в JSON на томе `auth-users` (`RETENTION_FILE`, по умолчанию `/app/data/retention.json`) и применяются:

1. при **старте backend** (после Ensure*, usecase `retention.ApplyFromStore`);
2. сразу при **сохранении** из UI или `PUT /api/system/retention` (`ALTER TABLE … MODIFY TTL`).

| Поле JSON             | Таблицы                                                                          | Дефолт |
|-----------------------|----------------------------------------------------------------------------------|--------|
| `traffic_logs_days`   | `traffic_logs`                                                                   | 30     |
| `edges_days`          | `traffic_edges_daily`, `traffic_edges_city_daily`, `traffic_edges_country_daily` | 30     |
| `parse_errors_days`   | `parse_errors`                                                                   | 7      |
| `system_metrics_days` | `system_metrics`                                                                 | 7      |

Диапазон каждого поля: **1…730** дней. `geo_ranges` без TTL.

**UI:** `/system` → вкладка **Pipeline** → блок «Срок хранения (TTL)» (только administrator).

---

### Резервное копирование ClickHouse

Native `BACKUP` / `RESTORE` на отдельный Docker-том `clickhouse-backups` (disk `backups` → `/var/lib/clickhouse-backups`). Это **не** замена HA: single-node appliance, бэкап защищает от потери `clickhouse-data` / ошибок оператора.

| Что | Детали |
|-----|--------|
| Конфиг disk | `clickhouse/config.d/backups.xml` |
| Том | `clickhouse-backups` (отдельно от `clickhouse-data`) |
| Скрипты | `scripts/backup-clickhouse.sh`, `scripts/restore-clickhouse.sh` |
| По умолчанию в бэкап | `traffic_logs`, `geo_ranges`, `reputation_ranges`, `parse_errors`, `system_metrics`, `traffic_edges_*` |
| Рядом | `*.auth.tgz` — снимок `/app/data` (users, retention, feeds, schedule); без `geo_index.snap`, `.nm_backend.lock`, `*.tmp` |

После добавления тома на уже установленном стенде:

```bash
cd /opt/network-monitor
docker compose up -d --build   # volume-perms (uid 101) + mount + backups.xml + backend
chmod +x scripts/backup-clickhouse.sh scripts/restore-clickhouse.sh
./scripts/backup-clickhouse.sh
```

Backend и ClickHouse пишут в `clickhouse-backups` под **одним uid 101**. Сервис `volume-perms` при старте делает `chown 101:101` на томах `clickhouse-backups` и `auth-users` (миграция со старого backend uid 10001). Если снова `permission denied` на `*.auth.tgz`:

```bash
docker compose run --rm volume-perms
docker compose up -d backend
```

**Env (backup):**

| Переменная | Дефолт | Смысл |
|------------|--------|--------|
| `BACKUP_ENABLED` | `1` | kill-switch: `0` запрещает ручное и автосоздание |
| `BACKUP_KEEP` | `7` | дефолт глубины, пока не сохранён schedule из UI |
| `BACKUP_INCLUDE_EDGES` | `1` | дефолт состава |
| `BACKUP_INCLUDE_AUTH` | `1` | дефолт auth tarball |
| `BACKUP_SCHEDULE_FILE` | `/app/data/backup_schedule.json` | расписание + политика (том auth-users) |

**Расписание (UI):** `/system` → **Резервное копирование** — ежедневно `hour:minute` + IANA timezone, keep (1…90), edges/auth. API: `GET/PUT /api/system/backup-schedule`. При включённом автобэкапе host-cron `backup-clickhouse.sh` не обязателен (не дублируйте оба без нужды).

**Cron (альтернатива, если UI-schedule выключен):**

```bash
30 2 * * * cd /opt/network-monitor && ./scripts/backup-clickhouse.sh >>/var/log/nm-backup.log 2>&1
```

**Restore:**

```bash
docker compose exec clickhouse ls -1 /var/lib/clickhouse-backups
# при непустых таблицах:
RESTORE_ALLOW_NONEMPTY=1 ./scripts/restore-clickhouse.sh nm-20260411T023000Z
```

Скрипт по умолчанию останавливает `backend` / `syslog-ng` на время restore и поднимает их обратно. Если edges не было в бэкапе — `POST /api/system/maintenance/backfill` (admin).

Планируйте место на диске: том бэкапов ≈ N × размер hot-данных. `down -v` / uninstall `--volumes` удалит и `clickhouse-backups`.

**UI:** `/system` → вкладка **Резервное копирование** (administrator): список, статус, «Создать бэкап», расписание, **Подключить / Отключить / Удалить**. В списке колонка **Источник** — `вручную` / `по расписанию` (маркер `{name}.source` на томе; старые бэкапы без маркера — «—»).

- **Подключить** — `RESTORE … AS nm_bak_*` (shadow для карты). Live и ingest не трогаются.
- **Отключить** — `DROP nm_bak_*`; бэкап на диске и live сохраняются.
- **Удалить** — стереть бэкап с тома (нельзя, пока подключён).
- На карте: переключатель **Live / Бэкап** (после Подключить).
- Колонка **Auth** — есть ли `*.auth.tgz` (снимок `/app/data`), не трафик.

Полный CLI restore (включая auth): `./scripts/restore-clickhouse.sh <name>`.

---

### Очистка данных ClickHouse

Удаляет все события, GeoIP, ошибки парсинга и метрики. **Схема таблиц сохраняется.**

```bash
cd /opt/network-monitor
bash clickhouse/reset_data.sh
```

---

### Мониторинг ingest

Скрипт для наблюдения за скоростью приёма событий:

```bash
cd /opt/network-monitor
./scripts/watch-ingest.sh          # интервал 2 сек (по умолчанию)
./scripts/watch-ingest.sh 5        # интервал 5 сек
```

Требует `curl` и `jq`. Выводит: recv/s, ins/s, **drop/s**, dropped, buffered, connections, state, queue depth.

---

### Логи и диагностика

```bash
cd /opt/network-monitor

# Все сервисы (последние 100 строк)
docker compose logs --tail=100

# Конкретный сервис (follow)
docker compose logs -f backend
docker compose logs -f syslog-ng
docker compose logs -f clickhouse
docker compose logs -f stats-collector

# Статус healthcheck
docker compose ps

# Проверка API (live/ready — публичные; остальное — с Bearer из .env)
curl -fsS http://127.0.0.1/api/live
curl -fsS http://127.0.0.1/api/ready
TOKEN="$(grep -E '^API_AUTH_TOKEN=' .env | cut -d= -f2-)"
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1/api/ingest/stats | jq .
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1/api/system/stats | jq .
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1/api/system/edges-agg | jq .

# Запросы к ClickHouse (пароль — в контейнере, из .env)
docker compose exec clickhouse sh -c 'clickhouse-client --password "$CLICKHOUSE_PASSWORD" --query "SELECT 1"'
docker compose exec clickhouse sh -c 'clickhouse-client --password "$CLICKHOUSE_PASSWORD" --query "
  SELECT vendor, count() AS cnt FROM traffic_logs GROUP BY vendor ORDER BY cnt DESC"'
```

**Типичные проблемы и куда смотреть:**

| Симптом                        | Куда смотреть                                      |
|--------------------------------|----------------------------------------------------|
| Нет событий на карте           | `docker compose logs syslog-ng`, `/api/ingest/stats` |
| Ошибки парсинга                | UI → «Ошибки парсинга», таблица `parse_errors`     |
| Backend не стартует            | `docker compose logs backend`, healthcheck         |
| ClickHouse OOM / медленные запросы | `install-profile.json`, увеличить профиль; для карты предпочтительнее период `1d+` + groupBy city/country (daily geo-agg) |
| ClickHouse CPU скачет при обновлении карты | abort предыдущих `/api/events`; `CH_MAX_THREADS` / `max_threads` в профиле; логи backend `geo edges agg: ready`; периоды `1h`/`6h` всегда читают `traffic_logs` |
| ClickHouse idle CPU высокий, мало данных | `system.trace_log`/`text_log` — см. `config.d/z_system_logs.xml`; `TRUNCATE`/`DROP` старых system-логов |
| Нет метрик на странице «Система» | `docker compose logs stats-collector`, cgroup, `/proc` и `/sys/fs/cgroup` на хосте |
| Превышена расчётная ёмкость      | `/system` → алёрты `capacity_high` / `capacity_exceeded`, пересчёт профиля |
| Drops под нагрузкой / очередь полная | `/system` (плитка Drops, стадии Syslog-NG и Backend Ingest); `pipeline.syslogng.dropped_total` / `queued`; `/api/ingest/stats` → `dropped_total`, `buffer_drops_total`; алёрты `syslogng_dropping*`, `ingest_dropping*`; `./scripts/watch-ingest.sh` |
| UDP/TCP EPS не разделяются       | Перезапустить `syslog-ng` (маркеры `@@nm/udp/@@` / `@@nm/tcp/@@`) |
| syslog-ng: kernel refused SO_RCVBUF | `net.core.rmem_max` / `wmem_max` на хосте (см. буферы профиля) |
| git pull: local changes (только chmod +x) | Предпочтительнее обновление из пакета (`./update.sh`). Иначе `git restore -- '*.sh'` затем `git pull --ff-only`; либо `git reset --hard origin/main` |
| `update.sh`: `CLICKHOUSE_PASSWORD is missing` на остановке | Баг **1.4.0**: `compose down` требовал секреты. Берите пакет **1.4.1+**. Не экспортируйте фиктивный пароль на весь `update.sh` — `./start.sh` подхватит его вместо `.env`. |
| syslog-ng ругается на `log-iw-size` / `flush_timeout` / нет `zz_profile.conf` | На сервере всё ещё старый `syslog-ng.conf`. `git log -1` и `grep flow-control-window-size syslog-ng.conf`; затем hard reset на `origin/main` и `./start.sh` |
| GeoIP upload → 502 / OOM, backend перезапускается | Не заливать большой CSV поверх уже загруженного индекса через браузер; `dmesg`/`oom-kill`; см. [GeoIP](#geoip) |
| GeoIP: `Failed to fetch` при смене страницы | Уход со страницы во время POST обрывает `fetch`; дождитесь окончания или `curl` с сервера |
| После рестарта backend страницы 500 (auth) | Обновить до ≥1.1.3 (индекс GeoIP грузится асинхронно); дождаться `geo index loaded` |
| TTL не применился / старые данные остаются | `/api/system/retention`, логи backend `retention:`, том `auth-users` (`retention.json`) |

---

### Обновление системы

Данные ClickHouse и учётки UI сохраняются (тома Docker не трогаем), меняется только код в `/opt/network-monitor`.
Не используйте `REMOVE_DOCKER_VOLUMES=1` / `docker compose down -v` при обновлении.

Один **`geoatlas-X.Y.Z.tar.gz`** на Ubuntu и на Oracle Linux / RHEL. `update.sh` не затирает:

- `.env`, `.admin_password_once`
- `docker-compose.override.yml`, `install-profile.json`, `install-modules.json`
- `certs/` (PEM)
- `syslog-ng.d/zz_profile.conf`, `syslog-ng.d/zz_ingest_auth.conf`
- `clickhouse/users.d/zz_install_limits.xml`

Образы пересобираются на сервере (`./start.sh`); нужен доступ к Docker Hub (базовые образы), сам GitHub после скачивания tar.gz не обязателен.

**Рекомендуемый путь — локальный пакет** (без `git pull`):

```bash
# 1. Скачать архив релиза (на машине с интернетом или сразу на сервер)
#    GitHub → Releases → Assets → geoatlas-X.Y.Z.tar.gz (+ .sha256)
VER=1.4.1
curl -fLO "https://github.com/varlahin-gena/network_monitor/releases/download/v${VER}/geoatlas-${VER}.tar.gz"
curl -fLO "https://github.com/varlahin-gena/network_monitor/releases/download/v${VER}/geoatlas-${VER}.tar.gz.sha256"
sha256sum -c "geoatlas-${VER}.tar.gz.sha256"

# 2. На сервере (Ubuntu и Oracle Linux одинаково)
sudo /opt/network-monitor/update.sh ./geoatlas-${VER}.tar.gz
```

Если на сервере ещё нет `update.sh` (установка до этого механизма):

```bash
tar -xzf geoatlas-${VER}.tar.gz
sudo ./geoatlas-${VER}/update.sh --package "$PWD/geoatlas-${VER}.tar.gz"
# если каталог не /opt/network-monitor:
# sudo ./geoatlas-${VER}/update.sh --package "$PWD/geoatlas-${VER}.tar.gz" --project-dir /opt/network_monitor
```

Скачать последний релиз с GitHub прямо на сервере:

```bash
sudo /opt/network-monitor/update.sh --download
```

Полезные опции: `--no-start`, `--no-stop`, `--project-dir DIR`. Проверка суммы: рядом с tar.gz файл `.sha256` или `NM_INSTALL_PACKAGE_SHA256=<hex>`.

Остановка при обновлении не требует реальных секретов (`compose down` с заглушками в процессе). `./start.sh` по-прежнему читает `.env` и не стартует без `CLICKHOUSE_PASSWORD` / токенов.

Первичная установка из уже скачанного пакета (без git) — OS-скрипт из архива:

```bash
tar -xzf geoatlas-${VER}.tar.gz
cd geoatlas-${VER}
sudo NM_INSTALL_SOURCE=package NM_INSTALL_PACKAGE="$PWD" ./deploy/ubuntu/install_ubuntu.sh
# Oracle Linux / RHEL: ./deploy/oracle_linux/install_oraclelinux.sh
```

**Через повторный запуск install-скрипта** (спросит релиз / `main` / пакет; для релиза предпочтёт tar.gz, иначе git):

```bash
sudo ./deploy/ubuntu/install_ubuntu.sh
# или
sudo ./deploy/oracle_linux/install_oraclelinux.sh
```

**Вручную через git** (ветка `main` или если пакета нет):

```bash
cd /opt/network-monitor
./stop.sh
git fetch origin --tags
# chmod +x на скриптах 100644 блокирует pull — сбросить только бит исполнения:
git restore --worktree --staged -- '*.sh' 'clickhouse/*.sh' 'deploy/**/*.sh' 'scripts/*.sh' 2>/dev/null || true
git pull --ff-only origin main   # на ветке; на теге обычно checkout нового тега (см. ниже)
./start.sh
```

Проверка, что syslog-ng 4.11 на месте: `git log -1 --oneline` (ожидается коммит с `flow-control-window-size`), `grep flow-control-window-size syslog-ng.conf`, `ls syslog-ng.d/zz_profile.conf`. Если `git pull` всё ещё ругается на локальные правки, а своих изменений в tracked-файлах нет: `git reset --hard origin/main` (`.env`, `docker-compose.override.yml`, `install-profile.json` в git не входят).

#### Переключение: релиз → `main`

```bash
cd /opt/network-monitor
./stop.sh
git fetch origin --tags
git checkout main
git pull --ff-only origin main

# чтобы в меню УЗ отображалось «main»
grep -q '^NM_INSTALL_SOURCE=' .env \
  && sed -i 's/^NM_INSTALL_SOURCE=.*/NM_INSTALL_SOURCE=main/' .env \
  || echo 'NM_INSTALL_SOURCE=main' >> .env
grep -q '^NM_INSTALL_REF=' .env \
  && sed -i 's/^NM_INSTALL_REF=.*/NM_INSTALL_REF=main/' .env \
  || echo 'NM_INSTALL_REF=main' >> .env

./start.sh
```

#### Откат: `main` → релиз

Предпочтительнее пакет того релиза: `sudo ./update.sh /path/to/geoatlas-X.Y.Z.tar.gz`. Через git:

```bash
cd /opt/network-monitor
./stop.sh
git fetch origin --tags

# последний релиз по semver, либо явно: TAG=v1.2.0
TAG=$(git tag -l 'v*' --sort=-v:refname | head -1)
echo "Откат на $TAG"
git checkout --force "$TAG"

grep -q '^NM_INSTALL_SOURCE=' .env \
  && sed -i "s/^NM_INSTALL_SOURCE=.*/NM_INSTALL_SOURCE=release/" .env \
  || echo 'NM_INSTALL_SOURCE=release' >> .env
grep -q '^NM_INSTALL_REF=' .env \
  && sed -i "s/^NM_INSTALL_REF=.*/NM_INSTALL_REF=${TAG}/" .env \
  || echo "NM_INSTALL_REF=${TAG}" >> .env

./start.sh
```

Если есть локальные правки в дереве: `git stash -u` перед `checkout`.  
`./start.sh` пересоберёт образы и обновит `install-meta.json` (строка версии в меню пользователя).

---

### Пересборка образов

Backend и stats-collector собираются из исходников. После изменения кода:

```bash
cd /opt/network-monitor
docker compose build --no-cache backend stats-collector
./start.sh
# или
docker compose up -d --build
```

---

## GeoIP

GeoIP-база загружается через веб-интерфейс (кнопка загрузки на главной странице) в виде CSV
или пополняется по одному диапазону со страницы **IP без GeoIP** (`/geo-missing`).
Поддерживается формат SIEM KUMA.

**Формат CSV:**

```csv
Network,Country,Region,City,Latitude,Longitude
10.0.0.0-10.0.255.254,Россия,OOO_Company,Москва,55.76167,37.60667
192.168.1.0-192.168.1.255,Россия,LAN,Office,55.75,37.62
```

На странице `/geo-missing` для IP без координат доступна кнопка **«добавить в базу»**
(форма с полями шаблона выше) и **«Выгрузить GeoIP CSV»** — скачивание актуальной базы.

Без GeoIP-базы узлы на карте не отображаются (нет координат).

### Загрузка GeoIP с сервера (рекомендуется для больших CSV)

Большие базы (сотни МБ) через браузер часто упираются в ширину канала между рабочей станцией и сервером.
**Не переключайте страницу/вкладку во время upload** — браузер оборвёт запрос (`Failed to fetch`).
Надёжнее положить CSV на сам хост и залить через API локально (нужен `API_AUTH_TOKEN` из `.env` или именованный токен scope **ops**/**admin**):

```bash
cd /opt/network-monitor
# скопировать файл на сервер, например:
# scp geoip.csv root@сервер:/opt/network-monitor/geoip.csv

set -a; . ./.env; set +a

curl -sS -w "\nHTTP %{http_code}\n" \
  -H "Authorization: Bearer $API_AUTH_TOKEN" \
  -H "Content-Type: text/csv" \
  --data-binary @/opt/network-monitor/geoip.csv \
  "http://127.0.0.1/upload-geo"
# если UI на 8080 (full-auto): http://127.0.0.1:8080/upload-geo

docker compose exec clickhouse sh -c 'clickhouse-client --password "$CLICKHOUSE_PASSWORD" -q "SELECT count() FROM geo_ranges"'
docker compose logs backend --since=10m 2>&1 | grep -iE 'geo index loaded|geo csv|upload|overlap|error'
```

Ожидаете: JSON с `"ok":true,"ranges":N`, `count() > 0` и в логах `geo index loaded`.
На большой базе backend может на 1–3 минуты занять много CPU/RAM — это нормально (парсинг и in-memory индекс).

### Повторная загрузка большой GeoIP и HTTP 502 / OOM

Индекс GeoIP целиком держится в RAM backend. Повторный upload того же большого CSV (миллионы диапазонов), когда индекс уже загружен, снова парсит файл в память **поверх** существующего индекса → пик RAM удваивается → Docker cgroup может убить процесс (`oom-kill` / `Memory cgroup out of memory`). Снаружи это часто выглядит как **HTTP 502** в UI, а контейнер `backend` ненадолго перезапускается.

**Сервер теперь режет опасные upload до OOM:**

| Ограничение | Env | Откуда дефолт |
|-------------|-----|----------------|
| Размер тела `/upload-geo` | `GEOIP_UPLOAD_MAX_BYTES` (или `MAX_GEO_UPLOAD_SIZE`) | `install-profile.json` → `limits.backend.memory_gb` (small≈512 MiB, medium≈1 GiB, …) |
| Число диапазонов в CSV | `GEOIP_UPLOAD_MAX_RANGES` | тот же профиль (small≈4 M) |
| Replace поверх крупного индекса | — | **409 до чтения body**, если индекс уже ≥ половины лимита ranges |

Ответы: **413** (файл/число ranges слишком велики), **409** (опасный replace — сразу, без парсинга CSV). `?dry_run=1` по-прежнему проверяет CSV, но не пишет в CH и не делает early replace-gate (лимит ranges после parse всё равно действует).

Ход загрузки смотрите в логах: `geo upload start` → `geo csv parsed` / `geo upload early reject` / `geo upload rejected …`. Точечная правка одной строки — страница **База GeoIP** (`/geo-ranges`), не полный CSV.

В логах после reload: `geo index loaded` с `ranges`, `heap_alloc_mb`, `heap_delta_mb`. Лимиты также в `GET /api/geo-ranges` → `limits.upload_max_*`.

Compact-снимок индекса пишется в `/app/data/geo_index.snap` (том `auth-users`, не входит в `*.auth.tgz`). После рестарта backend сначала поднимает снимок с диска (карта сразу с координатами), затем сверяет отпечаток с ClickHouse и при расхождении перечитывает `geo_ranges`. Выключить: `GEOIP_SNAPSHOT_FILE=off`.

Пока идёт полная загрузка из ClickHouse, HTTP API (в т.ч. auth) уже доступен. В логах: `geo index loaded from disk snapshot` и/или `geo index loaded`.

- Если в ClickHouse уже есть нужное число строк (`SELECT count() FROM geo_ranges`) — **перезаливать не нужно**.
- Одну строку меняйте в UI `/geo-ranges` (или API `PUT /api/geo-ranges`).
- Полная замена через UI: на `/geo-ranges` → **Очистить базу** (`POST /api/geo-ranges/clear`: TRUNCATE + сброс RAM-индекса, без рестарта) → **Загрузить CSV**. Без очистки повторный full upload даст **409**.
- Замену с сервера через `curl` (см. выше) тоже можно; при необходимости временно поднимите `GEOIP_UPLOAD_MAX_*` и memory backend.
- Проверка OOM: `dmesg -T | grep -i oom` и `docker compose logs backend -f` (`grep -iE 'geo upload|geo index|oom|geo ranges cleared'`).

---

## Веб-интерфейс

| URL                    | Страница              | Основные возможности                                                                                                                                 |
|------------------------|-----------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| `/login`          | Вход                  | Логин (роли admin / operator); смена пароля при `must_reset_password`                                                                                |
| `/`                    | Карта / глобус        | 2D/3D, группировка, фильтры status/репутации, конструктор поиска и шаблоны, порог событий, mono-дуги, экспорт PNG, загрузка логов/GeoIP, health pill |
| `/reputation`     | Репутация IP          | Списки и URL-фиды, каталог источников, refresh по расписанию, ручная загрузка; модуль можно отключить при установке                                  |
| `/parse-errors`   | Журнал ошибок парсинга| Поиск, удаление выбранных / всех, отправка в тест парсеров                                                                                           |
| `/geo-missing`    | IP без GeoIP          | Адреса без координат; добавление в GeoIP; выгрузка CSV; мгновенная перефильтрация списка                                                             |
| `/geo-ranges`     | База GeoIP            | Просмотр/правка диапазонов, выгрузка CSV                                                                                                             |
| `/parser-test`    | Тест парсеров         | До 200 строк за запрос, пресеты по вендорам, статусы parsed/skipped/error                                                                            |
| `/system`         | Системный мониторинг  | Обзор / Pipeline (Syslog-NG queued/drops + ingest EPS) / Безопасность / Графики / Резервное копирование; алёрты, ёмкость, профиль установки |
| `/users`          | Учётные записи        | Список/создание УЗ (скрыто, если UI-auth выключен)                                                                                                   |
| `/api-tokens`     | API-токены            | Именованные Bearer со scope read/ops/admin; секрет показывается один раз                                                                             |
| `/change-password`| Смена пароля          | Смена своего пароля                                                                                                                                  |

SPA (React Router): page-auth в UI; nginx `auth_request` для `/api/*`. Карта и смена пароля — любой залогиненный; system / parsers / geo / reputation / users / api-tokens — только **administrator**. Legacy `*.html` редиректятся на clean paths.

Unit-тесты карты (репутация / heatmap focus / coords helpers): `cd frontend && npm test` (vitest). Контрактные grep-проверки UI: `bash scripts/frontend-smoke.sh`.

### HTTP API

Контракт REST API (в т.ч. auth, events, geo, reputation, retention, tokens, search-templates, backups, `/metrics`): [`openapi.yaml`](openapi.yaml), версия документа OpenAPI **1.11.0**. Пробы: `GET /api/live` (процесс), `GET /api/ready` (ClickHouse + ingest); `GET /api/health` — alias live. Остальные эндпоинты — cookie-сессия и/или Bearer (`API_AUTH_TOKEN` / именованный токен со scope). Prometheus scrape: `GET /metrics` (Bearer≥ops / administrator).

## Лицензия

ГеоАтлас распространяется под [Apache License 2.0](LICENSE). Продукт бесплатный и общедоступный; релизы публикуются на GitHub.

Баги, вопросы и заказ доработок — через [Issues](https://github.com/varlahin-gena/network_monitor/issues).

## Безопасность

Уязвимости — только приватно, см. [SECURITY.md](SECURITY.md) (подтверждение в течение 5 рабочих дней). Обычные баги — в Issues.

Краткий checklist для прод-установки:

- Секреты только через `./start.sh` (`API_AUTH_TOKEN`, `SESSION_SECRET`, `INGEST_SHARED_SECRET`, `CLICKHOUSE_PASSWORD`)
- Пароли УЗ: минимум 10 символов, буква и цифра, не из common-list
- Одноразовый пароль admin: `.admin_password_once` (600), не stdout
- UI за HTTPS при доступе из сети; не публиковать ClickHouse `8123`/`9000` и backend `1514`
- ClickHouse `default`: networks loopback+RFC1918 (не `::/0`)
- Syslog `:514` **без TLS/auth** — ограничьте источником МСЭ (`NM_SYSLOG_ALLOW_FROM` / Security Group / firewall)
- Ingest внутри docker: маркер с `INGEST_SHARED_SECRET` + `INGEST_ALLOW_FROM=syslog-ng` (HTTP upload API — без маркера)
- Reputation URL-фиды: только публичные IPv4-хосты (private/metadata блокируются)
- Обновление: проверяйте `geoatlas-X.Y.Z.tar.gz.sha256` до `./update.sh`

## Структура репозитория

```
network_monitor/
├── go.work                           # workspace: backend + stats-collector + pkg/chconn + pkg/syslogngstats
├── backend/                          # Go: API, парсеры, geoip, ingest
│   ├── cmd/network-monitor/main.go
│   ├── Dockerfile
│   └── internal/
│       ├── adapter/
│       │   ├── httpapi/              # HTTP delivery (handlers, middleware, cookies/CSRF)
│       │   ├── clickhouse/           # pool/retention/maintenance + domain stores
│       │   │   ├── ingeststore/      # INSERT traffic_logs / parse_errors
│       │   │   ├── perrorstore/      # list/delete parse_errors
│       │   │   ├── trafficstore/     # map/events scans
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
│       ├── usecase/                  # application use cases + ports (bootstrap, retention, …)
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
│   ├── pack-release.sh               # dist/geoatlas-X.Y.Z.tar.gz (+ sha256)
│   ├── test-apply-package.sh         # CI: pack + наложение пакета
│   ├── test-compose-stop-env.sh      # CI: заглушки для compose down без секретов
│   ├── shellcheck.sh                 # CI: start/stop/update, scripts/, deploy/
│   ├── backup-clickhouse.sh          # native BACKUP → том clickhouse-backups
│   └── restore-clickhouse.sh         # RESTORE + optional auth tarball
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
│   ├── src/                          # React pages (Map, System, admin…)
│   ├── nginx.conf
│   ├── nginx-app.inc                 # SPA try_files @spa + auth_request API
│   ├── docker-entrypoint.sh          # config.js + HTTP/HTTPS default.conf
├── stats-collector/                  # Go: системные метрики
│   ├── main.go
│   └── internal/
│       ├── config/
│       └── collector/
├── openapi.yaml                      # контракт HTTP API (OpenAPI 1.11.0)
├── LICENSE / NOTICE                  # Apache License 2.0
├── SECURITY.md                       # как сообщать об уязвимостях
├── VERSION / CHANGELOG.md / RELEASING.md
├── .github/workflows/ci.yml
├── .github/workflows/release-tag.yml # тег v*: Release + geoatlas-X.Y.Z.tar.gz
├── start.sh / stop.sh / update.sh    # update.sh — наложение geoatlas-*.tar.gz
├── syslog-ng.conf
└── syslog-ng.d/                      # 00-keep.conf + zz_profile.conf.example; zz_profile.conf генерируется
```

**Генерируемые при установке (не в git). `update.sh` сохраняет runtime; `install-meta.json` заново пишет `./start.sh`:**

```
/opt/network-monitor/
├── docker-compose.override.yml   # Лимиты по профилю
├── .env                          # COMPOSE_PROFILES, секреты, лимиты
├── .admin_password_once          # одноразовый пароль admin (если генерировали)
├── install-profile.json          # Сводка установки
├── install-modules.json          # Выбор модулей
├── install-meta.json             # Версия для UI (переписывается при старте)
├── syslog-ng.d/zz_profile.conf   # Буферы syslog-ng
├── syslog-ng.d/zz_ingest_auth.conf
└── clickhouse/users.d/zz_install_limits.xml
```
