#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: bash scripts/preflight_prod_deploy.sh <env-file> <public-hostname>

Validates the production env contract and Docker prerequisites for the single-host deploy.
EOF
}

log() {
  printf 'preflight: %s\n' "$*" >&2
}

die() {
  printf 'preflight: ERROR: %s\n' "$*" >&2
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

container_has_network() {
  local container_name=$1
  local network_name=$2
  docker inspect -f '{{range $name, $_ := .NetworkSettings.Networks}}{{println $name}}{{end}}' "$container_name" 2>/dev/null | grep -Fxq "$network_name"
}

validate_host_container_name() {
  local container_name=$1
  local expected_service=$2

  if ! docker inspect "$container_name" >/dev/null 2>&1; then
    log "container '$container_name' is unused and available"
    return 0
  fi

  if ! container_has_network "$container_name" projects; then
    die "container '$container_name' already exists but is not attached to Docker network 'projects'; remove or rename it before deploy"
  fi

  local compose_service
  compose_service=$(docker inspect -f '{{ index .Config.Labels "com.docker.compose.service" }}' "$container_name" 2>/dev/null || true)
  if [[ -n "$compose_service" && "$compose_service" != "$expected_service" ]]; then
    die "container '$container_name' already exists for compose service '$compose_service', expected '$expected_service'"
  fi

  log "container name '$container_name' is either unused or intentionally reusable for service '$expected_service'"
}

check_file_permissions() {
  local stat_out file_mode file_owner current_uid perm_octal

  stat_out=$(stat -c '%a %u' "$ENV_FILE") || die "unable to stat env file '$ENV_FILE'"
  file_mode=${stat_out%% *}
  file_owner=${stat_out##* }
  current_uid=$(id -u)
  perm_octal=$((8#$file_mode))

  if (( (perm_octal & 077) != 0 )); then
    die "env file '$ENV_FILE' must not be readable by group/others (recommended: chmod 600 $ENV_FILE; current mode: $file_mode)"
  fi

  if (( (perm_octal & 0400) == 0 )); then
    die "env file '$ENV_FILE' must be readable by its owner"
  fi

  if [[ "$file_owner" != "$current_uid" && "$file_owner" != "0" ]]; then
    die "env file '$ENV_FILE' must be owned by the current user or root (owner uid: $file_owner, current uid: $current_uid)"
  fi
}

parse_db_url() {
  local regex='^postgres(ql)?://([^:/?#]+)(:([^@/?#]*))?@([^/?#:]+|\[[^]]+\])(:([0-9]+))?/([^?]+)'
  if [[ ! "$GM_DB_URL" =~ $regex ]]; then
    die "GM_DB_URL must be a postgres:// URL with explicit user, host, port, and database name"
  fi

  DB_USER=${BASH_REMATCH[2]}
  DB_HOST=${BASH_REMATCH[5]}
  DB_PORT=${BASH_REMATCH[7]:-5432}
  DB_NAME=${BASH_REMATCH[8]}
}

reject_localhost_defaults() {
  case "$DB_HOST" in
    localhost|127.0.0.1|::1|[::1])
      die "GM_DB_URL must not point at localhost for container deployment; use the existing Postgres container hostname instead"
      ;;
  esac

  if [[ "${GM_LLM_PROVIDER:-}" == "ollama" ]]; then
    [[ "$GM_LLM_OLLAMA_ENDPOINT" == "http://10.0.0.10:11434" ]] || die "GM_LLM_OLLAMA_ENDPOINT must be exactly 'http://10.0.0.10:11434' for production"
    [[ "$GM_LLM_OLLAMA_EMBEDDINGENDPOINT" == "http://ollama:11434" ]] || die "GM_LLM_OLLAMA_EMBEDDINGENDPOINT must be exactly 'http://ollama:11434' for production"
  fi
}

check_provider_contract() {
  case "${GM_LLM_PROVIDER:-}" in
    claude)
      if [[ -z "${GM_LLM_CLAUDE_APIKEY:-}" && -z "${GM_CLAUDE_API_KEY:-}" && -z "${ANTHROPIC_API_KEY:-}" ]]; then
        die "claude mode requires one of GM_LLM_CLAUDE_APIKEY, GM_CLAUDE_API_KEY, or ANTHROPIC_API_KEY"
      fi
      require_var GM_LLM_CLAUDE_MODEL
      require_var GM_LLM_CLAUDE_CONTEXTTOKENBUDGET
      ;;
    ollama)
      require_var GM_LLM_OLLAMA_ENDPOINT
      require_var GM_LLM_OLLAMA_MODEL
      require_var GM_LLM_OLLAMA_EMBEDDINGENDPOINT
      require_var GM_LLM_OLLAMA_EMBEDDINGMODEL
      require_var GM_LLM_OLLAMA_CONTEXTTOKENBUDGET
      require_var GM_LLM_OLLAMA_TIMEOUTSECONDS
      ;;
    *)
      die "GM_LLM_PROVIDER must be exactly 'claude' or 'ollama'"
      ;;
  esac
}

check_postgres_reachable() {
  local running_state health_state exec_status

  if docker inspect "$DB_HOST" >/dev/null 2>&1; then
    container_has_network "$DB_HOST" projects || die "Postgres container '$DB_HOST' is not attached to Docker network 'projects'"

    running_state=$(docker inspect -f '{{.State.Status}}' "$DB_HOST")
    [[ "$running_state" == "running" ]] || die "Postgres container '$DB_HOST' is not running (status: $running_state)"

    health_state=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$DB_HOST")
    if [[ -n "$health_state" && "$health_state" != "healthy" ]]; then
      die "Postgres container '$DB_HOST' is not healthy (health: $health_state)"
    fi

    if ! exec_status=$(docker exec "$DB_HOST" pg_isready -h 127.0.0.1 -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" 2>&1); then
      die "Postgres readiness check failed inside '$DB_HOST': $exec_status"
    fi

    log "Postgres container '$DB_HOST' is reachable and ready"
    return 0
  fi

  if ! bash -c "exec 3<>/dev/tcp/$DB_HOST/$DB_PORT" 2>/dev/null; then
    die "could not open TCP connection to Postgres host '$DB_HOST:$DB_PORT'; ensure it is reachable from the Docker host and attached to the API network path"
  fi
  exec 3>&-
  exec 3<&-
  log "Postgres host '$DB_HOST:$DB_PORT' accepted a TCP connection"
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
PUBLIC_HOSTNAME=$2

[[ -f "$ENV_FILE" ]] || die "env file '$ENV_FILE' does not exist"
[[ -n "$PUBLIC_HOSTNAME" ]] || die "public hostname argument must not be empty"

require_cmd bash
require_cmd docker
require_cmd stat
require_cmd grep

check_file_permissions

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

API_CONTAINER_NAME=${GM_API_CONTAINER_NAME:-gm-api}
WEB_CONTAINER_NAME=${GM_WEB_CONTAINER_NAME:-gm-web}

require_var GM_DB_URL
require_var GM_SERVER_PORT
require_var GM_SERVER_JWTSECRET
require_var CADDY_CONTAINER_NAME
require_var CADDY_SITE_CONFIG_PATH
require_var CLOUDFLARE_ZONE_ID
require_var CLOUDFLARE_API_TOKEN
require_var CLOUDFLARE_RECORD_TARGET

[[ "$GM_SERVER_PORT" =~ ^[0-9]+$ ]] || die "GM_SERVER_PORT must be numeric"

parse_db_url
check_provider_contract
reject_localhost_defaults

docker network inspect projects >/dev/null 2>&1 || die "Docker network 'projects' not found; create it before deploy"

docker inspect "$CADDY_CONTAINER_NAME" >/dev/null 2>&1 || die "Caddy container '$CADDY_CONTAINER_NAME' does not exist"
container_has_network "$CADDY_CONTAINER_NAME" projects || die "Caddy container '$CADDY_CONTAINER_NAME' is not attached to Docker network 'projects'"

validate_host_container_name "$API_CONTAINER_NAME" api
validate_host_container_name "$WEB_CONTAINER_NAME" web
check_postgres_reachable

log "validated production env contract for '$PUBLIC_HOSTNAME' using '$ENV_FILE'"
