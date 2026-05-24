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

case "$COMMAND" in
  up)
    SKIP_DOCKER_INSTALL=1 "$SCRIPT_DIR/backend/install.sh"
    "$SCRIPT_DIR/nginx/install.sh"
    "$SCRIPT_DIR/compose/up.sh"
    "$SCRIPT_DIR/backend/migrate-compose.sh"
    log "Local Compose stack is running at http://localhost:${HTTP_PORT:-19080}"
    ;;
  migrate)
    "$SCRIPT_DIR/backend/migrate-compose.sh"
    ;;
  verify)
    "$SCRIPT_DIR/backend/verify.sh"
    "$SCRIPT_DIR/nginx/verify.sh"
    ;;
  ps)
    compose_prod "$SCRIPT_DIR" ps
    ;;
  logs)
    compose_prod "$SCRIPT_DIR" logs -f api worker web
    ;;
  down)
    compose_prod "$SCRIPT_DIR" down
    ;;
  stop)
    compose_prod "$SCRIPT_DIR" stop
    ;;
  restart)
    compose_prod "$SCRIPT_DIR" restart
    ;;
  *)
    err "Usage: $0 [up|migrate|verify|ps|logs|down|stop|restart]"
    ;;
esac
