#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
require_vars DOMAIN FRONTEND_ROOT BACKEND_UPSTREAM

SITE_TEMPLATE="$SCRIPT_DIR/sites/ai.dailyturning.com.conf.template"
SITE_PATH="/etc/nginx/sites-available/$DOMAIN"
ENABLED_PATH="/etc/nginx/sites-enabled/$DOMAIN"

if ! command -v nginx >/dev/null 2>&1; then
  log "Installing Nginx"
  sudo apt-get update -y
  sudo apt-get install -y nginx
else
  log "Nginx already installed"
fi

TMP_FILE="$(mktemp)"
sed \
  -e "s|__DOMAIN__|$DOMAIN|g" \
  -e "s|__FRONTEND_ROOT__|$FRONTEND_ROOT|g" \
  -e "s|__BACKEND_UPSTREAM__|$BACKEND_UPSTREAM|g" \
  "$SITE_TEMPLATE" > "$TMP_FILE"

sudo cp "$TMP_FILE" "$SITE_PATH"
rm -f "$TMP_FILE"

sudo ln -sfn "$SITE_PATH" "$ENABLED_PATH"
if [[ -L /etc/nginx/sites-enabled/default ]]; then
  sudo rm -f /etc/nginx/sites-enabled/default
fi

sudo nginx -t
sudo systemctl enable nginx
sudo systemctl reload nginx || sudo systemctl restart nginx
