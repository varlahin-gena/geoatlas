# Участие в разработке ГеоАтласа

Спасибо за интерес к проекту. Issues и pull request на **русском или английском** приветствуются.

Уязвимости **не** публикуйте в Issues — см. [SECURITY.md](SECURITY.md).

## Как предложить изменение

1. Откройте [Issue](https://github.com/varlahin-gena/geoatlas/issues), если хотите обсудить идею до кода (не обязательно для мелких правок docs/typo).
2. Сделайте fork (или branch в своём клоне) от актуального `main`.
3. Внесите изменения в отдельной ветке с понятным именем.
4. Откройте **Pull Request** в `main`.
5. Дождитесь зелёного CI (обязательный check **CI tests (required)**) и code owner review.

Правила ветки `main` / `develop`: merge только через PR; прямой push запрещён ruleset’ом.

## Локальная разработка и тесты

Сборка стека, `go test`, `npm test` и полезные скрипты — в [docs/development.md](docs/development.md).

Релизные оси версий (продукт / OpenAPI / схема CH) — в [RELEASING.md](RELEASING.md).

## Политика тестов

Для **существенной новой функциональности** (новый API/парсер/детектор, заметное поведение UI или install) добавляйте автотесты в существующие suites:

- Go: `cd backend && go test ./...` (и при необходимости модули `pkg/`, `stats-collector`)
- Frontend: `cd frontend && npm test`

Мелкие правки документации и чисто косметические правки могут обходиться без новых тестов. CI уже гоняет unit/integration, lint (`golangci-lint`), CodeQL, govulncheck, Trivy и контракт релиза — зелёный PR обязателен.

## Требования к PR

- Описание: что меняется и зачем (для security — без публичного PoC, если уязвимость ещё не закрыта).
- Не коммитьте секреты (`.env`, ключи, пароли); для примеров — только placeholders.
- Если меняете `openapi.yaml` / `VERSION` / схему ClickHouse — следуйте [RELEASING.md](RELEASING.md) и обновляйте цитаты OpenAPI в README / `docs/ui.md`.
- Для UI/операторских изменений желателен краткий smoke из чеклиста в RELEASING (live/ready, логин), когда это уместно.

## Обсуждения

Баги, вопросы и запросы фич — [Issues](https://github.com/varlahin-gena/geoatlas/issues). Обсуждение кода — в комментариях к PR.
