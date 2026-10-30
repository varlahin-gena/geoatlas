# Документация ГеоАтлас

Краткий обзор продукта — в [корневом README](../README.md).

| Документ | Содержание |
|----------|------------|
| [architecture.md](architecture.md) | Сервисы, поток данных, product limits, Ingest SLO, TTL |
| [install.md](install.md) | Требования, пакет, Ubuntu / Oracle Linux, удаление |
| [syslog.md](syslog.md) | Подключение МСЭ / SIEM на :514 |
| [configuration.md](configuration.md) | Переменные `.env`, секреты, модули |
| [operations.md](operations.md) | Запуск, профили, retention, бэкапы, логи, обновление |
| [geoip.md](geoip.md) | CSV, загрузка с сервера, 502/OOM |
| [ui.md](ui.md) | Страницы SPA (карта, hunts, investigate, command palette), роли, re-auth, OpenAPI |
| [reputation.md](reputation.md) | Фиды и offline-списки |
| [anomalies.md](anomalies.md) | Типы аномалий, пороги UI, learning, ack/assign, разбор алерта |
| [development.md](development.md) | Локальная разработка из git |
| [roadmap.md](roadmap.md) | Намерения на 12 месяцев |
| [security-requirements.md](security-requirements.md) | Чего ждать / не ждать от продукта |
| [assurance-case.md](assurance-case.md) | Threat model и аргумент защищённости |
| [openssf-best-practices.md](openssf-best-practices.md) | OpenSSF Passing |
| [openssf-silver.md](openssf-silver.md) | OpenSSF Silver |
| [repo-layout.md](repo-layout.md) | Дерево каталогов репозитория |

Смежные файлы в корне: [SECURITY.md](../SECURITY.md), [RELEASING.md](../RELEASING.md), [CHANGELOG.md](../CHANGELOG.md), [CONTRIBUTING.md](../CONTRIBUTING.md), [CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md), [GOVERNANCE.md](../GOVERNANCE.md), [`openapi.yaml`](../openapi.yaml).
