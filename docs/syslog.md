# Syslog: подключение МСЭ / SIEM

После [установки](install.md) направьте логи межсетевого экрана или SIEM на сервер ГеоАтлас.

## Куда слать

| Параметр | Значение |
|----------|----------|
| Хост | IP (или DNS) сервера с ГеоАтлас |
| Порт | **514** |
| Протокол | **UDP и/или TCP** (оба слушаются, если включён модуль syslog) |
| TLS / auth на `:514` | **нет** — ограничьте источник файрволом |

Путь данных: МСЭ → **syslog-ng** (`:514`) → backend TCP **`:1514`** (внутри docker) → ClickHouse. Схема: [architecture.md](architecture.md).

Модуль syslog должен быть включён (`GA_MODULE_SYSLOG=1` / профиль `syslog` в `COMPOSE_PROFILES`). Иначе порт 514 не публикуется.

## Ограничить источник

Рекомендуется задать сеть МСЭ:

```bash
# в .env на сервере (пример)
GA_SYSLOG_ALLOW_FROM=10.0.0.0/8
```

Установщик / firewall helpers откроют **514/tcp+udp только с этого CIDR/IP**. Без переменной — с любого адреса (опасно в открытой сети). Дополнительно режьте Security Group / ACL на периметре.

См. также checklist в [README](../README.md#безопасность) и [SECURITY.md](../SECURITY.md).

## Маркеры транспорта

syslog-ng добавляет к сырой строке маркер с секретом (`INGEST_SHARED_SECRET`):

- `@@ga/udp/<token>/@@…` — приём по UDP
- `@@ga/tcp/<token>/@@…` — приём по TCP

Backend снимает маркер, проверяет токен и считает UDP/TCP EPS раздельно. Конфиг секрета: `syslog-ng.d/zz_ingest_auth.conf` (создаёт `./start.sh`, переживает update).

Ручная загрузка логов через UI/API **не** требует маркера (другой путь).

Если в UI UDP и TCP EPS «склеены» — перезапустите контейнер `syslog-ng`.

## Проверка после первого потока

1. **UI** (administrator): `/system` → вкладка **Pipeline** — queued/dropped syslog-ng, ingest EPS, плитка Drops.
2. **API** (Bearer ≥ ops, обычно `API_AUTH_TOKEN` из `.env`):

```bash
cd /opt/geoatlas
TOKEN="$(grep -E '^API_AUTH_TOKEN=' .env | cut -d= -f2-)"
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1/api/ingest/stats | jq .
curl -fsS http://127.0.0.1/api/live
curl -fsS http://127.0.0.1/api/ready
```

3. Скрипт: `./scripts/watch-ingest.sh` — recv/s, ins/s, **drop/s**.
4. Карта `/` — нужны события **и** GeoIP-база (без координат узлы не рисуются; см. [geoip.md](geoip.md)).

## Типичные проблемы

| Симптом | Что проверить |
|---------|----------------|
| Нет событий на карте | `docker compose logs syslog-ng`, `/api/ingest/stats`, модуль syslog, FW `:514` |
| Есть ingest, пустая карта | GeoIP загружена? `/geo-missing` |
| Ошибки парсинга | UI «Ошибки парсинга», вендор/формат строки |
| Drops / очередь растёт | [Ingest SLO](architecture.md#ingest-slo), профиль (`tune-resources.sh`), здоровье ClickHouse |
| `kernel refused SO_RCVBUF` | `net.core.rmem_max` / `wmem_max` на хосте — [буферы профиля](operations.md#профили-производительности) |

Полная таблица диагностики: [operations.md — логи](operations.md#логи-и-диагностика).
