#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
ensure_repo_present "$INSTALL_DIR"
require_vars DOMAIN

PROJECT_ROOT="$(repo_root "$INSTALL_DIR")"
BACKEND_PORT_VALUE="${BACKEND_PORT:-18088}"
API_UPSTREAM="${NGINX_API_UPSTREAM:-http://api:${BACKEND_PORT_VALUE}}"
LETSENCRYPT_DIR="${LETSENCRYPT_DIR:-/etc/letsencrypt}"
CERTBOT_WEBROOT="${CERTBOT_WEBROOT:-/var/lib/legal_api/certbot/www}"
NGINX_RENDERED_DIR="$PROJECT_ROOT/install/nginx/rendered"
NGINX_CONF_FILE="${NGINX_CONF_FILE:-$NGINX_RENDERED_DIR/default.conf}"
SSL_CERT_PATH="$LETSENCRYPT_DIR/live/$DOMAIN/fullchain.pem"
SSL_KEY_PATH="$LETSENCRYPT_DIR/live/$DOMAIN/privkey.pem"

write_proxy_headers() {
  cat <<EOF
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
EOF
}

write_proxy_location() {
  local location="$1"
  local streaming="${2:-0}"

  cat <<EOF
    location $location {
EOF
  write_proxy_headers
  if [[ "$streaming" == "1" ]]; then
    cat <<EOF
        proxy_set_header Connection "";
        proxy_buffering off;
        proxy_read_timeout 3600s;
EOF
  fi
  cat <<EOF
        proxy_pass $API_UPSTREAM;
    }

EOF
}

write_app_locations() {
  cat <<EOF
    root /usr/share/nginx/html;
    index index.html;

    location / {
        try_files \$uri \$uri/ /index.html;
    }

    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

EOF
  write_proxy_location "/auth/"
  write_proxy_location "/admin/"
  write_proxy_location "/documents"
  write_proxy_location "/document-versions/"
  write_proxy_location "/doc-types"
  write_proxy_location "/ingest-jobs"
  write_proxy_location "/conversations"
  write_proxy_location "/messages" "1"
  write_proxy_location "/search"
  write_proxy_location "/answer" "1"
  write_proxy_location "/chat" "1"
  write_proxy_location "/citations/"
  write_proxy_location "~ ^/assets/[^/]+/download$"
  write_proxy_location "/health"
  write_proxy_location "/metrics"
}

ensure_writable_dir() {
  local path="$1"
  if mkdir -p "$path" 2>/dev/null; then
    return
  fi
  sudo mkdir -p "$path"
}

ensure_writable_dir "$CERTBOT_WEBROOT"
ensure_writable_dir "$LETSENCRYPT_DIR"
mkdir -p "$NGINX_RENDERED_DIR"

if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet nginx; then
  warn "Disabling host Nginx so the compose web container owns ports 80/443"
  sudo systemctl disable --now nginx
fi

TMP_FILE="$(mktemp)"

if [[ "${ENABLE_SSL:-1}" == "1" && "${FORCE_HTTP_ONLY:-0}" != "1" && -f "$SSL_CERT_PATH" && -f "$SSL_KEY_PATH" ]]; then
  log "Rendering container Nginx config with HTTPS"
  cat > "$TMP_FILE" <<EOF
server {
    listen 80;
    server_name $DOMAIN;

    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    location / {
        return 301 https://\$host\$request_uri;
    }
}

server {
    listen 443 ssl http2;
    server_name $DOMAIN;

    ssl_certificate /etc/letsencrypt/live/$DOMAIN/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/$DOMAIN/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers off;

EOF
  write_app_locations >> "$TMP_FILE"
  cat >> "$TMP_FILE" <<EOF
}
EOF
else
  log "Rendering container Nginx config with HTTP"
  cat > "$TMP_FILE" <<EOF
server {
    listen 80;
    server_name $DOMAIN;

EOF
  write_app_locations >> "$TMP_FILE"
  cat >> "$TMP_FILE" <<EOF
}
EOF
fi

cp "$TMP_FILE" "$NGINX_CONF_FILE"
rm -f "$TMP_FILE"

log "Rendered container Nginx config at $NGINX_CONF_FILE"
