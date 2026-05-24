# Frontend AGENTS

## Purpose And Scope
This guide covers the Vite + React + TypeScript frontend in `frontend/`. It applies to:
- chat experience
- auth and redirect behavior
- admin and tuning pages
- vector admin tooling
- frontend API wrappers and shared types

Read the repo entrypoint first: [../AGENTS.md](../AGENTS.md)

## Area Ownership
Primary owned directories:
- `src/app/`
- `src/core/`
- `src/features/`
- `src/shared/`
- `src/playground/`
- `vite.config.ts`
- `tsconfig.json`

Do not casually edit:
- `src/core/api.ts` without checking backend contract expectations
- `src/playground/apiClient.js` without checking auth refresh, redirect, and token storage behavior
- route structure in `src/app/App.tsx` without considering auth guards and navigation flow

## Main Entrypoints And Critical Files
- Frontend entrypoint: `src/main.tsx`
- Route shell: `src/app/App.tsx`
- Navigation: `src/app/Navbar.tsx`
- Shared API layer: `src/core/api.ts`
- Shared types: `src/core/types.ts`
- Auth-aware Axios client: `src/playground/apiClient.js`
- Chat feature: `src/features/chat/`
- Admin auth and layout: `src/features/admin/`
- AI tuning pages: `src/features/admin/ai/`
- Vector admin pages: `src/features/admin/vectors/`

## Architecture Notes
- `src/app/` defines shell layout and route wiring.
- `src/core/` owns API wrappers, SSE helpers, utility helpers, and shared types.
- `src/features/chat/` owns conversations, message flow, citations, filters, and rendering.
- `src/features/admin/` owns doc types, documents, ingest jobs, auth guards, and tuning surfaces.
- `src/features/admin/ai/` owns guard policy, prompt, and retrieval config UIs.
- `src/features/admin/vectors/` owns collection dashboards, search debug, vector health, delete-by-filter, and reindex controls.
- Alias `@/*` points to `src/*`.

## Runtime Invariants
- `VITE_API_BASE_URL` controls the backend target.
- Local runtime is the root Docker Compose flow (`make up` or `./install/local-compose.sh up`), not the Vite dev server.
- Production frontend assets are built inside `frontend/Dockerfile.prod` and served by Nginx in the compose `web` container.
- The auth token is stored in local storage under `auth_token`.
- Axios interceptors handle auth token injection, refresh flow, and redirect behavior.
- Admin operations rely on backend support; do not fabricate frontend-only admin state that the backend cannot persist.
- Chat and admin screens share the same API base and auth behavior.
- SSE and streaming interactions must preserve current request and response expectations from backend endpoints.

## Required Commands For Verification
- From repo root, run local stack: `make up`
- From repo root, tail local stack logs: `make log`
- Install dependencies: `cd frontend && bun install`
- Build production bundle: `cd frontend && bun run build`

Use `bun run dev` or `bun run preview` only for isolated frontend debugging. They are not the standard local runtime path.

## Common Failure Modes
- Updating backend response shapes without updating `src/core/types.ts` and `src/core/api.ts`
- Breaking login refresh behavior by bypassing `src/playground/apiClient.js`
- Changing routes without updating guards or navigation links
- Adding admin UI for backend capabilities that are not actually routed or authorized
- Breaking vector admin pages by changing backend query params or response names without frontend sync

## When To Involve Another Agent Or Skill
- Use [`legal-api-frontend-admin-chat`](../.agents/skills/legal-api-frontend-admin-chat/SKILL.md) for nearly all frontend tasks in this repo.
- Involve [`legal-api-backend-feature`](../.agents/skills/legal-api-backend-feature/SKILL.md) when a UI request needs a backend endpoint or contract change.
- Involve [`legal-api-retrieval-answer`](../.agents/skills/legal-api-retrieval-answer/SKILL.md) for answer quality, citations, prompt behavior, or search result semantics shown in UI.
- Involve [`legal-api-ingest-vector`](../.agents/skills/legal-api-ingest-vector/SKILL.md) for vector dashboard, reindex, delete-by-filter, or ingest-driven UI changes.
- Involve [`review-agent`](../docs/agent-roles.md#review-agent) before sign-off on full-stack, auth, or streaming-sensitive changes.

## Definition Of Done
- Chat task: conversation flow, message rendering, citations, filters, and API calls remain coherent.
- Admin task: auth guard, form state, backend contract usage, and navigation flow are correct.
- Vector tooling task: request shapes, loading/error states, and backend integration are aligned with actual control-plane APIs.
