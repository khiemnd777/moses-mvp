# legal-api-repo-architect

## When To Use This Skill
Use this skill first when the request is ambiguous, cross-cutting, or could belong to more than one subsystem. This is the routing skill for the monorepo.

Preferred lead role: [`repo-architect`](../../../docs/agent-roles.md#repo-architect)

Fallback collaborator role: [`review-agent`](../../../docs/agent-roles.md#review-agent)

## What This Skill Owns
- initial task triage across `backend/`, `frontend/`, and `install/`
- bug versus feature classification
- mapping change impact before implementation
- selecting the lead skill and collaborator roles

It does not implement subsystem-specific logic. Hand off once routing is clear.

## Architecture Context Assumed
- Backend has two runtimes: API in `backend/cmd/api` and worker in `backend/cmd/worker`.
- Frontend is a single Vite app rooted at `frontend/src`.
- Deployment is script-driven from `install/`.
- Retrieval/answer and ingest/vector are specialized backend domains and should not be treated as generic backend work by default.

## Workflow
1. Read [../../../AGENTS.md](../../../AGENTS.md) and the local area guide for the suspected subsystem.
2. Classify the request:
   - `bug` if the user describes existing behavior as broken, regressed, inconsistent, or failing in a path that already exists
   - `feature` if the user is asking for net-new capability, surface area, workflow, or contract
   - if unclear, inspect whether the relevant surface already exists in the repo; if it does and the behavior is supposed to work, prefer `bug`
3. Identify the request type:
   - backend feature or auth/admin behavior
   - retrieval, prompt, answer, citation, or search behavior
   - ingest, worker, Qdrant, vector repair, or reindex behavior
   - frontend chat/admin/vector UI behavior
   - VPS install or deployment behavior
4. Map affected paths before deciding ownership.
5. Pick one lead skill and, if needed, one or two collaborator skills.
6. Name the lead agent role and explicit handoff roles.
7. List the verification surfaces that must be checked before completion.
8. Adjust verification posture based on class:
   - `bug`: verify the failing path and the nearest regression-prone neighbors
   - `feature`: verify the new happy path and the adjacent existing flows it extends

## Required Checks Before Finishing
- the request is classified as `bug` or `feature`
- exactly one lead skill is identified
- exactly one lead role is identified
- cross-cutting collaborators are named when required
- backend, frontend, and install impacts are all considered for full-stack work

## Common Regressions To Look For
- routing a retrieval issue to generic backend work
- routing a vector consistency issue to frontend or deploy work
- forgetting that worker runtime is affected by shared backend changes
- missing deployment impact for new env variables or route exposure

## Handoff Guidance
- Hand off to [`legal-api-backend-feature`](../legal-api-backend-feature/SKILL.md) for standard backend API/auth/admin changes.
- Hand off to [`legal-api-retrieval-answer`](../legal-api-retrieval-answer/SKILL.md) for search, prompt, answer, citation, or trace work.
- Hand off to [`legal-api-ingest-vector`](../legal-api-ingest-vector/SKILL.md) for ingest, worker, Qdrant, repair, or reindex work.
- Hand off to [`legal-api-frontend-admin-chat`](../legal-api-frontend-admin-chat/SKILL.md) for frontend chat/admin/vector UI work.
- Hand off to [`legal-api-deploy-vps`](../legal-api-deploy-vps/SKILL.md) for any `install/` or VPS behavior change.
