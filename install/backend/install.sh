#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
ensure_repo_present "$INSTALL_DIR"

PROJECT_ROOT="$(repo_root "$INSTALL_DIR")"
BACKEND_DIR="$PROJECT_ROOT/backend"

require_vars JWT_SECRET ADMIN_BOOTSTRAP_PASSWORD OPENAI_API_KEY POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB POSTGRES_HOST POSTGRES_PORT POSTGRES_SSLMODE QDRANT_HOST QDRANT_PORT QDRANT_COLLECTION
require_dir "$BACKEND_DIR/docker"
require_file "$BACKEND_DIR/docker/docker-compose.yml"

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

POSTGRES_DATA_DIR="${POSTGRES_DATA_DIR:-$BACKEND_DIR/data/postgres}"
QDRANT_STORAGE_DIR="${QDRANT_STORAGE_DIR:-$BACKEND_DIR/data/qdrant}"
UPLOADS_DATA_DIR="${UPLOADS_DATA_DIR:-$BACKEND_DIR/data/uploads}"

mkdir -p "$POSTGRES_DATA_DIR" "$QDRANT_STORAGE_DIR" "$UPLOADS_DATA_DIR"

log "Rendering backend/.env"
cat > "$BACKEND_DIR/.env" <<EOF
CONFIG_PATH=config/config.yaml
JWT_SECRET=$JWT_SECRET
ADMIN_BOOTSTRAP_PASSWORD=$ADMIN_BOOTSTRAP_PASSWORD
SERVER_HOST=${SERVER_HOST:-0.0.0.0}
PORT=${BACKEND_PORT:-8080}
POSTGRES_IMAGE=${POSTGRES_IMAGE:-postgres:15}
POSTGRES_USER=${POSTGRES_USER:-legal}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-legal}
POSTGRES_DB=${POSTGRES_DB:-legal_rag}
POSTGRES_HOST=${POSTGRES_HOST:-localhost}
POSTGRES_PORT=${POSTGRES_PORT:-5433}
DOCKER_POSTGRES_HOST=${DOCKER_POSTGRES_HOST:-postgres}
DOCKER_POSTGRES_PORT=${DOCKER_POSTGRES_PORT:-5432}
POSTGRES_SSLMODE=${POSTGRES_SSLMODE:-disable}
DATABASE_URL=${DATABASE_URL:-}
POSTGRES_DATA_DIR=$POSTGRES_DATA_DIR
QDRANT_IMAGE=${QDRANT_IMAGE:-qdrant/qdrant:v1.9.3}
QDRANT_HOST=${QDRANT_HOST:-localhost}
QDRANT_PORT=${QDRANT_PORT:-6333}
DOCKER_QDRANT_HOST=${DOCKER_QDRANT_HOST:-qdrant}
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
CORS_ALLOWED_ORIGINS=${CORS_ALLOWED_ORIGINS:-http://localhost:5173,https://${DOMAIN}}
QDRANT_STORAGE_DIR=$QDRANT_STORAGE_DIR
UPLOADS_DATA_DIR=$UPLOADS_DATA_DIR
EOF

if [[ "${SKIP_BACKEND_START:-0}" == "1" ]]; then
  warn "Skipping backend start because SKIP_BACKEND_START=1"
  exit 0
fi

log "Starting backend services"
cd "$BACKEND_DIR"
docker compose --env-file .env -f docker/docker-compose.yml up -d --build
