# Production RAG Rollout And Rollback Runbook

Scope: deploy/ops gates for production-grade RAG on the VPS Docker Compose runtime. This runbook does not change backend retrieval semantics; backend-owned slices must implement any application feature flag or retrieval behavior change.

## P0 Secret Response

- [ ] Rotate the OpenAI key that was previously present in `install/config.sh`.
- [ ] Confirm the old key is disabled in the OpenAI project and review recent usage for unexpected calls.
- [ ] Check git history and remote repository visibility for any commit that contained the key.
- [ ] Keep the replacement key only in the operator-managed production secret path or operator environment. Do not commit it.
- [ ] Run a secret scan before release:

```bash
rg --no-ignore -n "sk-[A-Za-z0-9_-]{20,}" install backend/docs
```

## Rollout Preconditions

- [ ] `install/config.sh` exists on the VPS with a real `OPENAI_API_KEY`, or the deploy runtime provides `OPENAI_API_KEY`; placeholder values are rejected by `install/backend/install.sh`.
- [ ] `OPENAI_EMBEDDINGS_MODEL` matches the Qdrant collection dimension. `text-embedding-3-small` expects 1536 dimensions.
- [ ] `QDRANT_COLLECTION` points to the production collection intended for the rollout.
- [ ] `CLIENT_MAX_BODY_SIZE` is set deliberately for the web container. The default sample value is `50m`, aligned with the backend upload body limit.
- [ ] `UPLOAD_AV_SCAN_MODE=clamd` and `UPLOAD_AV_FAIL_CLOSED=true` are set for production. `install/backend/install.sh` rejects disabled/fail-open AV settings unless a break-glass override is explicitly set.
- [ ] The VPS has enough memory for the `clamav` container and its signature reloads; plan capacity before enabling large uploads.
- [ ] Upload type policy is understood: the backend accepts `.pdf`, `.docx`, `.pptx`, `.txt`, and `.doc`, then validates file signatures before malware scanning and storage.
- [ ] The target release tag or commit SHA is known.
- [ ] Operators have admin credentials for verification.

## V1/V2 Feature Flag Concept

Use a two-lane rollout model for RAG behavior:

- `V1`: current production retrieval behavior and corpus.
- `V2`: candidate retrieval behavior, prompt/config profile, or collection/corpus strategy.

Until backend application code exposes a real runtime flag, treat V1/V2 as an operational release concept:

- V1 remains the rollback target.
- V2 deploys behind a release tag or isolated config change.
- Do not delete or rewrite V1 corpus data during V2 rollout.
- If V2 needs a different embedding model or incompatible vector dimension, use a separate Qdrant collection instead of mutating the V1 collection in place.

When an app-owned feature flag exists, the expected operator controls are:

- default to `RAG_RETRIEVAL_VERSION=v1`
- deploy V2 code/config with `v1` still active
- reindex or warm V2
- run V2 verification
- switch to `v2`
- keep the `v1` switch path valid until production confidence is established

## Rollout Steps

1. Render and start production:

```bash
cd /opt/legal_api/app
./install/install.sh
```

2. Verify shell and compose surfaces if running manually:

```bash
cd /opt/legal_api/app
bash -n install/install.sh install/backend/install.sh install/backend/verify.sh install/backend/verify_rag.sh install/nginx/install.sh
docker compose --env-file backend/.env -f install/docker-compose.prod.yml config --quiet
```

3. Run the production RAG gate:

```bash
cd /opt/legal_api/app/install
./backend/verify_rag.sh
```

If the admin password has changed after bootstrap, set `RAG_VERIFY_AUTH_PASSWORD` for this command.

4. Confirm Qdrant collection readiness:

- [ ] collection exists
- [ ] vector dimension validation passes
- [ ] payload indexes exist for `legal_domain`, `document_type`, `effective_status`, `document_number`, `article_number`, `issuing_authority`, and `signed_year`
- [ ] `points_count` / `vector_count` trend matches expected corpus size

5. Confirm vector health:

- [ ] `dimension_mismatch_detected=false`
- [ ] `chunk_vector_count_mismatch=false`
- [ ] `orphan_vectors_count=0`
- [ ] `missing_vectors_count=0`
- [ ] no unexpected bounded scan limit prevents the quick gate from being meaningful

6. Confirm sample retrieval:

- [ ] `POST /admin/qdrant/search_debug` returns at least one hit for the rollout query
- [ ] hits include chunk metadata (`chunk_id`, `document_version_id`)
- [ ] payload contains routing metadata such as `legal_domain`
- [ ] answer-level smoke test is reviewed separately by the retrieval/answer owner

7. Confirm upload malware scanning:

- [ ] `UPLOAD_AV_SCAN_MODE=clamd`
- [ ] `UPLOAD_AV_FAIL_CLOSED=true`
- [ ] `./backend/verify_rag.sh` rejects the EICAR upload probe with `malware_detected`
- [ ] a scanner outage returns `malware_scan_unavailable` and does not create a stored upload

## Reindex Verification

Use targeted reindex first. Use `reindex_all` only for broad corpus drift, embedding-model changes, or planned V2 corpus rebuilds.

Before reindex:

- [ ] capture `GET /admin/qdrant/collections/{QDRANT_COLLECTION}`
- [ ] capture `GET /admin/qdrant/vector_health?mode=quick`
- [ ] capture a sample `POST /admin/qdrant/search_debug`
- [ ] capture ingest job backlog counts from the admin UI or API

During reindex:

- [ ] prefer `POST /admin/qdrant/reindex_document` for scoped document/version issues
- [ ] for broad rebuilds, use `POST /admin/qdrant/reindex_all` with `confirm=true`, a non-empty reason, and a bounded `limit`
- [ ] monitor `api` and `worker` container logs for ingest and vector errors
- [ ] avoid repeated `reindex_all` triggers inside the endpoint rate-limit window

After reindex:

- [ ] all expected ingest jobs reach a terminal success state
- [ ] vector health returns no missing/orphan vectors
- [ ] collection dimension still matches the embedding model
- [ ] sample retrieval returns chunk-backed hits
- [ ] answer smoke checks are accepted by the retrieval/answer owner

## Rollback

Rollback should restore service behavior before destructive vector operations.

1. If a V1/V2 flag exists, switch back to V1 first.
2. If rollback is release-based, redeploy the previous known-good tag or SHA.
3. Do not delete the current Qdrant collection during the initial rollback.
4. If V2 used a separate collection, point config back to the V1 collection and redeploy/render.
5. Run:

```bash
cd /opt/legal_api/app/install
./backend/verify_rag.sh
```

6. If rollback follows a leaked-secret incident, rotate credentials before the rollback is considered complete.

## Escalation Boundaries

Hand off to backend/retrieval owners when:

- upload MIME, AV, or file-size enforcement must be changed in backend handlers
- V1/V2 feature flags must affect runtime retrieval behavior
- answer quality is wrong despite passing Qdrant search/debug gates
- reindex jobs fail because of ingest parsing, chunking, embeddings, or vector payload semantics
