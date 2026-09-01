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

Runtime `/config.js` is injected by the nginx entrypoint in Docker (`window.GA_CONFIG={}`).

## Routes

| Path | Guard |
|------|--------|
| `/login` | public |
| `/` | login (administrator / operator / dashboard) |
| `/anomalies` | login |
| `/investigate` | login (`?alert=` fingerprint) |
| `/change-password` | login (allow must-reset) |
| `/system`, `/users`, `/api-tokens`, `/parse-errors`, `/parser-test`, `/geo-missing`, `/geo-ranges`, `/reputation`, `/tls` | administrator |

Legacy `*.html` URLs redirect to the routes above (nginx + React Router).

## Docker

`frontend/Dockerfile` multi-stage: `npm ci && npm run build` → `nginx:1.30.4-alpine` with `dist/` and `nginx-app.inc`.
