# Install AGENTS

## Purpose And Scope
This guide covers the VPS Docker Compose deployment flow in `install/`. It applies to:
- repository sync on server
- backend dependency install and rendered env files
- Docker Compose startup for backend, worker, web, and infrastructure services
- migration execution
- container Nginx config and Certbot SSL issuance
- post-install verification

Read the repo entrypoint first: [../AGENTS.md](../AGENTS.md)

## Area Ownership
Primary owned directories and files:
- `install.sh`
- `docker-compose.prod.yml`
- `docker-compose.dev.yml`
- `sync-secrets.sh`
- `repo/`
- `backend/`
- `compose/`
- `nginx/`
- `lib/common.sh`
- `config.sh.sample`
- `secret.sh.sample`

Do not casually edit:
- script ordering in `install.sh`
- `.env` rendering in `backend/install.sh`
- container Nginx proxy locations without checking backend route coverage
- verification scripts without preserving the final smoke-test purpose

## Main Entrypoints And Critical Files
- Install orchestrator: `install.sh`
- Repo sync: `repo/sync.sh`
- Backend install/env render: `backend/install.sh`
- Compose startup: `compose/up.sh`
- Production compose: `docker-compose.prod.yml`
- Local dev compose: `docker-compose.dev.yml`
- Backend migrate: `backend/migrate.sh`
- Backend verify: `backend/verify.sh`
- Production RAG verify: `backend/verify_rag.sh`
- Container Nginx config render: `nginx/install.sh`
- SSL issuance: `nginx/issue-ssl.sh`
- Nginx verify: `nginx/verify.sh`
- Local secret sync: `sync-secrets.sh`
- Local Docker Compose entrypoint: `local-compose.sh`

## Deployment Flow
- Local operator prepares `install/config.sh` and syncs it to the VPS with `sync-secrets.sh`.
- GitHub Actions deploys on `v*` tag push or manual dispatch, SSHes into the VPS, and runs `install.sh` for the exact commit SHA.
- `install.sh` runs repo sync, backend env rendering, container Nginx config rendering, Docker Compose startup, migration, optional SSL, then verification.
- Backend install renders `backend/.env` from install variables.
- Docker Compose builds API, worker, and web images on the VPS.
- The web image builds frontend assets and serves them through Nginx inside the `web` container.
- Certbot runs as a compose service and stores certificates under `LETSENCRYPT_DIR`.
- Local development uses `local-compose.sh` with `docker-compose.dev.yml`, not direct host `go run` or host `bun run dev`.

## Runtime Invariants
- Production variables live in `config.sh`; backend runtime then derives `backend/.env` from them.
- Docker Compose is the production runtime path for backend, worker, frontend web serving, Postgres, Qdrant, and Certbot.
- Host Nginx is not part of the production runtime.
- SSL is optional at script level but enabled by default when `ENABLE_SSL` is not disabled.
- Verification scripts are the final guardrail. Keep them meaningful.

## Required Commands For Verification
- Full flow: `cd install && ./install.sh`
- Repo sync only: `cd install && ./repo/sync.sh`
- Backend install/env render only: `cd install && ./backend/install.sh`
- Compose up only: `cd install && ./compose/up.sh`
- Backend migrate only: `cd install && ./backend/migrate.sh`
- Backend verify only: `cd install && ./backend/verify.sh`
- Production RAG verify only: `cd install && ./backend/verify_rag.sh`
- Nginx config render only: `cd install && ./nginx/install.sh`
- Nginx verify only: `cd install && ./nginx/verify.sh`
- Secret sync only: `cd install && ./sync-secrets.sh`
- Local compose from repo root: `make up`, `make down`, `make stop`, `make restart`, `make log`
- Local compose direct from repo root: `./install/local-compose.sh up`
- Local frontend dependency add from repo root: `make web-add PKG=<package...>` or `./install/local-compose.sh web-add <package...>`

## Common Failure Modes
- Changing required variables without updating `require_vars` calls
- Breaking repo sync assumptions for branch or commit pinning
- Changing backend env rendering without checking app config load expectations
- Updating frontend image build assumptions without considering Bun lockfile behavior
- Missing container Nginx proxy coverage for routes used in production
- Editing verification scripts into no-op checks
- Putting private secrets in `VITE_*` values, which are browser-public

## When To Involve Another Agent Or Skill
- Use [`legal-api-deploy-vps`](../.agents/skills/legal-api-deploy-vps/SKILL.md) for all deployment and server automation work here.
- Involve [`legal-api-backend-feature`](../.agents/skills/legal-api-backend-feature/SKILL.md) if production env or backend startup assumptions change.
- Involve [`legal-api-frontend-admin-chat`](../.agents/skills/legal-api-frontend-admin-chat/SKILL.md) if frontend build inputs or app serving behavior change.
- Involve [`review-agent`](../docs/agent-roles.md#review-agent) before sign-off on deployment changes that touch env rendering, nginx routing, or verification.

## Definition Of Done
- Script task: execution order, required vars, and side effects are coherent with the existing install flow.
- Backend deployment task: rendered env, compose startup, migration execution, and health verification still align.
- Frontend deployment task: web image build and container Nginx serving are correct.
- Nginx or SSL task: rendered container config, proxy targets, Certbot storage, and verification behavior remain operational.
