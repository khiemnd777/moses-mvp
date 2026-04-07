#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
ensure_repo_present "$INSTALL_DIR"

PROJECT_ROOT="$(repo_root "$INSTALL_DIR")"
BACKEND_DIR="$PROJECT_ROOT/backend"

require_file "$BACKEND_DIR/.env"

set -a
. "$BACKEND_DIR/.env"
set +a

AUTH_USERNAME="${DEBUG_AUTH_USERNAME:-admin}"
AUTH_PASSWORD="${DEBUG_AUTH_PASSWORD:-${ADMIN_BOOTSTRAP_PASSWORD:-}}"
DEBUG_QUERY="${DEBUG_QUERY:-Thủ tục ly dị.}"
DEBUG_TOP_K="${DEBUG_TOP_K:-5}"
DEBUG_OUTPUT_DIR="${DEBUG_OUTPUT_DIR:-$PROJECT_ROOT/tmp/answer-drift}"
API_BASE_URL="${DEBUG_API_BASE_URL:-http://127.0.0.1:${BACKEND_PORT:-8080}}"
RUN_STAMP="$(date +%Y%m%d-%H%M%S)"
RUN_DIR="$DEBUG_OUTPUT_DIR/$RUN_STAMP"

require_vars POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB POSTGRES_HOST POSTGRES_PORT POSTGRES_SSLMODE
require_vars AUTH_PASSWORD

mkdir -p "$RUN_DIR"

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '%s' "$value"
}

extract_json_string() {
  local key="$1"
  local file="$2"
  sed -n "s/.*\"${key}\":\"\\([^\"]*\\)\".*/\\1/p" "$file" | head -n 1
}

curl_json() {
  local method="$1"
  local url="$2"
  local output_file="$3"
  local body_file="${4:-}"
  shift 4 || true
  local -a extra_args=("$@")
  local http_code

  if [[ -n "$body_file" ]]; then
    http_code="$(
      curl -sS -X "$method" "$url" \
        -H "Content-Type: application/json" \
        "${extra_args[@]}" \
        --data @"$body_file" \
        -o "$output_file" \
        -w "%{http_code}"
    )"
  else
    http_code="$(
      curl -sS -X "$method" "$url" \
        "${extra_args[@]}" \
        -o "$output_file" \
        -w "%{http_code}"
    )"
  fi

  printf '%s' "$http_code"
}

POSTGRES_DSN="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=${POSTGRES_SSLMODE}"

log "Collecting answer drift evidence into $RUN_DIR"

LOGIN_PAYLOAD="$RUN_DIR/login.request.json"
cat > "$LOGIN_PAYLOAD" <<EOF
{"username":"$(json_escape "$AUTH_USERNAME")","password":"$(json_escape "$AUTH_PASSWORD")"}
EOF

LOGIN_RESPONSE="$RUN_DIR/login.response.json"
login_code="$(curl_json POST "$API_BASE_URL/auth/login" "$LOGIN_RESPONSE" "$LOGIN_PAYLOAD")"
if [[ "$login_code" != "200" ]]; then
  err "Login failed with status $login_code. See $LOGIN_RESPONSE"
fi

ACCESS_TOKEN="$(extract_json_string "access_token" "$LOGIN_RESPONSE")"
if [[ -z "$ACCESS_TOKEN" ]]; then
  err "Login succeeded but access_token was not found in $LOGIN_RESPONSE"
fi

AUTH_HEADER="Authorization: Bearer $ACCESS_TOKEN"

QUERY_DEBUG_PAYLOAD="$RUN_DIR/query_debug.request.json"
cat > "$QUERY_DEBUG_PAYLOAD" <<EOF
{"query":"$(json_escape "$DEBUG_QUERY")","top_k":$DEBUG_TOP_K}
EOF

QUERY_DEBUG_RESPONSE="$RUN_DIR/query_debug.response.json"
query_debug_code="$(curl_json POST "$API_BASE_URL/doc-types/query-debug" "$QUERY_DEBUG_RESPONSE" "$QUERY_DEBUG_PAYLOAD" -H "$AUTH_HEADER")"

RETRIEVAL_CONFIG_RESPONSE="$RUN_DIR/retrieval_configs.response.json"
retrieval_config_code="$(curl_json GET "$API_BASE_URL/admin/ai/retrieval-configs" "$RETRIEVAL_CONFIG_RESPONSE" "" -H "$AUTH_HEADER")"

QDRANT_COLLECTIONS_RESPONSE="$RUN_DIR/qdrant_collections.response.json"
qdrant_collections_code="$(curl_json GET "$API_BASE_URL/admin/qdrant/collections" "$QDRANT_COLLECTIONS_RESPONSE" "" -H "$AUTH_HEADER")"

QDRANT_SEARCH_PAYLOAD="$RUN_DIR/qdrant_search_debug.request.json"
cat > "$QDRANT_SEARCH_PAYLOAD" <<EOF
{"query_text":"$(json_escape "$DEBUG_QUERY")","top_k":$DEBUG_TOP_K,"include_payload":true,"include_chunk_preview":true}
EOF

QDRANT_SEARCH_RESPONSE="$RUN_DIR/qdrant_search_debug.response.json"
qdrant_search_code="$(curl_json POST "$API_BASE_URL/admin/qdrant/search_debug" "$QDRANT_SEARCH_RESPONSE" "$QDRANT_SEARCH_PAYLOAD" -H "$AUTH_HEADER")"

DOC_TYPES_SQL="$RUN_DIR/doc_types_target.sql"
cat > "$DOC_TYPES_SQL" <<'EOF'
\pset pager off
\x off
SELECT code, form_hash, updated_at
FROM doc_types
WHERE code IN (
  'vn_marriage_family_law',
  'vn_resolution_marriage_family_01_2024',
  'vn_resolution_marriage_family_35_2000'
)
ORDER BY code;
EOF

psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f "$DOC_TYPES_SQL" > "$RUN_DIR/doc_types_target.txt"

DOC_TYPES_PROFILE_SQL="$RUN_DIR/doc_types_query_profile.sql"
cat > "$DOC_TYPES_PROFILE_SQL" <<'EOF'
\pset pager off
\x off
SELECT
  code,
  form_json->'query_profile'->'canonical_terms' AS canonical_terms,
  form_json->'query_profile'->'query_signals' AS query_signals,
  form_json->'query_profile'->'preferred_doc_types' AS preferred_doc_types,
  form_json->'query_profile'->'domain_topic_rules' AS domain_topic_rules
FROM doc_types
WHERE code IN (
  'vn_marriage_family_law',
  'vn_resolution_marriage_family_01_2024',
  'vn_resolution_marriage_family_35_2000'
)
ORDER BY code;
EOF

psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f "$DOC_TYPES_PROFILE_SQL" > "$RUN_DIR/doc_types_query_profile.txt"

CORPUS_COUNTS_SQL="$RUN_DIR/corpus_counts.sql"
cat > "$CORPUS_COUNTS_SQL" <<'EOF'
\pset pager off
\x off
SELECT
  dt.code,
  COUNT(DISTINCT d.id) AS document_count,
  COUNT(DISTINCT dv.id) AS version_count,
  COUNT(c.id) AS chunk_count
FROM doc_types dt
LEFT JOIN documents d ON d.doc_type_id = dt.id
LEFT JOIN document_versions dv ON dv.document_id = d.id
LEFT JOIN chunks c ON c.document_version_id = dv.id
WHERE dt.code IN (
  'vn_marriage_family_law',
  'vn_resolution_marriage_family_01_2024',
  'vn_resolution_marriage_family_35_2000'
)
GROUP BY dt.code
ORDER BY dt.code;
EOF

psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f "$CORPUS_COUNTS_SQL" > "$RUN_DIR/corpus_counts.txt"

CORPUS_SAMPLE_SQL="$RUN_DIR/corpus_sample.sql"
cat > "$CORPUS_SAMPLE_SQL" <<'EOF'
\pset pager off
\x off
SELECT
  dt.code,
  d.id AS document_id,
  d.title,
  dv.id AS document_version_id,
  dv.version,
  COUNT(c.id) AS chunk_count
FROM doc_types dt
JOIN documents d ON d.doc_type_id = dt.id
JOIN document_versions dv ON dv.document_id = d.id
LEFT JOIN chunks c ON c.document_version_id = dv.id
WHERE dt.code IN (
  'vn_marriage_family_law',
  'vn_resolution_marriage_family_01_2024',
  'vn_resolution_marriage_family_35_2000'
)
GROUP BY dt.code, d.id, d.title, dv.id, dv.version
ORDER BY dt.code, d.title, dv.version DESC;
EOF

psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f "$CORPUS_SAMPLE_SQL" > "$RUN_DIR/corpus_sample.txt"

RETRIEVAL_CONFIG_SQL="$RUN_DIR/retrieval_config.sql"
cat > "$RETRIEVAL_CONFIG_SQL" <<'EOF'
\pset pager off
\x off
SELECT
  id,
  name,
  enabled,
  default_top_k,
  rerank_enabled,
  adjacent_chunk_enabled,
  adjacent_chunk_window,
  max_context_chunks,
  max_context_chars,
  default_effective_status,
  preferred_doc_types_json,
  legal_domain_defaults_json,
  updated_at
FROM ai_retrieval_configs
ORDER BY enabled DESC, updated_at DESC, created_at DESC;
EOF

psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f "$RETRIEVAL_CONFIG_SQL" > "$RUN_DIR/retrieval_config.txt"

README_FILE="$RUN_DIR/README.txt"
cat > "$README_FILE" <<EOF
Answer drift diagnostic bundle
=============================

Generated at: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
API base URL: $API_BASE_URL
Query: $DEBUG_QUERY
Top K: $DEBUG_TOP_K

HTTP status summary
- /auth/login: $login_code
- /doc-types/query-debug: $query_debug_code
- /admin/ai/retrieval-configs: $retrieval_config_code
- /admin/qdrant/collections: $qdrant_collections_code
- /admin/qdrant/search_debug: $qdrant_search_code

Interpretation shortcuts
- If query_debug.response.json does not infer ly hon / marriage_family, inspect doc_types_target.txt and doc_types_query_profile.txt first.
- If query_debug.response.json is correct but qdrant_search_debug.response.json misses marriage-family hits, inspect corpus_counts.txt, corpus_sample.txt, and Qdrant data parity.
- If corpus exists in Postgres but Qdrant hits are wrong or empty, re-check ingest and vector state on this VPS.
- If both retrieval layers look correct but chat answer is still wrong, inspect answer traces and prompt/guard runtime config next.
EOF

log "Diagnostic bundle created at $RUN_DIR"
printf '%s\n' "$RUN_DIR"
