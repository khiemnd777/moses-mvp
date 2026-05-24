---
name: legal-api-retrieval-answer
description: Legal API retrieval and answer workflow for search ranking, prompt routing, citations, answer streaming, runtime config, and trace behavior. Use when work affects answer quality, context assembly, prompt behavior, citation shape, or retrieval observability.
---

# legal-api-retrieval-answer

## When To Use This Skill
Use this skill when the request is about:
- search results or ranking behavior
- prompt routing or answer composition
- citations or answer streaming
- runtime answer config, prompt config, or trace behavior
- retrieval observability and answer-quality debugging

Preferred lead role: [`retrieval-agent`](../../../docs/agent-roles.md#retrieval-agent)

Fallback collaborator role: [`review-agent`](../../../docs/agent-roles.md#review-agent)

## What This Skill Owns
- `backend/core/retrieval/`
- `backend/core/answer/`
- retrieval and answer paths in `backend/api/`
- answer and retrieval trace helpers in `backend/observability/`
- prompt files under `backend/config/prompts/` when intentionally changed

## Architecture Context Assumed
- Search planning and reranking live in `backend/core/retrieval/retrieval.go`.
- Answer generation and streaming are split across `backend/core/answer/` and `backend/api/answer_*`.
- Runtime answer behavior depends on prompt and guard data loaded through backend services and may be cached.
- Citations are part of the answer contract and may be consumed by the frontend chat UI.

## Workflow
1. Read [../legal-api-engineering-guardrails/SKILL.md](../legal-api-engineering-guardrails/SKILL.md) and [../../../backend/AGENTS.md](../../../backend/AGENTS.md).
2. Classify the task as `bug` or `feature`, then choose the first retrieval/answer feedback loop or vertical slice.
3. Identify whether the issue is:
   - retrieval candidate selection
   - reranking or context assembly
   - prompt selection or tone behavior
   - answer formatting or streaming
   - citation generation or trace emission
4. Inspect the relevant API path and core service together. Do not change one in isolation when the behavior crosses layers.
5. Preserve or intentionally update trace and observability output when changing answer behavior.
6. If ranking issues are actually caused by bad chunking or vector payloads, hand off to `legal-api-ingest-vector`.
7. If response shape changes affect UI rendering, coordinate with `legal-api-frontend-admin-chat`.

## Required Checks Before Finishing
- retrieval result ordering and context limits are still coherent
- citation fields remain stable for consumers
- prompt or runtime config changes have a clear invalidation path
- streaming and non-streaming answer paths stay aligned where required
- trace data still matches the changed behavior
- the selected retrieval/answer feedback loop or feature vertical slice has been run or its blocker is documented

## Common Regressions To Look For
- changing ranking behavior without updating trace expectations
- changing prompts without checking downstream answer formatting
- altering citation payloads without frontend sync
- masking ingest-quality problems as retrieval-only issues

## Handoff Guidance
- Hand off to [`legal-api-ingest-vector`](../legal-api-ingest-vector/SKILL.md) if chunk quality, vector payloads, or Qdrant state are the real cause.
- Hand off to [`legal-api-frontend-admin-chat`](../legal-api-frontend-admin-chat/SKILL.md) if answer or citation shape changes require UI updates.
- Hand off to [`legal-api-backend-feature`](../legal-api-backend-feature/SKILL.md) if the change is really about generic route/auth/admin behavior.
