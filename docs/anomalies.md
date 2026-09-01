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

## Детекторы (v2.3+)

Окна сканирования и пороги зависят от [профиля производительности](operations.md#профили-производительности) (`install-profile.json`). Сети предприятия обязательны для всех детекторов.

| Код | Окно | Логика (кратко) |
|-----|------|-----------------|
| `byte_surge` | 1 ч | Сравнение объёма байт от `src` в текущем часе с предыдущим; ratio ≥ порога и абсолютный минимум |
| `beaconing` | 24 ч | Периодическая low-volume связь `src→dst`: активность в ≥ N часах, низкий средний объём, регулярность интервалов |
| `lateral_fanout` | 15 мин | Один `src` обращается к множеству внутренних хостов enterprise-net (веер east-west) |

Классические детекторы (`port_scan`, `horizontal_scan`, `blocked_surge`, `new_country_dst`, `rep_new_peer`) работают как раньше; `blocked_surge` считает события, `byte_surge` — байты.

### Пороги новых детекторов по профилю

| Профиль | byte_surge ratio / abs min | beacon min ч / max avg bytes | lateral hosts / events |
|---------|----------------------------|------------------------------|------------------------|
| `tiny` | 5× / 50 MB | 8 / 200 KB | 15 / 30 |
| `small` | 5× / 80 MB | 10 / 250 KB | 20 / 40 |
| `medium` | 5× / 100 MB | 10 / 300 KB | 40 / 80 |
| `large` | 4× / 200 MB | 12 / 400 KB | 40 / 80 |
| `xlarge` | 4× / 400 MB | 14 / 500 KB | 50 / 100 |

`ByteSurgeFloor` и `BeaconMinRegularity` (0.55–0.6) задают нижнюю планку шума и минимальную «ровность» часовых интервалов; в env не выносятся — только через профиль.

## Разбор алерта (Investigation workspace)

Маршрут **`/investigate?alert=<fingerprint>`** — тонкий MVP без отдельного case store на backend.

| Возможность | Как |
|-------------|-----|
| Открыть | Кнопка «Разбор» на `/anomalies` или прямая ссылка с fingerprint |
| Контекст | Тип, severity, src/dst, окно, счётчик событий, статус ack |
| Связи (peers) | `GET /api/events` по query алерта (агрегированные рёбра, как на карте) |
| Ack / assign | Те же `POST /api/anomalies/{fingerprint}/ack` и `…/assign` |
| CSV peers | Клиентская выгрузка из загруженных peers |
| Шаблон поиска | Сохранение query алерта в личные search templates |
| Карта | Deep-link «На карте» (MapLibre не встраивается в workspace) |

Поиск алерта — за последние **7 суток** (`include_acked=1`, limit 200). Если fingerprint не найден, показывается предупреждение.

Роли: **administrator**, **operator**, **dashboard** (как `/anomalies`). Подробнее о страницах: [ui.md](ui.md).

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
