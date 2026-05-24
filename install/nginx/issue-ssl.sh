#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
ensure_repo_present "$INSTALL_DIR"
require_vars DOMAIN LETSENCRYPT_EMAIL

LETSENCRYPT_DIR="${LETSENCRYPT_DIR:-/etc/letsencrypt}"
SSL_CERT_PATH="$LETSENCRYPT_DIR/live/$DOMAIN/fullchain.pem"
SSL_KEY_PATH="$LETSENCRYPT_DIR/live/$DOMAIN/privkey.pem"

reload_web() {
  compose_prod "$INSTALL_DIR" up -d web
  compose_prod "$INSTALL_DIR" exec -T web nginx -s reload || compose_prod "$INSTALL_DIR" restart web
}

if [[ -f "$SSL_CERT_PATH" && -f "$SSL_KEY_PATH" ]]; then
  log "SSL certificate already exists for $DOMAIN"
  "$SCRIPT_DIR/install.sh"
  reload_web
  exit 0
fi

log "Starting HTTP web container for ACME challenge"
FORCE_HTTP_ONLY=1 "$SCRIPT_DIR/install.sh"
compose_prod "$INSTALL_DIR" up -d web

log "Issuing SSL certificate for $DOMAIN with Certbot container"
compose_prod "$INSTALL_DIR" run --rm --no-deps certbot certonly \
  --webroot \
  -w /var/www/certbot \
  -d "$DOMAIN" \
  --non-interactive \
  --agree-tos \
  -m "$LETSENCRYPT_EMAIL"

"$SCRIPT_DIR/install.sh"
reload_web
