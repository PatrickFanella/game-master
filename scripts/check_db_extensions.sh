#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: bash scripts/check_db_extensions.sh <env-file>

Verifies that vector and pgcrypto are available and installable before migrations.
EOF
}

log() {
  printf 'db-extensions: %s\n' "$*" >&2
}

die() {
  printf 'db-extensions: ERROR: %s\n' "$*" >&2
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
  DB_NAME=${BASH_REMATCH[8]}
  DB_USER=${BASH_REMATCH[2]}
}

resolve_db_client() {
  if docker inspect "$DB_HOST" >/dev/null 2>&1; then
    DB_CONTAINER=$DB_HOST
    DB_IMAGE=$(docker inspect -f '{{.Config.Image}}' "$DB_CONTAINER")
    return 0
  fi

  DB_CONTAINER=
  DB_IMAGE=${PG_CLIENT_IMAGE:-pgvector/pgvector:pg16}

  case "$DB_HOST" in
    localhost|127.0.0.1|::1|[::1])
      die "GM_DB_URL host '$DB_HOST' is not container-addressable; use the existing Postgres container hostname for production extension checks"
      ;;
  esac

  docker network inspect projects >/dev/null 2>&1 || die "could not resolve container '$DB_HOST' and Docker network 'projects' is unavailable for fallback client access"
}

run_psql() {
  if [[ -n "$DB_CONTAINER" ]]; then
    docker exec -i "$DB_CONTAINER" psql "$GM_DB_URL" -v ON_ERROR_STOP=1 -X -qAt
    return 0
  fi

  docker run --rm -i --network projects "$DB_IMAGE" psql "$GM_DB_URL" -v ON_ERROR_STOP=1 -X -qAt
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

[[ $# -eq 1 ]] || {
  usage
  exit 1
}

ENV_FILE=$1

[[ -f "$ENV_FILE" ]] || die "env file '$ENV_FILE' does not exist"

require_cmd bash
require_cmd docker
require_cmd grep

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

require_var GM_DB_URL
parse_db_url
resolve_db_client

mapfile -t available_extensions < <(run_psql <<'SQL'
SELECT name
FROM pg_available_extensions
WHERE name IN ('pgcrypto', 'vector')
ORDER BY name;
SQL
)

[[ " ${available_extensions[*]} " == *" pgcrypto "* ]] || die "required extension 'pgcrypto' is not available on database '$DB_NAME'"
[[ " ${available_extensions[*]} " == *" vector "* ]] || die "required extension 'vector' is not available on database '$DB_NAME'"

create_output=$(
  run_psql 2>&1 <<'SQL'
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
SQL
) || die "failed to ensure extensions 'vector' and 'pgcrypto' for role '$DB_USER' on database '$DB_NAME'; ensure the role can CREATE extensions and the server has both installed. Details: $create_output"

mapfile -t installed_extensions < <(run_psql <<'SQL'
SELECT extname
FROM pg_extension
WHERE extname IN ('pgcrypto', 'vector')
ORDER BY extname;
SQL
)

[[ " ${installed_extensions[*]} " == *" pgcrypto "* ]] || die "extension 'pgcrypto' is still missing after verification; ensure role '$DB_USER' can create extensions on database '$DB_NAME'"
[[ " ${installed_extensions[*]} " == *" vector "* ]] || die "extension 'vector' is still missing after verification; ensure role '$DB_USER' can create extensions on database '$DB_NAME'"

log "verified extensions for '$DB_NAME': ${installed_extensions[*]}"
