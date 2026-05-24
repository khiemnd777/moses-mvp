---
name: legal-api-frontend-admin-chat
description: Legal API frontend implementation for React routes, chat UX, auth redirects, admin screens, vector tooling, API wrappers, and shared types. Use for frontend UI work or when backend contract changes affect rendered behavior.
---

# legal-api-frontend-admin-chat

## When To Use This Skill
Use this skill for all frontend work in this repository, including:
- chat UX and conversation flows
- auth redirect behavior
- admin pages for doc types, documents, and ingest jobs
- AI tuning pages
- vector dashboard and control-plane pages
- frontend API wrapper and type updates required by backend changes

Preferred lead role: [`frontend-agent`](../../../docs/agent-roles.md#frontend-agent)

Fallback collaborator role: [`backend-api-agent`](../../../docs/agent-roles.md#backend-api-agent)

## What This Skill Owns
- `frontend/src/app/`
- `frontend/src/core/`
- `frontend/src/features/`
- `frontend/src/shared/`
- `frontend/src/playground/`

## Architecture Context Assumed
- The app route shell is defined in `frontend/src/app/App.tsx`.
- Backend communication is centralized in `frontend/src/core/api.ts` and `frontend/src/playground/apiClient.js`.
- The app includes chat, auth, admin, AI tuning, and vector tooling in a single frontend surface.
- Frontend auth relies on bearer token storage plus refresh-token-backed renewal via Axios interceptors.

## Workflow
1. Read [../legal-api-engineering-guardrails/SKILL.md](../legal-api-engineering-guardrails/SKILL.md) and [../../../frontend/AGENTS.md](../../../frontend/AGENTS.md).
2. Classify the task as `bug` or `feature`, then choose the first UI feedback loop or vertical slice.
3. Identify the route, feature module, shared type, and API wrapper involved.
4. Update UI behavior through the existing feature structure rather than adding ad hoc calls inside unrelated components.
5. If the task depends on a backend contract, verify it in `frontend/src/core/api.ts` and `frontend/src/core/types.ts`.
6. Preserve auth redirect and refresh behavior when working near `src/playground/apiClient.js`.
7. For vector admin pages, verify request names and response structures against the backend control-plane APIs.
8. Prototype only when the UI or state-model question is unclear and the prototype can be deleted or absorbed quickly.

## Required Checks Before Finishing
- route and guard behavior are still correct
- API wrapper and type definitions match backend behavior
- loading and error handling are coherent for the affected screen
- the production build still succeeds
- the selected UI feedback loop or feature vertical slice has been run or its blocker is documented

## Common Regressions To Look For
- bypassing the shared API layer and duplicating backend calls in components
- breaking refresh redirect behavior by misusing Axios clients
- updating UI labels or forms without matching backend request fields
- changing vector admin UI around backend assumptions that no longer hold

## Handoff Guidance
- Hand off to [`legal-api-backend-feature`](../legal-api-backend-feature/SKILL.md) when the UI requires a backend endpoint or contract change.
- Hand off to [`legal-api-retrieval-answer`](../legal-api-retrieval-answer/SKILL.md) for answer-quality, citations, or search semantics changes.
- Hand off to [`legal-api-ingest-vector`](../legal-api-ingest-vector/SKILL.md) for ingest, reindex, repair, or Qdrant-control behavior changes.
