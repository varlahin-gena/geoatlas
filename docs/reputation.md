# Репутация IP

Модуль подсвечивает и фильтрует «плохие» адреса на карте: offline-списки в ClickHouse и периодические URL-фиды.

UI: **`/reputation`** (только **administrator**). На карте — фильтр/подсветка по репутации.

## Offline-списки vs URL-фиды

| Источник | Как попадает | Куда |
|----------|--------------|------|
| Ручная загрузка | UI или `POST /upload-reputation` (Bearer ≥ ops) | таблица `reputation_ranges` |
| URL-фиды | фоновый fetch по расписанию; каталог публичных источников в UI | файл фидов + ranges после успешного скачивания |

Сиды по умолчанию — публичные netset’ы (FireHOL и др.); в каталоге UI можно добавить известные фиды (Spamhaus JSON, abuse.ch и т.п.).

## Расписание

- Интервал: `REPUTATION_FETCH_INTERVAL` (по умолчанию **6h**, не короче ~1 минуты).
- Кэш HTTP: ETag / Last-Modified, чтобы не качать без изменений.
- Клиент fetch: только **публичные IPv4**-хосты; private, link-local и metadata (например `169.254.169.254`) **блокируются** (SSRF-защита).

Lookup по частным адресам на карте репутацию не ставит (non-public IPv4 пропускаются).

## Как отключить

| Способ | Действие |
|--------|----------|
| Установка | снять модуль «Репутация IP» → `GA_MODULE_REPUTATION=0` → `REPUTATION_FETCH_ENABLED=false` |
| Runtime | в `.env`: `REPUTATION_FETCH_ENABLED=false`, перезапуск backend |

При выключении пункт `/reputation` скрывается из навигации; API модуля не обслуживает fetch/UI как включённый.

Связанные переменные: [configuration.md](configuration.md#репутация-и-аномалии).
