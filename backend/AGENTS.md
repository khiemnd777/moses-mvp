# Backend AGENTS

## Purpose And Scope
This guide covers the Go backend in `backend/`. It applies to:
- HTTP API routes
- auth and session flows
- retrieval and answer generation
- ingest and worker processing
- Qdrant and Postgres integration
- migrations, prompt config, and observability

Read the repo entrypoint first: [../AGENTS.md](../AGENTS.md)

## Area Ownership
Primary owned directories:
- `cmd/`
- `api/`
- `admin/`
- `core/`
- `domain/`
- `infra/`
- `internal/`
- `observability/`
- `pkg/`
- `config/`

Do not casually edit:
- `config/prompts/*.yaml` unless the task is explicitly about prompts or answer behavior
- `infra/migrations/*.sql` unless schema change is required and backwards compatibility has been considered
- `cmd/worker` behavior without checking ingest and vector repair implications

## Main Entrypoints And Critical Files
- API boot: `cmd/api/main.go`
- Worker boot: `cmd/worker/main.go`
- Route registration: `api/server.go`
- Handler wiring and runtime answer config: `api/handlers.go`
- Retrieval service: `core/retrieval/retrieval.go`
- Answer client and stream behavior: `core/answer/`
- Ingest service: `core/ingest/ingest.go`
- Auth service and handlers: `internal/auth/`
- Qdrant and vector consistency helpers: `infra/qdrant.go`, `infra/vector_*`
- Prompt files and sample config: `config/prompts/`, `config/config.yaml.sample`
- Migrations: `infra/migrations/`

## Architecture Notes
- `api/` owns HTTP transport, parsing, response envelopes, and route-level orchestration.
- `admin/` owns AI tuning and Qdrant control-plane endpoints.
- `core/retrieval` owns search planning, embedding query generation, reranking, adjacent expansion, and retrieval observability.
- `core/answer` owns answer model interactions and streaming behavior.
- `core/ingest` owns normalization, segmentation, metadata extraction, embeddings, chunk replacement, and vector upsert/delete logic.
- `infra/` owns persistence and external system adapters for Postgres, storage, and Qdrant.
- `internal/auth` owns JWT, refresh sessions, bootstrap, middleware, and password-change flow.
- `observability/` owns trace repository and metrics helpers.

## Runtime Invariants
- `backend/.env` is the source of truth. The app renders `config/config.yaml` from that env during startup.
- API and worker share config loading, Postgres setup, Qdrant setup, and some seed/repair initialization.
- The API depends on Postgres, Qdrant, and OpenAI health. `/health` reflects all three.
- Ingest writes must keep chunk rows and Qdrant points aligned, including stale vector cleanup.
- Retrieval and prompt behavior may use cached runtime config. Changes to prompt or retrieval config must consider invalidation paths.
- Auth changes affect login, refresh, logout, `/auth/me`, change-password, and guarded routes.
- Worker behavior includes stale job recovery, failed job requeue, ingest processing, and vector repair ticker execution.

## Required Commands For Verification
- Build API: `cd backend && go build -o bin/api ./cmd/api`
- Build worker: `cd backend && go build -o bin/worker ./cmd/worker`
- Run API locally: `cd backend && go run ./cmd/api`
- Run worker locally: `cd backend && go run ./cmd/worker`
- Run tests: `cd backend && go test ./...`
- Apply migrations: `cd backend && make migrate`
- Migration status: `cd backend && make migrate-status`

Use targeted tests when changing a subsystem with dedicated coverage, especially under:
- `api/*_test.go`
- `core/*/*_test.go`
- `internal/auth/*_test.go`
- `admin/service/*_test.go`

## Common Failure Modes
- Updating route behavior without updating request validation or error envelope conventions
- Changing auth middleware or token behavior without checking refresh and forced password change flows
- Editing retrieval ranking or context assembly without checking trace output and citation behavior
- Editing ingest without checking idempotency, partial recovery, stale vector deletion, and worker retries
- Adding a migration file without ensuring migration execution order remains coherent with `Makefile`
- Changing config env names or defaults without checking both local and install-rendered `.env`

## When To Involve Another Agent Or Skill
- Use [`legal-api-backend-feature`](../.codex/skills/legal-api-backend-feature/SKILL.md) for standard route, auth, config, and admin backend work.
- Use [`legal-api-retrieval-answer`](../.codex/skills/legal-api-retrieval-answer/SKILL.md) when the change affects search, prompts, answer generation, citations, or traces.
- Use [`legal-api-ingest-vector`](../.codex/skills/legal-api-ingest-vector/SKILL.md) when the change affects chunking, worker flow, Qdrant payloads, vector repair, or reindex paths.
- Involve [`frontend-agent`](../docs/agent-roles.md#frontend-agent) if a backend contract change affects `frontend/src/core/api.ts` or UI state.
- Involve [`deploy-agent`](../docs/agent-roles.md#deploy-agent) if env variables, compose assumptions, or production endpoints change.
- Involve [`review-agent`](../docs/agent-roles.md#review-agent) before sign-off on auth, retrieval, ingest, or migration-heavy work.

## Definition Of Done
- API work: routes, auth, validation, and persistence behavior are coherent and tested at the relevant layer.
- Retrieval or answer work: search behavior, ranking, prompt selection, citations, and traces are consistent.
- Ingest or vector work: worker path, chunk persistence, Qdrant operations, and repair behavior remain aligned.
- Migration work: schema change is represented in SQL, callable through existing migration commands, and safe for the current runtime assumptions.
