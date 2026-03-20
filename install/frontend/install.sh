#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
ensure_repo_present "$INSTALL_DIR"

PROJECT_ROOT="$(repo_root "$INSTALL_DIR")"
require_vars FRONTEND_ROOT

FRONTEND_DIST="$PROJECT_ROOT/frontend/dist"

require_dir "$FRONTEND_DIST"

log "Installing frontend assets to $FRONTEND_ROOT"
sudo mkdir -p "$FRONTEND_ROOT"
sudo rsync -av --delete "$FRONTEND_DIST/" "$FRONTEND_ROOT/"
sudo chown -R www-data:www-data "$FRONTEND_ROOT"
sudo find "$FRONTEND_ROOT" -type d -exec chmod 755 {} \;
sudo find "$FRONTEND_ROOT" -type f -exec chmod 644 {} \;
