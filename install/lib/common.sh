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

require_non_placeholder_var() {
  local name="$1"
  local value normalized

  value="${!name:-}"
  [[ -n "$value" ]] || err "Missing required variable: $name"

  normalized="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
  case "$normalized" in
    replace-with-*|your_*|your-*|changeme|change-me|placeholder|example|sample)
      err "$name is set to a placeholder; set a real production value outside git before deploy"
      ;;
  esac
}

require_min_length_var() {
  local name="$1"
  local min_length="$2"
  local value="${!name:-}"

  [[ -n "$value" ]] || err "Missing required variable: $name"
  if (( ${#value} < min_length )); then
    err "$name must be at least $min_length characters"
  fi
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

compose_project_name() {
  printf '%s\n' "${COMPOSE_PROJECT_NAME:-legal}"
}

compose_prod() {
  local install_dir="$1"
  shift

  local root env_file compose_file
  root="$(repo_root "$install_dir")"
  env_file="$root/backend/.env"
  compose_file="$root/install/docker-compose.prod.yml"

  require_file "$env_file"
  require_file "$compose_file"

  docker compose \
    --project-name "$(compose_project_name)" \
    --env-file "$env_file" \
    -f "$compose_file" \
    "$@"
}

compose_local() {
  local install_dir="$1"
  shift

  local root env_file compose_file
  root="$(repo_root "$install_dir")"
  env_file="$root/backend/.env"
  compose_file="$root/install/docker-compose.dev.yml"

  require_file "$env_file"
  require_file "$compose_file"

  docker compose \
    --project-name "$(compose_project_name)" \
    --env-file "$env_file" \
    -f "$compose_file" \
    "$@"
}
