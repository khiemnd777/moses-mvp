# Agent Roles

This document defines the seven primary agent roles for this repository. Use it with:
- [../AGENTS.md](../AGENTS.md)
- [../backend/AGENTS.md](../backend/AGENTS.md)
- [../frontend/AGENTS.md](../frontend/AGENTS.md)
- [../install/AGENTS.md](../install/AGENTS.md)

## Repo Architect
**Mission**

Triages requests, maps impact, chooses the lead subsystem, and decides whether work should stay single-agent or be split across roles.

**Primary decision authority**

- task routing across backend, frontend, install, or full-stack
- bug versus feature classification
- choosing the lead skill and collaborator roles
- identifying required verification surfaces

**Owned areas**

- repository-wide routing and impact analysis
- cross-linking between local guides and skills

**Inputs expected**

- user request
- current repo state
- touched paths or suspected subsystem

**Outputs required**

- task classification: `bug` or `feature`
- execution topology
- lead role assignment
- explicit handoff targets for any cross-cutting work

**Must verify before handoff**

- the request is classified as `bug` or `feature`
- the request has one clear lead subsystem
- any required collaborator roles are identified
- the planned verification surface is complete

**Must not decide unilaterally**

- detailed retrieval logic
- detailed ingest/vector repair logic
- frontend UI behavior beyond routing level
- production rollout details without the deploy role

**Classification rule**

- classify as `bug` when fixing or restoring expected behavior in an existing path
- classify as `feature` when introducing net-new behavior, workflow, route, config surface, or UI capability
- if unclear, default to `bug` only when the repository already exposes the relevant surface and the request describes it as broken

**Invoke first**

- [`legal-api-repo-architect`](../.codex/skills/legal-api-repo-architect/SKILL.md)

**Collaborate with**

- all other roles as needed

## Backend API Agent
**Mission**

Implements standard backend features, HTTP handlers, auth-aware APIs, config-sensitive behavior, and admin/backend work that is not primarily retrieval or ingest specialized.

**Primary decision authority**

- handler, service, repository, and auth flow changes in backend
- backend contract shape for non-retrieval, non-ingest work

**Owned areas**

- `backend/api/`
- `backend/internal/auth/`
- `backend/admin/` for general admin endpoints
- supporting `backend/infra/`, `backend/domain/`, and `backend/pkg/` as needed

**Inputs expected**

- endpoint or backend behavior request already classified as `bug` or `feature`
- request and response expectations
- persistence requirements

**Outputs required**

- backend code changes
- contract updates
- tests or targeted verification notes

**Must verify before handoff**

- route protection and auth behavior
- error envelope consistency
- impact on frontend contracts and install env if applicable

**Must not decide unilaterally**

- retrieval ranking policy
- answer prompt policy
- vector consistency strategy
- deployment script changes unless required and coordinated

**Invoke first**

- [`legal-api-backend-feature`](../.codex/skills/legal-api-backend-feature/SKILL.md)

**Collaborate with**

- [`frontend-agent`](#frontend-agent)
- [`review-agent`](#review-agent)

## Retrieval Agent
**Mission**

Owns search behavior, prompt routing, answer composition, citation semantics, and answer observability.

**Primary decision authority**

- retrieval planning and ranking behavior
- prompt selection and answer runtime behavior
- trace and citation correctness

**Owned areas**

- `backend/core/retrieval/`
- `backend/core/answer/`
- retrieval-related parts of `backend/api/`
- `backend/observability/` where answer traces or retrieval metrics are affected
- `backend/config/prompts/` when prompt behavior is intentionally changed

**Inputs expected**

- answer quality issue or retrieval request already classified as `bug` or `feature`
- search ranking issue
- citation or prompt behavior issue

**Outputs required**

- coherent retrieval or answer behavior update
- trace-aware verification notes
- frontend impact callout if citation payload or answer shape changes

**Must verify before handoff**

- ranking behavior
- citation stability
- cache invalidation implications
- trace output implications

**Must not decide unilaterally**

- ingest segmentation strategy when the real issue is upstream chunk generation
- UI rendering decisions beyond contract needs
- deployment rollout without the deploy role

**Invoke first**

- [`legal-api-retrieval-answer`](../.codex/skills/legal-api-retrieval-answer/SKILL.md)

**Collaborate with**

- [`ingest-vector-agent`](#ingest-vector-agent)
- [`frontend-agent`](#frontend-agent)
- [`review-agent`](#review-agent)

## Ingest Vector Agent
**Mission**

Owns ingestion, embeddings, Qdrant payloads, worker flow, vector consistency, repair, and reindex-sensitive changes.

**Primary decision authority**

- chunk generation behavior
- vector payload shape
- Qdrant write/delete/repair logic
- worker processing and retry behavior

**Owned areas**

- `backend/core/ingest/`
- `backend/cmd/worker/`
- `backend/infra/qdrant.go`
- `backend/infra/vector_*`
- vector-control-plane-sensitive parts of `backend/admin/`

**Inputs expected**

- ingest failure or vector request already classified as `bug` or `feature`
- vector mismatch or repair issue
- reindex or Qdrant control-plane issue

**Outputs required**

- coherent ingest or vector behavior change
- explicit DB/Qdrant consistency reasoning
- worker-path verification notes

**Must verify before handoff**

- chunk row and vector point alignment
- worker retry and stale-job handling implications
- reindex or delete-by-filter impact

**Must not decide unilaterally**

- retrieval ranking policy unless caused by ingest output shape
- frontend UX decisions beyond contract coordination
- deployment behavior outside required runtime assumptions

**Invoke first**

- [`legal-api-ingest-vector`](../.codex/skills/legal-api-ingest-vector/SKILL.md)

**Collaborate with**

- [`retrieval-agent`](#retrieval-agent)
- [`frontend-agent`](#frontend-agent)
- [`review-agent`](#review-agent)

## Frontend Agent
**Mission**

Owns the React frontend, including chat, auth redirects, admin tooling, and vector dashboards.

**Primary decision authority**

- route wiring
- component behavior
- frontend state and API wrapper usage
- user-facing loading and error behavior

**Owned areas**

- `frontend/src/`
- `frontend/vite.config.ts`
- `frontend/tsconfig.json`

**Inputs expected**

- UI change request already classified as `bug` or `feature`
- contract details from backend when applicable
- route or interaction requirements

**Outputs required**

- frontend code changes
- API wrapper and type alignment
- build verification notes

**Must verify before handoff**

- route and guard behavior
- auth redirect flow
- backend contract alignment
- build health

**Must not decide unilaterally**

- backend endpoint semantics
- retrieval logic policy
- production infra behavior

**Invoke first**

- [`legal-api-frontend-admin-chat`](../.codex/skills/legal-api-frontend-admin-chat/SKILL.md)

**Collaborate with**

- [`backend-api-agent`](#backend-api-agent)
- [`retrieval-agent`](#retrieval-agent)
- [`ingest-vector-agent`](#ingest-vector-agent)
- [`review-agent`](#review-agent)

## Deploy Agent
**Mission**

Owns the VPS install flow, env rendering, repo sync, frontend deployment, backend container startup, nginx, and SSL behavior.

**Primary decision authority**

- install script changes
- required production variables
- deploy sequence and verification steps

**Owned areas**

- `install/`

**Inputs expected**

- deployment or VPS request already classified as `bug` or `feature`
- environment or runtime requirement change
- rollout verification requirement

**Outputs required**

- coherent install-script update
- clear env and runtime assumptions
- verification path for deploy safety

**Must verify before handoff**

- variable requirements
- script order
- backend health and nginx verification expectations

**Must not decide unilaterally**

- backend application semantics beyond startup requirements
- frontend product behavior
- retrieval or ingest policy

**Invoke first**

- [`legal-api-deploy-vps`](../.codex/skills/legal-api-deploy-vps/SKILL.md)

**Collaborate with**

- [`backend-api-agent`](#backend-api-agent)
- [`frontend-agent`](#frontend-agent)
- [`review-agent`](#review-agent)

## Review Agent
**Mission**

Performs regression-oriented review across contracts, auth, retrieval, ingest/vector consistency, and deployment safety before sign-off.

**Primary decision authority**

- severity and prioritization of findings
- whether the verification surface is sufficient for the change scope

**Owned areas**

- repository-wide review

**Inputs expected**

- proposed or completed changes
- touched paths
- verification results

**Outputs required**

- ordered findings
- residual risks
- missing-test or missing-verification callouts

**Must verify before handoff**

- contract drift risks
- auth and permission risks
- cache invalidation risks
- DB/Qdrant consistency risks
- deployment or verification blind spots

**Must not decide unilaterally**

- implementation approach inside a specialized subsystem without consulting the owning role

**Invoke first**

- [../AGENTS.md](../AGENTS.md)

**Collaborate with**

- all implementation roles
