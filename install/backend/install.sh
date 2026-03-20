#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
ensure_repo_present "$INSTALL_DIR"

PROJECT_ROOT="$(repo_root "$INSTALL_DIR")"
BACKEND_DIR="$PROJECT_ROOT/backend"

require_vars JWT_SECRET ADMIN_BOOTSTRAP_PASSWORD OPENAI_API_KEY POSTGRES_DSN QDRANT_URL QDRANT_COLLECTION
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

mkdir -p "$BACKEND_DIR/data/postgres" "$BACKEND_DIR/data/qdrant" "$BACKEND_DIR/data/uploads"

log "Rendering backend/.env"
cat > "$BACKEND_DIR/.env" <<EOF
JWT_SECRET=$JWT_SECRET
ADMIN_BOOTSTRAP_PASSWORD=$ADMIN_BOOTSTRAP_PASSWORD
PORT=${BACKEND_PORT:-8080}
EOF

log "Rendering backend/config/config.yaml"
cat > "$BACKEND_DIR/config/config.yaml" <<EOF
server:
  host: "0.0.0.0"
  port: ${BACKEND_PORT:-8080}
postgres:
  dsn: "$POSTGRES_DSN"
qdrant:
  url: "$QDRANT_URL"
  collection: "$QDRANT_COLLECTION"
openai:
  api_key: "$OPENAI_API_KEY"
  embeddings_model: "${OPENAI_EMBEDDINGS_MODEL:-text-embedding-3-small}"
  chat_model: "${OPENAI_CHAT_MODEL:-gpt-4.1-mini}"
storage:
  root_dir: "${STORAGE_ROOT_DIR:-./data/uploads}"
ingest:
  default_segmenter: "${INGEST_DEFAULT_SEGMENTER:-free_text}"
  chunk_size: ${INGEST_CHUNK_SIZE:-800}
  chunk_overlap: ${INGEST_CHUNK_OVERLAP:-100}
prompts:
  guard: "config/prompts/guard.yaml"
  tone_default: "config/prompts/tone_default.yaml"
  tone_academic: "config/prompts/tone_academic.yaml"
  tone_procedure: "config/prompts/tone_procedure.yaml"
vector_repair:
  enabled: ${VECTOR_REPAIR_ENABLED:-true}
  interval: ${VECTOR_REPAIR_INTERVAL:-30s}
  max_tasks_per_pass: ${VECTOR_REPAIR_MAX_TASKS_PER_PASS:-20}
EOF

if [[ "${SKIP_BACKEND_START:-0}" == "1" ]]; then
  warn "Skipping backend start because SKIP_BACKEND_START=1"
  exit 0
fi

log "Starting backend services"
cd "$BACKEND_DIR"
docker compose --env-file .env -f docker/docker-compose.yml up -d --build
