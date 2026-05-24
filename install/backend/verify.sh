#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
ensure_repo_present "$INSTALL_DIR"

compose_runtime() {
  case "${COMPOSE_RUNTIME:-prod}" in
    local|dev)
      compose_local "$INSTALL_DIR" "$@"
      ;;
    prod|production)
      compose_prod "$INSTALL_DIR" "$@"
      ;;
    *)
      err "Unsupported COMPOSE_RUNTIME: ${COMPOSE_RUNTIME}"
      ;;
  esac
}

log "Docker service status"
compose_runtime ps

log "Backend health check"
health_body="$(mktemp)"
health_status="$(curl -sS -o "$health_body" -w "%{http_code}" "http://127.0.0.1:${BACKEND_PORT:-18088}/health")"
cat "$health_body"
echo

if [[ "$health_status" =~ ^2[0-9][0-9]$ ]]; then
  rm -f "$health_body"
  exit 0
fi

if [[ "${COMPOSE_RUNTIME:-prod}" =~ ^(local|dev)$ && "${OPENAI_API_KEY:-}" == "replace-with-openai-key" ]]; then
  if grep -q '"postgres":"ok"' "$health_body" && grep -q '"qdrant":"ok"' "$health_body"; then
    warn "Backend health reports OpenAI unavailable because local OPENAI_API_KEY is the placeholder; Postgres and Qdrant are healthy."
    rm -f "$health_body"
    exit 0
  fi
fi

rm -f "$health_body"
err "Backend health check failed with HTTP $health_status"
