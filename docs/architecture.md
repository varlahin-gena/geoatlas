# Архитектура

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

1. **Syslog**: МСЭ отправляет события на `<IP_сервера>:514` (TCP или UDP).
2. **syslog-ng** принимает сообщения и пересылает их по TCP на `backend:1514` с маркерами транспорта (`@@ga/udp/@@` / `@@ga/tcp/@@`).
3. **backend** снимает маркеры транспорта, парсит строки, обогащает GeoIP, пишет в ClickHouse; при старте `EnsureBaseSchema` / `Ensure*` создаёт базовые таблицы и агрегаты (`traffic_edges_*`, geo), применяет TTL из `retention.json` и при необходимости делает backfill. **Delivery contract — at-most-once / best-effort:** полная очередь ingest **не блокирует** TCP — лишние строки дропаются (`dropped_total`); при outage ClickHouse insert circuit ставит dequeue на паузу (очередь растёт → admission drops), а потери из processor-буфера учитываются отдельно (`buffer_drops_total`). Перед backend **syslog-ng уже буферизует** (см. ниже). Алерты и live-метрики drops — на `/system`.
4. **frontend** отдаёт статику и проксирует API-запросы на backend.
5. **stats-collector** каждые 30 секунд собирает метрики CPU/RAM контейнеров и состояние пайплайна (включая разбивку UDP/TCP).

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
