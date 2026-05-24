---
name: legal-api-regression-review
description: Regression-oriented review for Legal API changes across contracts, auth, retrieval, ingest/vector, frontend, and deploy safety. Use when reviewing completed or proposed changes, before sign-off on high-risk work, or when acting as legal-api-reviewer.
---

# legal-api-regression-review

## When To Use This Skill
Use this skill for review-only passes over proposed or completed Legal API changes.

Preferred lead role: [`review-agent`](../../../docs/agent-roles.md#review-agent)

Supporting skill: [`legal-api-engineering-guardrails`](../legal-api-engineering-guardrails/SKILL.md)

## Review Stance
Lead with bugs, risks, regressions, and missing verification. Summaries are secondary.

Do not rewrite implementation unless explicitly assigned a separate bounded fix. A review should make the next action obvious: fix now, verify more, or accept the residual risk.

## Inputs To Gather
- current user request and task classification
- `git diff` or touched files
- owning subsystem skill and role
- tests, builds, compose checks, browser checks, or manual verification already run
- related contracts between backend, frontend, worker, install, Postgres, and Qdrant

## Review Workflow
1. Read [../../../AGENTS.md](../../../AGENTS.md), [../../../docs/agent-roles.md](../../../docs/agent-roles.md), and the owning area guide for touched paths.
2. Map each touched file to an owner:
   - backend API/auth/admin
   - retrieval/answer/citations/traces
   - ingest/worker/vector/Qdrant
   - frontend chat/admin/vector UI
   - install/deploy/local compose
3. Check for contract drift before style concerns.
4. Check whether the verification signal actually exercises the behavior that changed.
5. Prefer concrete findings with file and line references. Avoid speculative findings unless the risk is material and testable.

## Risk Checklist
- Backend contracts: request parsing, response envelopes, auth middleware, migrations, config rendering, API and worker shared-code impact.
- Frontend contracts: `frontend/src/core/api.ts`, shared types, route guards, loading and error states, auth refresh redirects.
- Retrieval and answers: ranking changes, prompt/config cache invalidation, streaming/non-streaming parity, citation payload stability, trace expectations.
- Ingest and vectors: Postgres chunk rows versus Qdrant points, stale vector cleanup, worker retry and repair behavior, payload field compatibility.
- Deploy: required vars, rendered env files, Docker Compose service wiring, published ports, container Nginx, SSL, verify script coverage.
- Tests: regression path for bugs, happy path and adjacent existing flows for features, missing correct seam when no test is possible.

## Output Format
Start with ordered findings.

For each finding include:
- severity: `P0`, `P1`, `P2`, or `P3`
- file and line
- observed risk
- why it matters
- the smallest credible fix or verification step

If there are no findings, say so directly and list any residual verification gap.
