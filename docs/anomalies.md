# Аномалии

Фоновый движок ищет подозрительные паттерны в трафике и показывает их на карте и на странице **`/anomalies`**.

Доступ к списку / ack / assign — любой залогиненный (**administrator**, **operator**, **dashboard**). Статус движка (`GET /api/anomalies/status`) — Bearer ≥ **ops** или administrator.

Сканер работает только при наличии **сетей предприятия** (иначе `no_enterprise_nets`).

## Типы

| Код | Смысл | Severity |
|-----|--------|----------|
| `port_scan` | Сканирование портов | high |
| `horizontal_scan` | Сканирование подсети | high |
| `blocked_surge` | Всплеск блокировок | warn / high |
| `byte_surge` | Всплеск объёма байт (src, 1ч vs 1ч) | warn / high |
| `beaconing` | Периодическая low-volume связь за 24ч | warn / high |
| `lateral_fanout` | Веер на внутренние хосты enterprise-net | high |
| `new_country_dst` | Новая страна назначения | warn |
| `rep_new_peer` | Новая связь с репутационным адресом | high |

На карте — полоска/баннер со ссылкой на `/anomalies`. На странице аномалий:

- **«Связи»** — рёбра из `GET /api/events` по `src`/map.q (агрегированные peers, не сырой PCAP);
- **«Разбор»** — workspace [`/investigate?alert=<fingerprint>`](ui.md): шапка алерта, peers, ack/assign, клиентский CSV peers, сохранение query как search template, deep-link на карту (без встраивания MapLibre).

Fingerprint + URL = «кейс»; отдельного case store нет.

## Включение и интервал

| Переменная | По умолчанию | Назначение |
|------------|--------------|------------|
| `ANOMALY_ENABLED` | `true` | Выключатель движка |
| `ANOMALY_SCAN_INTERVAL` | `5m` | Период фонового скана |
| `ANOMALY_INCLUDE_PRIVATE` | `false` | Учитывать private IP (port/horizontal) |
| `ANOMALY_LEARNING_DAYS` | `3` | Окно обучения |
| `ANOMALY_SUPPRESS_HOURS` | `24` | Не повторять то же срабатывание сразу |

Полный список env: [configuration.md](configuration.md#репутация-и-аномалии).

## Ack и назначение

- **Ack** — `POST /api/anomalies/{fingerprint}/ack` (сессия login): событие отмечено обработанным; могут заполниться `ack_by` / `assigned_to`.
- **Assign** — `POST /api/anomalies/{fingerprint}/assign`: исполнитель из `GET /api/users/directory`.
- В UI на `/anomalies` — список, переключатель «включая ack», назначение исполнителя.

Контракт: [`openapi.yaml`](../openapi.yaml). Роли: [ui.md — доступ](ui.md#роли-и-доступ).
