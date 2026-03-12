# Admin Qdrant Control Plane API Contract (Phase 2.6 Lockdown)

Base path: `/admin/qdrant`

Auth: admin routes require `X-Admin-Key: <ADMIN_API_KEY>` or `Authorization: Bearer <ADMIN_API_KEY>` when admin key is configured.

Error envelope (all endpoints):

```json
{
  "error": {
    "code": "validation|invalid_request|unauthorized|rate_limited|qdrant_error|db_error",
    "message": "human readable",
    "details": {}
  }
}
```

Response contract conventions:
- Success responses include `status` and `summary`.
- `status` values are stable and endpoint-specific (`ok`, `accepted`, `not_found`).
- Internal raw errors are not returned to clients.

## GET `/collections`

Purpose: list all Qdrant collections with validation summary.

Response `200`:
- `status: "ok"`
- `summary: string`
- `collections: QdrantCollectionSummary[]`

`QdrantCollectionSummary`:
- `collection_name` string required
- `status` string required
- `points_count` int64 optional
- `vector_count` int64 optional
- `indexed_vectors_count` int64 optional
- `vector_dimension` int optional
- `distance_metric` string optional
- `validation` object required
- `payload_schema_summary` array optional

`validation`:
- `available` bool required
- `expected_dimension` int optional
- `passed` bool optional
- `message` string optional

## GET `/collections/{name}`

Purpose: inspect a single collection.

Path params:
- `name` required, regex: `^[A-Za-z0-9_-]{1,128}$`

Responses:
- `200` with `status="ok"`, `found=true`, and `collection`
- `200` with `status="not_found"`, `found=false` when collection does not exist
- `400` validation on invalid `name`

## POST `/search_debug`

Purpose: inspect retrieval candidates for a query.

Request:
- `query_text` string required, trimmed non-empty, max 2000 chars
- `top_k` int optional, default 10, bounds 1..50
- `metadata_filters` optional object:
  - `legal_domain[]`, `document_type[]`, `effective_status[]`, `document_number[]`, `article_number[]`
  - each list max 20 values, each value max 256 chars
- `collection` optional string (same name regex)
- `include_payload` bool optional
- `include_chunk_preview` bool optional

Response `200`:
- `status: "ok"`
- `summary` string
- `query_hash` string
- `top_k` int
- `filter_summary` string
- `collection` string
- `duration_ms` int64
- `hit_count` int
- `hits[]`

`hits[]` item:
- `rank` int
- `point_id` string
- `score` number
- `payload` object optional
- `chunk` object optional (`chunk_id`, `document_version_id`, `chunk_index`, `preview`, `citation`)

Validation errors:
- invalid/missing query
- invalid `top_k`
- invalid filter payload
- invalid collection name

## GET `/vector_health`

Purpose: bounded vector consistency diagnostics.

Query params:
- `mode`: `quick` (default) or `full`
- `full_scan=true` also forces full mode (compat)
- `batch_size` int optional, default 256, clamped to max 2000
- `chunk_batch_size` int optional, defaults to `batch_size`, clamped to max 2000
- `vector_batch_size` int optional, defaults to `batch_size`, clamped to max 2000
- `max_vectors_scanned` (or `max_vectors`) int optional
  - quick default 1000
  - full default 50000
- `max_chunks` int optional, defaults to `max_vectors_scanned`
- `max_scan_duration_ms` int optional
  - quick default 8000
  - full default 30000
  - absolute max clamp 120000

Response `200`:
- `status: "ok"`
- `summary` string
- `scan_mode` string
- `scanned_batches` int
- `scanned_vectors` int
- `scanned_chunks` int
- `duration_ms` int64
- `bounded` bool
- `orphan_vectors_count` int
- `missing_vectors_count` int
- `chunk_vector_count_mismatch` bool
- `dimension_mismatch_detected` bool
- `repairable_issues_detected` bool
- `repair_recommendation` string
- `samples[]` optional

## POST `/delete_by_filter`

Purpose: safe destructive vector operation with explicit guardrails.

Request:
- `collection` optional string (default service collection)
- `filter` required, must be non-empty and whitelisted
- `dry_run` bool
- `confirm` bool
- `reason` string optional (logged)

Semantics:
- exactly one of `dry_run=true` or `confirm=true` is required
- `dry_run=true`: estimate only, no delete execution
- `confirm=true`: executes delete
- empty filter rejected
- non-whitelisted fields rejected

Response `200`:
- `status: "ok"`
- `summary`: `"dry run completed"` or `"delete executed"`
- `collection` string
- `dry_run` bool
- `confirmed` bool
- `filter_summary` string
- `estimated_scope` int64 optional
- `scope_estimated` bool

## POST `/reindex_document`

Purpose: enqueue reindex for one document scope.

Request:
- exactly one of:
  - `document_id` string
  - `document_version_id` string
- `force` bool accepted for compatibility
- `reason` string optional (logged)

Response `202`:
- `status: "accepted"`
- `summary` string
- `accepted_count` int
- `created_count` int
- `skipped_count` int
- `items[]` optional:
  - `document_version_id`
  - `job_id`
  - `job_status`
  - `created`

Validation errors:
- missing scope
- both scope fields set
- nonexistent target

## POST `/reindex_all`

Purpose: enqueue bulk reindex with safety constraints.

Request:
- `confirm` bool required `true`
- `reason` string required, non-empty
- `doc_type_code` optional
- `collection` optional, must match configured collection when provided
- `status` optional enum:
  - `queued`, `pending`, `processing`, `done`, `failed`, `never_ingested`, or empty
- `limit` optional int, default 500, max 2000
- `force` bool accepted for compatibility

Response `202`:
- `status: "accepted"`
- `summary` string
- `scope` object (`doc_type_code`, `collection`, `status`) when provided
- `accepted_count` int
- `created_count` int
- `skipped_count` int

Validation errors:
- `confirm` false
- missing `reason`
- invalid `status`
- invalid/mismatched `collection`

## Rate limits

Per-admin-identity (`X-Admin-Actor`, fallback key/token/IP) route limits:
- `POST /search_debug`: 5 req / 1s
- `GET /vector_health`: 2 req / 1s
- `POST /delete_by_filter`: 1 req / 1s
- `POST /reindex_document`: 1 req / 10s
- `POST /reindex_all`: 1 req / 10s

Rate-limited response: `429` with `error.code="rate_limited"` and `Retry-After` header.
