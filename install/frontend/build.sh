#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
ensure_repo_present "$INSTALL_DIR"

PROJECT_ROOT="$(repo_root "$INSTALL_DIR")"
FRONTEND_DIR="$PROJECT_ROOT/frontend"
require_vars VITE_API_BASE_URL
require_dir "$FRONTEND_DIR"

cd "$FRONTEND_DIR"

log "Rendering frontend/.env.production.local"
cat > "$FRONTEND_DIR/.env.production.local" <<EOF
VITE_API_BASE_URL=$VITE_API_BASE_URL
EOF

if [[ -n "${VITE_ADMIN_API_KEY:-}" ]]; then
  cat >> "$FRONTEND_DIR/.env.production.local" <<EOF
VITE_ADMIN_API_KEY=$VITE_ADMIN_API_KEY
EOF
fi

if [[ -n "${VITE_ADMIN_BEARER_TOKEN:-}" ]]; then
  cat >> "$FRONTEND_DIR/.env.production.local" <<EOF
VITE_ADMIN_BEARER_TOKEN=$VITE_ADMIN_BEARER_TOKEN
EOF
fi

if ! command -v bun >/dev/null 2>&1; then
  log "Installing Bun"
  curl -fsSL https://bun.sh/install | bash
  export BUN_INSTALL="${BUN_INSTALL:-$HOME/.bun}"
  export PATH="$BUN_INSTALL/bin:$PATH"
fi

if ! command -v bun >/dev/null 2>&1; then
  err "Bun installation failed"
fi

if [[ -d "$FRONTEND_DIR/node_modules" ]]; then
  log "Removing existing frontend/node_modules before Bun install"
  rm -rf "$FRONTEND_DIR/node_modules"
fi

if [[ -f "$FRONTEND_DIR/package-lock.json" ]]; then
  log "Removing frontend/package-lock.json to avoid npm lockfile conflicts"
  rm -f "$FRONTEND_DIR/package-lock.json"
fi

log "Building frontend with Bun"
bun install --frozen-lockfile
bun run build
