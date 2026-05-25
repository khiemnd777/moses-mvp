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

require_non_placeholder_var OPENAI_API_KEY
require_vars BACKEND_PORT QDRANT_COLLECTION ADMIN_BOOTSTRAP_PASSWORD

API_BASE_URL="${RAG_VERIFY_API_BASE_URL:-http://127.0.0.1:${BACKEND_PORT}}"
AUTH_USERNAME="${RAG_VERIFY_AUTH_USERNAME:-admin}"
AUTH_PASSWORD="${RAG_VERIFY_AUTH_PASSWORD:-${ADMIN_BOOTSTRAP_PASSWORD:-}}"
VERIFY_QUERY="${RAG_VERIFY_QUERY:-Thu tuc ly hon.}"
VERIFY_TOP_K="${RAG_VERIFY_TOP_K:-5}"
VERIFY_MIN_HITS="${RAG_VERIFY_MIN_HITS:-1}"
VERIFY_ACTOR="${RAG_VERIFY_ACTOR:-rag-production-verify}"
VERIFY_OUTPUT_DIR="${RAG_VERIFY_OUTPUT_DIR:-$PROJECT_ROOT/tmp/rag-verify}"
VERIFY_REQUIRED_PAYLOAD_KEY="${RAG_VERIFY_REQUIRED_PAYLOAD_KEY:-legal_domain}"
VERIFY_UPLOAD_FILE="${RAG_VERIFY_UPLOAD_FILE:-}"
VERIFY_UPLOAD_TITLE="${RAG_VERIFY_UPLOAD_TITLE:-RAG production verification upload}"
VERIFY_UPLOAD_POLL_ATTEMPTS="${RAG_VERIFY_UPLOAD_POLL_ATTEMPTS:-24}"
VERIFY_UPLOAD_POLL_INTERVAL="${RAG_VERIFY_UPLOAD_POLL_INTERVAL:-5}"
VERIFY_AV_EICAR="${RAG_VERIFY_AV_EICAR:-auto}"
VECTOR_MAX_SCANNED="${RAG_VERIFY_MAX_VECTORS_SCANNED:-1000}"
VECTOR_MAX_CHUNKS="${RAG_VERIFY_MAX_CHUNKS:-1000}"
RUN_STAMP="$(date +%Y%m%d-%H%M%S)"
RUN_DIR="$VERIFY_OUTPUT_DIR/$RUN_STAMP"
UPLOAD_AV_SCAN_MODE="${UPLOAD_AV_SCAN_MODE:-disabled}"
UPLOAD_AV_SCAN_MODE="$(printf '%s' "$UPLOAD_AV_SCAN_MODE" | tr '[:upper:]' '[:lower:]')"
UPLOAD_AV_FAIL_CLOSED="${UPLOAD_AV_FAIL_CLOSED:-true}"
UPLOAD_AV_FAIL_CLOSED="$(printf '%s' "$UPLOAD_AV_FAIL_CLOSED" | tr '[:upper:]' '[:lower:]')"

[[ "$VERIFY_TOP_K" =~ ^[0-9]+$ ]] || err "RAG_VERIFY_TOP_K must be a positive integer"
[[ "$VERIFY_MIN_HITS" =~ ^[0-9]+$ ]] || err "RAG_VERIFY_MIN_HITS must be a non-negative integer"
[[ "$VECTOR_MAX_SCANNED" =~ ^[0-9]+$ ]] || err "RAG_VERIFY_MAX_VECTORS_SCANNED must be a positive integer"
[[ "$VECTOR_MAX_CHUNKS" =~ ^[0-9]+$ ]] || err "RAG_VERIFY_MAX_CHUNKS must be a positive integer"
[[ "$VERIFY_UPLOAD_POLL_ATTEMPTS" =~ ^[0-9]+$ ]] || err "RAG_VERIFY_UPLOAD_POLL_ATTEMPTS must be a positive integer"
[[ "$VERIFY_UPLOAD_POLL_INTERVAL" =~ ^[0-9]+$ ]] || err "RAG_VERIFY_UPLOAD_POLL_INTERVAL must be a positive integer"
require_vars AUTH_PASSWORD

umask 077
mkdir -p "$RUN_DIR"

cleanup() {
  local status="$?"
  if [[ "${RAG_VERIFY_KEEP_OUTPUT:-0}" == "1" || "$status" != "0" ]]; then
    warn "RAG verification outputs kept in $RUN_DIR"
  else
    rm -rf "$RUN_DIR"
  fi
}
trap cleanup EXIT

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

assert_http_success() {
  local label="$1"
  local status="$2"
  local response_file="$3"

  if [[ ! "$status" =~ ^2[0-9][0-9]$ ]]; then
    err "$label failed with HTTP $status; response saved at $response_file"
  fi
}

log "Production RAG gate: secret sanity"
require_non_placeholder_var OPENAI_API_KEY
if [[ "${VITE_API_BASE_URL:-}" == *"OPENAI"* || "${VITE_API_BASE_URL:-}" == sk-* ]]; then
  err "VITE_API_BASE_URL appears to contain a private value; VITE_* values are browser-public"
fi
case "$UPLOAD_AV_SCAN_MODE" in
  clamd)
    [[ "$UPLOAD_AV_FAIL_CLOSED" == "true" ]] || err "UPLOAD_AV_FAIL_CLOSED must be true for production RAG verification"
    require_vars UPLOAD_AV_CLAMD_ADDR
    ;;
  disabled)
    if [[ "${RAG_VERIFY_ALLOW_AV_DISABLED:-0}" != "1" ]]; then
      err "UPLOAD_AV_SCAN_MODE=disabled; set clamd for production or RAG_VERIFY_ALLOW_AV_DISABLED=1 for a break-glass verification"
    fi
    warn "UPLOAD_AV_SCAN_MODE=disabled; upload malware scan gate is skipped by explicit override"
    ;;
  *)
    err "Unsupported UPLOAD_AV_SCAN_MODE: $UPLOAD_AV_SCAN_MODE"
    ;;
esac

log "Production RAG gate: backend health"
HEALTH_RESPONSE="$RUN_DIR/health.response.json"
health_code="$(curl_json GET "$API_BASE_URL/health" "$HEALTH_RESPONSE")"
assert_http_success "Backend health" "$health_code" "$HEALTH_RESPONSE"

LOGIN_PAYLOAD="$RUN_DIR/login.request.json"
cat > "$LOGIN_PAYLOAD" <<EOF
{"username":"$(json_escape "$AUTH_USERNAME")","password":"$(json_escape "$AUTH_PASSWORD")"}
EOF

LOGIN_RESPONSE="$RUN_DIR/login.response.json"
login_code="$(curl_json POST "$API_BASE_URL/auth/login" "$LOGIN_RESPONSE" "$LOGIN_PAYLOAD")"
rm -f "$LOGIN_PAYLOAD"
assert_http_success "Admin login" "$login_code" "$LOGIN_RESPONSE"

ACCESS_TOKEN="$(extract_json_string "access_token" "$LOGIN_RESPONSE")"
rm -f "$LOGIN_RESPONSE"
if [[ -z "$ACCESS_TOKEN" ]]; then
  err "Admin login succeeded but access_token was not found"
fi

AUTH_HEADER="Authorization: Bearer $ACCESS_TOKEN"
ACTOR_HEADER="X-Admin-Actor: $VERIFY_ACTOR"

log "Production RAG gate: pipeline health and alert metrics"
PIPELINE_HEALTH_RESPONSE="$RUN_DIR/pipeline_health.response.json"
pipeline_health_code="$(curl_json GET "$API_BASE_URL/admin/pipeline/health?recent_hours=24&stale_minutes=15&limit=20" "$PIPELINE_HEALTH_RESPONSE" "" -H "$AUTH_HEADER" -H "$ACTOR_HEADER")"
assert_http_success "Pipeline health" "$pipeline_health_code" "$PIPELINE_HEALTH_RESPONSE"

pipeline_severity="$(extract_json_string "severity" "$PIPELINE_HEALTH_RESPONSE")"
case "$pipeline_severity" in
  ok)
    ;;
  degraded)
    warn "Pipeline health is degraded; response saved at $PIPELINE_HEALTH_RESPONSE"
    ;;
  critical)
    if [[ "${RAG_VERIFY_ALLOW_PIPELINE_CRITICAL:-0}" != "1" ]]; then
      err "Pipeline health is critical; response saved at $PIPELINE_HEALTH_RESPONSE"
    fi
    warn "RAG_VERIFY_ALLOW_PIPELINE_CRITICAL=1; allowing critical pipeline health"
    ;;
  *)
    err "Pipeline health response did not include a valid severity; response saved at $PIPELINE_HEALTH_RESPONSE"
    ;;
esac

METRICS_RESPONSE="$RUN_DIR/metrics.response.txt"
metrics_code="$(curl_json GET "$API_BASE_URL/metrics" "$METRICS_RESPONSE")"
assert_http_success "Prometheus metrics" "$metrics_code" "$METRICS_RESPONSE"
grep -Fq 'pipeline_metrics_up 1' "$METRICS_RESPONSE" || err "Pipeline metrics were not collected successfully; response saved at $METRICS_RESPONSE"
grep -Fq 'pipeline_health_status{severity="' "$METRICS_RESPONSE" || err "Pipeline health status metrics are missing; response saved at $METRICS_RESPONSE"
grep -Fq 'pipeline_stale_uploads' "$METRICS_RESPONSE" || err "Pipeline stale upload metric is missing; response saved at $METRICS_RESPONSE"

if [[ "$UPLOAD_AV_SCAN_MODE" == "clamd" && "$VERIFY_AV_EICAR" != "0" ]]; then
  log "Production RAG gate: upload malware scan rejects EICAR test file"
  EICAR_FILE="$RUN_DIR/eicar.txt"
  cat > "$EICAR_FILE" <<'EOF'
X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*
EOF
  EICAR_RESPONSE="$RUN_DIR/eicar.response.json"
  eicar_code="$(
    curl -sS -X POST "$API_BASE_URL/document-uploads" \
      -H "$AUTH_HEADER" \
      -F "file=@$EICAR_FILE" \
      -F "title=RAG production AV verification" \
      -o "$EICAR_RESPONSE" \
      -w "%{http_code}"
  )"
  if [[ "$eicar_code" != "400" ]]; then
    err "EICAR upload was not rejected as malware; HTTP $eicar_code response saved at $EICAR_RESPONSE"
  fi
  grep -Fq '"code":"malware_detected"' "$EICAR_RESPONSE" || err "EICAR upload did not return malware_detected; response saved at $EICAR_RESPONSE"
elif [[ "$VERIFY_AV_EICAR" == "1" ]]; then
  err "RAG_VERIFY_AV_EICAR=1 requires UPLOAD_AV_SCAN_MODE=clamd"
fi

log "Production RAG gate: Qdrant collection and payload indexes"
COLLECTION_RESPONSE="$RUN_DIR/qdrant_collection.response.json"
collection_code="$(curl_json GET "$API_BASE_URL/admin/qdrant/collections/$QDRANT_COLLECTION" "$COLLECTION_RESPONSE" "" -H "$AUTH_HEADER" -H "$ACTOR_HEADER")"
assert_http_success "Qdrant collection check" "$collection_code" "$COLLECTION_RESPONSE"

grep -Fq '"found":true' "$COLLECTION_RESPONSE" || err "Qdrant collection $QDRANT_COLLECTION was not found"
grep -Fq '"passed":true' "$COLLECTION_RESPONSE" || err "Qdrant collection dimension validation did not pass"

for key in legal_domain document_type effective_status document_number article_number issuing_authority signed_year; do
  grep -Fq "\"key\":\"$key\"" "$COLLECTION_RESPONSE" || err "Missing Qdrant payload index: $key"
done

log "Production RAG gate: vector health"
VECTOR_HEALTH_RESPONSE="$RUN_DIR/vector_health.response.json"
vector_health_code="$(curl_json GET "$API_BASE_URL/admin/qdrant/vector_health?mode=quick&max_vectors_scanned=$VECTOR_MAX_SCANNED&max_chunks=$VECTOR_MAX_CHUNKS" "$VECTOR_HEALTH_RESPONSE" "" -H "$AUTH_HEADER" -H "$ACTOR_HEADER")"
assert_http_success "Vector health" "$vector_health_code" "$VECTOR_HEALTH_RESPONSE"

grep -Fq '"dimension_mismatch_detected":false' "$VECTOR_HEALTH_RESPONSE" || err "Vector health reported dimension mismatch"
if [[ "${RAG_VERIFY_ALLOW_VECTOR_DRIFT:-0}" != "1" ]]; then
  grep -Fq '"chunk_vector_count_mismatch":false' "$VECTOR_HEALTH_RESPONSE" || err "Vector health reported chunk/vector count mismatch"
  grep -Fq '"orphan_vectors_count":0' "$VECTOR_HEALTH_RESPONSE" || err "Vector health reported orphan vectors"
  grep -Fq '"missing_vectors_count":0' "$VECTOR_HEALTH_RESPONSE" || err "Vector health reported missing vectors"
else
  warn "RAG_VERIFY_ALLOW_VECTOR_DRIFT=1; skipping missing/orphan/count strict gates"
fi

log "Production RAG gate: sample retrieval"
SEARCH_PAYLOAD="$RUN_DIR/search_debug.request.json"
cat > "$SEARCH_PAYLOAD" <<EOF
{"query_text":"$(json_escape "$VERIFY_QUERY")","top_k":$VERIFY_TOP_K,"include_payload":true,"include_chunk_preview":true}
EOF

SEARCH_RESPONSE="$RUN_DIR/search_debug.response.json"
search_code="$(curl_json POST "$API_BASE_URL/admin/qdrant/search_debug" "$SEARCH_RESPONSE" "$SEARCH_PAYLOAD" -H "$AUTH_HEADER" -H "$ACTOR_HEADER")"
assert_http_success "Sample retrieval" "$search_code" "$SEARCH_RESPONSE"

hit_count="$(sed -n 's/.*"hit_count":\([0-9][0-9]*\).*/\1/p' "$SEARCH_RESPONSE" | head -n 1)"
[[ "$hit_count" =~ ^[0-9]+$ ]] || err "Sample retrieval response did not include hit_count"
if (( hit_count < VERIFY_MIN_HITS )); then
  err "Sample retrieval returned $hit_count hits; expected at least $VERIFY_MIN_HITS"
fi
grep -Fq '"chunk_id"' "$SEARCH_RESPONSE" || err "Sample retrieval did not resolve chunk metadata"
if [[ -n "$VERIFY_REQUIRED_PAYLOAD_KEY" ]]; then
  grep -Fq "\"$VERIFY_REQUIRED_PAYLOAD_KEY\"" "$SEARCH_RESPONSE" || err "Sample retrieval payload did not include $VERIFY_REQUIRED_PAYLOAD_KEY"
fi

if [[ -n "$VERIFY_UPLOAD_FILE" ]]; then
  require_file "$VERIFY_UPLOAD_FILE"
  log "Production RAG gate: upload-only intake through API proxy"
  UPLOAD_RESPONSE="$RUN_DIR/upload.response.json"
  upload_code="$(
    curl -sS -X POST "$API_BASE_URL/document-uploads" \
      -H "$AUTH_HEADER" \
      -F "file=@$VERIFY_UPLOAD_FILE" \
      -F "title=$VERIFY_UPLOAD_TITLE" \
      -o "$UPLOAD_RESPONSE" \
      -w "%{http_code}"
  )"
  assert_http_success "Document upload intake" "$upload_code" "$UPLOAD_RESPONSE"
  UPLOAD_ID="$(extract_json_string "id" "$UPLOAD_RESPONSE")"
  [[ -n "$UPLOAD_ID" ]] || err "Upload intake succeeded but id was not found"
  grep -Eq '"status":"(uploaded|extracting|classified|profile_resolved|indexing|validating|ready)"' "$UPLOAD_RESPONSE" || err "Upload response did not include an expected processing status"

  if [[ "${RAG_VERIFY_UPLOAD_REQUIRE_READY:-0}" == "1" ]]; then
    log "Production RAG gate: waiting for uploaded document to become ready"
    UPLOADS_RESPONSE="$RUN_DIR/uploads.poll.response.json"
    ready="0"
    for _ in $(seq 1 "$VERIFY_UPLOAD_POLL_ATTEMPTS"); do
      uploads_code="$(curl_json GET "$API_BASE_URL/document-uploads?limit=100" "$UPLOADS_RESPONSE" "" -H "$AUTH_HEADER")"
      assert_http_success "Document upload poll" "$uploads_code" "$UPLOADS_RESPONSE"
      if grep -Fq "\"id\":\"$UPLOAD_ID\"" "$UPLOADS_RESPONSE" && grep -Fq '"status":"ready"' "$UPLOADS_RESPONSE"; then
        ready="1"
        break
      fi
      if grep -Fq "\"id\":\"$UPLOAD_ID\"" "$UPLOADS_RESPONSE" && grep -Eq '"status":"(extract_failed|classification_low_confidence|validation_failed|rejected)"' "$UPLOADS_RESPONSE"; then
        err "Uploaded document reached a failed review status; response saved at $UPLOADS_RESPONSE"
      fi
      sleep "$VERIFY_UPLOAD_POLL_INTERVAL"
    done
    [[ "$ready" == "1" ]] || err "Uploaded document did not become ready before timeout"
  fi
else
  warn "RAG_VERIFY_UPLOAD_FILE is not set; skipping upload-only intake gate"
fi

log "Upload proxy limit: ${CLIENT_MAX_BODY_SIZE:-50m}; see production runbook for upload type/size notes"
log "Production RAG verification passed"
