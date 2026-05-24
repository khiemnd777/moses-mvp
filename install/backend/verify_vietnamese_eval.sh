#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BACKEND_DIR="$PROJECT_ROOT/backend"

log() {
  printf '\033[1;34m[INFO]\033[0m %s\n' "$*"
}

log "Vietnamese eval suite: normalization, metadata, retrieval, and citation validation"
cd "$BACKEND_DIR"
GOCACHE="${GOCACHE:-$BACKEND_DIR/.gocache}" go test ./core/language ./core/legalmeta ./core/retrieval ./api -run VietnameseEval
log "Vietnamese eval suite passed"
