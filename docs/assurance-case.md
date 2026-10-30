# Assurance case (кратко)

Зачем: аргумент, что [требования безопасности](security-requirements.md) выполняются при заявленных границах доверия.
Не заменяет пентест и не утверждает отсутствие багов.

## 1. Модель угроз (сжато)

| Угроза | Граница | Контрмера |
|--------|---------|-----------|
| Несанкционированный доступ к UI/API | браузер ↔ nginx | auth по умолчанию, роли, сессии, API-токены, rate limit |
| Прямой доступ к backend / CH | хост / docker ports | `GA_REQUIRE_PROXY`, не публиковать порты, CH allowlist сетей |
| Подмена / перегруз syslog | `:514` | сетевой allowlist; at-most-once + алерты drops (не крипто-целостность) |
| Инъекции через upload / JSON / path | API | `threatprot`, лимиты тел, валидация GeoIP/reputation |
| SSRF через reputation feeds | backend → Internet | `safeurl` (только публичный IPv4) |
| Утечка секретов из образа/репо | CI / git | секреты в `.env`; gitleaks; не коммитить `.env` |
| Подмена установочного пакета | GitHub Releases | HTTPS + `.sha256` + cosign keyless |
| Эскалация через Dozzle | opt-in профиль | UI только administrator; осознанный риск Docker socket |

Известные принятые риски (не «дыры продукта» в смысле CVE): syslog без TLS, IPv4-only, single-host — см. [architecture.md](architecture.md) и [SECURITY.md](../SECURITY.md).

## 2. Trust boundaries

См. таблицу в [security-requirements.md](security-requirements.md#границы-доверия). Кратко: операторский хост и внутренняя docker-сеть — доверенные; внешний syslog и браузер — нет; Internet для фидов — ограничен allowlist’ом адресов.

## 3. Secure design (как применяем)

- **Fail-safe defaults** — auth включён; insecure-флаги только явно.
- **Least privilege** — роли UI; CI `permissions: contents: read` по умолчанию; job’ы с минимальными правами.
- **Complete mediation** — маршруты за auth_request / матрицей ролей (`auth_matrix` в тестах).
- **Defense in depth** — nginx headers + backend threatprot + сетевые ограничения.
- **Не доверять клиенту** — лимиты и валидация на сервере.

## 4. Типичные слабости реализации — как закрываем

| Класс | Практика в репо |
|-------|-----------------|
| Authn/authz flaws | роли, тесты матрицы, re-auth на чувствительных действиях |
| Injection | парсеры логов без exec; SQL через параметризацию/слои CH; JSON limits |
| XSS | SPA + заголовки; не вставляем сырой syslog в HTML без экранирования в UI-коде |
| SSRF | `internal/safeurl` |
| Secrets | bcrypt паролей; CSPRNG для токенов; cosign на релизах |
| Supply chain | Dependabot, govulncheck, CodeQL, Trivy/Grype, pinned Actions, Scorecard |

Динамический анализ: unit/integration/fuzz в CI ([development.md](development.md), workflows). Memory-unsafe языков в продукте нет (Go + TypeScript) — критерий dynamic_analysis_unsafe = N/A.

## 5. Вывод

При соблюдении операторского checklist и без отключения guard’ов заявленные требования из [security-requirements.md](security-requirements.md) выполняются **в рамках модели угроз выше**. Остаточный риск — дефекты реализации и принятые ограничения appliance; канал закрытия — [SECURITY.md](../SECURITY.md) и релизы.
