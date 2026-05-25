# Phase 2.6 Production Verification Checklist (Qdrant Control Plane)

Use this checklist for pre-Phase-3 sign-off.

- [ ] Admin auth verified on all `/admin/qdrant/*` routes (`401` on missing/invalid credentials).
- [ ] Non-admin rejection behavior validated (error envelope stable).
- [ ] Request validation verified for malformed JSON and invalid payload shapes.
- [ ] `dry_run`/`confirm` semantics verified for `delete_by_filter`.
- [ ] `reindex_document` scope guardrails verified.
- [ ] `reindex_all` confirm/reason/scope guardrails verified.
- [ ] Rate limiting verified on mutating and scan endpoints (`429` + `Retry-After`).
- [ ] `vector_health` bounded scan semantics verified (mode, limits, clamps).
- [ ] Stable response contract fields verified (`status`, `summary`, core payload keys).
- [ ] Error shape and code taxonomy documented and stable.
- [ ] Prometheus metrics exposed and scrape-verified on `/metrics`.
- [ ] Alert rules configured for orphan/missing/errors/latency/reindex/delete failures.
- [ ] Operational runbook reviewed by on-call.
- [ ] Audit logs for admin operations are usable (operation, actor, route, result).
- [ ] Endpoint and integration tests pass in CI (`go test ./...`).

## Production RAG Gate Addendum

- [ ] OpenAI key rotated if any committed or shared key was exposed.
- [ ] `install/backend/install.sh` rejects placeholder `OPENAI_API_KEY` before rendering production `backend/.env`.
- [ ] `install/backend/verify_rag.sh` passes on the VPS after deploy and reindex.
- [ ] Payload indexes verified for `legal_domain`, `document_type`, `effective_status`, `document_number`, `article_number`, `issuing_authority`, and `signed_year`.
- [ ] Sample `search_debug` query returns chunk-backed hits with routing payload metadata.
- [ ] Upload proxy limit (`CLIENT_MAX_BODY_SIZE`) is explicitly reviewed. Backend MIME/type enforcement is not a deploy-time control today.
- [ ] Rollout/rollback checklist reviewed in `backend/docs/production/rag-production-rollout-runbook.md`.
