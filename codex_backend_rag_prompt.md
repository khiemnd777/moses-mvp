# Codex Prompt – Generate Backend Go RAG Platform

## Purpose
Generate a complete backend project for a Legal RAG platform with Dynamic Vectorization forms.

---

## Prompt

You are a senior Go engineer. Generate a complete backend project for a Legal RAG platform with Dynamic Vectorization forms. The backend includes:
- HTTP API server (Fiber)
- Worker service for ingestion/re-index jobs
- PostgreSQL persistence (database/sql)
- Qdrant vector database integration
- OpenAI Embedding API integration
- OpenAI GPT-4.1 mini for answer generation
- YAML-based configuration and prompt system
- DocType Form schema: segment rules + metadata schema + mapping rules
- Deterministic hashing for content and form config to trigger re-index

### Constraints / Requirements
1) Language: Go  
2) Framework: Fiber  
3) DB: PostgreSQL using database/sql (no ORM)  
4) VectorDB: Qdrant via HTTP API  
5) Embedding: OpenAI Embeddings API  
6) LLM: OpenAI GPT-4.1 mini  
7) Dockerfiles + docker-compose (api, worker, postgres, qdrant)  
8) SQL migrations  
9) Structured logging (English)  
10) Modular architecture: core / domain / infra / api  
11) Must run locally  
12) Provide Makefile  

---

## Repository Layout

```
backend/
├── cmd/
│   ├── api/main.go
│   └── worker/main.go
├── config/
│   ├── config.yaml
│   └── prompts/
│       ├── guard.yaml
│       ├── tone_default.yaml
│       ├── tone_academic.yaml
│       └── tone_procedure.yaml
├── core/
│   ├── schema/doc_type_form.go
│   ├── ingest/
│   ├── embedding/
│   ├── retrieval/
│   └── answer/
├── domain/
├── infra/
├── api/
├── pkg/
├── docker/
├── Makefile
└── go.mod
```

---

## Core Concepts

### A) DocType Form Schema
- version
- doc_type (code, name)
- segment_rules (strategy, hierarchy, normalization)
- metadata_schema (fields, types)
- mapping_rules (extractors)
- reindex_policy

Include:
- Validate()
- Hash() using sha256 canonical JSON

### B) PostgreSQL Schema
Tables:
- doc_types
- documents
- document_assets
- document_versions
- chunks
- ingest_jobs
- query_logs
- answer_logs

Include proper indexes.

### C) Ingestion Pipeline
Steps:
1. Load asset
2. Normalize text
3. Segment text
4. Extract metadata
5. Embed chunks
6. Upsert Qdrant
7. Persist chunks
8. Update job status

Segmenters:
- legal_article
- judgement_structure
- free_text

### D) Retrieval + Answer
Endpoints:
- POST /search
- POST /answer

Require citations with deterministic citation_id.

### E) Worker Queue
Simple polling worker:
- queued → running → done/failed

---

## HTTP API

- POST /doc-types
- GET /doc-types
- PUT /doc-types/:id/form
- POST /documents
- POST /documents/:id/assets
- POST /documents/:id/versions
- POST /document-versions/:id/ingest
- POST /search
- POST /answer

Error envelope:
```
{ "error": { "code": "...", "message": "...", "details": ... } }
```

---

## Config (config.yaml)
- server
- postgres
- qdrant
- openai
- storage
- ingest

---

## Prompts
- Guard prompt: no hallucination, citations required
- Tone prompts: style control

---

## Deliverables
- Full source code
- SQL migrations
- Docker setup
- Seed doc_type (legal_normative)
- Minimal unit tests

Generate all files. Ensure the code compiles and runs locally.
