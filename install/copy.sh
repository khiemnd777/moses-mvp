#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$SCRIPT_DIR/lib/common.sh"

SECRET_FILE="${SECRET_FILE:-$SCRIPT_DIR/secret.sh}"
require_file "$SECRET_FILE"
source "$SECRET_FILE"

if [[ -z "${SERVER_IP:-}" || -z "${REMOTE_USER:-}" || -z "${REMOTE_DIR:-}" ]]; then
  err "secret.sh must define SERVER_IP, REMOTE_USER, and REMOTE_DIR"
fi

SSH_PORT="${SSH_PORT:-22}"
SSH_OPTS=("-p" "$SSH_PORT")
if [[ -n "${SSH_IDENTITY_FILE:-}" ]]; then
  SSH_OPTS+=("-i" "$SSH_IDENTITY_FILE")
fi

log "Creating remote directory"
ssh "${SSH_OPTS[@]}" "$REMOTE_USER@$SERVER_IP" "mkdir -p '$REMOTE_DIR'"

log "Syncing install scripts"
rsync -av --delete \
  -e "ssh -p $SSH_PORT${SSH_IDENTITY_FILE:+ -i $SSH_IDENTITY_FILE}" \
  --exclude 'secret.sh' \
  --exclude '.DS_Store' \
  "$SCRIPT_DIR/" "$REMOTE_USER@$SERVER_IP:$REMOTE_DIR/install/"

log "Copy completed to $REMOTE_USER@$SERVER_IP:$REMOTE_DIR"
