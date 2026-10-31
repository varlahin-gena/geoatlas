# Доступность (accessibility)

Operator UI ГеоАтлас ориентируется на [WCAG 2.0](https://www.w3.org/TR/WCAG20/) там, где это разумно для внутреннего веб-приложения (не публичный маркетинг-сайт).

## Что уже есть

- Семантика и клавиатура: `label` у полей форм, `role`/`aria-*` у навигации, табов, диалогов, live-регионов статуса.
- Skip-links на экранах входа / смены пароля / карты и в общем admin shell (`К содержимому` → `#main-content`).
- Текст альтернатив для логотипа (`alt`); декоративные иконки — `aria-hidden`.
- Контраст и фокус: тема в `frontend/src/styles/theme.css` / `common.css`; видимый `:focus` у skip-link и интерактивных контролов.
- Ошибки форм — `role="alert"`.

Карта (MapLibre / deck.gl) — визуальный инструмент; текстовые фильтры, чипы и панели рядом дают эквивалентный доступ к данным без опоры только на цвет.

## Как улучшаем

Баги и предложения по a11y принимаются через [GitHub Issues](https://github.com/varlahin-gena/geoatlas/issues) (метка или префикс `a11y:` приветствуются). Регрессии в навигации с клавиатуры / screen reader — приоритет наравне с функциональными багами UI.

OpenSSF Silver `accessibility_best_practices` (SHOULD): **Met** — см. [openssf-silver.md](openssf-silver.md).
