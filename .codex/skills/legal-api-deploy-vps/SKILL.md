# legal-api-deploy-vps

## When To Use This Skill
Use this skill for any change under `install/` or any request involving:
- VPS install flow
- repo sync on server
- backend Docker Compose startup
- rendered production env files
- frontend build and asset deployment
- nginx config or SSL issuance
- post-install verification

Preferred lead role: [`deploy-agent`](../../../docs/agent-roles.md#deploy-agent)

Fallback collaborator role: [`review-agent`](../../../docs/agent-roles.md#review-agent)

## What This Skill Owns
- all files under `install/`

## Architecture Context Assumed
- `install/install.sh` is the top-level orchestrator.
- The deployment flow syncs the repo, installs backend dependencies, renders `backend/.env`, starts Docker Compose, runs migrations, builds frontend assets with Bun, installs them to the web root, installs nginx config, optionally issues SSL, and runs verification scripts.
- Production config is sourced from `install/config.sh`, not from local development env files.

## Workflow
1. Read [../../../install/AGENTS.md](../../../install/AGENTS.md).
2. Identify whether the change belongs to:
   - repo sync and branch or SHA selection
   - backend dependency install or env rendering
   - migration readiness and DB checks
   - frontend build and asset deployment
   - nginx proxying or SSL issuance
   - verification and smoke testing
3. Preserve the existing install order unless the task explicitly requires changing the orchestration.
4. Keep variable requirements explicit through `require_vars` or equivalent checks.
5. When changing env rendering, verify the target variables still satisfy backend runtime expectations.
6. When changing nginx, verify coverage for actual backend routes used in production.

## Required Checks Before Finishing
- scripts still compose correctly from `install/install.sh`
- required variables are explicit
- rendered files match runtime expectations
- verification scripts still check something meaningful

## Common Regressions To Look For
- adding new deploy assumptions without wiring them into `config.sh`
- changing backend env rendering without considering `backend/internal/config`
- updating frontend build behavior without considering Bun installation and lockfile cleanup
- changing nginx locations without checking real backend endpoints

## Handoff Guidance
- Hand off to [`legal-api-backend-feature`](../legal-api-backend-feature/SKILL.md) if deployment changes are driven by backend runtime or env schema changes.
- Hand off to [`legal-api-frontend-admin-chat`](../legal-api-frontend-admin-chat/SKILL.md) if deploy changes are driven by frontend build inputs or app serving behavior.
- Hand off to [`legal-api-repo-architect`](../legal-api-repo-architect/SKILL.md) if the request spans deployment and application behavior broadly.
