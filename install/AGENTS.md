# Install AGENTS

## Purpose And Scope
This guide covers the VPS installation and deployment flow in `install/`. It applies to:
- repository sync on server
- backend dependency install and Docker Compose startup
- migration execution
- frontend build and asset install
- nginx config and SSL issuance
- post-install verification

Read the repo entrypoint first: [../AGENTS.md](../AGENTS.md)

## Area Ownership
Primary owned directories and files:
- `install.sh`
- `repo/`
- `backend/`
- `frontend/`
- `nginx/`
- `lib/common.sh`
- `config.sh.sample`
- `secret.sh.sample`

Do not casually edit:
- script ordering in `install.sh`
- `.env` rendering in `backend/install.sh`
- nginx proxy locations without checking backend route coverage
- verification scripts without preserving the final smoke-test purpose

## Main Entrypoints And Critical Files
- Install orchestrator: `install.sh`
- Repo sync: `repo/sync.sh`
- Backend install: `backend/install.sh`
- Backend migrate: `backend/migrate.sh`
- Backend verify: `backend/verify.sh`
- Frontend build: `frontend/build.sh`
- Frontend install: `frontend/install.sh`
- Nginx install: `nginx/install.sh`
- SSL issuance: `nginx/issue-ssl.sh`
- Nginx verify: `nginx/verify.sh`

## Deployment Flow
- `copy.sh` uploads the install bundle from a local machine.
- VPS operator prepares `config.sh`.
- `install.sh` runs repo sync, backend install, migration, frontend build, frontend install, nginx install, optional SSL, then verification.
- Backend install renders `backend/.env` from install variables and starts Docker Compose services.
- Frontend build renders `.env.production.local`, installs Bun if needed, and builds `frontend/dist`.
- Frontend install copies built assets into the configured web root.
- Nginx install renders the site config and proxies API endpoints to the backend upstream.

## Runtime Invariants
- Production variables live in `config.sh`; backend runtime then derives `backend/.env` from them.
- Backend Docker Compose is the production runtime path for the backend.
- Frontend production assets are built from repo source on the VPS.
- SSL is optional at script level but enabled by default when `ENABLE_SSL` is not disabled.
- Verification scripts are the final guardrail. Keep them meaningful.

## Required Commands For Verification
- Full flow: `cd install && ./install.sh`
- Repo sync only: `cd install && ./repo/sync.sh`
- Backend install only: `cd install && ./backend/install.sh`
- Backend migrate only: `cd install && ./backend/migrate.sh`
- Backend verify only: `cd install && ./backend/verify.sh`
- Frontend build only: `cd install && ./frontend/build.sh`
- Frontend install only: `cd install && ./frontend/install.sh`
- Nginx install only: `cd install && ./nginx/install.sh`
- Nginx verify only: `cd install && ./nginx/verify.sh`

## Common Failure Modes
- Changing required variables without updating `require_vars` calls
- Breaking repo sync assumptions for branch or commit pinning
- Changing backend env rendering without checking app config load expectations
- Updating frontend build assumptions without considering Bun install and lockfile behavior
- Missing nginx proxy coverage for routes used in production
- Editing verification scripts into no-op checks

## When To Involve Another Agent Or Skill
- Use [`legal-api-deploy-vps`](../.codex/skills/legal-api-deploy-vps/SKILL.md) for all deployment and server automation work here.
- Involve [`legal-api-backend-feature`](../.codex/skills/legal-api-backend-feature/SKILL.md) if production env or backend startup assumptions change.
- Involve [`legal-api-frontend-admin-chat`](../.codex/skills/legal-api-frontend-admin-chat/SKILL.md) if frontend build inputs or asset expectations change.
- Involve [`review-agent`](../docs/agent-roles.md#review-agent) before sign-off on deployment changes that touch env rendering, nginx routing, or verification.

## Definition Of Done
- Script task: execution order, required vars, and side effects are coherent with the existing install flow.
- Backend deployment task: rendered env, compose startup, migration execution, and health verification still align.
- Frontend deployment task: build output and copied asset path are correct for nginx serving.
- Nginx or SSL task: rendered site config, proxy targets, and verification behavior remain operational.
