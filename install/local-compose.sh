#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${INSTALL_CONFIG_FILE:-$SCRIPT_DIR/config.local.sh}"
COMMAND="${1:-up}"

source "$SCRIPT_DIR/lib/common.sh"

if [[ ! -f "$CONFIG_FILE" ]]; then
  if [[ "$CONFIG_FILE" == "$SCRIPT_DIR/config.local.sh" && -f "$SCRIPT_DIR/config.local.sh.sample" ]]; then
    cp "$SCRIPT_DIR/config.local.sh.sample" "$CONFIG_FILE"
    warn "Created $CONFIG_FILE from config.local.sh.sample; edit OPENAI_API_KEY if you need AI calls."
  else
    err "Missing local config: $CONFIG_FILE"
  fi
fi

export INSTALL_CONFIG_FILE="$CONFIG_FILE"
load_install_config "$SCRIPT_DIR"

compose_service_running() {
  local service="$1"
  compose_local "$SCRIPT_DIR" ps --status running --services | grep -qx "$service"
}

run_web_command() {
  if compose_service_running web; then
    compose_local "$SCRIPT_DIR" exec -T web "$@"
  else
    compose_local "$SCRIPT_DIR" run --rm --no-deps web "$@"
  fi
}

case "$COMMAND" in
  up)
    SKIP_DOCKER_INSTALL=1 "$SCRIPT_DIR/backend/install.sh"
    log "Starting local development Docker Compose stack"
    compose_local "$SCRIPT_DIR" up -d --build
    COMPOSE_RUNTIME=local "$SCRIPT_DIR/backend/migrate-compose.sh"
    log "Local dev stack is running at http://localhost:${HTTP_PORT:-19080}"
    ;;
  migrate)
    COMPOSE_RUNTIME=local "$SCRIPT_DIR/backend/migrate-compose.sh"
    ;;
  verify)
    COMPOSE_RUNTIME=local "$SCRIPT_DIR/backend/verify.sh"
    log "Frontend dev server smoke test"
    curl -fsSI "http://127.0.0.1:${HTTP_PORT:-19080}" >/dev/null
    log "Local dev web is responding at http://localhost:${HTTP_PORT:-19080}"
    ;;
  ps)
    compose_local "$SCRIPT_DIR" ps
    ;;
  logs)
    compose_local "$SCRIPT_DIR" logs -f api worker web
    ;;
  web-install|frontend-install)
    run_web_command bun install
    ;;
  web-add|frontend-add)
    shift || true
    [[ "$#" -gt 0 ]] || err "Usage: $0 $COMMAND <package...>"
    run_web_command bun add "$@"
    ;;
  down)
    compose_local "$SCRIPT_DIR" down
    ;;
  stop)
    compose_local "$SCRIPT_DIR" stop
    ;;
  restart)
    compose_local "$SCRIPT_DIR" restart
    ;;
  *)
    err "Usage: $0 [up|migrate|verify|ps|logs|web-install|web-add <package...>|down|stop|restart]"
    ;;
esac
