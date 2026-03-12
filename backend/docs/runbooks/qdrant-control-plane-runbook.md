# Qdrant Control Plane Runbook

Scope: operational usage of `/admin/qdrant/*` in production.

## Preconditions
- Use admin credentials (`X-Admin-Key` or Bearer token).
- Set `X-Admin-Actor` to a traceable identity (`oncall-email`, `incident-id`).
- Capture request/response payloads in incident notes.

## 1) Inspect collections
1. Call `GET /admin/qdrant/collections`.
2. Check `collections[].validation.passed` and `vector_dimension`.
3. If a target collection is suspicious, call `GET /admin/qdrant/collections/{name}`.
4. If `status=not_found`, confirm environment/collection config before any repair action.

## 2) Debug retrieval with search_debug
1. Call `POST /admin/qdrant/search_debug` with `query_text` and bounded `top_k` (start with 5-10).
2. Add `metadata_filters` to reproduce production retrieval constraints.
3. Check `hits[].chunk.chunk_id` and `hits[].chunk.document_version_id` for chunk resolution.
4. If hits are empty or low-quality, verify query embedding health and filter correctness before reindex.

## 3) Interpret vector_health output
1. Start with `GET /admin/qdrant/vector_health?mode=quick`.
2. Review `missing_vectors_count`, `orphan_vectors_count`, `bounded`, and `repairable_issues_detected`.
3. If quick mode shows issues, run scoped full mode with explicit limits:
   - `mode=full`
   - controlled `max_vectors_scanned`, `max_chunks`, `max_scan_duration_ms`
4. Treat `dimension_mismatch_detected=true` as a high-priority configuration mismatch.

## 4) Safely use delete_by_filter
1. Always run `POST /admin/qdrant/delete_by_filter` with `dry_run=true` first.
2. Ensure `filter_summary` and `estimated_scope` are expected.
3. Use only whitelisted fields (`document_id`, `document_version_id`, `chunk_id`).
4. Execute with `confirm=true` only after operator review.
5. Record `reason` in request payload for auditability.

## 5) Trigger reindex_document
1. Use `POST /admin/qdrant/reindex_document` with either `document_id` or `document_version_id` (never both).
2. Verify `status=accepted` and `items[].job_id`.
3. Track ingest job completion in ingest job admin APIs/logs.

## 6) Trigger reindex_all safely
1. Confirm incident scope and define reason.
2. Call `POST /admin/qdrant/reindex_all` with:
   - `confirm=true`
   - non-empty `reason`
   - optional narrowing via `doc_type_code`, `status`, and bounded `limit`
3. Verify `accepted_count`, `created_count`, and `skipped_count`.
4. Avoid repeated triggers within short windows.

## 7) When orphan vectors are detected
1. Confirm with quick scan, then bounded full scan.
2. Identify sample point IDs from `samples`.
3. Validate whether corresponding chunk IDs still exist in Postgres.
4. If stale data is confirmed, run targeted `delete_by_filter` dry run then confirm.
5. Re-run quick health scan to verify count reduction.

## 8) When missing vectors are detected
1. Confirm with quick scan and bounded full scan.
2. Reindex targeted versions via `reindex_document` first.
3. Use `reindex_all` only when issue spans broad scope.
4. Re-run health scan and track `missing_vectors_count` trend.

## 9) When health scan exceeds safety limits
1. If response indicates bounded/duration-limited scans, keep scan bounded.
2. Reduce `batch_size` and `max_vectors_scanned`.
3. Split scans by incident windows instead of one large full pass.
4. Investigate Qdrant latency and database pressure before increasing limits.

## 10) When admin rate limiting triggers unexpectedly
1. Check caller identity (`X-Admin-Actor`, API key sharing patterns).
2. Confirm no automation loop is re-triggering endpoints.
3. Stagger repeated operations (especially `delete_by_filter`, `reindex_document`, `reindex_all`).
4. If legitimate burst is required, coordinate a temporary operational procedure rather than bypassing guardrails.
