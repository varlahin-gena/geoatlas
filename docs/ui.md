# Веб-интерфейс и HTTP API

## Страницы UI

| URL                    | Страница              | Кто                 | Основные возможности                                                                                                                                 |
|------------------------|-----------------------|---------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| `/login`               | Вход                  | public              | Логин; смена пароля при `must_reset_password`                                                                                                        |
| `/`                    | Карта / глобус        | login               | 2D/3D, группировка, фильтры status/репутации, конструктор поиска и шаблоны, порог событий, mono-дуги, экспорт PNG, загрузка логов/GeoIP (admin)      |
| `/anomalies`           | Аномалии              | login               | Список срабатываний, ack / assign; кнопка «Разбор»; баннер на карте                                                                                  |
| `/investigate`         | Разбор алерта         | login               | Workspace по `?alert=<fingerprint>`: шапка, peers, ack/assign, CSV, шаблон поиска, ссылка на карту                                                   |
| `/reputation`          | Репутация IP          | administrator       | Списки и URL-фиды, каталог, refresh; модуль можно отключить                                                                                          |
| `/parse-errors`        | Журнал ошибок парсинга| administrator       | Поиск, удаление, отправка в тест парсеров                                                                                                            |
| `/geo-missing`         | IP без GeoIP          | administrator       | Адреса без координат; добавление в GeoIP; выгрузка CSV                                                                                               |
| `/geo-ranges`          | База GeoIP            | administrator       | Просмотр/правка диапазонов, выгрузка CSV                                                                                                             |
| `/parser-test`         | Тест парсеров         | administrator       | До 200 строк, пресеты вендоров, parsed/skipped/error                                                                                                 |
| `/system`              | Системный мониторинг  | administrator       | Обзор / Pipeline / Безопасность / Графики / Резервное копирование                                                                                    |
| `/tls`                 | TLS / сертификаты     | administrator       | Сведения о HTTPS/сертификатах UI                                                                                                                     |
| `/dozzle/`             | Логи контейнеров      | administrator       | Dozzle (профиль `dozzle`); вне React SPA                                                                                                             |
| `/users`               | Учётные записи        | administrator       | УЗ: administrator, operator, dashboard                                                                                                               |
| `/api-tokens`          | API-токены            | administrator       | Именованные Bearer: read / ops / admin; секрет один раз                                                                                              |
| `/change-password`     | Смена пароля          | login               | Смена своего пароля                                                                                                                                  |

SPA (React Router); nginx `auth_request` для `/api/*` и `/dozzle/`.

## Роли и доступ

| Роль | Cookie-сессия | UI | API (кратко) |
|------|---------------|-----|--------------|
| **administrator** | обычный TTL (`SESSION_TTL_HOURS`, по умолчанию 12 ч) | все страницы + `/dozzle/` | полные права (admin middleware) |
| **operator** | обычный TTL | карта, аномалии, разбор, смена пароля | login-tier: events, anomalies ack/assign, search templates; **без** uploads / system / geo / reputation / users / tokens |
| **dashboard** | **длительная** (~видеостена) | как operator | как operator по login-tier |

Загрузки логов/GeoIP, ingest-stats, system и прочие **ops**-маршруты: сессия **administrator** или Bearer ≥ **ops**. Одна cookie operator **не** открывает ops API.

### Cookie vs Bearer

- **Cookie** `ga_session` после `POST /api/auth/login`; CSRF — `ga_csrf`.
- **Bearer** `Authorization: Bearer …`:
  - env `API_AUTH_TOKEN` → всегда scope **admin**;
  - env `API_OPS_TOKEN` → **ops** (sidecars);
  - именованные токены UI `/api-tokens`: **read ⊂ ops ⊂ admin**.

Матрица маршрутов в коде: `backend/internal/adapter/httpapi/auth_matrix_test.go`. Env: [configuration.md](configuration.md#авторизация).

## Репутация и аномалии

- [Репутация IP](reputation.md) — фиды, SSRF-ограничения, как выключить.
- [Аномалии](anomalies.md) — типы, интервал скана, ack/assign, workspace `/investigate`.

### Разбор алерта (`/investigate`)

Страница открывается с параметром `?alert=<fingerprint>` (кнопка «Разбор» на `/anomalies`).

1. Загружается алерт из `GET /api/anomalies` (окно 7 суток, включая ack).
2. Панель peers — те же агрегированные связи, что в «Связи» на списке аномалий (`GET /api/events`).
3. Действия: закрыть (ack), назначить исполнителя, скопировать ссылки на разбор и карту, сохранить query как шаблон поиска, скачать CSV peers.

Отдельного backend API для «кейсов» нет: fingerprint в URL — идентификатор расследования.

## HTTP API

Контракт REST API (в т.ч. auth, events, geo, reputation, retention, tokens, search-templates, backups, аномалии, `/metrics`): [`openapi.yaml`](../openapi.yaml), версия документа OpenAPI **1.15.0**. Пробы: `GET /api/live` (процесс), `GET /api/ready` (ClickHouse + ingest); `GET /api/health` — alias live. Остальные эндпоинты — cookie-сессия и/или Bearer (`API_AUTH_TOKEN` / именованный токен со scope). Prometheus scrape: `GET /metrics` (Bearer≥ops / administrator).

При смене `info.version` в `openapi.yaml` обновите цитату `OpenAPI **N**` здесь и в [README](../README.md) (CI: `scripts/check-release-contract.sh`).
