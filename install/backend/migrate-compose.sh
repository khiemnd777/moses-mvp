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
# shellcheck disable=SC1090
. "$BACKEND_DIR/.env"
set +a

POSTGRES_WAIT_RETRIES="${POSTGRES_WAIT_RETRIES:-30}"
POSTGRES_WAIT_INTERVAL="${POSTGRES_WAIT_INTERVAL:-2}"
MIGRATION_TABLE="${MIGRATION_TABLE:-schema_migrations}"

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

psql_compose() {
  compose_runtime exec -T -e PGPASSWORD="$POSTGRES_PASSWORD" postgres \
    psql \
      -h 127.0.0.1 \
      -p "$DOCKER_POSTGRES_PORT" \
      -U "$POSTGRES_USER" \
      -d "$POSTGRES_DB" \
      "$@"
}

log "Waiting for Compose Postgres at postgres:${DOCKER_POSTGRES_PORT}/${POSTGRES_DB}"
for ((i = 1; i <= POSTGRES_WAIT_RETRIES; i++)); do
  if compose_runtime exec -T postgres \
    pg_isready \
      -h 127.0.0.1 \
      -p "$DOCKER_POSTGRES_PORT" \
      -U "$POSTGRES_USER" \
      -d "$POSTGRES_DB" >/dev/null 2>&1; then
    break
  fi

  if [[ "$i" -eq "$POSTGRES_WAIT_RETRIES" ]]; then
    err "Compose Postgres did not become ready after ${POSTGRES_WAIT_RETRIES} attempts"
  fi

  sleep "$POSTGRES_WAIT_INTERVAL"
done

log "Running database migrations through Compose Postgres"
psql_compose -v ON_ERROR_STOP=1 \
  -c "CREATE TABLE IF NOT EXISTS ${MIGRATION_TABLE} (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW());"

for file in "$BACKEND_DIR"/infra/migrations/[0-9]*.sql; do
  version="$(basename "$file")"
  applied="$(psql_compose -At -v ON_ERROR_STOP=1 -c "SELECT 1 FROM ${MIGRATION_TABLE} WHERE version = '${version}' LIMIT 1;")"

  if [[ "$applied" == "1" ]]; then
    echo "Skipping $version (already applied)"
    continue
  fi

  echo "Applying $version"
  psql_compose -v ON_ERROR_STOP=1 < "$file"
  psql_compose -v ON_ERROR_STOP=1 -c "INSERT INTO ${MIGRATION_TABLE} (version) VALUES ('${version}');"
done
