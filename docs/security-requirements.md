# Требования безопасности (чего ждать от продукта)

Документ для операторов и аудиторов: что ГеоАтлас **гарантирует** и что **осознанно не гарантирует**.
Процесс сообщений об уязвимостях — [SECURITY.md](../SECURITY.md). Обоснование «почему требования выполняются» — [assurance-case.md](assurance-case.md).

## Границы доверия

| Зона | Доверие |
|------|---------|
| Хост Docker / compose / `.env` | полностью доверенная сторона оператора |
| Сеть до `:514` (syslog) | **не** аутентифицирована; доверять только после сетевого ограничения |
| Браузер → nginx (`:80`/`:443`) | пользователи с ролями; сессия / API-токены |
| nginx → backend | внутренняя docker-сеть + proxy gate (`GA_REQUIRE_PROXY`) |
| backend → ClickHouse | внутренняя сеть; пароль из секретов |
| URL-фиды репутации | только публичные IPv4; private/metadata блокируются |

## Что продукт обеспечивает

1. **Аутентификация и роли** — по умолчанию включены (`administrator` / `operator` / `dashboard`); пароли УЗ — bcrypt; чувствительные действия — re-auth где предусмотрено.
2. **Секреты вне образа** — `API_AUTH_TOKEN`, `SESSION_SECRET`, `INGEST_SHARED_SECRET`, `CLICKHOUSE_PASSWORD` задаются при старте / в `.env`, не зашиты в git.
3. **Edge hardening** — proxy gate, rate limit API, лимиты JSON, security headers на nginx; опциональный HTTPS + HSTS.
4. **Ingest allowlist** — TCP ingest от ожидаемых peer’ов (`INGEST_ALLOW_FROM`); хостовый FW / `GA_SYSLOG_ALLOW_FROM` для `:514`.
5. **Валидация входов** — размеры upload, структура JSON, безопасный fetch репутации (`safeurl`), парсеры без выполнения кода из лога.
6. **Целостность поставок** — релизы с `.sha256`; опционально cosign/Sigstore (см. [install.md](install.md)).
7. **Ответ на уязвимости** — приватный advisory, цель подтверждения ≤5 рабочих дней ([SECURITY.md](../SECURITY.md)).

## Чего продукт не обещает

- Конфиденциальность и целостность syslog на `:514` без TLS/auth (нужен периметр оператора).
- HA / мульти-инстанс backend, at-least-once доставка (ingest — at-most-once, возможны drops).
- Защиту при отключённых guard’ах (`AUTH_DISABLED`, `API_AUTH_DISABLED`, `GA_ALLOW_INSECURE`, открытый backend мимо nginx).
- Закрытие CVE upstream (ClickHouse, nginx, Docker, ядро) сверх закреплённых тегов образов и политики обновления.
- Формальную гарантию отсутствия дефектов; badge OpenSSF — про практики, не про «zero vuln».

## Ожидания к оператору

- Секреты и пароли по checklist в [README](../README.md#безопасность).
- UI из сети — за HTTPS; не публиковать `8123`/`9000`/`8080`/`1514`.
- Ограничить источники syslog; проверять `.sha256` (и при возможности cosign) перед `./update.sh`.
