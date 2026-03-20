#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$INSTALL_DIR/lib/common.sh"

load_install_config "$INSTALL_DIR"
require_vars GIT_REPO_URL GIT_BRANCH
require_vars APP_ROOT

log "Installing git"
sudo apt-get update -y
sudo apt-get install -y git

PARENT_DIR="$(dirname "$APP_ROOT")"
mkdir -p "$PARENT_DIR"

if [[ ! -d "$APP_ROOT/.git" ]]; then
  log "Cloning repository into $APP_ROOT"
  git clone --branch "$GIT_BRANCH" "$GIT_REPO_URL" "$APP_ROOT"
else
  log "Updating repository in $APP_ROOT"
  git -C "$APP_ROOT" fetch --all --tags
  git -C "$APP_ROOT" checkout "$GIT_BRANCH"
  git -C "$APP_ROOT" pull --ff-only origin "$GIT_BRANCH"
fi

if [[ -n "${GIT_COMMIT_SHA:-}" ]]; then
  log "Checking out commit $GIT_COMMIT_SHA"
  git -C "$APP_ROOT" checkout "$GIT_COMMIT_SHA"
fi
