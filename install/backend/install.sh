#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
ensure_repo_present "$INSTALL_DIR"

PROJECT_ROOT="$(repo_root "$INSTALL_DIR")"
BACKEND_DIR="$PROJECT_ROOT/backend"
DEFAULT_NGINX_CONF_FILE="$PROJECT_ROOT/install/nginx/rendered/default.conf"

require_vars JWT_SECRET ADMIN_BOOTSTRAP_PASSWORD OPENAI_API_KEY POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB POSTGRES_HOST POSTGRES_PORT POSTGRES_SSLMODE QDRANT_HOST QDRANT_PORT QDRANT_COLLECTION DOMAIN VITE_API_BASE_URL
case "${COMPOSE_RUNTIME:-prod}" in
  local|dev)
    ;;
  *)
    require_non_placeholder_var JWT_SECRET
    require_min_length_var JWT_SECRET 32
    require_non_placeholder_var ADMIN_BOOTSTRAP_PASSWORD
    require_min_length_var ADMIN_BOOTSTRAP_PASSWORD 12
    require_non_placeholder_var OPENAI_API_KEY
    ;;
esac
require_dir "$BACKEND_DIR/docker"
require_file "$PROJECT_ROOT/install/docker-compose.prod.yml"

if [[ "${SKIP_DOCKER_INSTALL:-0}" == "1" ]]; then
  log "Skipping Docker dependency install"
else
  log "Installing backend dependencies"
  sudo apt-get update -y
  sudo apt-get install -y ca-certificates curl gnupg lsb-release postgresql-client make

  if ! command -v docker >/dev/null 2>&1; then
    log "Installing Docker"
    sudo install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    sudo chmod a+r /etc/apt/keyrings/docker.gpg
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
      $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
      sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
    sudo apt-get update -y
    sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  fi

  sudo systemctl enable docker
  sudo systemctl start docker
fi

if ! command -v docker >/dev/null 2>&1; then
  err "Docker is required"
fi

POSTGRES_DATA_DIR="${POSTGRES_DATA_DIR:-$BACKEND_DIR/data/postgres}"
QDRANT_STORAGE_DIR="${QDRANT_STORAGE_DIR:-$BACKEND_DIR/data/qdrant}"
UPLOADS_DATA_DIR="${UPLOADS_DATA_DIR:-$BACKEND_DIR/data/uploads}"
CLAMAV_DB_DIR="${CLAMAV_DB_DIR:-$BACKEND_DIR/data/clamav}"
LETSENCRYPT_DIR="${LETSENCRYPT_DIR:-/etc/letsencrypt}"
CERTBOT_WEBROOT="${CERTBOT_WEBROOT:-/var/lib/legal_api/certbot/www}"
NGINX_CONF_FILE="${NGINX_CONF_FILE:-$DEFAULT_NGINX_CONF_FILE}"

case "${COMPOSE_RUNTIME:-prod}" in
  local|dev)
    DEFAULT_UPLOAD_AV_SCAN_MODE="disabled"
    ;;
  *)
    DEFAULT_UPLOAD_AV_SCAN_MODE="clamd"
    ;;
esac

UPLOAD_AV_SCAN_MODE="${UPLOAD_AV_SCAN_MODE:-$DEFAULT_UPLOAD_AV_SCAN_MODE}"
UPLOAD_AV_SCAN_MODE="$(printf '%s' "$UPLOAD_AV_SCAN_MODE" | tr '[:upper:]' '[:lower:]')"
UPLOAD_AV_CLAMD_ADDR="${UPLOAD_AV_CLAMD_ADDR:-tcp://clamav:3310}"
UPLOAD_AV_SCAN_TIMEOUT="${UPLOAD_AV_SCAN_TIMEOUT:-30s}"
UPLOAD_AV_FAIL_CLOSED="${UPLOAD_AV_FAIL_CLOSED:-true}"
UPLOAD_AV_FAIL_CLOSED="$(printf '%s' "$UPLOAD_AV_FAIL_CLOSED" | tr '[:upper:]' '[:lower:]')"
CLAMAV_IMAGE="${CLAMAV_IMAGE:-clamav/clamav:1.5_base}"
CLAMD_STARTUP_TIMEOUT="${CLAMD_STARTUP_TIMEOUT:-1800}"
FRESHCLAM_CHECKS="${FRESHCLAM_CHECKS:-12}"
case "$UPLOAD_AV_SCAN_MODE" in
  disabled|clamd)
    ;;
  *)
    err "UPLOAD_AV_SCAN_MODE must be disabled or clamd"
    ;;
esac
case "$UPLOAD_AV_FAIL_CLOSED" in
  true|false)
    ;;
  *)
    err "UPLOAD_AV_FAIL_CLOSED must be true or false"
    ;;
esac
case "${COMPOSE_RUNTIME:-prod}" in
  local|dev)
    ;;
  *)
    if [[ "$UPLOAD_AV_SCAN_MODE" == "disabled" && "${ALLOW_UPLOAD_AV_DISABLED:-0}" != "1" ]]; then
      err "UPLOAD_AV_SCAN_MODE=disabled is not allowed for production deploy; set clamd or ALLOW_UPLOAD_AV_DISABLED=1 for a break-glass deploy"
    fi
    if [[ "$UPLOAD_AV_FAIL_CLOSED" != "true" && "${ALLOW_UPLOAD_AV_FAIL_OPEN:-0}" != "1" ]]; then
      err "UPLOAD_AV_FAIL_CLOSED must be true for production deploy; set ALLOW_UPLOAD_AV_FAIL_OPEN=1 for a break-glass deploy"
    fi
    ;;
esac

mkdir -p "$POSTGRES_DATA_DIR" "$QDRANT_STORAGE_DIR" "$UPLOADS_DATA_DIR" "$CLAMAV_DB_DIR"

log "Rendering backend/.env"
cat > "$BACKEND_DIR/.env" <<EOF
CONFIG_PATH=config/config.yaml
COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME:-legal}
JWT_SECRET=$JWT_SECRET
ADMIN_BOOTSTRAP_PASSWORD=$ADMIN_BOOTSTRAP_PASSWORD
SERVER_HOST=${SERVER_HOST:-0.0.0.0}
PORT=${BACKEND_PORT:-18088}
API_BIND=${API_BIND:-127.0.0.1}
POSTGRES_IMAGE=${POSTGRES_IMAGE:-postgres:15}
POSTGRES_USER=${POSTGRES_USER:-legal}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-legal}
POSTGRES_DB=${POSTGRES_DB:-legal_rag}
POSTGRES_HOST=${POSTGRES_HOST:-localhost}
POSTGRES_PORT=${POSTGRES_PORT:-15433}
POSTGRES_BIND=${POSTGRES_BIND:-127.0.0.1}
DOCKER_POSTGRES_HOST=${DOCKER_POSTGRES_HOST:-postgres}
DOCKER_POSTGRES_PORT=${DOCKER_POSTGRES_PORT:-15432}
POSTGRES_SSLMODE=${POSTGRES_SSLMODE:-disable}
DATABASE_URL=${DATABASE_URL:-}
POSTGRES_DATA_DIR=$POSTGRES_DATA_DIR
QDRANT_IMAGE=${QDRANT_IMAGE:-qdrant/qdrant:v1.9.3}
QDRANT_HOST=${QDRANT_HOST:-localhost}
QDRANT_PORT=${QDRANT_PORT:-16334}
QDRANT_BIND=${QDRANT_BIND:-127.0.0.1}
DOCKER_QDRANT_HOST=${DOCKER_QDRANT_HOST:-qdrant}
DOCKER_QDRANT_PORT=${DOCKER_QDRANT_PORT:-16333}
QDRANT_COLLECTION=$QDRANT_COLLECTION
OPENAI_API_KEY=$OPENAI_API_KEY
OPENAI_EMBEDDINGS_MODEL=${OPENAI_EMBEDDINGS_MODEL:-text-embedding-3-small}
OPENAI_CHAT_MODEL=${OPENAI_CHAT_MODEL:-gpt-4.1-mini}
STORAGE_ROOT_DIR=${STORAGE_ROOT_DIR:-/app/data/uploads}
DOCKER_STORAGE_ROOT_DIR=${DOCKER_STORAGE_ROOT_DIR:-$STORAGE_ROOT_DIR}
INGEST_DEFAULT_SEGMENTER=${INGEST_DEFAULT_SEGMENTER:-free_text}
INGEST_CHUNK_SIZE=${INGEST_CHUNK_SIZE:-800}
INGEST_CHUNK_OVERLAP=${INGEST_CHUNK_OVERLAP:-100}
GUARD_PROMPT_PATH=${GUARD_PROMPT_PATH:-config/prompts/guard.yaml}
TONE_DEFAULT_PROMPT_PATH=${TONE_DEFAULT_PROMPT_PATH:-config/prompts/tone_default.yaml}
TONE_ACADEMIC_PROMPT_PATH=${TONE_ACADEMIC_PROMPT_PATH:-config/prompts/tone_academic.yaml}
TONE_PROCEDURE_PROMPT_PATH=${TONE_PROCEDURE_PROMPT_PATH:-config/prompts/tone_procedure.yaml}
VECTOR_REPAIR_ENABLED=${VECTOR_REPAIR_ENABLED:-true}
VECTOR_REPAIR_INTERVAL=${VECTOR_REPAIR_INTERVAL:-30s}
VECTOR_REPAIR_MAX_TASKS_PER_PASS=${VECTOR_REPAIR_MAX_TASKS_PER_PASS:-20}
UPLOAD_AV_SCAN_MODE=$UPLOAD_AV_SCAN_MODE
UPLOAD_AV_CLAMD_ADDR=$UPLOAD_AV_CLAMD_ADDR
UPLOAD_AV_SCAN_TIMEOUT=$UPLOAD_AV_SCAN_TIMEOUT
UPLOAD_AV_FAIL_CLOSED=$UPLOAD_AV_FAIL_CLOSED
CLAMAV_IMAGE=$CLAMAV_IMAGE
CLAMAV_DB_DIR=$CLAMAV_DB_DIR
CLAMD_STARTUP_TIMEOUT=$CLAMD_STARTUP_TIMEOUT
FRESHCLAM_CHECKS=$FRESHCLAM_CHECKS
CORS_ALLOWED_ORIGINS=${CORS_ALLOWED_ORIGINS:-http://localhost:5173,https://${DOMAIN}}
PUBLIC_BASE_URL=${PUBLIC_BASE_URL:-${WEB_PUBLIC_URL:-$VITE_API_BASE_URL}}
QDRANT_STORAGE_DIR=$QDRANT_STORAGE_DIR
UPLOADS_DATA_DIR=$UPLOADS_DATA_DIR
DOMAIN=$DOMAIN
HTTP_PORT=${HTTP_PORT:-80}
HTTPS_PORT=${HTTPS_PORT:-443}
HTTP_BIND=${HTTP_BIND:-0.0.0.0}
HTTPS_BIND=${HTTPS_BIND:-0.0.0.0}
LETSENCRYPT_DIR=$LETSENCRYPT_DIR
CERTBOT_WEBROOT=$CERTBOT_WEBROOT
NGINX_CONF_FILE=$NGINX_CONF_FILE
VITE_API_BASE_URL=$VITE_API_BASE_URL
VITE_ADMIN_API_KEY=${VITE_ADMIN_API_KEY:-}
VITE_ADMIN_BEARER_TOKEN=${VITE_ADMIN_BEARER_TOKEN:-}
EOF

if [[ "${SKIP_BACKEND_START:-0}" == "1" ]]; then
  warn "SKIP_BACKEND_START is set; production compose startup is handled by install/compose/up.sh"
fi
