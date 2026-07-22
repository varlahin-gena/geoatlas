# ГеоАтлас

Визуализатор сетевых взаимодействий на базе логов межсетевых экранов (МСЭ).
Принимает syslog с файрволов или из SIEM, парсит события, обогащает их геоданными
(IP → страна/координаты) и строит карту сетевых связей в веб-интерфейсе.

---

## Содержание

- [Возможности](#возможности)
- [Архитектура](#архитектура)
- [Требования](#требования)
- [Быстрый старт](#быстрый-старт)
- [Установка](#установка)
  - [Ubuntu (автоматическая)](#ubuntu-автоматическая)
  - [Oracle Linux / RHEL (автоматическая)](#oracle-linux--rhel-автоматическая)
  - [Ручная установка](#ручная-установка)
- [Удаление](#удаление)
  - [Ubuntu](#ubuntu)
  - [Oracle Linux / RHEL](#oracle-linux--rhel)
  - [Остановка без удаления проекта](#остановка-без-удаления-проекта)
- [Запуск и остановка](#запуск-и-остановка)
- [Обслуживание](#обслуживание)
  - [Профили производительности](#профили-производительности)
  - [Срок хранения (TTL / retention)](#срок-хранения-ttl--retention)
  - [Очистка данных ClickHouse](#очистка-данных-clickhouse)
  - [Backfill агрегатов рёбер](#backfill-агрегатов-рёбер)
  - [Мониторинг ingest](#мониторинг-ingest)
  - [Логи и диагностика](#логи-и-диагностика)
  - [Обновление системы](#обновление-системы)
  - [Пересборка образов](#пересборка-образов)
- [Настройка МСЭ и SIEM](#настройка-мсэ-и-siem)
- [GeoIP](#geoip)
- [Веб-интерфейс](#веб-интерфейс)
- [Авторизация](#авторизация)
- [HTTP API](#http-api)
- [Нагрузочное тестирование](#нагрузочное-тестирование)
- [Переменные окружения](#переменные-окружения)
- [Структура репозитория](#структура-репозитория)
- [CI](#ci)
- [Безопасность](#безопасность)
- [Устранение неполадок](#устранение-неполадок)

---

## Возможности

- Приём логов по **syslog** (TCP/UDP, порт 514)
- Ручная загрузка логов через веб-интерфейс
- Парсинг **UserGate, FortiGate, Cisco ASA, Cisco FTD (FirePower), Cowrie (honeypot)** и универсальный фолбэк (Generic KV).
- **Осознанный пропуск** событий: распознанные, но несетевые строки (например, часть `cowrie.*`) не попадают в `parse_errors`
- **Авторизация по умолчанию**: роли `administrator` / `operator`, cookie-сессии, CSRF, Bearer `API_AUTH_TOKEN`; управление УЗ в UI
- Загрузка и правка **GeoIP-базы** (CSV SIEM KUMA, страница `/geo-ranges.html`, IP без координат на `/geo-missing.html`)
- Хранение и аналитика в **ClickHouse**
- **Настраиваемый TTL (retention)** таблиц из UI `/system.html`
- Построение связей на карте (2D) и глобусе (3D); на карту попадают только узлы/рёбра с координатами
- Фильтрация: все / разрешённые / заблокированные (на клиенте, без повторного запроса к API)
- Группировка узлов: по IP / по подсети `/24` / **по городу (по умолчанию)** / по стране; при отсутствии города — фолбэк на центр страны
- **Тест парсеров** в браузере: статусы parsed / skipped / error, гео-обогащение, пресеты по вендорам
- **Журнал ошибок парсинга**: поиск, выборочное и полное удаление, передача строк в «Тест парсеров»
- Страница системного мониторинга (вкладки Обзор / Pipeline / Безопасность / Графики): метрики контейнеров, пайплайна (в т.ч. **UDP/TCP EPS**, drops), **форма TTL**, неуспешные логины, хранилище, профиль установки, **индикатор ёмкости**, алёрты
- Индикатор здоровья системы на главной странице (ссылка на `/system.html`)

---

## Архитектура

Система состоит из **пяти сервисов**, оркеструемых через Docker Compose.
Ядро всегда поднимается: **clickhouse + backend + frontend**.
`syslog-ng` и `stats-collector` включаются через compose-профили (`COMPOSE_PROFILES=syslog,stats` — по умолчанию в `start.sh`, если в `.env` ещё нет).

```
МСЭ (UserGate, FortiGate, …) или SIEM
      │ syslog (514/tcp, 514/udp)          [профиль syslog]
      ▼
 ┌───────────┐   TCP :1514 (@@nm/udp/@@, @@nm/tcp/@@)   ┌──────────┐
 │ syslog-ng │ ────────────────────────────────────────► │ backend  │
 └───────────┘                                         │ (Go)     │
                                                       │ parser + │
 пользователь ── login / upload-logs / upload-geo ───► geoip +   │
                                                       │ ingest + │
                                                       │ aggregator│
 ┌────────────────┐                                    └────┬─────┘
 │ stats-collector│ ─────────────────────────────────────►│ ClickHouse
 └────────────────┘   system_metrics   [профиль stats]      │ (25.8)
                                                       ┌─────▼──────┐
 браузер ──────────────── :80 ────────► frontend (nginx) ── /api/* ─────► │ traffic_*  │
                                      auth_request                    │ geo_ranges │
                                      (admin / login)                 │ parse_errors│
                                                                      └────────────┘
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
2. **syslog-ng** принимает сообщения, при необходимости буферизует на диск и пересылает их по TCP на `backend:1514` с маркерами транспорта (`@@nm/udp/@@` / `@@nm/tcp/@@`).
3. **backend** снимает маркеры транспорта, парсит строки, обогащает GeoIP, пишет в ClickHouse; при старте `Ensure*` создаёт/обновляет агрегаты (`traffic_edges_*`, geo), применяет TTL из `retention.json` и при необходимости делает backfill. Полная очередь ingest **не блокирует** TCP — лишние строки дропаются (`dropped_total`, алерты на `/system.html`).
4. **frontend** отдаёт статику и проксирует API-запросы на backend.
5. **stats-collector** каждые 30 секунд собирает метрики CPU/RAM контейнеров и состояние пайплайна (включая разбивку UDP/TCP).

Источник правды по схеме агрегатов/MV — Go `storage.Ensure*` (не `init.sql`). `init.sql` — только cold bootstrap базовых таблиц на пустом томе; `clickhouse/migrate_*.sql` — ручной ops-fallback.

### Хранение данных (TTL)

Дефолты ниже задаются в `init.sql` / `Ensure*`. После старта backend применяет значения из **`RETENTION_FILE`** (`/app/data/retention.json` на томе `auth-users`) через `ALTER TABLE … MODIFY TTL`. Менять можно в UI (`/system.html` → Pipeline → «Срок хранения») или через API — см. [Срок хранения (TTL / retention)](#срок-хранения-ttl--retention).

| Таблица / объект                    | Срок по умолчанию | Источник дефолта |
|------------------------------------|-------------------|------------------|
| `traffic_logs`                     | 30 дней           | `init.sql` / retention `traffic_logs_days` |
| `traffic_edges_daily` (+ MV)       | 30 дней           | `EnsureEdgesAgg` / retention `edges_days` |
| `traffic_edges_city_daily` (+ MV)  | 30 дней           | `EnsureGeoEdgesAgg` / retention `edges_days` |
| `traffic_edges_country_daily` (+ MV)| 30 дней          | `EnsureGeoEdgesAgg` / retention `edges_days` |
| `parse_errors`                     | 7 дней            | `init.sql` / retention `parse_errors_days` |
| `system_metrics`                   | 7 дней            | `init.sql` / retention `system_metrics_days` |
| `geo_ranges`                       | без TTL           | `init.sql` (не настраивается) |
| `nm_schema_version`                | без TTL           | `Ensure*` (метаданные схемы) |

Допустимый диапазон настраиваемых дней: **1…730**. На `traffic_logs` / `parse_errors` / `system_metrics` включён `ttl_only_drop_parts`: истечение удаляет дневную партицию целиком (без дорогих row-level TTL merges). Уменьшение TTL удалит старые партиции при следующем TTL merge/drop в ClickHouse.

Geo backfill (`ALTER UPDATE`) на старте (если не `SKIP_STARTUP_BACKFILL`) запускается **после** schema Ensure* и только по окну `GEO_BACKFILL_LOOKBACK_DAYS` (по умолчанию 7).

Системные логи ClickHouse (`trace_log`, `text_log`, `metric_log`, …) по умолчанию **отключены** (`clickhouse/config.d/z_system_logs.xml`): на малых объёмах данных они иначе раздуваются сильнее `traffic_logs` и держат высокий idle CPU. `query_log` остаётся включённым. Образ ClickHouse в compose: **`clickhouse/clickhouse-server:25.8.28.1`**.

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
- Открытые порты **80** (UI) и **514/tcp**, **514/udp** (syslog)
- Доступ к `/proc` и `/sys/fs/cgroup` хоста (нужен `stats-collector`)

### Сетевые порты

| Порт        | Протокол | Назначение              | Доступ снаружи |
|-------------|----------|-------------------------|----------------|
| 80          | TCP      | Веб-интерфейс           | Да             |
| 514         | TCP/UDP  | Syslog от МСЭ           | Да             |
| 8080        | TCP      | Backend API             | Нет (docker)   |
| 1514        | TCP      | Ingest от syslog-ng     | Нет (docker)   |
| 8123 / 9000 | TCP      | ClickHouse HTTP/native  | Нет (docker)   |

---

## Быстрый старт

После установки (см. ниже) система доступна по адресу:

```
http://<IP_сервера>/
```

По умолчанию включена UI-авторизация. Первый вход:

| Учётка | Пароль | Роль |
|--------|--------|------|
| `admin` | `admin` | administrator |
| `operator` | `operator` | operator |

Обе учётки создаются с `must_reset_password: true` — после входа нужно сменить пароль (мин. 8 символов).
`./start.sh` при необходимости генерирует `API_AUTH_TOKEN` и `SESSION_SECRET` в `.env`.

Проверка работоспособности:

```bash
curl -fsS http://127.0.0.1/api/health
```

---

## Установка

Каталог установки по умолчанию: **`/opt/network-monitor`**.

### Ubuntu (автоматическая)

Скрипт устанавливает Docker, клонирует репозиторий, **интерактивно предлагает модули** и профиль производительности (диалоги YAD / whiptail), настраивает UFW и запускает стек.

```bash
# Скачать скрипт (или скопировать из репозитория)
curl -fsSL -o install_ubuntu.sh \
  https://raw.githubusercontent.com/varlahin-gena/network_monitor/main/deploy/ubuntu/install_ubuntu.sh
chmod +x install_ubuntu.sh

# Запуск от root
sudo ./install_ubuntu.sh
```

**Что делает скрипт:**

1. Обновляет списки пакетов (`apt-get update`)
2. Устанавливает `curl`, `git`, `ufw`, `whiptail` (и `yad`, если есть графический дисплей)
3. Устанавливает Docker Engine и compose plugin (если ещё нет)
4. Клонирует или обновляет репозиторий в `/opt/network-monitor`
5. Спрашивает, какие модули ставить (checklist: авторизация, API-токен, syslog-ng, stats-collector)
6. Запускает детектор ресурсов и предлагает профиль (radiolist)
7. Настраивает UFW (порты 80 и при необходимости 514)
8. Вызывает `./start.sh` (можно отказаться на последнем шаге)

**Выбор модулей (интерактивно или через env):**

| Модуль | По умолчанию | Что даёт |
|--------|--------------|----------|
| UI-авторизация | вкл. | логин, роли admin/operator (`AUTH_DISABLED` при отказе) |
| API Bearer-токен | вкл. | защита мутирующих API (`API_AUTH_DISABLED` при отказе) |
| syslog-ng | вкл. | приём syslog на `:514` (Compose profile `syslog`) |
| stats-collector | вкл. | метрики / `system.html` (Compose profile `stats`) |

Ядро (ClickHouse + Backend + Frontend) ставится всегда.

**Переменные окружения установки (Ubuntu):**

| Переменная          | По умолчанию                                              | Описание                                      |
|---------------------|-----------------------------------------------------------|-----------------------------------------------|
| `REPO_URL`          | `https://github.com/varlahin-gena/network_monitor.git`   | URL репозитория                               |
| `BRANCH`            | `main`                                                    | Ветка для клонирования                        |
| `ENABLE_UFW`        | `1`                                                       | Настраивать правила UFW (`0` — пропустить)    |
| `UFW_AUTO_ENABLE`   | `0`                                                       | Автоматически включить UFW, если выключен     |
| `NM_AUTO_PROFILE`   | —                                                         | Принять рекомендованный профиль без вопросов  |
| `NM_FORCE_PROFILE`  | —                                                         | Принудительный профиль: `tiny`…`xlarge`       |
| `NM_SKIP_PROFILE`   | —                                                         | Не генерировать override (значения по умолчанию) |
| `NM_AUTO_MODULES`   | —                                                         | Все модули по умолчанию, без вопросов         |
| `NM_MODULES`        | —                                                         | Список: `auth,api_auth,syslog,stats` (или `all`) |
| `NM_ENABLE_AUTH`    | —                                                         | `0`/`1` — UI-авторизация                      |
| `NM_ENABLE_API_AUTH`| —                                                         | `0`/`1` — Bearer-токен API                    |
| `NM_ENABLE_SYSLOG`  | —                                                         | `0`/`1` — syslog-ng                           |
| `NM_ENABLE_STATS`   | —                                                         | `0`/`1` — stats-collector                     |
| `NM_UI`             | авто                                                      | Бэкенд диалогов: `yad` \| `whiptail` \| `dialog` \| `text` |

Интерактивные шаги используют **YAD** (если есть дисплей), иначе **whiptail/dialog**, иначе текстовые вопросы. Для CI/Ansible задайте `NM_AUTO_MODULES=1` / `NM_AUTO_PROFILE=1` или `NM_UI=text`.

**Примеры:**

```bash
# Установка с другой ветки и автоматическим включением UFW
sudo BRANCH=develop UFW_AUTO_ENABLE=1 ./install_ubuntu.sh

# Установка без интерактивного выбора профиля и модулей
sudo NM_AUTO_PROFILE=1 NM_AUTO_MODULES=1 ./install_ubuntu.sh

# Установка с принудительным профилем «medium»
sudo NM_FORCE_PROFILE=medium ./install_ubuntu.sh

# Без UI-авторизации, остальное по умолчанию
sudo NM_ENABLE_AUTH=0 ./install_ubuntu.sh

# Только ядро + syslog (без auth, api_auth и stats)
sudo NM_MODULES=syslog ./install_ubuntu.sh
```

---

### Oracle Linux / RHEL (автоматическая)

Поддерживаются Oracle Linux, RHEL, Rocky Linux, AlmaLinux, CentOS.

```bash
curl -fsSL -o install_oraclelinux.sh \
  https://raw.githubusercontent.com/varlahin-gena/network_monitor/main/deploy/oracle_linux/install_oraclelinux.sh
chmod +x install_oraclelinux.sh

sudo ./install_oraclelinux.sh
```

**Что делает скрипт:**

1. Удаляет конфликтующие пакеты (`podman`, `buildah`, `runc`) при необходимости
2. Устанавливает Docker CE из официального репозитория
3. Настраивает SELinux (`container_manage_cgroup`) или переводит в permissive (опционально)
4. Клонирует/обновляет репозиторий, предлагает модули и профиль, настраивает firewalld и запускает стек

**Переменные окружения установки (Oracle Linux):**

| Переменная          | По умолчанию                                              | Описание                                      |
|---------------------|-----------------------------------------------------------|-----------------------------------------------|
| `REPO_URL`          | `https://github.com/varlahin-gena/network_monitor.git`   | URL репозитория                               |
| `BRANCH`            | `main`                                                    | Ветка для клонирования                        |
| `ENABLE_FIREWALL`   | `1`                                                       | Настраивать firewalld (`0` — пропустить)      |
| `DISABLE_SELINUX`   | `0`                                                       | Перевести SELinux в permissive (`1`)          |
| `NM_AUTO_PROFILE`   | —                                                         | См. Ubuntu                                    |
| `NM_FORCE_PROFILE`  | —                                                         | См. Ubuntu                                    |
| `NM_SKIP_PROFILE`   | —                                                         | См. Ubuntu                                    |
| `NM_AUTO_MODULES`   | —                                                         | См. Ubuntu                                    |
| `NM_MODULES`        | —                                                         | См. Ubuntu                                    |
| `NM_ENABLE_AUTH`    | —                                                         | См. Ubuntu                                    |
| `NM_UI`             | авто                                                      | См. Ubuntu (`yad` \| `whiptail` \| `dialog` \| `text`) |

**Примеры:**

```bash
# Установка с отключением SELinux enforcement
sudo DISABLE_SELINUX=1 ./install_oraclelinux.sh

# Установка без настройки firewalld
sudo ENABLE_FIREWALL=0 ./install_oraclelinux.sh

# Без UI-авторизации, авто-профиль
sudo NM_ENABLE_AUTH=0 NM_AUTO_PROFILE=1 ./install_oraclelinux.sh
```

---

### Ручная установка

Если автоматические скрипты не подходят:

```bash
# 1. Установить Docker и compose plugin (см. docs.docker.com)

# 2. Клонировать репозиторий
sudo mkdir -p /opt
sudo git clone -b main https://github.com/varlahin-gena/network_monitor.git /opt/network-monitor
cd /opt/network-monitor

# 3. Права на скрипты
chmod +x start.sh stop.sh scripts/tune-resources.sh \
  deploy/common/detect_resources.sh deploy/common/select_modules.sh deploy/common/ui.sh

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
sudo ufw allow 514/tcp
sudo ufw allow 514/udp

# Oracle Linux / RHEL (firewalld)
sudo firewall-cmd --permanent --add-port=80/tcp
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

Скрипт покажет **аудит** (контейнеры, volumes, размер каталога) и предложит **интерактивное меню** (YAD / whiptail):
1. Безопасное удаление — stop + файлы + firewall, данные ClickHouse сохраняются
2. Полное удаление (purge) — включая volumes и образы
3. Только остановить стек
4. Настроить вручную

Бэкенд диалогов тот же, что при установке (`NM_UI`).

### Ubuntu / Debian

```bash
sudo bash deploy/ubuntu/uninstall_ubuntu.sh
```

### Oracle Linux / RHEL

```bash
sudo bash deploy/oracle_linux/uninstall_oraclelinux.sh
```

### CLI-опции

| Опция | Описание |
|-------|----------|
| `--dry-run` | Показать аудит и план без изменений |
| `-y`, `--yes` | Без подтверждения (`FORCE=1`) |
| `--clean` | Preset: stop + файлы + firewall (данные сохраняются) |
| `--purge` | Preset: полное удаление включая ClickHouse |
| `--stop` | Preset: только остановить docker compose |
| `--volumes` | Удалить Docker volumes |
| `--images` | Удалить локально собранные образы |
| `--keep-files` | Сохранить каталог `/opt/network-monitor` |
| `--no-firewall` | Не трогать правила firewall |

### Переменные окружения (Ansible / CI)

| Переменная              | По умолчанию | Описание                                           |
|-------------------------|--------------|----------------------------------------------------|
| `NM_UNINSTALL_PRESET`   | —            | `stop` \| `clean` \| `purge`                       |
| `NM_DRY_RUN`            | `0`          | `1` — только план                                  |
| `REMOVE_DOCKER_VOLUMES` | `0`          | `1` — удалить тома ClickHouse (**необратимо**)     |
| `REMOVE_PROJECT_FILES`  | `1`          | `0` — сохранить каталог `/opt/network-monitor`     |
| `REMOVE_IMAGES`         | `0`          | `1` — удалить локально собранные образы            |
| `REMOVE_FIREWALL_RULES` | `1`          | Удалить правила firewall (UFW / firewalld)         |
| `REMOVE_UFW_RULES`      | —            | Legacy alias для Ubuntu → `REMOVE_FIREWALL_RULES`    |
| `FORCE`                 | `0`          | `1` — без интерактивного подтверждения             |
| `NM_UI`                 | авто         | `yad` \| `whiptail` \| `dialog` \| `text`          |

**Примеры:**

```bash
# Интерактивно с аудитом
sudo bash deploy/uninstall.sh

# Посмотреть план без изменений
sudo bash deploy/uninstall.sh --dry-run

# Полное удаление без вопросов
sudo bash deploy/uninstall.sh --purge --yes

# Только остановить стек, сохранить данные и файлы
sudo bash deploy/uninstall.sh --stop --yes

# CI/Ansible — полная зачистка
sudo NM_UNINSTALL_PRESET=purge FORCE=1 bash deploy/uninstall.sh

# Остановить стек и удалить образы, но сохранить данные
sudo REMOVE_IMAGES=1 REMOVE_DOCKER_VOLUMES=0 FORCE=1 bash deploy/uninstall.sh
```

> Docker Engine при удалении **не деинсталлируется** — только стек ГеоАтлас.

### Остановка без удаления проекта

```bash
cd /opt/network-monitor
./stop.sh
```

Остановка **с удалением данных ClickHouse**:

```bash
REMOVE_DOCKER_VOLUMES=1 ./stop.sh
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
```

**Переменные `start.sh`:**

| Переменная        | По умолчанию              | Описание                          |
|-------------------|---------------------------|-----------------------------------|
| `DO_BUILD`        | `1`                       | `0` — не пересобирать образы      |
| `HEALTH_TIMEOUT`  | `120`                     | Таймаут ожидания `/api/health`, с |
| `HEALTH_URL`      | `http://127.0.0.1/api/health` | URL проверки здоровья         |

**Переменные `stop.sh`:**

| Переменная              | По умолчанию | Описание                                |
|-------------------------|--------------|-----------------------------------------|
| `REMOVE_DOCKER_VOLUMES` | `0`          | `1` — удалить тома (`docker compose down -v`) |

**Прямые команды Docker Compose:**

```bash
cd /opt/network-monitor

# Статус контейнеров
docker compose ps

# Запуск в фоне
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

| Файл                              | Назначение                                    |
|-----------------------------------|-----------------------------------------------|
| `docker-compose.override.yml`     | Лимиты CPU/RAM контейнеров, параметры ingest  |
| `.env`                            | Переменные для compose                        |
| `clickhouse/users.d/zz_install_limits.xml` | Лимиты памяти запросов ClickHouse    |
| `install-profile.json`            | Сводка профиля (отображается в UI «Система»)  |

**Доступные профили:**

| Профиль  | Сервер (ориентир)       | ClickHouse | Backend | Workers | EPS (событий/с) |
|----------|-------------------------|------------|---------|---------|-----------------|
| `tiny`   | ≤2 CPU / ≤4 GiB RAM     | 2 GiB      | 1 GiB   | 1       | 500 – 2 000     |
| `small`  | ≤4 CPU / ≤8 GiB RAM     | 3 GiB / 3 CPU | 2 GiB / 2 CPU | 2   | 5 000 – 12 000  |
| `medium` | ≤8 CPU / ≤16 GiB RAM    | 6 GiB      | 4 GiB   | 4       | 10 000 – 25 000 |
| `large`  | ≤16 CPU / ≤32 GiB RAM   | 12 GiB     | 8 GiB   | 8       | 25 000 – 80 000 |
| `xlarge` | >16 CPU / >32 GiB RAM   | 24 GiB     | 16 GiB  | 12      | 80 000 – 200 000|

**Пересчёт профиля после изменения ресурсов сервера:**

```bash
cd /opt/network-monitor

# Интерактивный выбор
./scripts/tune-resources.sh

# Автоматически — рекомендованный профиль
NM_AUTO_PROFILE=1 ./scripts/tune-resources.sh

# Принудительно задать профиль
NM_FORCE_PROFILE=large ./scripts/tune-resources.sh

# Без override (вернуться к значениям docker-compose.yml)
NM_SKIP_PROFILE=1 ./scripts/tune-resources.sh
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

TTL таблиц ClickHouse настраивается без правки `init.sql`. Настройки хранятся в JSON на томе `auth-users` (`RETENTION_FILE`, по умолчанию `/app/data/retention.json`) и применяются:

1. при **старте backend** (после Ensure*, usecase `retention.ApplyFromStore`);
2. сразу при **сохранении** из UI или `PUT /api/system/retention` (`ALTER TABLE … MODIFY TTL`).

| Поле JSON | Таблицы | Дефолт |
|-----------|---------|--------|
| `traffic_logs_days` | `traffic_logs` | 30 |
| `edges_days` | `traffic_edges_daily`, `traffic_edges_city_daily`, `traffic_edges_country_daily` | 30 |
| `parse_errors_days` | `parse_errors` | 7 |
| `system_metrics_days` | `system_metrics` | 7 |

Диапазон каждого поля: **1…730** дней. `geo_ranges` без TTL.

**UI:** `/system.html` → вкладка **Pipeline** → блок «Срок хранения (TTL)» (только administrator).

**API:**

```bash
TOKEN="$(grep -E '^API_AUTH_TOKEN=' .env | cut -d= -f2-)"

# текущие значения
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1/api/system/retention | jq .

# изменить (CSRF не нужен для Bearer)
curl -s -X PUT http://127.0.0.1/api/system/retention \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"traffic_logs_days":60,"edges_days":60,"parse_errors_days":14,"system_metrics_days":14}' | jq .
```

Ответ содержит объект `retention` (и `updated_at` после сохранения). Для cookie-сессии на `PUT` нужен CSRF (`nm_csrf` / `X-CSRF-Token`).

---

### Очистка данных ClickHouse

Удаляет все события, GeoIP, ошибки парсинга и метрики. **Схема таблиц сохраняется.**

```bash
cd /opt/network-monitor
bash clickhouse/reset_data.sh
```

Скрипт выполняет `TRUNCATE` таблиц из `clickhouse/reset_data.sql` и перезапускает backend.

> `reset_data.sql` очищает `traffic_logs`, `traffic_edges_daily`, `geo_ranges`, `parse_errors`, `system_metrics`.
> Таблицы `traffic_edges_city_daily` / `traffic_edges_country_daily` и `nm_schema_version` **не** truncate'ятся — при полной зачистке выполните отдельно или пересоздайте volume.

**Ручная очистка через clickhouse-client:**

```bash
cd /opt/network-monitor
docker compose exec -T clickhouse clickhouse-client --multiquery < clickhouse/reset_data.sql
docker compose restart backend
```

**Проверка количества строк:**

```bash
docker compose exec -T clickhouse clickhouse-client --query "
  SELECT 'traffic_logs' AS tbl, count() FROM traffic_logs
  UNION ALL SELECT 'geo_ranges', count() FROM geo_ranges
  UNION ALL SELECT 'parse_errors', count() FROM parse_errors
  FORMAT PrettyCompact"
```

---

### Backfill агрегатов рёбер

При старте backend **автоматически**:

1. `EnsureTrafficLogsSuccess` — выражение `success` MATERIALIZED  
2. `EnsureEdgesAggSchema` / `EnsureGeoEdgesAggSchema` — таблицы/MV  
3. По умолчанию сразу же `BackfillEdgesAgg` / `BackfillGeoEdgesAgg` (+ geo enrich)  

Чтобы **не** грузить ClickHouse тяжёлым backfill при каждом рестарте (прод под нагрузкой):

```bash
# в .env / docker-compose
SKIP_STARTUP_BACKFILL=1
```

Тогда на старте только schema + проверка «уже ready?»; иначе state=`pending`,
карта временно читает `traffic_logs`. Запуск backfill вручную:

```bash
TOKEN="$(grep -E '^API_AUTH_TOKEN=' .env | cut -d= -f2-)"
curl -X POST http://127.0.0.1/api/system/maintenance/backfill \
  -H "Authorization: Bearer $TOKEN"
# прогресс:
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1/api/system/edges-agg | jq .
```

Предпочтительный путь после сбоя Ensure*: **перезапустить backend** (если `SKIP_STARTUP_BACKFILL` не задан)
или вызвать `POST /api/system/maintenance/backfill`.

Если нужно вручную через SQL (бэкап, Ensure не дошёл):

```bash
cd /opt/network-monitor
bash clickhouse/backfill_edges_agg.sh
```

Скрипт применяет `migrate_edges_agg.sql` и дозаполняет дни из `traffic_logs`. Списки action в SQL-fallback могут отставать от `model` — после ручного migrate лучше всё равно перезапустить backend или вызвать maintenance/backfill.
---

### Мониторинг ingest

Скрипт для наблюдения за скоростью приёма событий (удобен при нагрузочном тестировании):

```bash
cd /opt/network-monitor
./scripts/watch-ingest.sh          # интервал 2 сек (по умолчанию)
./scripts/watch-ingest.sh 5        # интервал 5 сек
```

Требует `curl` и `jq`. Выводит: recv/s, ins/s, **drop/s**, dropped, buffered, connections, state, queue depth.

**Capacity SLO (drops):** при полной очереди (`INGEST_QUEUE_SIZE`) новые строки **дропаются** (non-blocking), TCP не блокируется. Целевое состояние — `drops_per_sec == 0`.

| Условие | Алерт (`/system.html`) | Действие |
|---------|------------------------|----------|
| `drops_per_sec == 0` и `dropped_total == 0` | — | норма |
| `drops_per_sec > 0` | `ingest_dropping` (warn) | инцидент ёмкости: поднять профиль (`tune-resources.sh`) или снизить EPS на входе |
| `drops_per_sec >= 100` | `ingest_dropping_critical` (error) | срочно: профиль / ограничение источников |
| `dropped_total > 0` при нулевом rate | `ingest_dropped_total` (warn) | был пик с прошлым стартом — проверить, не растёт ли счётчик |

**Проверка backpressure / drops (soak):**

```bash
# Ускорить TCP-drops: уменьшить очередь и перезапустить backend, затем flood на :514
# INGEST_QUEUE_SIZE=500 docker compose up -d backend
./scripts/soak-queue-drops.sh
```

Скрипт грузит `POST /api/ingest` и печатает `dropped_total` + ingest alerts из `/api/system/stats`. Unit-покрытие non-blocking drops: `go test ./internal/ingest/ -run EnqueueFlood`.

**Пример ответа `/api/ingest/stats`:**

```bash
TOKEN="$(grep -E '^API_AUTH_TOKEN=' .env | cut -d= -f2-)"
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1/api/ingest/stats | jq '{
  state, received_total, inserted_total, dropped_total,
  queue_depth, queue_capacity, buffered_lines,
  udp: .udp.received_total, tcp: .tcp.received_total
}'
```

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

# Проверка API (health — публичный; остальное — с Bearer из .env)
curl -fsS http://127.0.0.1/api/health
TOKEN="$(grep -E '^API_AUTH_TOKEN=' .env | cut -d= -f2-)"
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1/api/ingest/stats | jq .
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1/api/system/stats | jq .
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1/api/system/edges-agg | jq .

# Запросы к ClickHouse
docker compose exec clickhouse clickhouse-client --query "SELECT 1"
docker compose exec clickhouse clickhouse-client --query "
  SELECT vendor, count() AS cnt FROM traffic_logs GROUP BY vendor ORDER BY cnt DESC"
```

**Типичные проблемы и куда смотреть:**

| Симптом                        | Куда смотреть                                      |
|--------------------------------|----------------------------------------------------|
| Нет событий на карте           | `docker compose logs syslog-ng`, `/api/ingest/stats` |
| Ошибки парсинга                | UI → «Ошибки парсинга», таблица `parse_errors`     |
| Backend не стартует            | `docker compose logs backend`, healthcheck         |
| ClickHouse OOM / медленные запросы | `install-profile.json`, увеличить профиль       |
| ClickHouse idle CPU высокий, мало данных | `system.trace_log`/`text_log` — см. `config.d/z_system_logs.xml`; `TRUNCATE`/`DROP` старых system-логов |
| Нет метрик на странице «Система» | `docker compose logs stats-collector`, cgroup, `/proc` и `/sys/fs/cgroup` на хосте |
| Превышена расчётная ёмкость      | `/system.html` → алёрты `capacity_high` / `capacity_exceeded`, пересчёт профиля |
| Drops под нагрузкой / очередь полная | `/api/ingest/stats` → `dropped_total`, `/system.html` → `ingest_dropping*`, `./scripts/watch-ingest.sh` |
| UDP/TCP EPS не разделяются       | Перезапустить `syslog-ng` (маркеры `@@nm/udp/@@` / `@@nm/tcp/@@`) |
| TTL не применился / старые данные остаются | `/api/system/retention`, логи backend `retention:`, том `auth-users` (`retention.json`) |

---

### Обновление системы

**Через повторный запуск install-скрипта** (рекомендуется — скрипт делает `git pull`):

```bash
sudo ./deploy/ubuntu/install_ubuntu.sh
# или
sudo ./deploy/oracle_linux/install_oraclelinux.sh
```

Локальные изменения будут сохранены в `git stash` перед обновлением.

**Вручную:**

```bash
cd /opt/network-monitor
./stop.sh
git fetch origin
git pull --ff-only origin main
./start.sh
```

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

## Настройка МСЭ и SIEM

Направьте syslog-события на IP сервера ГеоАтлас:

```
<IP_сервера>:514
```

Поддерживаются **TCP и UDP**. Рекомендуется TCP для надёжной доставки при высокой нагрузке.

**UserGate** — настройка remote syslog на NGFW.

**FortiGate** — `config log syslogd setting` / `config log syslogd2 setting`.

**Cisco ASA / FTD** — logging host, logging enable.

**Cowrie (honeypot)** — `output:jsonlog` (`cowrie.json`) или `output:remotesyslog` на `:514`; в граф попадают `cowrie.session.connect` и `cowrie.login.*` с парой src/dst IP. События `direct-tcpip.*` пропускаются (dst там — цель прокси, не honeypot).

**Из SIEM** — пересылка (forward) сырых событий МСЭ на `:514` в неизменённом виде.

После настройки проверьте поступление:

```bash
TOKEN="$(grep -E '^API_AUTH_TOKEN=' .env | cut -d= -f2-)"
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1/api/ingest/stats \
  | jq '{received: .received_total, state: .state, connections: .connections}'
```

---

## GeoIP

GeoIP-база загружается через веб-интерфейс (кнопка загрузки на главной странице) в виде CSV
или пополняется по одному диапазону со страницы **IP без GeoIP** (`/geo-missing.html`).
Поддерживается формат SIEM KUMA.

**Формат CSV:**

```csv
Network,Country,Region,City,Latitude,Longitude
10.0.0.0-10.0.255.254,Россия,OOO_Company,Москва,55.76167,37.60667
192.168.1.0-192.168.1.255,Россия,LAN,Office,55.75,37.62
```

На странице `/geo-missing.html` для IP без координат доступна кнопка **«добавить в базу»**
(форма с полями шаблона выше) и **«Выгрузить GeoIP CSV»** — скачивание актуальной базы.

**Через API** (если задан `API_AUTH_TOKEN`):

```bash
curl -X POST http://127.0.0.1/upload-geo \
  -H "Authorization: Bearer <API_AUTH_TOKEN>" \
  -F "file=@geo.csv"

# добавить один диапазон
curl -X POST http://127.0.0.1/api/geo-ranges \
  -H "Authorization: Bearer <API_AUTH_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"network":"203.0.113.10","country":"Россия","region":"Org","city":"Москва","lat":55.75,"lon":37.62}'

# выгрузить базу
curl -OJ http://127.0.0.1/api/geo-ranges/export \
  -H "Authorization: Bearer <API_AUTH_TOKEN>"
```

После загрузки индекс GeoIP перезагружается **в фоне** через `adapter/geojob.Scheduler` (сериализация: без параллельных ALTER; ответ API сразу, поле `reload: "scheduled"`). Для больших баз это может занять несколько минут.

`GEO_ENRICH_ON_INGEST=true` (по умолчанию) заполняет пустые/unknown/Reserved/ISO country из GeoIP при INSERT вместе с city/coords. `false` — аварийный opt-out только для country. Post-ingest backfill (`EnrichLogsMissingGeo` в окне `GEO_BACKFILL_LOOKBACK_DAYS`) дозаполняет и координаты, и страну, затем пересобирает daily geo-edges.

Без GeoIP-базы узлы на карте не отображаются (нет координат).

---

## Веб-интерфейс

| URL                    | Страница              | Основные возможности |
|------------------------|-----------------------|----------------------|
| `/login.html`          | Вход                  | Логин (роли admin / operator); смена пароля при `must_reset_password` |
| `/`                    | Карта / глобус        | 2D/3D, группировка (город по умолчанию), фильтры, поиск, порог событий, загрузка логов/GeoIP, индикатор здоровья |
| `/parse-errors.html`   | Журнал ошибок парсинга| Поиск, удаление выбранных / всех, отправка в тест парсеров |
| `/geo-missing.html`    | IP без GeoIP          | Адреса без координат; добавление в GeoIP; мгновенная перефильтрация списка |
| `/geo-ranges.html`     | База GeoIP            | Просмотр/правка диапазонов, выгрузка CSV |
| `/parser-test.html`    | Тест парсеров         | До 200 строк за запрос, пресеты по вендорам, статусы parsed/skipped/error |
| `/system.html`         | Системный мониторинг  | Вкладки: Обзор (профиль, health, контейнеры), Pipeline (EPS/drops, **TTL retention**), Безопасность (неуспешные логины), Графики; алёрты и ёмкость |
| `/users.html`          | Учётные записи        | Список/создание УЗ (только administrator) |
| `/change-password.html`| Смена пароля          | Смена своего пароля |

Nginx: карта и смена пароля — любой залогиненный; system / parsers / geo / users — только **administrator** (или Bearer / `AUTH_DISABLED`).

---

## Авторизация

По умолчанию UI и API **закрыты**. Открытый режим только для local/dev: `AUTH_DISABLED=1` и/или `API_AUTH_DISABLED=1` **и** `NM_ALLOW_INSECURE=1`.

| Механизм | Как работает |
|----------|--------------|
| Cookie `nm_session` | HMAC-сессия после `POST /api/auth/login` (HttpOnly, SameSite=Strict, TTL = `SESSION_TTL_HOURS`) |
| Cookie `nm_csrf` + заголовок `X-CSRF-Token` | Обязательны для мутирующих запросов с cookie-сессией; Bearer CSRF не требует |
| Bearer `API_AUTH_TOKEN` | Доступ уровня administrator к admin/ops и к карте |
| Роли | `administrator` — полный UI/API; `operator` — карта + `GET /api/system/status` + смена своего пароля |
| Хранилище УЗ | volume `auth-users` → `/app/data/users.json` (`AUTH_USERS_FILE`) |
| Throttle логина | ~10 неудач / IP / мин → блокировка ~5 мин; счётчик в `/api/system/stats` |

**Нюанс:** `API_AUTH_DISABLED=1` открывает только **ops**-маршруты (ingest, upload, export geo, metrics). Admin-маршруты (system, parse-*, users, geo-missing list) по-прежнему требуют administrator или Bearer.

---

## HTTP API

Слои middleware:

| Слой | Условие доступа |
|------|-----------------|
| **public** | без проверки |
| **loginMW** | сессия / Bearer / `AUTH_DISABLED` |
| **adminMW** | administrator / Bearer / `AUTH_DISABLED` |
| **opsMW** | administrator / Bearer / `AUTH_DISABLED` / `API_AUTH_DISABLED` |
| **csrf** | мутации с cookie-сессией (Bearer и `AUTH_DISABLED` пропускают) |

Frontend nginx дополнительно проверяет `auth_request` на `/api/*` и uploads (кроме `/api/health` и `/api/auth/*`). Backend — источник правды по ролям.

### Публичные

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/health`, `/api/health` | Liveness/readiness (включая ping ClickHouse) |
| POST | `/api/auth/login` | Вход (JSON: username, password) |
| POST | `/api/auth/logout` | Выход (+ CSRF при сессии) |
| GET | `/api/auth/me` | Текущий пользователь (401 без сессии) |
| POST | `/api/auth/change-password` | Смена пароля (+ CSRF) |
| GET | `/api/auth/check` | Probe для nginx (любой логин / Bearer) |
| GET | `/api/auth/check-admin` | Probe для nginx (admin / Bearer / `API_AUTH_DISABLED`) |

### Карта (loginMW)

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/events` | События / рёбра для карты |
| GET | `/api/system/status` | Краткий статус (индикатор «Система ОК») |

**Параметры `GET /api/events`:**

| Параметр   | По умолчанию | Описание |
|------------|--------------|----------|
| `days`     | `1`          | Глубина выборки, 1–30 дней (если не заданы другие параметры времени) |
| `minutes`  | —            | Скользящее окно в минутах (1–43200), например `15` |
| `hours`    | —            | Скользящее окно в часах (1–720), например `6` |
| `from`     | —            | Начало произвольного диапазона (RFC3339 или `YYYY-MM-DDTHH:MM`) |
| `to`       | сейчас       | Конец произвольного диапазона (RFC3339 или `YYYY-MM-DDTHH:MM`) |
| `group_by` | `city`       | `ip` \| `subnet` \| `city` \| `country` |
| `limit`    | `10000`      | Ограничение сырых пар (макс. 50 000) |

Узлы и рёбра без GeoIP-координат **всегда** отфильтровываются (`stats.skipped_no_geo` в ответе).

### Управление УЗ (adminMW)

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/users` | Список учёток |
| POST | `/api/users` | Создать УЗ |
| POST | `/api/users/{username}/role` | Сменить роль |
| POST | `/api/users/{username}/full-name` | Сменить ФИО |
| POST | `/api/users/{username}/reset-password` | Сбросить пароль (`must_reset_password`) |
| DELETE | `/api/users/{username}` | Удалить УЗ |

### Ops (opsMW) — administrator / Bearer / `API_AUTH_DISABLED`

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/ingest/stats` | Статистика ingest (UDP/TCP, backlog, drops, connections) |
| GET | `/api/geo-ranges/export` | Скачать GeoIP CSV (формат SIEM KUMA) |
| POST | `/api/geo-ranges` | Добавить один диапазон |
| PUT | `/api/geo-ranges` | Изменить диапазон (`original_network` + новые поля) |
| POST | `/api/ingest` | Пакетная отправка строк логов (до 1 GiB) |
| POST | `/upload-logs` | Загрузка файла логов (до 1 GiB) |
| POST | `/upload-geo` | Загрузка GeoIP CSV (до 1 GiB); `?dry_run=1` — только валидация |

### Admin (adminMW) — administrator / Bearer / `AUTH_DISABLED`

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/system/stats` | Системные метрики и алёрты |
| GET | `/api/system/history` | История метрик (`?period=1h\|6h\|24h\|7d`) |
| GET | `/api/system/edges-agg` | Статус агрегации рёбер |
| POST | `/api/system/maintenance/backfill` | Запуск edges/geo backfill (+ geo enrich), async 202 |
| GET | `/api/system/install-profile` | Профиль установки |
| GET | `/api/system/retention` | Текущие TTL (дни) таблиц CH |
| PUT | `/api/system/retention` | Сохранить TTL + `ALTER TABLE MODIFY TTL` (1…730; + CSRF для cookie) |
| GET | `/api/geo-missing` | IP без координат для карты |
| GET | `/api/geo-ranges` | Список диапазонов `geo_ranges` (JSON) |
| GET | `/api/parse-errors` | Журнал ошибок парсинга |
| GET | `/api/parse-samples` | Примеры событий для теста |
| POST | `/api/parse-test` | Тест разбора строк (до 8 MiB, макс. 200 строк) |
| POST | `/api/parse-errors/delete` | Удаление по `ids[]` или `{"all": true}` |

**Параметры `GET /api/parse-errors`:**

| Параметр | По умолчанию | Описание |
|----------|--------------|----------|
| `limit`  | `500`        | Макс. 5 000 записей |
| `search` | —            | Поиск по `raw` и `reason` |

**Лимиты тела запроса:** логи и GeoIP — до **1 GiB** (`MAX_LOG_UPLOAD_SIZE` / `MAX_GEO_UPLOAD_SIZE`); JSON — до **1 MiB**; parse-test — до **8 MiB**.

Bearer-токен:

```
Authorization: Bearer <API_AUTH_TOKEN>
```

Read-эндпоинты защищены **таймаутами** (60 с для тяжёлых запросов, 5 с для health). В access-логах и ответе есть **`X-Request-ID`** (принять от клиента или сгенерировать).

**Семантика ingest:** запись в ClickHouse — **at-least-once**. HTTP `/api/ingest` и `/upload-logs` ставят строки в **ту же очередь**, что syslog TCP. При полной очереди (`INGEST_QUEUE_SIZE`) строки **дропаются** (non-blocking): syslog TCP не блокируется; HTTP отвечает **503** + `Retry-After` и `stats.dropped` (не 200). Клиент должен backoff/retry. При сбое flush батч повторяется; после частичного ack возможны дубликаты в `traffic_logs` (для карты обычно приемлемо). Колонка `traffic_logs.raw` **не заполняется**. Ingest-пул по умолчанию — `async_insert=1` + `wait_for_async_insert=1` (`CH_INGEST_ASYNC_INSERT`). Буферы traffic/parse_errors при outage CH ограничены по размеру.

---

## Нагрузочное тестирование

Для проверки пределов ingest и API есть отдельный репозиторий [loadtest-for-network_monitor](https://github.com/varlahin-gena/loadtest-for-network_monitor):

- сценарии baseline, ramp, read, mixed, stress, soak, spike;
- генератор на основе `samples.go` с Zipf-распределением IP;
- тест полного пути syslog-ng → backend → ClickHouse;
- сценарии `dirty` (ошибки парсинга) и `skipped` (осознанный пропуск Cowrie).

Мониторинг во время теста: `./scripts/watch-ingest.sh` (в т.ч. **drop/s**) и страница `/system.html`.

Быстрый soak в этом репозитории (HTTP load + проверка stats/alerts):

```bash
./scripts/soak-queue-drops.sh
# API_AUTH_TOKEN=... SOAK_LINES=100000 ./scripts/soak-queue-drops.sh
```

Для форсированных TCP queue drops: уменьшить `INGEST_QUEUE_SIZE`, перезапустить backend и flood на `:514`.

---

## Переменные окружения

### Backend

| Переменная                    | По умолчанию              | Описание |
|-------------------------------|---------------------------|----------|
| `LISTEN_ADDR`                 | `:8080`                   | Адрес HTTP API |
| `INGEST_LISTEN_ADDR`          | `:1514`                   | Единый ingest-listener (TCP от syslog-ng) |
| `INGEST_UDP_LISTEN_ADDR`      | *(пусто)*                 | Отдельный listener для UDP (альтернатива единому) |
| `INGEST_TCP_LISTEN_ADDR`      | *(пусто)*                 | Отдельный listener для TCP (альтернатива единому) |
| `CLICKHOUSE_HOST`             | `clickhouse`              | Хост ClickHouse |
| `CLICKHOUSE_PORT`             | `9000`                    | Порт ClickHouse (native) |
| `CLICKHOUSE_USER`             | `default`                 | Пользователь ClickHouse |
| `CLICKHOUSE_PASSWORD`         | *(пусто)*                 | Пароль ClickHouse |
| `CLICKHOUSE_DATABASE`         | `default`                 | База ClickHouse |
| `INGEST_WORKERS`              | `4`                       | Число worker-горутин ingest |
| `INGEST_BATCH_SIZE`           | `10000`                   | Размер батча INSERT |
| `INGEST_QUEUE_SIZE`           | `300000`                  | Размер очереди строк (при переполнении — drop) |
| `INGEST_FLUSH_SEC`            | `3`                       | Интервал сброса батча, сек |
| `INGEST_MAX_CONNECTIONS`      | `256`                     | Макс. одновременных TCP-сессий ingest |
| `INGEST_CONN_IDLE_SEC`        | `300`                     | Idle timeout TCP-сессии ingest, сек |
| `QUERY_TIMEOUT_SEC`           | `180`                     | Таймаут запросов/ingest flush, сек |
| `GEO_ENRICH_ON_INGEST`        | `true`                    | Обогащать country из GeoIP при INSERT (`false` = opt-out) |
| `GEO_BACKFILL_LOOKBACK_DAYS`  | `7`                       | Окно geo mutations (`0` = весь объём) |
| `SKIP_STARTUP_BACKFILL`       | `false`                   | `true` — на старте только schema Ensure*; backfill через `POST /api/system/maintenance/backfill` |
| `MAX_LOG_UPLOAD_SIZE`         | `1073741824` (1 GiB)      | Лимит тела `/upload-logs`, `/api/ingest` |
| `MAX_GEO_UPLOAD_SIZE`         | `1073741824` (1 GiB)      | Лимит тела `/upload-geo` |
| `CH_MAX_MEMORY_USAGE`         | `2147483648`              | Лимит памяти запроса CH (байты) |
| `CH_EXTERNAL_GROUP_BY_BYTES`  | `268435456`               | Порог external GROUP BY |
| `CH_EXTERNAL_SORT_BYTES`      | `268435456`               | Порог external SORT |
| `CH_INGEST_MAX_OPEN_CONNS`    | `4`                       | Пул CH для write/ingest |
| `CH_INGEST_ASYNC_INSERT`      | `true`                    | `async_insert=1` + `wait_for_async_insert=1` на ingest-пуле |
| `CH_API_MAX_OPEN_CONNS`       | `8`                       | Пул CH для API-запросов |
| `CH_BACKGROUND_MAX_OPEN_CONNS`| `2`                       | Пул CH для Ensure*/adapter/geojob |
| `INSTALL_PROFILE_PATH`        | `/app/install-profile.json` | Путь к JSON профиля установки |
| `API_AUTH_TOKEN`              | *(обязателен)*            | Bearer-токен; генерируется `start.sh` → `.env` |
| `API_AUTH_DISABLED`           | `false`                   | Открыть opsMW без токена (только + `NM_ALLOW_INSECURE`) |
| `AUTH_DISABLED`               | `false`                   | Отключить UI-логин (только + `NM_ALLOW_INSECURE`) |
| `SESSION_SECRET`              | *(обязателен при UI auth)* | Секрет cookie-сессий; генерируется `start.sh` |
| `SESSION_TTL_HOURS`           | `12`                      | TTL сессии |
| `AUTH_ADMIN_USER` / `PASSWORD`| `admin` / `admin`         | Seed administrator (только если файла УЗ ещё нет) |
| `AUTH_OPERATOR_USER` / `PASSWORD` | `operator` / `operator` | Seed operator |
| `AUTH_USERS_FILE`             | `/app/data/users.json`    | Файл учёток (volume `auth-users`) |
| `RETENTION_FILE`              | `/app/data/retention.json`| JSON с TTL таблиц CH (тот же том `auth-users`) |
| `NM_ALLOW_INSECURE`           | `0`                       | `1` — плейсхолдеры секретов и/или `*_DISABLED` (local/dev) |
| `LOG_LEVEL`                   | `info`                    | `debug` \| `info` \| `warn` \| `error` |
| `LOG_FORMAT`                  | `text`                    | `text` \| `json` |
| `TZ`                          | `Europe/Moscow`           | Часовой пояс |

### Compose / установка

| Переменная | Описание |
|------------|----------|
| `COMPOSE_PROFILES` | `syslog`, `stats` (через запятую); пусто = только ядро |
| `NM_MODULES` / `NM_AUTO_MODULES` | Выбор модулей при install (см. `deploy/common/select_modules.sh`) |

### stats-collector

| Переменная           | По умолчанию                              | Описание                    |
|----------------------|-------------------------------------------|-----------------------------|
| `COLLECT_INTERVAL`   | `30`                                      | Интервал сбора, сек         |
| `BACKEND_HEALTH_URL` | `http://backend:8080/health`              | URL health backend          |
| `INGEST_STATS_URL`   | `http://backend:8080/api/ingest/stats`    | URL статистики ingest       |
| `CGROUP_ROOT`        | `/sys/fs/cgroup`                          | Корень cgroup хоста         |
| `HOST_PROC`          | `/host/proc`                              | /proc хоста (внутри контейнера) |

---

## Структура репозитория

```
network_monitor/
├── go.work                           # workspace: backend + stats-collector + pkg/chconn
├── backend/                          # Go: API, парсеры, geoip, ingest, storage
│   ├── cmd/network-monitor/main.go
│   ├── Dockerfile
│   └── internal/
│       ├── adapter/
│       │   ├── httpapi/              # HTTP delivery (handlers, middleware, cookies/CSRF)
│       │   ├── clickhouse/           # CH repos + ReloadableGeoIndex + RetentionApplier
│       │   ├── geojob/               # сериализация reload GeoIP + backfill
│       │   ├── parseradapter/        # parser port
│       │   ├── geoipcodec/           # GeoIP CSV/CIDR helpers
│       │   ├── bootstrapadapter/     # Ensure*/Backfill* для usecase/bootstrap
│       │   ├── retentionfile/        # JSON-store TTL (`retention.json`)
│       │   └── systemlive/           # live ingest/profile adapters
│       ├── usecase/                  # application use cases + ports (bootstrap, retention, …)
│       ├── auth/                     # users / sessions / roles
│       ├── config/                   # конфигурация из env
│       ├── geoip/                    # импорт CSV и in-memory индекс
│       ├── ingest/                   # syslog TCP, очередь, batch INSERT
│       ├── installprofile/           # чтение install-profile.json
│       ├── logging/                  # slog setup
│       ├── model/                    # доменные структуры, action-списки
│       ├── mapagg/                   # агрегация рёбер/узлов для карты
│       ├── parser/                   # парсеры вендоров
│       └── storage/                  # ClickHouse (facade)
│           ├── aggstate/             # Prefer* / статус edges agg
│           ├── migrate/              # Ensure* / DDL / backfill (SoT схемы)
│           ├── query/                # Scan* / settings
│           └── sqlclause/            # общие SQL-выражения
├── pkg/
│   └── chconn/                       # общий ClickHouse connect
├── clickhouse/
│   ├── config.d/override.xml
│   ├── users.d/query_limits.xml
│   ├── init.sql                      # cold bootstrap базовых таблиц
│   ├── migrate_*.sql                 # ops-fallback (не SoT; предпочтителен Ensure*)
│   ├── backfill_edges_agg.sh
│   └── reset_data.sql / reset_data.sh
├── deploy/
│   ├── uninstall.sh
│   ├── common/                       # detect_resources, select_modules, ui, …
│   ├── ubuntu/
│   └── oracle_linux/
├── docker-compose.yml
├── frontend/                         # nginx + статика
│   ├── index.html / index.css        # карта / глобус
│   ├── system.html / system.css      # мониторинг
│   ├── login.html / change-password.html / auth-form.css
│   ├── users.html / parse-errors.html / geo-missing.html / geo-ranges.html / parser-test.html
│   ├── auth.js / common.js           # auth + общие UI-хелперы
│   ├── theme.css / common.css        # токены тем + общий каркас
│   ├── js/                           # map-*.js, system-app.js
│   ├── vendor/                       # deck.gl + uPlot (офлайн)
│   ├── favicon.svg
│   ├── data/countries.geojson        # контуры стран для карты
│   └── nginx.conf
├── scripts/
│   ├── tune-resources.sh
│   ├── watch-ingest.sh               # EPS + drop/s
│   ├── soak-queue-drops.sh           # smoke нагрузка + проверка drops/alerts
│   └── frontend-smoke.sh             # контракт auth/статики без полного стека
├── stats-collector/                  # Go: системные метрики
│   ├── main.go
│   └── internal/
│       ├── config/
│       └── collector/
├── openapi.yaml                      # контракт HTTP API (в т.ч. retention)
├── .github/workflows/ci.yml
├── start.sh / stop.sh
└── syslog-ng.conf
```

**Генерируемые при установке (не в git):**

```
/opt/network-monitor/
├── docker-compose.override.yml   # Лимиты по профилю
├── .env                          # COMPOSE_PROFILES, секреты, лимиты
├── install-profile.json          # Сводка установки
└── clickhouse/users.d/zz_install_limits.xml
```

Docker volume `auth-users` хранит `/app/data/users.json` и `/app/data/retention.json` между перезапусками.

---

## CI

GitHub Actions (`.github/workflows/ci.yml`) на `push`/`pull_request`:

| Job | Что делает |
|-----|------------|
| `backend` | `go vet`, `go test -race`, короткий fuzz (parser/ingest), golangci-lint |
| `chconn` | тесты `pkg/chconn` |
| `backend-integration` | integration-тесты storage против ClickHouse `25.8.28.1` |
| `stats-collector` | vet / test / lint |
| `frontend-smoke` | `scripts/frontend-smoke.sh` + smoke `login.html` / `auth.js` |

Локально без полного стека:

```bash
bash scripts/frontend-smoke.sh
cd backend && go test ./...
```

---

## Безопасность

- По умолчанию включены **UI-логин** и **Bearer-токен**; плейсхолдеры `dev-insecure-*` отвергаются без `NM_ALLOW_INSECURE=1`
- Seed-пароли `admin`/`admin` и `operator`/`operator` требуют смены при первом входе
- Мутирующие запросы с cookie защищены **CSRF** (`nm_csrf` / `X-CSRF-Token`)
- Сервисы запускаются с `no-new-privileges`
- `stats-collector` работает в режиме `read_only` с `tmpfs:/tmp`; `/proc` и cgroup хоста монтируются только на чтение
- Backend (8080) и ClickHouse (8123/9000) **не публикуются** наружу — доступны только внутри docker-сети `network-monitor`
- Backend обрабатывает **SIGINT/SIGTERM** с graceful shutdown (до 15 с); фоновые geo-job'ы корректно останавливаются
- Веб-интерфейс рассчитан на **закрытый (внутренний) контур**
- При публикации наружу:
  - не отключайте auth без крайней необходимости; храните сильные `API_AUTH_TOKEN` / `SESSION_SECRET`
  - ограничьте доступ к порту 80 (VPN, reverse proxy, IP whitelist)
  - не открывайте ClickHouse и backend API напрямую

---

## Устранение неполадок

### Стек не стартует / health check timeout

```bash
cd /opt/network-monitor
docker compose ps
docker compose logs --tail=80 clickhouse
docker compose logs --tail=80 backend
```

Типичная причина отказа backend: плейсхолдеры секретов или `AUTH_DISABLED`/`API_AUTH_DISABLED` без `NM_ALLOW_INSECURE=1`. Проверьте `.env` и логи `reject insecure`.

ClickHouse может долго инициализироваться при первом запуске. Увеличьте таймаут:

```bash
HEALTH_TIMEOUT=300 ./start.sh
```

### Syslog приходит, но событий нет

```bash
# Проверить syslog-ng
docker compose logs -f syslog-ng

# Проверить ingest backend
TOKEN="$(grep -E '^API_AUTH_TOKEN=' .env | cut -d= -f2-)"
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1/api/ingest/stats | jq .

# Проверить, открыт ли порт 514 на хосте
ss -ulnp | grep 514
ss -tlnp | grep 514
```

### Высокая нагрузка / отставание ingest

1. Проверить профиль: `cat install-profile.json`
2. Пересчитать профиль: `NM_FORCE_PROFILE=large ./scripts/tune-resources.sh`
3. Мониторить: `./scripts/watch-ingest.sh`
4. Проверить RAM/CPU хоста: `free -h`, `docker stats`

### Очистить всё и начать заново

```bash
cd /opt/network-monitor
REMOVE_DOCKER_VOLUMES=1 ./stop.sh
bash clickhouse/reset_data.sh   # если тома сохранены, но нужно очистить данные
./start.sh
```

### Полная переустановка

```bash
sudo bash deploy/uninstall.sh --purge --yes
sudo bash deploy/ubuntu/install_ubuntu.sh
```

---

## Поддерживаемые парсеры

| Парсер        | Вендор / формат              | Статус              |
|---------------|------------------------------|---------------------|
| UserGateCEF   | UserGate NGFW (CEF)          | Стабильный          |
| FortigateCEF  | FortiGate (CEF)              | Стабильный          |
| CiscoASA      | Cisco ASA (сырой syslog)     | Частичная поддержка |
| CiscoFTD      | Cisco FTD / FirePower        | Частичная поддержка |
| CowrieJSON    | Honeypot Cowrie (jsonlog / remotesyslog) | Частичная поддержка; в граф — `session.connect` и `login.*` с src/dst; `direct-tcpip.*` и прочие — **skip** |
| GenericKV     | Универсальный key=value      | Фолбэк              |

Тестирование парсеров: `/parser-test.html` или `POST /api/parse-test`. Ответ включает поля `parsed`, `skipped`, `errors` и детали по каждой строке.
