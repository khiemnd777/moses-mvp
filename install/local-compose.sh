#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${INSTALL_CONFIG_FILE:-$SCRIPT_DIR/config.local.sh}"
COMMAND="${1:-up}"

source "$SCRIPT_DIR/lib/common.sh"

if [[ ! -f "$CONFIG_FILE" ]]; then
  if [[ "$CONFIG_FILE" == "$SCRIPT_DIR/config.local.sh" && -f "$SCRIPT_DIR/config.local.sh.sample" ]]; then
    cp "$SCRIPT_DIR/config.local.sh.sample" "$CONFIG_FILE"
    warn "Created $CONFIG_FILE from config.local.sh.sample; edit OPENAI_API_KEY if you need AI calls."
  else
    err "Missing local config: $CONFIG_FILE"
  fi
fi

export INSTALL_CONFIG_FILE="$CONFIG_FILE"
load_install_config "$SCRIPT_DIR"

compose_service_running() {
  local service="$1"
  compose_local "$SCRIPT_DIR" ps --status running --services | grep -qx "$service"
}

compose_status_rows() {
  compose_local "$SCRIPT_DIR" ps -a --format '{{.Service}}\t{{.State}}\t{{.Status}}'
}

local_compose_services() {
  compose_local "$SCRIPT_DIR" config --services
}

print_compose_status_rows() {
  local rows="$1"

  if [[ -z "$rows" ]]; then
    echo "No local Compose containers found."
    return
  fi

  printf '%-12s %-12s %s\n' "SERVICE" "STATE" "STATUS"
  printf '%s\n' "$rows" | awk -F '\t' '{ printf "%-12s %-12s %s\n", $1, $2, $3 }'
}

service_state_from_rows() {
  local rows="$1"
  local service="$2"

  awk -F '\t' -v svc="$service" '$1 == svc { print tolower($2); exit }' <<<"$rows"
}

all_services_running() {
  local rows="$1"
  shift

  local service state
  for service in "$@"; do
    state="$(service_state_from_rows "$rows" "$service")"
    [[ "$state" == "running" ]] || return 1
  done
}

all_services_stopped() {
  local rows="$1"
  shift

  local service state
  for service in "$@"; do
    state="$(service_state_from_rows "$rows" "$service")"
    case "$state" in
      ""|exited|dead)
        ;;
      *)
        return 1
        ;;
    esac
  done
}

all_services_removed() {
  local rows="$1"
  shift

  local service state
  for service in "$@"; do
    state="$(service_state_from_rows "$rows" "$service")"
    [[ -z "$state" ]] || return 1
  done
}

wait_for_compose_state() {
  local target="$1"
  local timeout="${LOCAL_COMPOSE_WAIT_TIMEOUT:-120}"
  local interval="${LOCAL_COMPOSE_WAIT_INTERVAL:-2}"
  local start elapsed rows
  local -a services=()

  [[ "$timeout" =~ ^[0-9]+$ ]] || err "LOCAL_COMPOSE_WAIT_TIMEOUT must be a whole number of seconds"
  while IFS= read -r service; do
    [[ -n "$service" ]] && services+=("$service")
  done < <(local_compose_services)
  [[ "${#services[@]}" -gt 0 ]] || err "No local Compose services found"

  case "$target" in
    running)
      log "Waiting for local Compose services to finish starting"
      ;;
    stopped)
      log "Waiting for local Compose services to finish stopping"
      ;;
    removed)
      log "Waiting for local Compose containers to be removed"
      ;;
    *)
      err "Unsupported compose wait target: $target"
      ;;
  esac

  start="$(date +%s)"
  while true; do
    rows="$(compose_status_rows)"
    print_compose_status_rows "$rows"

    case "$target" in
      running)
        if all_services_running "$rows" "${services[@]}"; then
          log "All local Compose services are running"
          return
        fi
        ;;
      stopped)
        if all_services_stopped "$rows" "${services[@]}"; then
          log "All local Compose services are stopped"
          return
        fi
        ;;
      removed)
        if all_services_removed "$rows" "${services[@]}"; then
          log "All local Compose containers are removed"
          return
        fi
        ;;
    esac

    elapsed=$(($(date +%s) - start))
    if (( elapsed >= timeout )); then
      err "Timed out waiting for local Compose services to reach state: $target"
    fi

    sleep "$interval"
  done
}

run_web_command() {
  if compose_service_running web; then
    compose_local "$SCRIPT_DIR" exec -T web "$@"
  else
    compose_local "$SCRIPT_DIR" run --rm --no-deps web "$@"
  fi
}

start_local_stack() {
  local action="$1"

  COMPOSE_RUNTIME=local SKIP_DOCKER_INSTALL=1 "$SCRIPT_DIR/backend/install.sh"
  log "$action local development Docker Compose stack"
  compose_local "$SCRIPT_DIR" up -d --build
  COMPOSE_RUNTIME=local "$SCRIPT_DIR/backend/migrate-compose.sh"
  wait_for_compose_state running
  log "Local dev stack is running at http://localhost:${HTTP_PORT:-19080}"
}

case "$COMMAND" in
  up)
    start_local_stack "Starting"
    ;;
  migrate)
    COMPOSE_RUNTIME=local "$SCRIPT_DIR/backend/migrate-compose.sh"
    ;;
  verify)
    COMPOSE_RUNTIME=local "$SCRIPT_DIR/backend/verify.sh"
    log "Frontend dev server smoke test"
    curl -fsSI "http://127.0.0.1:${HTTP_PORT:-19080}" >/dev/null
    log "Local dev web is responding at http://localhost:${HTTP_PORT:-19080}"
    ;;
  ps)
    compose_local "$SCRIPT_DIR" ps
    ;;
  logs)
    compose_local "$SCRIPT_DIR" logs -f api worker web
    ;;
  web-install|frontend-install)
    run_web_command bun install
    ;;
  web-add|frontend-add)
    shift || true
    [[ "$#" -gt 0 ]] || err "Usage: $0 $COMMAND <package...>"
    run_web_command bun add "$@"
    ;;
  down)
    compose_local "$SCRIPT_DIR" down
    wait_for_compose_state removed
    ;;
  stop)
    compose_local "$SCRIPT_DIR" stop
    wait_for_compose_state stopped
    ;;
  restart)
    start_local_stack "Restarting"
    ;;
  *)
    err "Usage: $0 [up|migrate|verify|ps|logs|web-install|web-add <package...>|down|stop|restart]"
    ;;
esac
