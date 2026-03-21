#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
ensure_repo_present "$INSTALL_DIR"

PROJECT_ROOT="$(repo_root "$INSTALL_DIR")"
BACKEND_DIR="$PROJECT_ROOT/backend"
require_file "$BACKEND_DIR/Makefile"
require_file "$BACKEND_DIR/.env"

log "Running database migrations"
cd "$BACKEND_DIR"
# Reuse the rendered backend env so readiness check and migration target the same database.
set -a
. "$BACKEND_DIR/.env"
set +a

POSTGRES_WAIT_RETRIES="${POSTGRES_WAIT_RETRIES:-30}"
POSTGRES_WAIT_INTERVAL="${POSTGRES_WAIT_INTERVAL:-2}"

log "Waiting for Postgres at ${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}"
for ((i = 1; i <= POSTGRES_WAIT_RETRIES; i++)); do
  if pg_isready \
    -h "$POSTGRES_HOST" \
    -p "$POSTGRES_PORT" \
    -U "$POSTGRES_USER" \
    -d "$POSTGRES_DB" >/dev/null 2>&1; then
    break
  fi

  if [[ "$i" -eq "$POSTGRES_WAIT_RETRIES" ]]; then
    err "Postgres did not become ready after ${POSTGRES_WAIT_RETRIES} attempts"
  fi

  sleep "$POSTGRES_WAIT_INTERVAL"
done

make migrate
