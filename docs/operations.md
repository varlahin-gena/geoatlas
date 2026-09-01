# Обслуживание

## Запуск и остановка

```bash
cd /opt/geoatlas

# Запуск (с пересборкой образов)
./start.sh

# Запуск без пересборки
DO_BUILD=0 ./start.sh

# Остановка (данные сохраняются)
./stop.sh

# Обновление из пакета (см. «Обновление системы»)
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

## Профили производительности

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

## Срок хранения (TTL / retention)

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

Дефолты таблиц — в [архитектуре](architecture.md#хранение-данных-ttl).

## Резервное копирование ClickHouse

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

## Очистка данных ClickHouse

Удаляет все события, GeoIP, ошибки парсинга и метрики. **Схема таблиц сохраняется.**

```bash
cd /opt/geoatlas
bash clickhouse/reset_data.sh
```

## Мониторинг ingest

Скрипт для наблюдения за скоростью приёма событий:

```bash
cd /opt/geoatlas
./scripts/watch-ingest.sh          # интервал 2 сек (по умолчанию)
./scripts/watch-ingest.sh 5        # интервал 5 сек
```

Требует `curl` и `jq`. Выводит: recv/s, ins/s, **drop/s**, dropped, buffered, connections, state, queue depth.

Пороги инцидентов — [Ingest SLO](architecture.md#ingest-slo).

## Логи и диагностика

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
| GeoIP upload → 502 / OOM, backend          | Не заливать большой CSV поверх уже загруженного индекса через браузер; `dmesg`/`oom-kill`; см. [GeoIP](geoip.md)                                                |
| перезапускается                            |                                                                                                                                                                 |
| GeoIP: `Failed to fetch` при смене         | Уход со страницы во время POST обрывает `fetch`; дождитесь окончания или `curl` с сервера                                                                       |
| страницы                                   |                                                                                                                                                                 |

## Обновление системы

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
VER=2.3.0   # нужный релиз
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
