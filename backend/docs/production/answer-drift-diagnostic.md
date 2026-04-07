# Answer Drift Diagnostic On VPS

Use this when local and VPS test return different answers for the same legal question even though the deployed code is the same.

## What the script collects

Run:

```bash
cd install && bash ./backend/debug_answer_drift.sh
```

Optional overrides:

```bash
DEBUG_AUTH_PASSWORD='current-admin-password' \
DEBUG_QUERY='Thủ tục ly dị.' \
DEBUG_TOP_K=5 \
DEBUG_API_BASE_URL='http://127.0.0.1:8080' \
cd install && bash ./backend/debug_answer_drift.sh
```

The script loads `backend/.env`, logs in as `admin`, and writes a timestamped bundle under `tmp/answer-drift/<timestamp>/`.

It collects:

- `query_debug.response.json` from `POST /doc-types/query-debug`
- `retrieval_configs.response.json` from `GET /admin/ai/retrieval-configs`
- `qdrant_collections.response.json` from `GET /admin/qdrant/collections`
- `qdrant_search_debug.response.json` from `POST /admin/qdrant/search_debug`
- `doc_types_target.txt` for the seeded marriage-family doc types
- `doc_types_query_profile.txt` for the effective `query_profile` content in Postgres
- `corpus_counts.txt` and `corpus_sample.txt` for document, version, and chunk presence
- `retrieval_config.txt` for direct database state of `ai_retrieval_configs`

## How to read the bundle

- If `query_debug.response.json` does not infer `ly hon` or `marriage_family`, the problem is before retrieval ranking.
  Check `doc_types_target.txt` and `doc_types_query_profile.txt`.
- If `query_debug.response.json` looks correct but `qdrant_search_debug.response.json` does not return marriage-family hits, the problem is corpus or vector parity.
  Check `corpus_counts.txt`, `corpus_sample.txt`, and Qdrant state.
- If Postgres contains the expected corpus but Qdrant hits are missing or unrelated, treat it as ingest/vector drift.
- If both query debug and Qdrant search look correct but the final answer is still wrong, inspect answer traces and prompt or guard runtime config next.

## Notes

- The script defaults to `admin` and uses `ADMIN_BOOTSTRAP_PASSWORD` from `backend/.env` unless `DEBUG_AUTH_PASSWORD` is set.
- If the admin password has been changed after bootstrap, provide `DEBUG_AUTH_PASSWORD` explicitly.
- The script is intended to run on the VPS where `backend/.env` points at the same Postgres and API runtime being debugged.
