# Frontend (ГеоАтлас SPA)

React + TypeScript + Vite. Многостраничный SPA, в проде отдаётся через nginx.

Страницы, роли и OpenAPI — [docs/ui.md](../docs/ui.md). Локальный стек из git — [docs/development.md](../docs/development.md).

## Разработка

```bash
cd frontend
npm install
npm run dev          # http://127.0.0.1:5173 (proxy /api → :8080)
npm test             # vitest
npm run build        # dist/
```

В Docker runtime-конфиг `/config.js` подставляет entrypoint nginx (`window.GA_CONFIG={}`).

## Маршруты

| Путь | Доступ |
|------|--------|
| `/login` | публичный |
| `/` | вход (administrator / operator / dashboard) |
| `/anomalies` | вход |
| `/investigate` | вход (`?alert=` fingerprint) |
| `/change-password` | вход (в т.ч. must-reset) |
| `/system`, `/users`, `/api-tokens`, `/parse-errors`, `/parsers`, `/parser-test`, `/geo-missing`, `/geo-ranges`, `/reputation`, `/tls` | administrator |

Старые URL `*.html` редиректят на маршруты выше (nginx + React Router).

## Docker

`frontend/Dockerfile` — multi-stage: `npm ci && npm run build` → `nginx:1.30.4-alpine` с `dist/` и `nginx-app.inc`.
