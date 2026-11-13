# Архитектура

Система состоит из **шести сервисов**, оркеструемых через Docker Compose.
Ядро всегда поднимается: **clickhouse + backend + frontend**.
`syslog-ng`, `stats-collector` и `dozzle` включаются через compose-профили (`COMPOSE_PROFILES=syslog,stats,dozzle` — по умолчанию при установке / в `.env.example`).

Все long-running сервисы в одной docker-сети **`geoatlas`** (bridge). Снаружи публикуются только:

| Снаружи (хост) | Куда | Зачем |
|----------------|------|--------|
| `:80` / `:443` | `frontend` | UI + прокси API |
| `:514` + `5514–5529` (tcp/udp) | `syslog-ng` | приём логов МСЭ / SIEM |

`backend:8080`, `backend:1514`, `clickhouse:8123/9000`, `dozzle:8080`, `syslog-ng:9577/9578` — **только внутри** сети (`expose`), с хоста напрямую не торчат.

```
                    ┌──────────────────────────────────────────────────────────┐
                    │               docker network: geoatlas                   │
                    │                                                          │
 МСЭ / SIEM         │  ┌───────────┐  TCP :1514 + маркер @@ga/…   ┌─────────┐ │
 :514 / connectors ─┼─►│ syslog-ng │ ────────────────────────────►│ backend │ │
                    │  └─────┬─────┘  (disk-buffer, best-effort)  │  (Go)   │ │
                    │        │                                    └────┬────┘ │
                    │        │ :9577 /stats   :9578 /reload            │      │
                    │        │◄── scrape ── backend / stats-collector ─┤      │
                    │        │◄── reload ── backend (коннекторы) ──────┘      │
                    │                                              native     │
                    │                                              :9000      │
                    │  ┌────────────────┐                            ▼        │
                    │  │stats-collector │ ── INSERT system_metrics ► clickhouse│
                    │  └───────┬────────┘  + HTTP backend /live               │
                    │          │             + /api/ingest/stats              │
                    │          └─────────── + HTTP syslog-ng :9577            │
                    │                                                          │
 браузер :80/:443 ──┼─► frontend (nginx) ── /api/* ──► backend:8080           │
                    │        │ auth_request                                    │
                    │        └── /dozzle/ ──► dozzle:8080   [профиль dozzle]  │
                    └──────────────────────────────────────────────────────────┘
```

**Важно:** UI **не** ходит в ClickHouse. Чтение: браузер → nginx → backend HTTP API → ClickHouse. Anomaly engine, GeoIP, бэкапы и репутация — фоновые jobs внутри `backend`, не отдельные контейнеры.

### Кто с кем говорит

| От | К | Протокол / порт | Назначение |
|----|---|-----------------|------------|
| МСЭ / SIEM | `syslog-ng` | UDP/TCP `:514` (+ connector ports) | сырой syslog |
| `syslog-ng` | `backend` | TCP `:1514` | форвард с маркером `@@ga/{udp\|tcp}/<token>/<connector_id>/@@` + disk-buffer |
| `backend` | `clickhouse` | native `:9000` | INSERT событий / SELECT для API и anomaly |
| `stats-collector` | `clickhouse` | native `:9000` | INSERT `system_metrics` |
| `stats-collector` | `backend` | HTTP `:8080` | `/live`, `/api/ingest/stats` (Bearer ops/admin) |
| `stats-collector` | `syslog-ng` | HTTP `:9577` | scrape drops/queue |
| `backend` | `syslog-ng` | HTTP `:9577` / `:9578` | scrape stats; reload после Apply коннекторов |
| браузер | `frontend` | HTTP(S) `:80`/`:443` | SPA + API edge |
| `frontend` | `backend` | HTTP `:8080` | `/api/*`, `auth_request` → `/api/auth/check*` |
| `frontend` | `dozzle` | HTTP `:8080` | `/dozzle/` (только administrator) |
| `dozzle` | Docker Engine | `docker.sock` | realtime-логи / actions контейнеров стека |
| `backend` ↔ `syslog-ng` | том `./syslog-ng.d` | файлы | backend пишет `zz_connectors.conf`, syslog-ng читает |

Одноразовый init-контейнер `volume-perms` (не в таблице сервисов ниже) выравнивает uid **101** на shared-томах и сразу завершается.

## Сервисы

| Сервис            | Контейнер          | Профиль     | Назначение                                              |
|-------------------|--------------------|-------------|---------------------------------------------------------|
| `frontend`        | `frontend`         | *(ядро)*    | Веб-интерфейс, nginx, auth_request, прокси `/api/*`     |
| `backend`         | `backend`          | *(ядро)*    | Парсинг, GeoIP, ingest, агрегация, HTTP API, auth       |
| `clickhouse`      | `clickhouse`       | *(ядро)*    | Хранилище и аналитика (только внутренняя docker-сеть)   |
| `syslog-ng`       | `syslog-ng`        | `syslog`    | Приём syslog от МСЭ, буферизация, передача в backend    |
| `stats-collector` | `stats-collector`  | `stats`     | Сбор системных метрик контейнеров в ClickHouse          |
| `dozzle`          | `dozzle`           | `dozzle`    | Realtime-логи контейнеров UI `/dozzle/` (admin; opt-in) |

## Поток данных

1. **Syslog**: МСЭ отправляет события на `<IP_сервера>:514` (TCP или UDP), либо на порт коннектора (`5514–5529` по умолчанию).
2. **syslog-ng** принимает сообщения, буферизует (disk-buffer) и пересылает по TCP на `backend:1514` с маркером `@@ga/{udp|tcp}/<token>/<connector_id>/@@…` (`INGEST_SHARED_SECRET`).
3. **backend** снимает маркер (проверка токена + `connector_id`), парсит строки, обогащает GeoIP, пишет в ClickHouse; при старте `EnsureBaseSchema` / `Ensure*` создаёт базовые таблицы и агрегаты (`traffic_edges_*`, geo), применяет TTL из `retention.json` и при необходимости делает backfill. **Delivery contract — at-most-once / best-effort:** полная очередь ingest **не блокирует** TCP — лишние строки дропаются (`dropped_total`); при outage ClickHouse insert circuit ставит dequeue на паузу (очередь растёт → admission drops), а потери из processor-буфера учитываются отдельно (`buffer_drops_total`). Перед backend **syslog-ng уже буферизует**. Алерты и live-метрики drops — на `/system`.
4. **frontend** отдаёт статику SPA и проксирует `/api/*` на `backend:8080` (с `auth_request`); `/dozzle/` — на контейнер `dozzle`.
5. **stats-collector** каждые 30 с: cgroup CPU/RAM, HTTP scrape backend (`/live`, `/api/ingest/stats`) и syslog-ng (`:9577`), пишет `system_metrics` в ClickHouse.
6. **Anomaly engine** (фоновый job в backend): каждые `ANOMALY_SCAN_INTERVAL` сканирует `traffic_logs` в ClickHouse, пишет срабатывания в JSON-store (`/app/data`), отдаёт через `GET /api/anomalies`. Требуются сети предприятия. Типы, пороги UI и learning — [anomalies.md](anomalies.md).
7. **Edge**: браузер ходит только в nginx (`:80`/`:443`); backend `:8080` за proxy gate (`GA_REQUIRE_PROXY`) не принимает прямые запросы с хоста (кроме loopback / Bearer). Rate limit и JSON-hardening — [configuration.md](configuration.md#api-threat-protection-edge).

## Product limits (appliance)

| Ограничение                   | Суть                                                                                                                                           |
|-------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------|
| **IPv4-only**                 | Success path GeoIP / карта / lookup — IPv4. IPv6 не обогащается и не строится на карте.                                                        |
| **Single-host control plane** | Учётки, API-токены, retention, reputation feeds, schedules — JSON на `/app/data`. Backend берёт exclusive lock (`/app/data/.ga_backend.lock`). |
| **Один процесс backend**      | Ingest TCP, HTTP API, GeoIP RAM и background jobs (geo/backup/anomaly/reputation) — один контейнер. OOM/рестарт = пауза и приёма, и UI.        |

## Ingest SLO

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

См. также [подключение syslog](syslog.md), [профили производительности](operations.md#профили-производительности) и [мониторинг ingest](operations.md#мониторинг-ingest).

## Хранение данных (TTL)

Дефолты ниже, менять можно в UI (`/system` → Pipeline → «Срок хранения»). Подробнее — [срок хранения](operations.md#срок-хранения-ttl--retention).

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
