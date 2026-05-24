# Legal RAG Frontend

A Vite + React + TypeScript frontend for the Legal RAG platform.

## Local Runtime

Run the app through the root Docker Compose flow:

```bash
cd /Users/khiemnguyen/Works/project_legal_ai/legal_api
make up
```

The local app is served by the compose `web` container at:

```text
http://localhost:19080
```

Useful commands:

```bash
make log
make restart
make stop
make down
```

## Build Check

For frontend-only verification:

```bash
cd frontend
bun install
bun run build
```

Use `bun run dev` or `bun run preview` only for isolated frontend debugging. They are not the standard local runtime path.

## Env

- `VITE_API_BASE_URL` is the browser-public base URL for the Legal API.
- Local compose sets `VITE_API_BASE_URL=http://localhost:19080`.
- Production compose passes `VITE_API_BASE_URL` from `install/config.sh` into `frontend/Dockerfile.prod`.
- Do not put private secrets in `VITE_*` values because they are baked into the browser bundle.
