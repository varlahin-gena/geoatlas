# Frontend (ГеоАтлас SPA)

React + TypeScript + Vite. Multi-route SPA served by nginx.

## Dev

```bash
cd frontend
npm install
npm run dev          # http://127.0.0.1:5173 (proxy /api → :8080)
npm test             # vitest
npm run build        # dist/
```

Runtime `/config.js` is injected by the nginx entrypoint in Docker (`window.NM_CONFIG={}`).

## Routes

| Path | Guard |
|------|--------|
| `/login` | public |
| `/` | login |
| `/change-password` | login (allow must-reset) |
| `/system`, `/users`, `/api-tokens`, `/parse-errors`, `/parser-test`, `/geo-missing`, `/geo-ranges`, `/reputation` | admin |

Legacy `*.html` URLs redirect to the routes above (nginx + React Router).

## Docker

`frontend/Dockerfile` multi-stage: `npm install && npm run build` → `nginx:1.27-alpine` with `dist/` and `nginx-app.inc`.
