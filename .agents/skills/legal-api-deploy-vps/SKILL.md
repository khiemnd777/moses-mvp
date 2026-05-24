---
name: legal-api-deploy-vps
description: Legal API deployment and install workflow for VPS setup, Docker Compose runtime, GitHub Actions deploy, env rendering, local compose, container Nginx, SSL, and verification. Use for any install, deployment, runtime script, secret sync, or production/local compose change.
---

# legal-api-deploy-vps

## When To Use This Skill
Use this skill for any change under `install/` or any request involving:
- VPS install flow
- repo sync on server
- Docker Compose startup for API, worker, web, Postgres, Qdrant, and Certbot
- rendered production env files
- frontend image build and container Nginx serving
- container Nginx config or SSL issuance
- GitHub Actions deployment over SSH
- local secret sync to VPS
- local Docker Compose runtime
- post-install verification

Preferred lead role: [`deploy-agent`](../../../docs/agent-roles.md#deploy-agent)

Fallback collaborator role: [`review-agent`](../../../docs/agent-roles.md#review-agent)

## What This Skill Owns
- all files under `install/`

## Architecture Context Assumed
- `install/install.sh` is the top-level orchestrator.
- Production deploy is triggered by `.github/workflows/deploy.yml`, SSHes into the VPS, checks out the exact commit SHA, and runs `install/install.sh`.
- The production flow syncs the repo, renders `backend/.env`, renders container Nginx config, starts Docker Compose, runs migrations, optionally issues SSL through the Certbot compose service, and runs verification scripts.
- The `web` compose service builds the React frontend inside `frontend/Dockerfile.prod` and serves it through Nginx inside the container.
- Host Nginx is not part of the production runtime; the compose `web` service owns HTTP and HTTPS ports.
- Local development uses the same compose file through `install/local-compose.sh` and root `make up|down|stop|restart|log`.
- Production config is sourced from `install/config.sh`, not from local development env files.
- Local config is sourced from `install/config.local.sh`, created from `install/config.local.sh.sample` on first local compose run.

## Workflow
1. Read [../legal-api-engineering-guardrails/SKILL.md](../legal-api-engineering-guardrails/SKILL.md) and [../../../install/AGENTS.md](../../../install/AGENTS.md).
2. Classify the task as `bug` or `feature`, then choose the first deploy feedback loop or vertical slice.
3. Identify whether the change belongs to:
   - repo sync and branch or SHA selection
   - backend dependency install or env rendering
   - migration readiness and DB checks
   - Docker Compose service wiring
   - frontend Docker image build or web container serving
   - container Nginx proxying or SSL issuance
   - GitHub Actions deploy orchestration
   - local compose and Makefile entrypoints
   - verification and smoke testing
4. Preserve the existing install order unless the task explicitly requires changing the orchestration.
5. Keep variable requirements explicit through `require_vars` or equivalent checks.
6. When changing env rendering, verify the target variables still satisfy backend runtime expectations.
7. When changing Nginx, verify coverage for actual backend routes used in production and confirm the config is mounted into the `web` service.
8. When changing local compose behavior, verify the published ports do not conflict with other local containers.
9. For deploy bugs, prefer a script or compose config check that reproduces the broken path before editing.

## Required Checks Before Finishing
- scripts still compose correctly from `install/install.sh`
- required variables are explicit
- rendered files match runtime expectations
- verification scripts still check something meaningful
- `docker compose --env-file backend/.env -f install/docker-compose.prod.yml config --quiet` succeeds after env rendering
- root `make up|down|stop|restart|log` still maps to local compose commands
- the selected deploy feedback loop or feature vertical slice has been run or its blocker is documented

## Common Regressions To Look For
- adding new deploy assumptions without wiring them into `config.sh`
- changing backend env rendering without considering `backend/internal/config`
- updating frontend Docker build behavior without considering `frontend/bun.lockb` and browser-public `VITE_*` values
- changing container Nginx locations without checking real backend endpoints
- accidentally reintroducing host Nginx as the production runtime
- leaking private values into GitHub Actions secrets or `VITE_*` build args
- changing local ports without checking currently running Docker published ports

## Handoff Guidance
- Hand off to [`legal-api-backend-feature`](../legal-api-backend-feature/SKILL.md) if deployment changes are driven by backend runtime or env schema changes.
- Hand off to [`legal-api-frontend-admin-chat`](../legal-api-frontend-admin-chat/SKILL.md) if deploy changes are driven by frontend build inputs or app serving behavior.
- Hand off to [`legal-api-repo-architect`](../legal-api-repo-architect/SKILL.md) if the request spans deployment and application behavior broadly.
