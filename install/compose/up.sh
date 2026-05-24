#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
ensure_repo_present "$INSTALL_DIR"

PROJECT_ROOT="$(repo_root "$INSTALL_DIR")"
require_file "$PROJECT_ROOT/backend/.env"
require_file "$PROJECT_ROOT/install/docker-compose.prod.yml"
require_file "${NGINX_CONF_FILE:-$PROJECT_ROOT/install/nginx/rendered/default.conf}"

log "Starting production Docker Compose stack"
compose_prod "$INSTALL_DIR" up -d --build
