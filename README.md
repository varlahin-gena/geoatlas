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
  - [Ubuntu](#ubuntu)
  - [Oracle Linux / RHEL](#oracle-linux--rhel)
  - [Ручная установка из пакета](#ручная-установка-из-пакета)
- [Удаление](#удаление)
  - [Быстрый старт (автоопределение ОС)](#быстрый-старт-автоопределение-ос)
  - [Ubuntu / Debian](#ubuntu--debian)
  - [Oracle Linux / RHEL](#oracle-linux--rhel-1)
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
- **Аномалии на карте**
- **Конструктор поиска** на карте (гибридный query builder) и **личные шаблоны** запросов; у администратора — просмотр всех шаблонов
- **Группировка узлов**: по IP / по подсети `/24` / **по городу (по умолчанию)** / по стране
- **Тест парсеров** в браузере: статусы parsed / skipped / error, гео-обогащение, пресеты по вендорам
- **Журнал ошибок парсинга**: поиск, выборочное и полное удаление, передача строк в «Тест парсеров»
- **Страница системного мониторинга** (Обзор / Pipeline / Безопасность / Графики / Резервное копирование)
- **Логи контейнеров**: realtime stdout стека в UI `/dozzle/` для administrator;

---

## Архитектура

Система состоит из **шести сервисов**, оркеструемых через Docker Compose.
Ядро всегда поднимается: **clickhouse + backend + frontend**.
`syslog-ng`, `stats-collector` и `dozzle` включаются через compose-профили (`COMPOSE_PROFILES=syslog,stats,dozzle` — по умолчанию при установке / в `.env.example`).

```
МСЭ (UserGate, FortiGate, …) или SIEM
      │ syslog (514/tcp, 514/udp)          [профиль syslog]
      ▼
 ┌───────────┐   TCP :1514 (@@ga/udp/@@, @@ga/tcp/@@)                      ┌───────────┐
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
| `dozzle`          | `dozzle`           | `dozzle`    | Realtime-логи контейнеров UI `/dozzle/` (admin; opt-in) |

### Поток данных

1. **Syslog**: МСЭ отправляет события на `<IP_сервера>:514` (TCP или UDP).
2. **syslog-ng** принимает сообщения и пересылает их по TCP на `backend:1514` с маркерами транспорта (`@@ga/udp/@@` / `@@ga/tcp/@@`).
3. **backend** снимает маркеры транспорта, парсит строки, обогащает GeoIP, пишет в ClickHouse; при старте `EnsureBaseSchema` / `Ensure*` создаёт базовые таблицы и агрегаты (`traffic_edges_*`, geo), применяет TTL из `retention.json` и при необходимости делает backfill. **Delivery contract — at-most-once / best-effort:** полная очередь ingest **не блокирует** TCP — лишние строки дропаются (`dropped_total`); при outage ClickHouse insert circuit ставит dequeue на паузу (очередь растёт → admission drops), а потери из processor-буфера учитываются отдельно (`buffer_drops_total`). Перед backend **syslog-ng уже буферизует** (см. ниже). Алерты и live-метрики drops — на `/system`.
4. **frontend** отдаёт статику и проксирует API-запросы на backend.
5. **stats-collector** каждые 30 секунд собирает метрики CPU/RAM контейнеров и состояние пайплайна (включая разбивку UDP/TCP).

### Product limits (appliance)

| Ограничение                   | Суть                                                                                                                                           |
|-------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------|
| **IPv4-only**                 | Success path GeoIP / карта / lookup — IPv4. IPv6 не обогащается и не строится на карте.                                                        |
| **Single-host control plane** | Учётки, API-токены, retention, reputation feeds, schedules — JSON на `/app/data`. Backend берёт exclusive lock (`/app/data/.ga_backend.lock`). |
| **Один процесс backend**      | Ingest TCP, HTTP API, GeoIP RAM и background jobs (geo/backup/anomaly/reputation) — один контейнер. OOM/рестарт = пауза и приёма, и UI.        |

### Ingest SLO

Доставка — **best-effort / at-most-once** (очередь может дропать). Product SLO описывает, когда потери — инцидент (алёрты на `/system`, поле `ingest_slo` в `/api/system/stats`, метрики Prometheus):

| Сигнал                                 | Warn           | Critical (error)              |
|----------------------------------------|----------------|-------------------------------|
| Queue depth / byte budget              | ≥ 75% capacity | ≥ 90%                         |
| Processor buffer lines                 | > 10k          | > 100k                        |
| Admission / buffer / syslog-ng drops/s | любое > 0      | ≥ 100/s                       |
| Insert circuit open                    | да (warn)      | (эскалация через queue/drops) |
| Pipeline lag (при ненулевом rate)      | > 60s          | > 300s                        |
| EPS vs install profile max             | > 105%         | > 125%                        |

Runbook кратко: warn drops → проверить стадию Syslog-NG (queued/dropped) / syslog-ng buffer / profile / CH; critical → `tune-resources.sh` или снизить входной EPS; circuit open → здоровье ClickHouse.

### Хранение данных (TTL)

Дефолты ниже, менять можно в UI (`/system` → Pipeline → «Срок хранения»).

| Таблица / объект                    | Срок по умолчанию | Источник дефолта                                   |
|-------------------------------------|-------------------|----------------------------------------------------|
| `traffic_logs`                      | 30 дней           | EnsureBaseSchema / retention `traffic_logs_days`   |
| `traffic_edges_daily` (+ MV)        | 30 дней           | `EnsureEdgesAgg` / retention `edges_days`          |
| `traffic_edges_city_daily` (+ MV)   | 30 дней           | `EnsureGeoEdgesAgg` / retention `edges_days`       |
| `traffic_edges_country_daily` (+ MV)| 30 дней           | `EnsureGeoEdgesAgg` / retention `edges_days`       |
| `parse_errors`                      | 7 дней            | EnsureBaseSchema / retention `parse_errors_days`   |
| `system_metrics`                    | 7 дней            | EnsureBaseSchema / retention `system_metrics_days` |
| `geo_ranges`                        | без TTL           | EnsureBaseSchema (не настраивается)                |
| `ga_schema_version`                 | без TTL           | `Ensure*` (метаданные схемы)                       |

Допустимый диапазон настраиваемых дней: **1…730**. На `traffic_logs` / `parse_errors` / `system_metrics` / `traffic_edges_*` включён `ttl_only_drop_parts` при **дневных** партициях: истечение удаляет дневную партицию целиком. Уменьшение TTL удалит старые партиции при следующем TTL merge/drop в ClickHouse.

Системные логи ClickHouse (`trace_log`, `text_log`, `metric_log`, …) по умолчанию **отключены** (`clickhouse/config.d/z_system_logs.xml`): на малых объёмах данных они иначе раздуваются сильнее `traffic_logs` и держат высокий idle CPU. `query_log` остаётся включённым. Образ ClickHouse в compose: **`geoatlas-clickhouse`** (база `clickhouse/clickhouse-server:25.8.33.6` + Ubuntu security upgrade openssl/libssl3/perl-base).

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

Положите PEM в `certs/fullchain.pem` и `certs/privkey.pem` (см. `certs/README.md`).

| Переменная     | По умолчанию | Назначение                                                      |
|----------------|--------------|-----------------------------------------------------------------|
| `HTTPS_ENABLED`| `auto`       | `1`/`true` — вкл.; `0` — выкл.; `auto` — вкл. если есть оба PEM |
| `HTTPS_PORT`   | `443`        | Хостовый порт TLS                                               |
| `HTTP_REDIRECT`| `1`          | Редирект HTTP→HTTPS                                             |
| `HTTP_PORT`    | `80`         | HTTP (и редирект)                                               |

Установщик (Ubuntu / Oracle Linux) спрашивает HTTPS в пошаговом режиме (при TTY). 

---

## Быстрый старт

После установки (см. ниже) система доступна по адресу:

```
http://<IP_сервера>/
```

При HTTPS (сертификаты в `certs/` + `HTTPS_ENABLED`): `https://<IP_сервера>/`.

По умолчанию включена UI-авторизация. Первый вход — учётка **admin** (роль administrator). Пароля по умолчанию нет - пошаговая установка спрашивает пароль (дважды, мин. 8 символов);

---

## Установка

Каталог установки по умолчанию: **`/opt/geoatlas`**.

### Установочный пакет

Один архив **`geoatlas-X.Y.Z.tar.gz`** (плюс `.sha256`; SBOM `.cdx.json` / `.spdx.json` — для аудита, не для установки) с [GitHub Releases](https://github.com/varlahin-gena/geoatlas/releases) — исходники стека, **оба** установщика (`deploy/ubuntu/…`, `deploy/oracle_linux/…`), `update.sh` и (для прод-пакета) каталог **`images/`** с готовыми Docker-образами. Это не `.deb`/`.rpm`: Docker и файрвол хоста ставит OS-скрипт из пакета.

| Задача                                | Что запускать                                              |
|---------------------------------------|------------------------------------------------------------|
| Первая установка, Ubuntu              | `deploy/ubuntu/install_ubuntu.sh` из распакованного пакета |
| Первая установка, Oracle Linux / RHEL | `deploy/oracle_linux/install_oraclelinux.sh` из пакета     |
| Обновление                            | `/opt/geoatlas/update.sh` + новый tar.gz                   |

**Общий первый шаг на сервере** (Ubuntu и Oracle Linux одинаково):

```bash
VER=2.1.3   # или нужный релиз
cd /tmp
curl -fLO "https://github.com/varlahin-gena/geoatlas/releases/download/v${VER}/geoatlas-${VER}.tar.gz"
curl -fLO "https://github.com/varlahin-gena/geoatlas/releases/download/v${VER}/geoatlas-${VER}.tar.gz.sha256"
sha256sum -c "geoatlas-${VER}.tar.gz.sha256"
tar -xzf "geoatlas-${VER}.tar.gz"
cd "geoatlas-${VER}"
```

Дальше — установщик вашей ОС (см. ниже) или `./update.sh` для уже стоящей системы.

### Ubuntu

После шагов выше (пакет в `/tmp/geoatlas-${VER}`):

```bash
sudo ./deploy/ubuntu/install_ubuntu.sh
```

Скрипт устанавливает Docker, накладывает пакет в `/opt/geoatlas`, **интерактивно предлагает модули** и профиль производительности, настраивает UFW и запускает стек.

Диалоги установки и удаления — **TUI** (`whiptail` → `dialog` → текст). Долгие шаги (apt/Docker) показывают **gauge**.

**Что делает скрипт:**

1. Обновляет списки пакетов
2. Устанавливает `curl`, `ufw`, `whiptail` (опционально `dialog`)
3. Устанавливает Docker Engine и compose plugin (если ещё нет)
4. Накладывает пакет из текущего каталога в `/opt/geoatlas`
5. Спрашивает, какие модули ставить (checklist: авторизация, API-токен, syslog-ng, stats-collector, репутация IP, Dozzle)
6. Спрашивает **HTTPS** (свои PEM; можно оставить только HTTP)
7. Спрашивает **порт(ы)**: при HTTPS — порт TLS, затем HTTP (редирект); при HTTP-only — порт UI (80 / 8080 или свой)
8. Запускает детектор ресурсов и предлагает профиль
9. Настраивает UFW (HTTP, при HTTPS — TLS-порт, и при необходимости 514)
10. Вызывает `./start.sh` (можно отказаться на последнем шаге)

**Выбор модулей (интерактивно или через env):**

| Модуль           | По умолчанию | Что даёт                                                                        |
|------------------|--------------|---------------------------------------------------------------------------------|
| UI-авторизация   | вкл.         | логин, роли admin/operator/dashboard (`AUTH_DISABLED` при отказе)               |
| API Bearer-токен | вкл.         | защита мутирующих API (`API_AUTH_DISABLED` при отказе)                          |
| syslog-ng        | вкл.         | приём syslog на `:514` (Compose profile `syslog`)                               |
| stats-collector  | вкл.         | метрики / `/system` (Compose profile `stats`)                                   |
| Репутация IP     | вкл.         | модуль целиком; при отказе `REPUTATION_FETCH_ENABLED=false` (API/UI/фиды выкл.) |
| Dozzle           | вкл.         | realtime-логи UI `/dozzle/` (profile `dozzle`; `docker.sock` + start/stop/restart) |

Ядро (ClickHouse + Backend + Frontend) ставится всегда.

---

### Oracle Linux / RHEL

Поддерживаются Oracle Linux, RHEL, Rocky Linux, AlmaLinux, CentOS. **Тот же** `geoatlas-X.Y.Z.tar.gz`, что для Ubuntu.

После распаковки пакета на сервере:

```bash
sudo ./deploy/oracle_linux/install_oraclelinux.sh
```

**Что делает скрипт:**

1. Удаляет конфликтующие пакеты (`podman`, `buildah`, `runc`) при необходимости
2. Устанавливает Docker CE из официального репозитория
3. Настраивает SELinux (`container_manage_cgroup`) или переводит в permissive (опционально)
4. Накладывает пакет в `/opt/geoatlas`, предлагает модули, HTTPS, HTTP-порт, профиль, настраивает firewalld и запускает стек

---

### Ручная установка из пакета

Если нужны те же шаги без TUI (Docker уже установлен):

```bash
# Пакет уже скачан и распакован (см. «Установочный пакет»)
sudo mkdir -p /opt
sudo cp -a "$PWD" /opt/geoatlas
cd /opt/geoatlas

chmod +x start.sh stop.sh update.sh scripts/tune-resources.sh \
  deploy/common/detect_resources.sh deploy/common/select_modules.sh deploy/common/ui.sh \
  deploy/common/admin_auth.sh

# (Рекомендуется) модули и профиль
./deploy/common/select_modules.sh .
./scripts/tune-resources.sh
# или неинтерактивно:
# GA_ENABLE_AUTH=0 GA_AUTO_MODULES=1 ./deploy/common/select_modules.sh .
# GA_AUTO_PROFILE=1 ./deploy/common/detect_resources.sh .

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
cd /opt/geoatlas   # или каталог, откуда запускали скрипт
sudo bash deploy/uninstall.sh
```

Скрипт покажет **аудит** (контейнеры, volumes, размер каталога) и предложит **интерактивное меню** (whiptail / dialog):
1. Безопасное удаление — stop + файлы + firewall, данные ClickHouse сохраняются
2. Полное удаление (purge) — включая volumes и образы
3. Только остановить стек
4. Настроить вручную

Долгие шаги (compose down, rm, firewall) идут через **gauge**. Бэкенд диалогов тот же, что при установке (`GA_UI`).

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
cd /opt/geoatlas
./stop.sh
```

---

## Запуск и остановка

```bash
cd /opt/geoatlas

# Запуск (с пересборкой образов)
./start.sh

# Запуск без пересборки
DO_BUILD=0 ./start.sh

# Остановка (данные сохраняются)
./stop.sh

# Обновление из пакета (см. README «Обновление системы»)
# sudo ./update.sh /path/to/geoatlas-X.Y.Z.tar.gz
```

**Прямые команды Docker Compose:**

```bash
cd /opt/geoatlas

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

| Файл                                       | Назначение                                                     |
|--------------------------------------------|----------------------------------------------------------------|
| `docker-compose.override.yml`              | Лимиты CPU/RAM контейнеров, параметры ingest, `CH_MAX_THREADS` |
| `.env`                                     | Переменные для compose (`SYSLOG_STATS_URL` при модуле syslog)  |
| `clickhouse/users.d/zz_install_limits.xml` | Лимиты памяти и `max_threads` запросов ClickHouse              |
| `syslog-ng.d/zz_profile.conf`              | `@define` буферов syslog-ng (fifo / window / disk)             |
| `install-profile.json`                     | Сводка профиля (отображается в UI «Система»)                   |

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
cd /opt/geoatlas

# Интерактивный выбор
./scripts/tune-resources.sh 

# Автоматически — рекомендованный профиль
GA_AUTO_PROFILE=1 ./scripts/tune-resources.sh

# Принудительно задать профиль
GA_FORCE_PROFILE=large ./scripts/tune-resources.sh

```

Скрипт `tune-resources.sh` автоматически перезапускает стек, если он уже запущен.

Просмотр текущего профиля:

```bash
cat /opt/geoatlas/install-profile.json | jq .
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

| Что                  | Детали                                                                                                                   |
|----------------------|--------------------------------------------------------------------------------------------------------------------------|
| Конфиг disk          | `clickhouse/config.d/backups.xml`                                                                                        |
| Том                  | `clickhouse-backups` (отдельно от `clickhouse-data`)                                                                     |
| Скрипты              | `scripts/backup-clickhouse.sh`, `scripts/restore-clickhouse.sh`                                                          |
| По умолчанию в бэкап | `traffic_logs`, `geo_ranges`, `reputation_ranges`, `parse_errors`, `system_metrics`, `traffic_edges_*`                   |
| Рядом                | `*.auth.tgz` — снимок `/app/data` (users, retention, feeds, schedule); без `geo_index.snap`, `.ga_backend.lock`, `*.tmp` |

Backend и ClickHouse пишут в `clickhouse-backups` под **одним uid 101**. Сервис `volume-perms` при старте делает `chown 101:101` на томах `clickhouse-backups` и `auth-users` (миграция со старого backend uid 10001).

**UI:** `/system` → вкладка **Резервное копирование** (administrator): список, статус, «Создать бэкап», расписание, **Подключить / Отключить / Удалить**. В списке колонка **Источник** — `вручную` / `по расписанию` (маркер `{name}.source` на томе; старые бэкапы без маркера — «—»).

- **Подключить** — `RESTORE … AS ga_bak_*` (shadow для карты). Live и ingest не трогаются.
- **Отключить** — `DROP ga_bak_*`; бэкап на диске и live сохраняются.
- **Удалить** — стереть бэкап с тома (нельзя, пока подключён).
- На карте: переключатель **Live / Бэкап** (после Подключить).
- Колонка **Auth** — есть ли `*.auth.tgz` (снимок `/app/data`), не трафик.

---

### Очистка данных ClickHouse

Удаляет все события, GeoIP, ошибки парсинга и метрики. **Схема таблиц сохраняется.**

```bash
cd /opt/geoatlas
bash clickhouse/reset_data.sh
```

---

### Мониторинг ingest

Скрипт для наблюдения за скоростью приёма событий:

```bash
cd /opt/geoatlas
./scripts/watch-ingest.sh          # интервал 2 сек (по умолчанию)
./scripts/watch-ingest.sh 5        # интервал 5 сек
```

Требует `curl` и `jq`. Выводит: recv/s, ins/s, **drop/s**, dropped, buffered, connections, state, queue depth.

---

### Логи и диагностика

```bash
cd /opt/geoatlas

# Все сервисы (последние 100 строк)
docker compose logs --tail=100

# Конкретный сервис (follow)
docker compose logs -f backend
docker compose logs -f syslog-ng
docker compose logs -f clickhouse
docker compose logs -f stats-collector
docker compose logs -f dozzle

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

| Симптом                                    | Куда смотреть                                                                                                                                                   |
|--------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Нет событий на карте                       | `docker compose logs syslog-ng`, `/api/ingest/stats`                                                                                                            |
| Ошибки парсинга                            | UI → «Ошибки парсинга», таблица `parse_errors`                                                                                                                  |
| Backend не стартует                        | `docker compose logs backend`, healthcheck                                                                                                                      |
| ClickHouse OOM / медленные запросы         | `install-profile.json`, увеличить профиль; для карты предпочтительнее период `1d+` + groupBy city/country (daily geo-agg)                                       |
| ClickHouse CPU скачет при обновлении карты | abort предыдущих `/api/events`; `CH_MAX_THREADS` / `max_threads` в профиле; логи backend `geo edges agg: ready`; периоды `1h`/`6h` всегда читают `traffic_logs` |
| ClickHouse idle CPU высокий, мало данных   | `system.trace_log`/`text_log` — см. `config.d/z_system_logs.xml`; `TRUNCATE`/`DROP` старых system-логов                                                         |
| Нет метрик на странице «Система»           | `docker compose logs stats-collector`, cgroup, `/proc` и `/sys/fs/cgroup` на хосте                                                                              |
| Превышена расчётная ёмкость                | `/system` → алёрты `capacity_high` / `capacity_exceeded`, пересчёт профиля                                                                                      |
| Drops под нагрузкой / очередь полная       | `/system` (плитка Drops, стадии Syslog-NG и Backend Ingest); `pipeline.syslogng.dropped_total` / `queued`;                                                      |
|                                            | `/api/ingest/stats` → `dropped_total`, `buffer_drops_total`; алёрты `syslogng_dropping*`, `ingest_dropping*`; `./scripts/watch-ingest.sh`                       |
| UDP/TCP EPS не разделяются                 | Перезапустить `syslog-ng` (маркеры `@@ga/udp/@@` / `@@ga/tcp/@@`)                                                                                               |
| syslog-ng: kernel refused SO_RCVBUF        | `net.core.rmem_max` / `wmem_max` на хосте (см. буферы профиля)                                                                                                  |
| GeoIP upload → 502 / OOM, backend          | Не заливать большой CSV поверх уже загруженного индекса через браузер; `dmesg`/`oom-kill`; см. [GeoIP](#geoip)                                                  |
| перезапускается                            |                                                                                                                                                                 |
| GeoIP: `Failed to fetch` при смене         | Уход со страницы во время POST обрывает `fetch`; дождитесь окончания или `curl` с сервера                                                                       |
| страницы                                   |                                                                                                                                                                 |

---

### Обновление системы

Данные ClickHouse и учётки UI сохраняются (тома Docker не трогаем), меняется только код в `/opt/geoatlas`.

Один **`geoatlas-X.Y.Z.tar.gz`** на Ubuntu и на Oracle Linux / RHEL. `update.sh` не затирает:

- `.env`, `.admin_password_once`
- `docker-compose.override.yml`, `install-profile.json`, `install-modules.json`
- `certs/` (PEM)
- `syslog-ng.d/zz_profile.conf`, `syslog-ng.d/zz_ingest_auth.conf`
- `clickhouse/users.d/zz_install_limits.xml`

Образы пересобираются на сервере (`./start.sh`); нужен доступ к Docker Hub (базовые образы).

**Единственный путь обновления** — скачать пакет на сервер, проверить SHA-256, `./update.sh`:

```bash
VER=2.1.3   # нужный релиз
cd /tmp
curl -fLO "https://github.com/varlahin-gena/geoatlas/releases/download/v${VER}/geoatlas-${VER}.tar.gz"
curl -fLO "https://github.com/varlahin-gena/geoatlas/releases/download/v${VER}/geoatlas-${VER}.tar.gz.sha256"
sha256sum -c "geoatlas-${VER}.tar.gz.sha256"

tar -xzf "geoatlas-${VER}.tar.gz"
sudo ./geoatlas-${VER}/update.sh --package "$PWD/geoatlas-${VER}.tar.gz" --project-dir /opt/geoatlas
# если каталог с подчёркиванием:
# sudo ./geoatlas-${VER}/update.sh --package "$PWD/geoatlas-${VER}.tar.gz" --project-dir /opt/geoatlas
```

Если `update.sh` уже есть в каталоге установки:

```bash
cd /tmp
# … скачать и sha256sum -c, как выше …
sudo /opt/geoatlas/update.sh ./geoatlas-${VER}.tar.gz
```

**Откат на более старый релиз** — тот же сценарий: скачать `geoatlas-X.Y.Z.tar.gz` нужной версии с GitHub Releases и `./update.sh` (тома Docker и `.env` сохраняются).

```bash
tar -xzf geoatlas-${VER}.tar.gz
cd geoatlas-${VER}
sudo GA_INSTALL_SOURCE=package GA_INSTALL_PACKAGE="$PWD" ./deploy/ubuntu/install_ubuntu.sh
# Oracle Linux / RHEL: ./deploy/oracle_linux/install_oraclelinux.sh
```

**Через повторный запуск install-скрипта** (спросит релиз / `main` / пакет; для релиза предпочтёт tar.gz, иначе git):

```bash
sudo ./deploy/ubuntu/install_ubuntu.sh
# или
sudo ./deploy/oracle_linux/install_oraclelinux.sh
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
Надёжнее положить CSV на хост и залить через API **с самого сервера** (localhost → nginx → backend).

Нужен Bearer с scope **ops** или **admin**: токен из контейнера `backend` (ниже) или именованный токен из UI `/api-tokens`. Токен берите через `docker exec backend` — так вы гарантированно совпадаете с тем, что реально проверяет API (не зависит от `source .env` / `COMPOSE_PROJECT_NAME`).

**1. Скопировать CSV на сервер** (с рабочей станции):

```bash
scp geoip.csv root@сервер:/opt/geoatlas/geoip.csv
```

**2. На сервере — токен из живого backend:**

```bash
cd /opt/geoatlas
# контейнер должен быть Up (healthy): docker ps --filter name=^backend$
TOKEN="$(docker exec backend printenv API_AUTH_TOKEN)"
echo -n "$TOKEN" | wc -c   # ожидаете > 0 (обычно 32+)
```

**3. Проверить авторизацию** (ожидаете HTTP `200`):

```bash
# если UI не на 80-м порту — подставьте HTTP_PORT из .env (например :8080)
curl -sS -o /tmp/auth_body -w "HTTP %{http_code}\n" \
  -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1/api/auth/check-ops"
cat /tmp/auth_body
```

При **401** токен пустой/неверный или UI на другом порту. Не продолжайте upload, пока `check-ops` не вернёт 200.

**4. (Опционально) dry-run без записи** — отловит пересечения диапазонов и лимиты:

```bash
curl -sS -w "\nHTTP %{http_code}\n" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: text/csv" \
  --data-binary @/opt/geoatlas/geoip.csv \
  "http://127.0.0.1/upload-geo?dry_run=1"
```

**5. Залить CSV:**

```bash
curl -sS -w "\nHTTP %{http_code}\n" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: text/csv" \
  --data-binary @/opt/geoatlas/geoip.csv \
  "http://127.0.0.1/upload-geo"
```

Ожидаете JSON `{"ok":true,"ranges":N}` и HTTP `200`.

**6. Проверить результат:**

```bash
docker exec clickhouse sh -c \
  'clickhouse-client --password "$CLICKHOUSE_PASSWORD" -q "SELECT count() FROM geo_ranges"'
docker logs backend --since=10m 2>&1 | grep -iE 'geo index loaded|geo csv|upload|overlap|error'
```

В логах — `geo index loaded`. На большой базе backend 1–3 минуты грузит CPU/RAM (парсинг и in-memory индекс) — это нормально.

**Типичные ответы upload:**

| HTTP | Причина | Что делать |
|------|---------|------------|
| **401** | нет/неверный Bearer | шаг 2–3; токен только из `docker exec backend` |
| **400** `overlapping geo ranges` | в CSV пересекаются сети (границы включительные) | править CSV, снова `dry_run=1` |
| **409** | опасный replace / мало RAM / занят heavy-job | см. ниже: очистить базу или не перезаливать |
| **413** | файл или число ranges сверх лимита | уменьшить CSV / `GEOIP_UPLOAD_MAX_*` / профиль памяти |

Если nginx снова отдаёт 401, а `docker exec backend` токен верный — заливка мимо nginx (прямо в процесс backend):

```bash
TOKEN="$(docker exec backend printenv API_AUTH_TOKEN)"
docker cp /opt/geoatlas/geoip.csv backend:/tmp/geoip.csv
docker exec backend wget -qO- \
  --header="Authorization: Bearer $TOKEN" \
  --header="Content-Type: text/csv" \
  --post-file=/tmp/geoip.csv \
  http://127.0.0.1:8080/upload-geo
```

### Повторная загрузка большой GeoIP и HTTP 502 / OOM

Индекс GeoIP целиком держится в RAM backend. Повторный upload того же большого CSV (миллионы диапазонов), когда индекс уже загружен, снова парсит файл в память **поверх** существующего индекса → пик RAM удваивается → Docker cgroup может убить процесс (`oom-kill` / `Memory cgroup out of memory`). Снаружи это часто выглядит как **HTTP 502** в UI, а контейнер `backend` ненадолго перезапускается.

**Сервер теперь режет опасные upload до OOM:**

| Ограничение                     | Env                                                  | Откуда дефолт                                                                        |
|---------------------------------|------------------------------------------------------|--------------------------------------------------------------------------------------|
| Размер тела `/upload-geo`       | `GEOIP_UPLOAD_MAX_BYTES` (или `MAX_GEO_UPLOAD_SIZE`) | `install-profile.json` → `limits.backend.memory_gb` (small≈512 MiB, medium≈1 GiB, …) |
| Число диапазонов в CSV          | `GEOIP_UPLOAD_MAX_RANGES`                            | тот же профиль (small≈4 M)                                                           |
| Replace поверх крупного индекса | —                                                    | **409 до чтения body**, если индекс уже ≥ половины лимита ranges                     |
| Soft mem headroom               | из `install-profile` `limits.backend.memory_gb` × ¾  | **409 после parse**, если `HeapAlloc + upload ≈` выше soft limit (запас 512 MiB)     |

Ответы: **413** (файл/число ranges слишком велики), **409** (опасный replace / нехватка headroom / занят heavy-job слот).

Ход загрузки смотрите в логах: `geo upload start` → `geo csv parsed` / `geo upload early reject` / `geo upload rejected …`. Точечная правка одной строки — страница **База GeoIP** (`/geo-ranges`), не полный CSV.

---

## Веб-интерфейс

| URL                    | Страница              | Основные возможности                                                                                                                                 |
|------------------------|-----------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| `/login`               | Вход                  | Логин (роли admin / operator / dashboard); смена пароля при `must_reset_password`                                                                    |
| `/`                    | Карта / глобус        | 2D/3D, группировка, фильтры status/репутации, конструктор поиска и шаблоны, порог событий, mono-дуги, экспорт PNG, загрузка логов/GeoIP, health pill |
| `/reputation`          | Репутация IP          | Списки и URL-фиды, каталог источников, refresh по расписанию, ручная загрузка; модуль можно отключить при установке                                  |
| `/parse-errors`        | Журнал ошибок парсинга| Поиск, удаление выбранных / всех, отправка в тест парсеров                                                                                           |
| `/geo-missing`         | IP без GeoIP          | Адреса без координат; добавление в GeoIP; выгрузка CSV; мгновенная перефильтрация списка                                                             |
| `/geo-ranges`          | База GeoIP            | Просмотр/правка диапазонов, выгрузка CSV                                                                                                             |
| `/parser-test`         | Тест парсеров         | До 200 строк за запрос, пресеты по вендорам, статусы parsed/skipped/error                                                                            |
| `/system`              | Системный мониторинг  | Обзор / Pipeline (Syslog-NG queued/drops + ingest EPS) / Безопасность / Графики / Резервное копирование; алёрты, ёмкость, профиль установки          |
| `/dozzle/`             | Логи контейнеров      | Dozzle (профиль `dozzle`): realtime stdout стека, start/stop/restart; только administrator; вне React SPA                                            |
| `/users`               | Учётные записи        | Список/создание УЗ: administrator, operator, dashboard (скрыто, если UI-auth выключен)                                                               |
| `/api-tokens`          | API-токены            | Именованные Bearer со scope read/ops/admin; секрет показывается один раз                                                                             |
| `/change-password`     | Смена пароля          | Смена своего пароля                                                                                                                                  |

SPA (React Router): page-auth в UI; nginx `auth_request` для `/api/*` и `/dozzle/`. Карта и смена пароля — любой залогиненный (**administrator**, **operator**, **dashboard**); system / dozzle / parsers / geo / reputation / users / api-tokens — только **administrator**. Роль **dashboard** отличается от operator длительной cookie-сессией (видеостена).

### HTTP API

Контракт REST API (в т.ч. auth, events, geo, reputation, retention, tokens, search-templates, backups, аномалии, `/metrics`): [`openapi.yaml`](openapi.yaml), версия документа OpenAPI **1.15.0**. Пробы: `GET /api/live` (процесс), `GET /api/ready` (ClickHouse + ingest); `GET /api/health` — alias live. Остальные эндпоинты — cookie-сессия и/или Bearer (`API_AUTH_TOKEN` / именованный токен со scope). Prometheus scrape: `GET /metrics` (Bearer≥ops / administrator).

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
