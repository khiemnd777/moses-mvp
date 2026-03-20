#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '\033[1;34m[INFO]\033[0m %s\n' "$*"
}

warn() {
  printf '\033[1;33m[WARN]\033[0m %s\n' "$*"
}

err() {
  printf '\033[1;31m[ERROR]\033[0m %s\n' "$*" >&2
  exit 1
}

require_file() {
  local path="$1"
  [[ -f "$path" ]] || err "Missing required file: $path"
}

require_dir() {
  local path="$1"
  [[ -d "$path" ]] || err "Missing required directory: $path"
}

require_vars() {
  local name
  for name in "$@"; do
    [[ -n "${!name:-}" ]] || err "Missing required variable: $name"
  done
}

load_install_config() {
  local install_dir="$1"
  local config_file="${INSTALL_CONFIG_FILE:-$install_dir/config.sh}"

  require_file "$config_file"
  # shellcheck disable=SC1090
  source "$config_file"
}

repo_root() {
  local install_dir="$1"
  load_install_config "$install_dir"
  require_vars APP_ROOT

  printf '%s\n' "$APP_ROOT"
}

ensure_repo_present() {
  local install_dir="$1"
  local root

  root="$(repo_root "$install_dir")"
  if [[ -d "$root/.git" ]]; then
    return
  fi

  "$install_dir/repo/sync.sh"
}
