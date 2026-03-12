# Backend

This backend uses Postgres and Qdrant. The easiest way to run infra is Docker Compose.

## 1) Install infra: Postgres, Qdrant

### Option A: Docker Compose (recommended)

1. Install Docker Desktop.
2. From `backend/`, start the infra containers:

```bash
cd backend

docker compose -f docker/docker-compose.yml up -d postgres qdrant
```

This exposes:
- Postgres on `localhost:5432` (user `legal`, password `legal`, db `legal_rag`)
- Qdrant on `localhost:6333`

### Option B: Native installs

If you prefer native installs, install:
- Postgres 15
- Qdrant v1.9.x

Then update `config/config.yaml` (or create a local copy) to point at your local endpoints.

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
2. Create a local config file and point it to localhost:

```bash
cd backend

cp config/config.yaml config/config.local.yaml
```

Edit `config/config.local.yaml`:
- `postgres.dsn`: use `postgres://legal:legal@localhost:5432/legal_rag?sslmode=disable`
- `qdrant.url`: use `http://localhost:6333`
- `openai.api_key`: set your API key

3. Run the API:

```bash
cd backend

CONFIG_PATH=config/config.local.yaml go run ./cmd/api
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
- If you run the API in Docker, it uses `config/config.yaml` and the service hostnames (`postgres`, `qdrant`).
