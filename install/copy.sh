#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$SCRIPT_DIR/lib/common.sh"

SECRET_FILE="${SECRET_FILE:-$SCRIPT_DIR/secret.sh}"
require_file "$SECRET_FILE"
source "$SECRET_FILE"

SOURCE_DIR="${SOURCE_DIR:-$SCRIPT_DIR}"
DEST_DIR="${DEST_DIR:-${REMOTE_DIR:-}}"

if [[ -z "${SERVER_IP:-}" || -z "${REMOTE_USER:-}" || -z "${DEST_DIR:-}" ]]; then
  err "secret.sh must define SERVER_IP, REMOTE_USER, and DEST_DIR"
fi

SSH_PORT="${SSH_PORT:-22}"
SSH_OPTS=("-p" "$SSH_PORT")
if [[ -n "${SSH_IDENTITY_FILE:-}" ]]; then
  SSH_OPTS+=("-i" "$SSH_IDENTITY_FILE")
fi

if [[ -n "${PASSWORD:-}" ]]; then
  if ! command -v sshpass >/dev/null 2>&1; then
    err "PASSWORD is set but sshpass is not installed"
  fi
  SSH_CMD=(sshpass -p "$PASSWORD" ssh)
  RSYNC_RSH="sshpass -p $PASSWORD ssh -p $SSH_PORT"
  if [[ -n "${SSH_IDENTITY_FILE:-}" ]]; then
    RSYNC_RSH+=" -i $SSH_IDENTITY_FILE"
  fi
else
  SSH_CMD=(ssh)
  RSYNC_RSH="ssh -p $SSH_PORT"
  if [[ -n "${SSH_IDENTITY_FILE:-}" ]]; then
    RSYNC_RSH+=" -i $SSH_IDENTITY_FILE"
  fi
fi

log "Creating remote directory"
"${SSH_CMD[@]}" "${SSH_OPTS[@]}" "$REMOTE_USER@$SERVER_IP" "mkdir -p '$DEST_DIR'"

log "Syncing install scripts"
rsync -av --delete \
  -e "$RSYNC_RSH" \
  --exclude 'secret.sh' \
  --exclude '.DS_Store' \
  "$SOURCE_DIR/" "$REMOTE_USER@$SERVER_IP:$DEST_DIR/"

log "Setting executable permissions on remote scripts"
"${SSH_CMD[@]}" "${SSH_OPTS[@]}" "$REMOTE_USER@$SERVER_IP" \
  "find '$DEST_DIR' -type f \\( -name '*.sh' -o -name '*.bash' \\) -exec chmod 755 {} +"

log "Copy completed to $REMOTE_USER@$SERVER_IP:$DEST_DIR"
