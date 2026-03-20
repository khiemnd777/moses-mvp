# Legal RAG Frontend

A Vite + React + TypeScript frontend for the Legal RAG platform.

## Requirements
- Bun

## Setup
```
bun install
cp .env.example .env
bun run dev
```

## Build
```
bun run build
bun run preview
```

## Env
- `VITE_API_BASE_URL` Base URL for the Legal API backend
- Prefer same-origin in production, for example `https://ai.dailyturning.com` behind Nginx proxying API routes.
- If you point the frontend to a separate API origin, make sure that origin is listed in backend `CORS_ALLOWED_ORIGINS`.
