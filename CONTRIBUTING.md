# Участие в разработке ГеоАтласа

Спасибо за интерес к проекту. Issues и pull request на **русском или английском** приветствуются.

Уязвимости **не** публикуйте в Issues — см. [SECURITY.md](SECURITY.md).  
Поведение в сообществе — [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).  
Роли и решения — [GOVERNANCE.md](GOVERNANCE.md).

## Как предложить изменение

1. Откройте [Issue](https://github.com/varlahin-gena/geoatlas/issues), если хотите обсудить идею до кода (не обязательно для мелких правок docs/typo).
2. Fork (или branch) от актуального `main`.
3. Внесите изменения в отдельной ветке с понятным именем.
4. Каждый коммит — с **DCO** (см. ниже).
5. Откройте **Pull Request** в `main`.
6. Дождитесь зелёного CI: **DCO**, обязательный check **CI tests (required)** и review code owner’а.

Правила ветки `main` / `develop`: merge только через PR; прямой push запрещён ruleset’ом.

## Developer Certificate of Origin (DCO)

Проект использует [DCO 1.1](https://developercertificate.org/). Коммиты в PR **обязаны** содержать строку:

```text
Signed-off-by: Имя Фамилия <email@example.com>
```

Email должен совпадать с автором коммита (`git config user.name` / `user.email`).

```bash
git commit -s -m "Кратко: зачем изменение"
```

Проверка на каждом PR: workflow **DCO** (`.github/workflows/dco.yml`). Без валидного `Signed-off-by` merge блокируется.

**Dependabot:** коммиты бота подписаны как `Signed-off-by: dependabot[bot] <support@github.com>`, при том что author email — `users.noreply.github.com`; CI принимает эту пару как валидный DCO.

Исправляя историю в своей ветке:

```bash
git rebase HEAD~N --signoff   # осторожно: только своя feature-ветка
git push --force-with-lease
```

## Стандарты кода

| Стек | Стиль | Автопроверка в CI |
|------|--------|-------------------|
| Go (`backend/`, `pkg/`, …) | `gofmt` / `go vet`; идиомы Effective Go | `golangci-lint` |
| TypeScript / React (`frontend/`) | проектный ESLint (`npm run lint`), TypeScript strict | ESLint + `typecheck` + Vitest |
| Shell / YAML workflows | безопасные практики скриптов | shellcheck / actionlint / zizmor где подключено |
| OpenAPI / схема CH | [RELEASING.md](RELEASING.md) | `check-release-contract.sh` |

Новый код должен проходить те же проверки, что и CI. Форматтеры и линтеры не отключать «чтобы прошло».

## Локальная разработка и тесты

Сборка стека, `go test`, `npm test` — [docs/development.md](docs/development.md).  
Оси версий (продукт / OpenAPI / схема CH) — [RELEASING.md](RELEASING.md).

## Политика тестов

Для **существенной новой функциональности** (новый API/парсер/детектор, заметное поведение UI или install) добавляйте автотесты:

- Go: `cd backend && go test ./...` (при необходимости `pkg/`, `stats-collector`)
- Frontend: `cd frontend && npm test`

**Регрессии:** для ≥50% багфиксов за последние месяцы ожидаем тест, который ловил бы дефект (unit или integration) — если баг в тестируемом слое.

Мелкие правки документации и чисто косметические правки могут обходиться без новых тестов. CI уже гоняет unit/integration, lint, CodeQL, govulncheck, Trivy и контракт релиза — зелёный PR обязателен.

## Требования к PR

- Описание: что меняется и зачем (для security — без публичного PoC, если уязвимость ещё не закрыта).
- Не коммитьте секреты (`.env`, ключи, пароли); для примеров — только placeholders.
- Если меняете `openapi.yaml` / `VERSION` / схему ClickHouse — следуйте [RELEASING.md](RELEASING.md) и обновляйте цитаты OpenAPI в README / `docs/ui.md`.
- Для UI/операторских изменений желателен краткий smoke из чеклиста в RELEASING (live/ready, логин), когда это уместно.

## Обсуждения

Баги, вопросы и запросы фич — [Issues](https://github.com/varlahin-gena/geoatlas/issues). Обсуждение кода — в комментариях к PR.
