# legal-api-ingest-vector

## When To Use This Skill
Use this skill when the request is about:
- document ingest jobs
- chunk generation, normalization, or metadata extraction
- embeddings or Qdrant point payloads
- worker processing, retries, or stale-job recovery
- vector repair, delete-by-filter, or reindex workflows

Preferred lead role: [`ingest-vector-agent`](../../../docs/agent-roles.md#ingest-vector-agent)

Fallback collaborator role: [`review-agent`](../../../docs/agent-roles.md#review-agent)

## What This Skill Owns
- `backend/core/ingest/`
- `backend/cmd/worker/`
- Qdrant and vector consistency helpers in `backend/infra/`
- vector control-plane-sensitive parts of `backend/admin/`

## Architecture Context Assumed
- Ingest creates or replaces chunk rows in Postgres and upserts matching vectors in Qdrant.
- Worker runtime in `backend/cmd/worker/main.go` resets stale jobs, requeues failed jobs, processes queued jobs, and runs vector repair passes.
- Vector IDs and payloads must remain consistent with document version and chunk identity.
- Admin vector tooling in the frontend depends on backend vector-control capabilities remaining coherent.

## Workflow
1. Read [../../../backend/AGENTS.md](../../../backend/AGENTS.md).
2. Determine which category the issue belongs to:
   - content normalization or segmentation
   - metadata extraction and payload composition
   - vector write/delete behavior
   - worker retry or processing lifecycle
   - repair or reindex support
3. Trace the full path from source asset to chunk rows to vector points.
4. Prefer fixes that keep Postgres and Qdrant state synchronized, including cleanup of stale vectors.
5. Check whether the worker runtime and admin control-plane paths both need updates.
6. If the visible symptom is ranking or answer quality, coordinate with `legal-api-retrieval-answer`.
7. If the visible symptom is dashboard or admin workflow breakage, coordinate with `legal-api-frontend-admin-chat`.

## Required Checks Before Finishing
- DB chunk records and Qdrant point payloads remain aligned
- worker retry, stale reset, and repair loops still make sense
- delete, reindex, or repair commands remain safe and scoped
- any API or admin surface touched remains contract-compatible

## Common Regressions To Look For
- upserting new vectors without deleting stale ones
- changing payload field names without checking retrieval filters or admin pages
- fixing API-triggered ingest but forgetting worker execution path
- assuming a ranking problem is purely retrieval when chunk or payload quality is degraded

## Handoff Guidance
- Hand off to [`legal-api-retrieval-answer`](../legal-api-retrieval-answer/SKILL.md) when ranking, citations, or prompt behavior becomes the primary issue.
- Hand off to [`legal-api-frontend-admin-chat`](../legal-api-frontend-admin-chat/SKILL.md) when vector dashboards or ingest-related UI need to reflect the backend change.
- Hand off to [`legal-api-deploy-vps`](../legal-api-deploy-vps/SKILL.md) if runtime env or production Qdrant assumptions must change.
