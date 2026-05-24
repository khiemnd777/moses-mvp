#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
ensure_repo_present "$INSTALL_DIR"

log "Docker service status"
compose_prod "$INSTALL_DIR" ps

log "Backend health check"
curl -fsS "http://127.0.0.1:${BACKEND_PORT:-18088}/health"
echo
