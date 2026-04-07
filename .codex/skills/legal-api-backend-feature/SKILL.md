# legal-api-backend-feature

## When To Use This Skill
Use this skill for standard backend work in `backend/` that is not primarily retrieval/answer-specific and not primarily ingest/vector-specific.

Examples:
- add or modify HTTP endpoints
- update auth or session behavior
- adjust admin endpoints outside retrieval and vector-specialized logic
- change backend validation, persistence wiring, or config-sensitive behavior

Preferred lead role: [`backend-api-agent`](../../../docs/agent-roles.md#backend-api-agent)

Fallback collaborator role: [`review-agent`](../../../docs/agent-roles.md#review-agent)

## What This Skill Owns
- `backend/api/`
- `backend/internal/auth/`
- general backend behavior in `backend/admin/`
- supporting backend data flow through `backend/infra/`, `backend/domain/`, and `backend/pkg/`

## Architecture Context Assumed
- `backend/cmd/api/main.go` boots env loading, config rendering, DB/Qdrant init, auth bootstrap, and HTTP server startup.
- `backend/api/server.go` registers public, authenticated, and admin routes.
- The backend uses Postgres, Qdrant, and OpenAI-backed answer services in the main runtime.
- Error envelope conventions are defined in handler helpers and should remain consistent.

## Workflow
1. Read [../../../backend/AGENTS.md](../../../backend/AGENTS.md).
2. Find the live route, handler, repository, and auth entrypoints involved in the request.
3. Decide whether the task remains in generic backend scope:
   - if retrieval, prompt, citation, or answer behavior is primary, hand off to `legal-api-retrieval-answer`
   - if ingest, worker, Qdrant payloads, repair, or reindex behavior is primary, hand off to `legal-api-ingest-vector`
4. Implement changes along the existing backend layering instead of bypassing it.
5. Update tests closest to the changed layer.
6. Check whether `frontend/src/core/api.ts` or frontend types must change.
7. Check whether env names, startup assumptions, or deployment scripts are affected.

## Required Checks Before Finishing
- route registration and auth middleware are correct
- request parsing and response envelope conventions are preserved
- DB and repository interactions are coherent
- frontend contract implications are called out
- API and worker shared-code impact has been considered

## Common Regressions To Look For
- changing auth behavior without checking refresh and change-password flow
- adding a backend route but not exposing it through the frontend API layer when needed
- changing env names or defaults without considering install rendering
- mixing retrieval or ingest-specific logic into generic handlers

## Handoff Guidance
- Hand off to [`legal-api-retrieval-answer`](../legal-api-retrieval-answer/SKILL.md) for search ranking, prompts, citations, or answer behavior.
- Hand off to [`legal-api-ingest-vector`](../legal-api-ingest-vector/SKILL.md) for chunking, embeddings, Qdrant writes, worker loops, or repair.
- Hand off to [`legal-api-frontend-admin-chat`](../legal-api-frontend-admin-chat/SKILL.md) when request or response shapes affect UI behavior.
- Hand off to [`legal-api-deploy-vps`](../legal-api-deploy-vps/SKILL.md) if deploy-time env or script behavior changes.
