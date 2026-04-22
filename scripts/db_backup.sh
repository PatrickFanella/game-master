#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: bash scripts/db_backup.sh <env-file> <output-path>

Creates a custom-format Postgres dump using the existing database container when possible.
EOF
}

log() {
  printf 'db-backup: %s\n' "$*" >&2
}

die() {
  printf 'db-backup: ERROR: %s\n' "$*" >&2
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

resolve_db_container() {
  if docker inspect "$DB_HOST" >/dev/null 2>&1; then
    DB_CONTAINER=$DB_HOST
    DB_IMAGE=$(docker inspect -f '{{.Config.Image}}' "$DB_CONTAINER")
    return 0
  fi

  DB_CONTAINER=
  DB_IMAGE=${PG_CLIENT_IMAGE:-pgvector/pgvector:pg16}

  case "$DB_HOST" in
    localhost|127.0.0.1|::1|[::1])
      die "GM_DB_URL host '$DB_HOST' is not container-addressable; use the existing Postgres container hostname for production backup flow"
      ;;
  esac

  docker network inspect projects >/dev/null 2>&1 || die "could not resolve container '$DB_HOST' and Docker network 'projects' is unavailable for fallback client access"
}

run_pg_dump() {
  if [[ -n "$DB_CONTAINER" ]]; then
    docker exec -i "$DB_CONTAINER" pg_dump --dbname="$GM_DB_URL" --format=custom --no-owner --no-privileges
    return 0
  fi

  docker run --rm -i --network projects "$DB_IMAGE" pg_dump --dbname="$GM_DB_URL" --format=custom --no-owner --no-privileges
}

validate_dump() {
  local dump_file=$1

  if [[ -n "$DB_CONTAINER" ]]; then
    docker exec -i "$DB_CONTAINER" pg_restore --list >/dev/null < "$dump_file"
    return 0
  fi

  docker run --rm -i "$DB_IMAGE" pg_restore --list >/dev/null < "$dump_file"
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

[[ $# -eq 2 ]] || {
  usage
  exit 1
}

ENV_FILE=$1
OUTPUT_FILE=$2

[[ -f "$ENV_FILE" ]] || die "env file '$ENV_FILE' does not exist"
[[ -n "$OUTPUT_FILE" ]] || die "output path must not be empty"

require_cmd bash
require_cmd docker
require_cmd mktemp
require_cmd mv
require_cmd mkdir

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

require_var GM_DB_URL
parse_db_url
resolve_db_container

mkdir -p "$(dirname "$OUTPUT_FILE")"

TMP_OUTPUT=$(mktemp "${OUTPUT_FILE}.tmp.XXXXXX")
cleanup() {
  rm -f "$TMP_OUTPUT"
}
trap cleanup EXIT

log "creating custom-format dump at '$OUTPUT_FILE'"
run_pg_dump > "$TMP_OUTPUT"

[[ -s "$TMP_OUTPUT" ]] || die "backup artifact '$TMP_OUTPUT' is empty"
validate_dump "$TMP_OUTPUT" || die "backup artifact '$TMP_OUTPUT' is not a valid custom-format dump"

mv "$TMP_OUTPUT" "$OUTPUT_FILE"
chmod 600 "$OUTPUT_FILE"

log "backup complete via ${DB_CONTAINER:-client-container} -> '$OUTPUT_FILE'"
