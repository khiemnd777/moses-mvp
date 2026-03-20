#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

source "$SCRIPT_DIR/lib/common.sh"
load_install_config "$SCRIPT_DIR"

"$SCRIPT_DIR/repo/sync.sh"
"$SCRIPT_DIR/backend/install.sh"
"$SCRIPT_DIR/backend/migrate.sh"
"$SCRIPT_DIR/frontend/build.sh"
"$SCRIPT_DIR/frontend/install.sh"
"$SCRIPT_DIR/nginx/install.sh"

if [[ "${ENABLE_SSL:-1}" == "1" ]]; then
  "$SCRIPT_DIR/nginx/issue-ssl.sh"
fi

"$SCRIPT_DIR/backend/verify.sh"
"$SCRIPT_DIR/nginx/verify.sh"

log "Install completed"
