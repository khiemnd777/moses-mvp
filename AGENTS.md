# AGENTS.md

## Purpose
This repository is a monorepo for the `legal_api` platform. It contains:
- a Go backend API and worker
- a React frontend for chat and admin operations
- VPS installation and deployment scripts
- repo-local skills and agent-role documentation for agentic execution

Use this file as the entrypoint for routing work. Read the local area guide before editing a subsystem:
- [backend/AGENTS.md](backend/AGENTS.md)
- [frontend/AGENTS.md](frontend/AGENTS.md)
- [install/AGENTS.md](install/AGENTS.md)
- [docs/agent-roles.md](docs/agent-roles.md)
- [.codex/skills/](.codex/skills/)

## Repository Map
- `backend/`: Go services, migrations, prompt config, worker logic, tests
- `frontend/`: Vite + React + TypeScript UI for chat, auth, admin, and vector tooling
- `install/`: Linux VPS install, repo sync, backend deploy, frontend build/install, nginx, SSL
- `docs/`: repository-local operational docs, including agent roles
- `.codex/skills/`: repository-local skill definitions

## Ownership And Routing
- Backend work: API handlers, auth, retrieval, answer generation, ingest, worker, migrations, Qdrant, Postgres
- Frontend work: UI, route structure, auth redirect behavior, chat UX, admin tools, frontend contract wiring
- Install work: server provisioning flow, repo sync, rendered env files, Docker Compose, nginx, SSL, verification
- Full-stack work: start here, then involve the relevant local guide and matching skills

If a task spans multiple areas, route it like this:
- backend contract change plus UI consumption: involve backend first, then frontend
- retrieval or answer-quality work: involve retrieval before generic backend
- ingest, Qdrant, reindex, or vector consistency work: involve ingest/vector before generic backend
- deployment or VPS breakage: involve install before app-level changes

## Task Classification
Every new request should be classified before implementation. The default taxonomy is:
- `bug`: existing behavior is wrong, broken, regressed, inconsistent, missing in a path that already exists, or failing against stated expectations
- `feature`: new behavior, new surface area, new workflow, new contract, or an intentional extension of current capabilities

Use these cues to classify:
- classify as `bug` when the request references broken output, failing flow, regression, mismatch, inconsistency, error handling gap, stale data, incorrect auth behavior, wrong ranking, or deploy drift
- classify as `feature` when the request adds a route, screen, admin action, filter, workflow, config surface, automation, or capability not already present
- classify as `bug` if uncertain and the requested behavior is supposed to exist already
- classify as `feature` if the user explicitly wants a new capability even if it resembles an existing flow

After classifying, route like this:
- `bug` plus unclear ownership: start with `legal-api-repo-architect`, then hand off to the owning subsystem
- `bug` in ranking, citations, prompts, or answers: `legal-api-retrieval-answer`
- `bug` in ingest, worker, vector mismatch, repair, or reindex: `legal-api-ingest-vector`
- `bug` in HTTP, auth, admin CRUD, or config-backed backend behavior: `legal-api-backend-feature`
- `bug` in UI, chat flow, admin screens, or frontend auth redirects: `legal-api-frontend-admin-chat`
- `bug` in VPS install, nginx, SSL, sync, or rendered production env: `legal-api-deploy-vps`
- `feature` requests follow the same subsystem routing, but require explicit contract and verification planning before implementation

Verification emphasis differs by class:
- `bug`: reproduce or describe the failing path, fix the minimal owning surface, and verify the regression path directly
- `feature`: define the new contract or workflow first, then verify happy path plus existing neighboring flows that could regress

## Main Runtime Facts
- Backend API entrypoint: `backend/cmd/api/main.go`
- Backend worker entrypoint: `backend/cmd/worker/main.go`
- Frontend app entrypoint: `frontend/src/main.tsx`
- Frontend route shell: `frontend/src/app/App.tsx`
- Install orchestrator: `install/install.sh`
- Backend reads `backend/.env`, then renders `backend/config/config.yaml` at runtime
- Backend depends on Postgres and Qdrant
- Frontend depends on `VITE_API_BASE_URL` and stores auth token in local storage

## Shared Commands
- Backend local API: `cd backend && go run ./cmd/api`
- Backend local worker: `cd backend && go run ./cmd/worker`
- Backend migrations: `cd backend && make migrate`
- Backend migration status: `cd backend && make migrate-status`
- Backend tests: `cd backend && go test ./...`
- Frontend dev: `cd frontend && bun run dev`
- Frontend build: `cd frontend && bun run build`
- Frontend preview: `cd frontend && bun run preview`
- Install flow entrypoint: `cd install && ./install.sh`

## Cross-Cutting Invariants
- Do not treat `backend/config/config.yaml` as the source of truth. The source is `backend/.env`; the YAML is rendered from it.
- Backend changes can affect both API runtime and worker runtime. Check both if the changed code is used in shared packages.
- Retrieval, answer generation, and ingest are separate concerns. Do not collapse them into one generic backend change.
- Ingest and vector operations must preserve consistency between chunk rows in Postgres and vector payloads in Qdrant.
- Frontend admin pages mirror real backend capabilities. Do not add UI-only behavior that has no backend support.
- Deployment scripts render production env files. Do not assume local `.env` behavior matches VPS behavior unless verified.

## Common Failure Modes
- Updating backend request or response shapes without updating `frontend/src/core/api.ts` and related types
- Changing prompt, guard, or retrieval config paths without considering runtime cache invalidation
- Editing ingest logic without validating worker retry and vector repair paths
- Changing deployment scripts without checking downstream `verify.sh` steps
- Assuming README content is fully current; prefer entrypoints and active code paths

## Skills And Roles
Primary skills:
- [`legal-api-repo-architect`](.codex/skills/legal-api-repo-architect/SKILL.md)
- [`legal-api-backend-feature`](.codex/skills/legal-api-backend-feature/SKILL.md)
- [`legal-api-retrieval-answer`](.codex/skills/legal-api-retrieval-answer/SKILL.md)
- [`legal-api-ingest-vector`](.codex/skills/legal-api-ingest-vector/SKILL.md)
- [`legal-api-frontend-admin-chat`](.codex/skills/legal-api-frontend-admin-chat/SKILL.md)
- [`legal-api-deploy-vps`](.codex/skills/legal-api-deploy-vps/SKILL.md)

Primary roles:
- [`repo-architect`](docs/agent-roles.md#repo-architect)
- [`backend-api-agent`](docs/agent-roles.md#backend-api-agent)
- [`retrieval-agent`](docs/agent-roles.md#retrieval-agent)
- [`ingest-vector-agent`](docs/agent-roles.md#ingest-vector-agent)
- [`frontend-agent`](docs/agent-roles.md#frontend-agent)
- [`deploy-agent`](docs/agent-roles.md#deploy-agent)
- [`review-agent`](docs/agent-roles.md#review-agent)

## When To Involve Another Agent Or Skill
- Involve `legal-api-repo-architect` when the request is unclear, cross-cutting, or could land in more than one subsystem.
- Involve `legal-api-repo-architect` first when bug versus feature classification is unclear.
- Involve `legal-api-retrieval-answer` for search ranking, answer quality, citation behavior, prompt routing, or trace semantics.
- Involve `legal-api-ingest-vector` for chunking, embeddings, Qdrant payloads, worker loops, vector repair, or reindex behavior.
- Involve `legal-api-frontend-admin-chat` whenever a backend contract change affects rendered UI or user interaction flow.
- Involve `legal-api-deploy-vps` for any change touching `install/`, server paths, nginx, SSL, or rendered production env.
- Involve `review-agent` before sign-off on full-stack, auth, retrieval, or deploy-sensitive work.

## Definition Of Done
- Backend task: code, tests, and command paths are coherent across API and worker if shared.
- Frontend task: route behavior, API calls, error handling, and build path remain coherent.
- Install task: script order, required vars, rendered files, and verify steps are still aligned.
- Full-stack task: backend contract, frontend use, and deployment assumptions all line up.
