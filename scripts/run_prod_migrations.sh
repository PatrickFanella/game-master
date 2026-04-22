#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: bash scripts/run_prod_migrations.sh <env-file> [goose-status-path]

Runs goose up in an ephemeral Go container and records goose status after success.
EOF
}

log() {
  printf 'db-migrate: %s\n' "$*" >&2
}

die() {
  printf 'db-migrate: ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command '$1' not found"
}

require_var() {
  local name=$1
  if [[ -z "${!name:-}" ]]; then
    die "required env var '$name' is missing or empty in $ENV_FILE"
  fi
}

parse_db_url() {
  local regex='^postgres(ql)?://([^:/?#]+)(:([^@/?#]*))?@([^/?#:]+|\[[^]]+\])(:([0-9]+))?/([^?]+)'
  if [[ ! "$GM_DB_URL" =~ $regex ]]; then
    die "GM_DB_URL must be a postgres:// URL with explicit user, host, port, and database name"
  fi

  DB_HOST=${BASH_REMATCH[5]}
}

resolve_network() {
  if docker inspect "$DB_HOST" >/dev/null 2>&1; then
    DB_NETWORK=$(docker inspect -f '{{range $name, $_ := .NetworkSettings.Networks}}{{println $name}}{{end}}' "$DB_HOST" | grep -Fx 'projects' | awk 'NR==1 {print; exit}')
    if [[ -z "$DB_NETWORK" ]]; then
      DB_NETWORK=$(docker inspect -f '{{range $name, $_ := .NetworkSettings.Networks}}{{println $name}}{{end}}' "$DB_HOST" | awk 'NR==1 {print; exit}')
    fi
  else
    DB_NETWORK=${MIGRATION_DOCKER_NETWORK:-projects}
  fi

  [[ -n "$DB_NETWORK" ]] || die "could not determine Docker network for migration runner"
  docker network inspect "$DB_NETWORK" >/dev/null 2>&1 || die "Docker network '$DB_NETWORK' is not available for migration runner"
}

run_goose_flow() {
  local status_dir status_basename

  status_dir=$(dirname "$STATUS_FILE")
  status_basename=$(basename "$STATUS_FILE")

  docker run --rm \
    --network "$DB_NETWORK" \
    -e GOOSE_DB_URL="$GM_DB_URL" \
    -e STATUS_BASENAME="$status_basename" \
    -v "$MIGRATIONS_DIR:/migrations:ro" \
    -v "$status_dir:/statusdir" \
    -v edda_goose_mod_cache:/go/pkg/mod \
    -v edda_goose_build_cache:/root/.cache/go-build \
    golang:1.25-alpine \
    sh -ceu '
      goose_cmd="github.com/pressly/goose/v3/cmd/goose@v3.24.1"
      go run "$goose_cmd" -dir /migrations postgres "$GOOSE_DB_URL" up
      go run "$goose_cmd" -dir /migrations postgres "$GOOSE_DB_URL" status > "/statusdir/$STATUS_BASENAME" 2>&1
      cat "/statusdir/$STATUS_BASENAME"
    '
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if (( $# < 1 || $# > 2 )); then
  usage
  exit 1
fi

ENV_FILE=$1
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
STATUS_FILE=${2:-$REPO_ROOT/.sisyphus/evidence/goose-status-$(date -u +%Y%m%dT%H%M%SZ).txt}
MIGRATIONS_DIR="$REPO_ROOT/migrations"

[[ -f "$ENV_FILE" ]] || die "env file '$ENV_FILE' does not exist"
[[ -d "$MIGRATIONS_DIR" ]] || die "migrations directory '$MIGRATIONS_DIR' does not exist"

require_cmd bash
require_cmd docker
require_cmd mkdir
require_cmd tee

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

require_var GM_DB_URL
parse_db_url
resolve_network

mkdir -p "$(dirname "$STATUS_FILE")"

log "verifying required Postgres extensions before migration"
bash "$SCRIPT_DIR/check_db_extensions.sh" "$ENV_FILE"

log "running goose up on network '$DB_NETWORK'"
run_goose_flow >/dev/null

[[ -s "$STATUS_FILE" ]] || die "goose status artifact '$STATUS_FILE' is empty"

log "migration flow complete; goose status stored at '$STATUS_FILE'"
