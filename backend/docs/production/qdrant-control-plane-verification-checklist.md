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
