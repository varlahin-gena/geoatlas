# Управление проектом ГеоАтлас

## Модель решений

Maintainer-led FLOSS:

1. Изменения в `main` — только через Pull Request после зелёного CI (в т.ч. DCO) и review code owner’а.
2. Scope продукта, релизы `vX.Y.Z` и breaking changes решает **Maintainer**.
3. Security — приватный advisory ([SECURITY.md](SECURITY.md)), затем публичный релиз.
4. Обсуждения — GitHub Issues / PR; формальных голосований нет.

## Роли

| Роль | Кто | Обязанности |
|------|-----|-------------|
| **Maintainer** | владелец репозитория (`@varlahin-gena`) | merge, теги/релизы, security response, CODEOWNERS, branch protection, CoC |
| **Contributor** | авторы Issues / PR | код, тесты, docs по [CONTRIBUTING.md](CONTRIBUTING.md); DCO (`Signed-off-by`) |
| **Security reporter** | внешний исследователь | приватный отчёт; credit по желанию |

Code owners: [`.github/CODEOWNERS`](.github/CODEOWNERS).

## Непрерывность доступа (access continuity)

Если Maintainer подтверждённо не может продолжать поддержку, в течение **одной недели** проект должен уметь:

- создавать и закрывать Issues;
- принимать или отклонять PR;
- выпускать релиз `vX.Y.Z` (тег + GitHub Release + пакет).

Как это обеспечивается:

1. **GitHub admin** — у Maintainer; инструкция передачи прав и запасной контакт — в **приватном lockbox** Maintainer’а (не в git). Преемник с admin может восстановить CI secrets и branch rules.
2. **Подпись релизов и тегов** — cosign keyless (Sigstore / GitHub OIDC) в `.github/workflows/release-tag.yml` для tarball и attestation `vX.Y.Z` (`.gittag`); локальный GPG-ключ Maintainer’а не нужен; достаточно прав тегировать и зелёного workflow.
3. **Дистрибуция** — GitHub Releases (`geoatlas-X.Y.Z.tar.gz`); отдельный продуктовый DNS для продолжения поставок не требуется.
4. **Преемник** — Maintainer может добавить второго admin и обновить этот файл. До этого bus factor = 1 (см. ниже).

## Bus factor

Целевой bus factor ≥ 2. Пока один Maintainer делает релизы и обязательный review, **фактический bus factor = 1**.

- OpenSSF Silver `bus_factor` (SHOULD) — **Unmet** (честно); это **единственный** блокер выдачи Silver-бейджа при закрытых остальных SHOULD.
- MUST `access_continuity` — **Met** за счёт плана выше.

## Изменение этого документа

Через PR с обоснованием; merge — Maintainer.
