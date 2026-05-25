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
require_non_placeholder_var JWT_SECRET
require_min_length_var JWT_SECRET 32
require_non_placeholder_var ADMIN_BOOTSTRAP_PASSWORD
require_non_placeholder_var OPENAI_API_KEY

SSH_PORT="${SSH_PORT:-22}"
REMOTE_CONFIG_PATH="${REMOTE_CONFIG_PATH:-$APP_ROOT/install/config.sh}"
REMOTE_CONFIG_DIR="$(dirname "$REMOTE_CONFIG_PATH")"
SYNC_GITHUB_ACTIONS_SECRETS="${SYNC_GITHUB_ACTIONS_SECRETS:-1}"
SYNC_DEPLOY_KEY="${SYNC_DEPLOY_KEY:-1}"
GITHUB_ENVIRONMENT="${GITHUB_ENVIRONMENT:-production}"
GITHUB_DEPLOY_KEY_PATH="${GITHUB_DEPLOY_KEY_PATH:-$HOME/.ssh/legal_api_github_actions}"
GITHUB_DEPLOY_KEY_COMMENT="${GITHUB_DEPLOY_KEY_COMMENT:-github-actions-legal-api}"

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

github_repo() {
  if [[ -n "${GITHUB_REPO:-}" ]]; then
    printf '%s\n' "$GITHUB_REPO"
    return
  fi

  local origin
  origin="$(git -C "$SCRIPT_DIR/.." config --get remote.origin.url || true)"
  case "$origin" in
    git@github.com:*)
      origin="${origin#git@github.com:}"
      printf '%s\n' "${origin%.git}"
      ;;
    https://github.com/*)
      origin="${origin#https://github.com/}"
      printf '%s\n' "${origin%.git}"
      ;;
    *)
      err "Set GITHUB_REPO=owner/repo in secret.sh; could not infer it from origin remote"
      ;;
  esac
}

ensure_github_deploy_key() {
  if [[ "$SYNC_DEPLOY_KEY" != "1" ]]; then
    return
  fi

  if [[ ! -f "$GITHUB_DEPLOY_KEY_PATH" ]]; then
    log "Generating GitHub Actions deploy key at $GITHUB_DEPLOY_KEY_PATH"
    mkdir -p "$(dirname "$GITHUB_DEPLOY_KEY_PATH")"
    chmod 700 "$(dirname "$GITHUB_DEPLOY_KEY_PATH")"
    ssh-keygen -t ed25519 -N "" -C "$GITHUB_DEPLOY_KEY_COMMENT" -f "$GITHUB_DEPLOY_KEY_PATH" >/dev/null
    chmod 600 "$GITHUB_DEPLOY_KEY_PATH"
  fi

  require_file "$GITHUB_DEPLOY_KEY_PATH"
  require_file "$GITHUB_DEPLOY_KEY_PATH.pub"

  log "Installing GitHub Actions deploy public key on VPS"
  local public_key
  public_key="$(cat "$GITHUB_DEPLOY_KEY_PATH.pub")"
  printf '%s\n' "$public_key" | "${SSH_CMD[@]}" "${SSH_OPTS[@]}" "$REMOTE_USER@$SERVER_IP" '
    set -eu
    umask 077
    mkdir -p ~/.ssh
    touch ~/.ssh/authorized_keys
    read -r key
    grep -qxF "$key" ~/.ssh/authorized_keys || printf "%s\n" "$key" >> ~/.ssh/authorized_keys
    chmod 700 ~/.ssh
    chmod 600 ~/.ssh/authorized_keys
  '

  log "Testing deploy key SSH access"
  ssh -i "$GITHUB_DEPLOY_KEY_PATH" \
    -p "$SSH_PORT" \
    -o BatchMode=yes \
    -o StrictHostKeyChecking=accept-new \
    "$REMOTE_USER@$SERVER_IP" "true"
}

sync_github_actions_secrets() {
  if [[ "$SYNC_GITHUB_ACTIONS_SECRETS" != "1" ]]; then
    return
  fi

  command -v gh >/dev/null 2>&1 || err "gh is required to sync GitHub Actions secrets"
  command -v ssh-keyscan >/dev/null 2>&1 || err "ssh-keyscan is required to sync VPS_KNOWN_HOSTS"
  gh auth status >/dev/null

  local repo known_hosts
  repo="$(github_repo)"

  if [[ "$SYNC_DEPLOY_KEY" == "1" ]]; then
    require_file "$GITHUB_DEPLOY_KEY_PATH"
  fi

  if ! gh api "repos/$repo/environments/$GITHUB_ENVIRONMENT" >/dev/null 2>&1; then
    log "Creating GitHub environment $GITHUB_ENVIRONMENT"
    gh api -X PUT "repos/$repo/environments/$GITHUB_ENVIRONMENT" >/dev/null
  fi

  log "Collecting VPS known_hosts entry"
  known_hosts="$(ssh-keyscan -p "$SSH_PORT" "$SERVER_IP" 2>/dev/null)"
  [[ -n "$known_hosts" ]] || err "Could not collect SSH host key for $SERVER_IP:$SSH_PORT"

  log "Syncing GitHub Actions production deploy secrets"
  gh secret set VPS_HOST --repo "$repo" --env "$GITHUB_ENVIRONMENT" --body "$SERVER_IP" >/dev/null
  gh secret set VPS_USER --repo "$repo" --env "$GITHUB_ENVIRONMENT" --body "$REMOTE_USER" >/dev/null
  gh secret set VPS_PORT --repo "$repo" --env "$GITHUB_ENVIRONMENT" --body "$SSH_PORT" >/dev/null
  gh secret set VPS_KNOWN_HOSTS --repo "$repo" --env "$GITHUB_ENVIRONMENT" --body "$known_hosts" >/dev/null

  if [[ "$SYNC_DEPLOY_KEY" == "1" ]]; then
    gh secret set VPS_SSH_KEY --repo "$repo" --env "$GITHUB_ENVIRONMENT" < "$GITHUB_DEPLOY_KEY_PATH" >/dev/null
  fi

  log "Syncing GitHub Actions production deploy variables"
  gh variable set VPS_APP_ROOT --repo "$repo" --env "$GITHUB_ENVIRONMENT" --body "$APP_ROOT" >/dev/null
}

log "Creating remote config directory"
"${SSH_CMD[@]}" "${SSH_OPTS[@]}" "$REMOTE_USER@$SERVER_IP" "mkdir -p '$REMOTE_CONFIG_DIR'"

log "Syncing install/config.sh to VPS"
rsync -av \
  -e "$RSYNC_RSH" \
  "$CONFIG_FILE" "$REMOTE_USER@$SERVER_IP:$REMOTE_CONFIG_PATH"

log "Restricting remote config permissions"
"${SSH_CMD[@]}" "${SSH_OPTS[@]}" "$REMOTE_USER@$SERVER_IP" "chmod 600 '$REMOTE_CONFIG_PATH'"

ensure_github_deploy_key
sync_github_actions_secrets

log "Secrets synced to VPS and GitHub Actions"
