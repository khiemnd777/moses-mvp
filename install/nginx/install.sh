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
SSL_CERT_PATH="/etc/letsencrypt/live/$DOMAIN/fullchain.pem"
SSL_KEY_PATH="/etc/letsencrypt/live/$DOMAIN/privkey.pem"

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

if [[ -f "$SSL_CERT_PATH" && -f "$SSL_KEY_PATH" ]]; then
  cat >> "$TMP_FILE" <<EOF

server {
    listen 443 ssl http2;
    server_name $DOMAIN;

    ssl_certificate $SSL_CERT_PATH;
    ssl_certificate_key $SSL_KEY_PATH;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    root $FRONTEND_ROOT;
    index index.html;

    location / {
        try_files \$uri \$uri/ /index.html;
    }

    location /auth/ {
        proxy_pass $BACKEND_UPSTREAM;
        include proxy_params;
    }

    location /admin/ {
        proxy_pass $BACKEND_UPSTREAM;
        include proxy_params;
    }

    location /documents {
        proxy_pass $BACKEND_UPSTREAM;
        include proxy_params;
    }

    location /document-versions/ {
        proxy_pass $BACKEND_UPSTREAM;
        include proxy_params;
    }

    location /doc-types {
        proxy_pass $BACKEND_UPSTREAM;
        include proxy_params;
    }

    location /ingest-jobs {
        proxy_pass $BACKEND_UPSTREAM;
        include proxy_params;
    }

    location /conversations {
        proxy_pass $BACKEND_UPSTREAM;
        include proxy_params;
    }

    location /messages {
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_buffering off;
        proxy_read_timeout 3600s;
        proxy_pass $BACKEND_UPSTREAM;
        include proxy_params;
    }

    location /search {
        proxy_pass $BACKEND_UPSTREAM;
        include proxy_params;
    }

    location /answer {
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_buffering off;
        proxy_read_timeout 3600s;
        proxy_pass $BACKEND_UPSTREAM;
        include proxy_params;
    }

    location /health {
        proxy_pass $BACKEND_UPSTREAM;
        include proxy_params;
    }

    location /metrics {
        proxy_pass $BACKEND_UPSTREAM;
        include proxy_params;
    }
}
EOF
fi

sudo cp "$TMP_FILE" "$SITE_PATH"
rm -f "$TMP_FILE"

sudo ln -sfn "$SITE_PATH" "$ENABLED_PATH"
if [[ -L /etc/nginx/sites-enabled/default ]]; then
  sudo rm -f /etc/nginx/sites-enabled/default
fi

sudo nginx -t
sudo systemctl enable nginx
sudo systemctl reload nginx || sudo systemctl restart nginx
