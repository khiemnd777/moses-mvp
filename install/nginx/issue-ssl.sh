#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
require_vars DOMAIN LETSENCRYPT_EMAIL

if [[ -d "/etc/letsencrypt/live/$DOMAIN" ]]; then
  log "SSL certificate already exists for $DOMAIN"
  sudo nginx -t
  sudo systemctl reload nginx
  exit 0
fi

if ! command -v certbot >/dev/null 2>&1; then
  log "Installing Certbot"
  sudo apt-get update -y
  sudo apt-get install -y certbot python3-certbot-nginx
else
  log "Certbot already installed"
fi

log "Issuing SSL certificate for $DOMAIN"
sudo certbot --nginx \
  -d "$DOMAIN" \
  --non-interactive \
  --agree-tos \
  -m "$LETSENCRYPT_EMAIL" \
  --redirect

sudo nginx -t
sudo systemctl reload nginx
