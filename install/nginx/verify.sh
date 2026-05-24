#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
require_vars DOMAIN

log "Container Nginx config test"
compose_prod "$INSTALL_DIR" exec -T web nginx -t

if [[ -n "${WEB_PUBLIC_URL:-}" ]]; then
  log "Public web smoke test"
  curl -fsSIk "$WEB_PUBLIC_URL"
  exit 0
fi

http_url="http://$DOMAIN"
if [[ "${HTTP_PORT:-80}" != "80" ]]; then
  http_url="http://$DOMAIN:${HTTP_PORT}"
fi

https_url="https://$DOMAIN"
if [[ "${HTTPS_PORT:-443}" != "443" ]]; then
  https_url="https://$DOMAIN:${HTTPS_PORT}"
fi

if [[ "${ENABLE_SSL:-1}" == "1" ]]; then
  log "HTTPS smoke test"
  curl -fsSIk "$https_url"
else
  log "HTTP smoke test"
  curl -fsSI "$http_url"
fi
