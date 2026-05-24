#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

source "$SCRIPT_DIR/lib/common.sh"

SECRET_FILE="${SECRET_FILE:-$SCRIPT_DIR/secret.sh}"
CONFIG_FILE="${CONFIG_FILE:-$SCRIPT_DIR/config.sh}"

require_file "$SECRET_FILE"
require_file "$CONFIG_FILE"

# shellcheck disable=SC1090
source "$SECRET_FILE"
# shellcheck disable=SC1090
source "$CONFIG_FILE"

require_vars SERVER_IP REMOTE_USER APP_ROOT

SSH_PORT="${SSH_PORT:-22}"
REMOTE_CONFIG_PATH="${REMOTE_CONFIG_PATH:-$APP_ROOT/install/config.sh}"
REMOTE_CONFIG_DIR="$(dirname "$REMOTE_CONFIG_PATH")"

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

log "Creating remote config directory"
"${SSH_CMD[@]}" "${SSH_OPTS[@]}" "$REMOTE_USER@$SERVER_IP" "mkdir -p '$REMOTE_CONFIG_DIR'"

log "Syncing install/config.sh to VPS"
rsync -av \
  -e "$RSYNC_RSH" \
  "$CONFIG_FILE" "$REMOTE_USER@$SERVER_IP:$REMOTE_CONFIG_PATH"

log "Restricting remote config permissions"
"${SSH_CMD[@]}" "${SSH_OPTS[@]}" "$REMOTE_USER@$SERVER_IP" "chmod 600 '$REMOTE_CONFIG_PATH'"

log "Secrets synced to $REMOTE_USER@$SERVER_IP:$REMOTE_CONFIG_PATH"
