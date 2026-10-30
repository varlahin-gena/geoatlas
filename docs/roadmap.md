# Roadmap (12 месяцев)

Что ГеоАтлас **намерен** делать и **не намерен** в ближайший год.
Приоритеты могут сдвигаться из‑за security и обратной связи в Issues.

## Делаем

1. **Стабильность ingest и ClickHouse** — Ingest SLO, меньше drops на пиках, предсказуемый retention/TTL.
2. **Детекторы аномалий** — доводка типов (scan, beaconing, lateral, byte_surge, reputation peer), пороги по профилю нагрузки.
3. **Операторский UX** — карта, hunts, investigate, system monitoring; ломка OpenAPI только с bump версии.
4. **Supply-chain / OpenSSF** — удержание Passing; Silver (документы, DCO, coverage ≥80%, подписанные релизы); Dependabot, govulncheck, Scorecard.
5. **Парсеры МСЭ** — семплы UserGate / FortiGate / ASA / FTD / Cowrie; осознанный skip несетевых событий.
6. **Документация оператора** — install / update / HTTPS / syslog allowlist.

## Не делаем (осознанно)

- Мульти-хост control plane / HA backend (appliance = один хост).
- IPv6 GeoIP / карта как основной путь.
- Syslog TLS/auth на `:514` как обязательный дефолт (ограничение сети: firewall / `GA_SYSLOG_ALLOW_FROM`).
- Облачный SaaS control plane или marketplace плагинов.
- Windows как целевая OS appliance (сервер: Ubuntu / Oracle Linux / RHEL-совместимые).

## Как предложить тему

Issue → обсуждение → PR. Существенные фичи — с автотестами по [CONTRIBUTING.md](../CONTRIBUTING.md).
