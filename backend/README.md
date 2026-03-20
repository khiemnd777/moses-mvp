# Backend

This backend uses Postgres and Qdrant. `backend/.env` is the single source of truth; `config/config.yaml` is rendered from it at runtime, and local migration commands also read `backend/.env`.

## 1) Install infra: Postgres, Qdrant

### Option A: Docker Compose (recommended)

1. Install Docker Desktop.
2. From `backend/`, start the infra containers:

```bash
cd backend

docker compose -f docker/docker-compose.yml up -d postgres qdrant
```

This exposes:
- Postgres on `localhost:5433` (user `legal`, password `legal`, db `legal_rag`)
- Qdrant on `localhost:6333`
- Data paths default to `backend/data/postgres`, `backend/data/qdrant`, and `backend/data/uploads`

If you need different host-side bind mounts, set them in `backend/.env`:
- `POSTGRES_DATA_DIR`
- `QDRANT_STORAGE_DIR`
- `UPLOADS_DATA_DIR`

If you run the backend in Docker, the container runtime endpoints also come from `backend/.env`:
- `DOCKER_POSTGRES_HOST`
- `DOCKER_POSTGRES_PORT`
- `DOCKER_QDRANT_HOST`
- `DOCKER_STORAGE_ROOT_DIR`

### Option B: Native installs

If you prefer native installs, install:
- Postgres 15
- Qdrant v1.9.x

Then update `backend/.env` to point at your local endpoints. The app will render `config/config.yaml` from that file automatically.

## 2) Start/stop Qdrant

If you used Docker Compose:

Start Qdrant:
```bash
cd backend

docker compose -f docker/docker-compose.yml up -d qdrant
```

Stop Qdrant:
```bash
cd backend

docker compose -f docker/docker-compose.yml stop qdrant
```

Remove the container and data volume:
```bash
cd backend

docker compose -f docker/docker-compose.yml down
```

## 3) Start/stop API

### Option A: Run API in Docker

Start:
```bash
cd backend

docker compose -f docker/docker-compose.yml up -d api
```

Stop:
```bash
cd backend

docker compose -f docker/docker-compose.yml stop api
```

### Option B: Run API locally with Go

1. Ensure Postgres and Qdrant are running.
2. Create a local env file:

```bash
cd backend

cp .env.example .env
```

Edit `backend/.env`:
- `POSTGRES_HOST`: use `localhost`
- `POSTGRES_PORT`: use `5433`
- `QDRANT_HOST`: use `localhost`
- `QDRANT_PORT`: use `6333`
- `OPENAI_API_KEY`: set your API key

3. Run the API:

```bash
cd backend

go run ./cmd/api
```

Stop it with `Ctrl+C`.

## 4) Database migrations (with version tracking)

Migration state is tracked in Postgres table `schema_migrations`.

Apply pending migrations:
```bash
cd backend
make migrate
```

Show applied versions:
```bash
cd backend
make migrate-status
```

If your database already exists and was migrated before this tracking was added, baseline first (mark as applied without executing SQL files):
```bash
cd backend
make migrate-baseline
```

Then use `make migrate` normally for new migration files.

---

Notes:
- Default API port is `8080`.
- If you run the API in Docker, Compose reads `backend/.env`, and the app renders `config/config.yaml` from the same values on startup.
- Backend CORS is configured by env var `CORS_ALLOWED_ORIGINS`, defaulting to `http://localhost:5173`.
- Example local run with a custom frontend origin:

```bash
cd backend

CORS_ALLOWED_ORIGINS=http://localhost:5173,https://ai.dailyturning.com go run ./cmd/api
```
