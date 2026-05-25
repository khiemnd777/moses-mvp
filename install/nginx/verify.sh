#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
require_vars DOMAIN

log "Container Nginx config test"
compose_prod "$INSTALL_DIR" exec -T web nginx -t

smoke_path="${WEB_SMOKE_PATH:-/playground/login}"
container_url="http://127.0.0.1:${HTTP_PORT:-80}${smoke_path}"

extract_asset_refs() {
  grep -Eo '/assets/[^"]+\.(js|css)' | sort | tr '\n' ' '
}

if [[ -n "${WEB_PUBLIC_URL:-}" ]]; then
  log "Public web smoke test"
  public_url="${WEB_PUBLIC_URL%/}${smoke_path}"
  public_html="$(curl -fsSLk "$public_url")"
  container_html="$(curl -fsSL "$container_url")"
  public_assets="$(printf '%s' "$public_html" | extract_asset_refs)"
  container_assets="$(printf '%s' "$container_html" | extract_asset_refs)"
  [[ -n "$public_assets" ]] || err "Public web smoke test did not find built asset references at $public_url"
  [[ -n "$container_assets" ]] || err "Container web smoke test did not find built asset references at $container_url"
  if [[ "$public_assets" != "$container_assets" ]]; then
    err "Public web assets do not match container assets. Check the edge proxy upstream for $WEB_PUBLIC_URL"
  fi
  curl -fsSIk "$public_url"
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
