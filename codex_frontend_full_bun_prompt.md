# Codex Prompt – Full Frontend (Vite + React + TypeScript, Bun)

## Scope
Generate a complete **ViteJS + React + TypeScript** frontend using **Bun**, including:
- ChatGPT-like Chat UI (You–Me style)
- Streaming chat (SSE)
- Admin UI (DocTypes, Dynamic Forms, Documents, Ingest Jobs)
- Citations, Sources panel, Markdown rendering
- Connect to Legal RAG Backend API

---

## Prompt

You are a senior frontend engineer. Generate a complete **ViteJS + React + TypeScript** SPA for a Legal RAG platform, using **Bun** (not npm/yarn). The app has two main areas:

1. **Chat UI** – ChatGPT-like experience with streaming assistant responses  
2. **Admin UI** – Manage Doc Types (Dynamic Vectorization Forms), Documents, Ingest Jobs, and QA Playground

The frontend connects to an existing Legal API backend.

---

## Hard Constraints
1. Package manager/runtime: **Bun only**
2. Framework: Vite + React + TypeScript
3. Allowed deps: axios, react-router-dom, zustand, react-markdown, remark-gfm
4. Styling: plain CSS or CSS modules
5. Must run with:
   - `bun install`
   - `bun run dev`
6. Must compile with `bun run build`
7. Provide `.env.example`, `README.md`, `bun.lockb`

---

## Backend API Assumptions

### Base URL
Configured via:
```
VITE_API_BASE_URL
```

### Answer (non-stream)
`POST /answer`

### Answer (stream – SSE)
`POST /answer/stream`
- event: token
- event: citations
- event: done
- event: error

Fallback to non-stream if unavailable.

### Admin APIs
- Doc types CRUD
- Documents CRUD
- Upload assets
- Ingest jobs enqueue & status

Backend error envelope:
```json
{ "error": { "code": "...", "message": "...", "details": ... } }
```

---

## Chat UI Requirements
- ChatGPT-like layout
- Assistant left, user right
- Streaming assistant message
- Stop streaming button
- Retry last message
- Markdown rendering
- Clickable inline citations
- Collapsible Sources panel
- Filters: tone, topK, effective status, domain, doc type
- Persist chat state & filters in localStorage

---

## Admin UI Requirements

### Doc Types
- List
- Create
- Edit Dynamic Form:
  - Segment Rules Editor
  - Metadata Schema Editor
  - Mapping Rules Editor
  - Raw JSON/YAML editor
  - Canonical JSON hash preview

### Documents
- List
- Create
- Upload asset
- Create version
- Enqueue ingest job

### Ingest Jobs
- Poll status
- Show progress & errors

### Playground
- Run /search
- Run /answer

---

## Project Structure

```
frontend/
├── index.html
├── vite.config.ts
├── package.json
├── bun.lockb
├── tsconfig.json
├── .env.example
├── README.md
└── src/
    ├── main.tsx
    ├── app/
    ├── core/
    ├── features/
    │   ├── chat/
    │   └── admin/
    └── shared/
```

(Generate all subfolders and files as specified in the prompt.)

---

## Streaming Implementation
- Use fetch + AbortController
- Parse SSE events manually
- Append tokens live
- Attach citations on completion

---

## State Management
Use Zustand.

### Chat store
- messages
- filters
- streaming state
- actions: sendMessage, stopStreaming, retryLast, resetChat

### Admin store
- selected doc type
- last opened document

---

## Utilities
- Canonical JSON stringify
- SHA256 hash via WebCrypto
- SSE client abstraction

---

## Scripts
package.json must include:
- dev
- build
- preview

All runnable via:
```
bun run <script>
```

---

## Deliverables
- Full source code
- All components, stores, utils
- Clean TypeScript types
- Working Bun-based setup

Generate all files. Do not omit files. Ensure the project runs correctly with Bun.
