#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: bash scripts/rollback_prod.sh <env-file> <rollback-manifest> <db-dump>

Default mode is simulation (`GM_ROLLBACK_MODE=simulate`).
Set `GM_ROLLBACK_MODE=apply` to perform the rollback on a staging-safe target.

Simulation validates the manifest, dump, compose env/Caddy restore plan, and exact Docker/DB commands without mutating the running deployment.
EOF
}

log() {
  printf 'rollback-prod: %s\n' "$*" >&2
}

die() {
  printf 'rollback-prod: ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command '$1' not found"
}

require_file() {
  [[ -f "$1" ]] || die "required file '$1' does not exist"
}

require_var() {
  local name=$1
  if [[ -z "${!name:-}" ]]; then
    die "required value '$name' is missing"
  fi
}

container_has_network() {
  local container_name=$1
  local network_name=$2
  docker inspect -f '{{range $name, $_ := .NetworkSettings.Networks}}{{println $name}}{{end}}' "$container_name" 2>/dev/null | grep -Fxq "$network_name"
}

validate_release_tag() {
  local release_tag=$1
  [[ "$release_tag" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]] || die "release tag '$release_tag' must match Docker tag charset [A-Za-z0-9_.-] and be <=128 chars"
}

validate_container_name() {
  local container_name=$1
  local label=$2
  [[ "$container_name" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || die "$label '$container_name' contains unsupported characters"
}

validate_existing_target_container() {
  local container_name=$1
  local expected_service=$2

  if ! docker inspect "$container_name" >/dev/null 2>&1; then
    return 0
  fi

  container_has_network "$container_name" projects || die "container '$container_name' is not attached to Docker network 'projects'"

  local compose_service
  compose_service=$(docker inspect -f '{{ index .Config.Labels "com.docker.compose.service" }}' "$container_name" 2>/dev/null || true)
  [[ "$compose_service" == "$expected_service" ]] || die "container '$container_name' belongs to compose service '$compose_service', expected '$expected_service'"
}

load_manifest_file() {
  local manifest_path=$1
  local line key value

  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ -n "$line" ]] || continue
    [[ "$line" == *=* ]] || die "manifest '$manifest_path' contains invalid line '$line'"

    key=${line%%=*}
    value=${line#*=}

    case "$key" in
      GM_RELEASE_TAG|GM_API_PREVIOUS_IMAGE|GM_WEB_PREVIOUS_IMAGE|GM_API_TARGET_IMAGE|GM_WEB_TARGET_IMAGE|DEPLOY_ENV_FILE|DEPLOY_COMPOSE_FILE|DEPLOY_RUN_DIR|DEPLOY_IMAGE_SOURCE|DEPLOY_BACKUP_ARTIFACT|DEPLOY_CADDY_SOURCE_CONFIG|DEPLOY_PUBLIC_HOSTNAME|DEPLOY_COMPOSE_ENV_PATH|CADDY_CONTAINER_NAME|CADDY_SITE_CONFIG_PATH|CADDY_RUN_CONFIG_PATH|CADDY_RUN_CONFIG_ADAPTER|DEPLOY_PRE_CUTOVER_STATE|DEPLOY_COMPOSE_ENV_STAGED_FROM|DEPLOY_COMPOSE_ENV_BACKUP|DEPLOY_POST_CUTOVER_STATE|DEPLOY_POST_COMPOSE_INSPECT|CADDY_BACKUP_METADATA)
        printf -v "$key" '%s' "$value"
        ;;
      *)
        die "manifest '$manifest_path' contains unsupported key '$key'"
        ;;
    esac
  done <"$manifest_path"
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

discover_db_client_path() {
  if docker inspect "$DB_HOST" >/dev/null 2>&1; then
    DB_CONTAINER=$DB_HOST
    DB_IMAGE=$(docker inspect -f '{{.Config.Image}}' "$DB_CONTAINER")
    return 0
  fi

  DB_CONTAINER=
  DB_IMAGE=${PG_CLIENT_IMAGE:-pgvector/pgvector:pg16}
  docker network inspect projects >/dev/null 2>&1 || die "DB host '$DB_HOST' is not a container and Docker network 'projects' is unavailable"
}

validate_dump() {
  if [[ -n "$DB_CONTAINER" ]]; then
    docker exec -i "$DB_CONTAINER" pg_restore --list >/dev/null < "$DUMP_FILE"
    return 0
  fi

  docker run --rm -i "$DB_IMAGE" pg_restore --list >/dev/null < "$DUMP_FILE"
}

set_or_append_env_key() {
  local file_path=$1
  local key=$2
  local value=$3

  python3 - "$file_path" "$key" "$value" <<'PY'
import sys
from pathlib import Path

file_path, key, value = sys.argv[1:4]
path = Path(file_path)
lines = path.read_text(encoding='utf-8').splitlines()
updated = False
for index, line in enumerate(lines):
    if line.startswith(f"{key}="):
        lines[index] = f"{key}={value}"
        updated = True
        break
if not updated:
    lines.append(f"{key}={value}")
path.write_text("\n".join(lines) + "\n", encoding='utf-8')
PY
}

stage_compose_env_for_rollback() {
  mkdir -p "$(dirname "$COMPOSE_ENV_PATH")"

  if [[ -n "${DEPLOY_COMPOSE_ENV_BACKUP:-}" && -f "$DEPLOY_COMPOSE_ENV_BACKUP" ]]; then
    cp "$DEPLOY_COMPOSE_ENV_BACKUP" "$COMPOSE_ENV_PATH"
    log "restored compose env backup '$DEPLOY_COMPOSE_ENV_BACKUP' -> '$COMPOSE_ENV_PATH'"
  else
    cp "$ENV_FILE" "$COMPOSE_ENV_PATH"
    log "compose env backup unavailable; staged current env '$ENV_FILE' -> '$COMPOSE_ENV_PATH'"
  fi

  set_or_append_env_key "$COMPOSE_ENV_PATH" GM_RELEASE_TAG "$ROLLBACK_RELEASE_TAG"
  chmod 600 "$COMPOSE_ENV_PATH"
}

tag_previous_images_for_compose() {
  require_previous_images_present
  docker tag "$GM_API_PREVIOUS_IMAGE" "gm-api:$ROLLBACK_RELEASE_TAG"
  docker tag "$GM_WEB_PREVIOUS_IMAGE" "gm-web:$ROLLBACK_RELEASE_TAG"
}

require_previous_images_present() {
  docker image inspect "$GM_API_PREVIOUS_IMAGE" >/dev/null 2>&1 || die "previous API image '$GM_API_PREVIOUS_IMAGE' is not present locally"
  docker image inspect "$GM_WEB_PREVIOUS_IMAGE" >/dev/null 2>&1 || die "previous web image '$GM_WEB_PREVIOUS_IMAGE' is not present locally"
}

restore_caddy_snapshot() {
  if [[ -z "$CADDY_BACKUP_METADATA_FILE" ]]; then
    log "no Caddy backup metadata recorded in rollback manifest; skipping Caddy restore"
    return 0
  fi

  require_file "$CADDY_BACKUP_METADATA_FILE"

  set -a
  # shellcheck disable=SC1090
  . "$CADDY_BACKUP_METADATA_FILE"
  set +a

  require_var CADDY_CONTAINER_NAME
  require_var CADDY_SITE_CONFIG_PATH
  require_var CADDY_RUN_CONFIG_PATH
  require_var CADDY_RUN_CONFIG_ADAPTER

  if [[ "${CADDY_SITE_BACKUP_STATE:-}" == "present" ]]; then
    require_file "$CADDY_SITE_BACKUP_FILE"
    docker cp "$CADDY_SITE_BACKUP_FILE" "$CADDY_CONTAINER_NAME:${CADDY_SITE_CONFIG_PATH}.rollback-tmp"
    docker exec \
      -e TMP_TARGET_PATH="${CADDY_SITE_CONFIG_PATH}.rollback-tmp" \
      -e TARGET_PATH="$CADDY_SITE_CONFIG_PATH" \
      "$CADDY_CONTAINER_NAME" \
      sh -ceu 'mv "$TMP_TARGET_PATH" "$TARGET_PATH"'
  else
    docker exec -e TARGET_PATH="$CADDY_SITE_CONFIG_PATH" "$CADDY_CONTAINER_NAME" sh -ceu 'rm -f "$TARGET_PATH"'
  fi

  if [[ -f "${ACTIVE_CADDY_CONFIG_BACKUP:-}" ]]; then
    docker cp "$ACTIVE_CADDY_CONFIG_BACKUP" "$CADDY_CONTAINER_NAME:$CADDY_RUN_CONFIG_PATH"
  fi

  docker exec \
    -e RUN_CONFIG_PATH="$CADDY_RUN_CONFIG_PATH" \
    -e RUN_CONFIG_ADAPTER="$CADDY_RUN_CONFIG_ADAPTER" \
    "$CADDY_CONTAINER_NAME" \
    sh -ceu 'caddy reload --config "$RUN_CONFIG_PATH" --adapter "$RUN_CONFIG_ADAPTER" >/dev/null'
}

restore_db_dump() {
  if [[ -n "$DB_CONTAINER" ]]; then
    docker exec -i "$DB_CONTAINER" pg_restore \
      --clean \
      --if-exists \
      --no-owner \
      --no-privileges \
      --exit-on-error \
      --dbname="$GM_DB_URL" < "$DUMP_FILE"
    return 0
  fi

  docker run --rm -i --network projects "$DB_IMAGE" pg_restore \
    --clean \
    --if-exists \
    --no-owner \
    --no-privileges \
    --exit-on-error \
    --dbname="$GM_DB_URL" < "$DUMP_FILE"
}

wait_for_container_state() {
  local container_name=$1
  local desired_state=$2
  local deadline=$((SECONDS + 180))
  local current_state

  while (( SECONDS < deadline )); do
    current_state=$(docker inspect -f '{{.State.Status}}' "$container_name" 2>/dev/null || true)
    if [[ "$current_state" == "$desired_state" ]]; then
      return 0
    fi
    sleep 2
  done

  die "container '$container_name' did not reach state '$desired_state'"
}

remove_target_container_if_present() {
  local container_name=$1

  if ! docker inspect "$container_name" >/dev/null 2>&1; then
    return 0
  fi

  docker rm -f "$container_name" >/dev/null
}

quiesce_application_containers() {
  validate_existing_target_container "$GM_API_CONTAINER_NAME" api
  validate_existing_target_container "$GM_WEB_CONTAINER_NAME" web
  remove_target_container_if_present "$GM_API_CONTAINER_NAME"
  remove_target_container_if_present "$GM_WEB_CONTAINER_NAME"
}

simulate_plan() {
  local api_image_present web_image_present

  require_previous_images_present

  if docker image inspect "$GM_API_PREVIOUS_IMAGE" >/dev/null 2>&1; then
    api_image_present=yes
  else
    api_image_present=no
  fi
  if docker image inspect "$GM_WEB_PREVIOUS_IMAGE" >/dev/null 2>&1; then
    web_image_present=yes
  else
    web_image_present=no
  fi

  cat >"$RUN_DIR/rollback-plan.txt" <<EOF
Rollback mode: simulate
Compose file: $COMPOSE_FILE
Compose env path: $COMPOSE_ENV_PATH
Compose env backup: ${DEPLOY_COMPOSE_ENV_BACKUP:-none}
API container name: $GM_API_CONTAINER_NAME
Web container name: $GM_WEB_CONTAINER_NAME
Rollback release tag: $ROLLBACK_RELEASE_TAG
Previous API image: $GM_API_PREVIOUS_IMAGE (present locally: $api_image_present)
Previous web image: $GM_WEB_PREVIOUS_IMAGE (present locally: $web_image_present)
DB dump: $DUMP_FILE
DB target: $GM_DB_URL
DB restore strategy: pg_restore --clean --if-exists --no-owner --no-privileges --exit-on-error
Caddy metadata: ${CADDY_BACKUP_METADATA_FILE:-none}

Planned apply sequence:
  1. Validate dump with pg_restore --list
  2. docker tag "$GM_API_PREVIOUS_IMAGE" "gm-api:$ROLLBACK_RELEASE_TAG"
  3. docker tag "$GM_WEB_PREVIOUS_IMAGE" "gm-web:$ROLLBACK_RELEASE_TAG"
  4. Restore compose env backup to '$COMPOSE_ENV_PATH' and force GM_RELEASE_TAG=$ROLLBACK_RELEASE_TAG
  5. docker rm -f $GM_API_CONTAINER_NAME $GM_WEB_CONTAINER_NAME (when present) to quiesce app containers before DB restore
  6. Restore DB from '$DUMP_FILE' with pg_restore; do not run goose down or reverse migrations
  7. docker compose -f "$COMPOSE_FILE" --env-file "$COMPOSE_ENV_PATH" up -d --no-build --force-recreate api web
  8. Restore Caddy snapshot when metadata is present
EOF

  cat >"$SUMMARY_FILE" <<EOF
Rollback simulation succeeded.
Env file: $ENV_FILE
Manifest: $MANIFEST_FILE
Dump: $DUMP_FILE
Run dir: $RUN_DIR
Mode: simulate
Plan: $RUN_DIR/rollback-plan.txt
EOF
}

apply_rollback() {
  log "tagging previous images into compose-compatible rollback tags"
  tag_previous_images_for_compose

  log "staging compose env for rollback"
  stage_compose_env_for_rollback

  log "quiescing running application containers before DB restore"
  quiesce_application_containers

  log "restoring database dump via pg_restore"
  restore_db_dump

  log "restoring previous application images via docker compose"
  docker compose -f "$COMPOSE_FILE" --env-file "$COMPOSE_ENV_PATH" up -d --no-build --force-recreate api web
  wait_for_container_state "$GM_API_CONTAINER_NAME" running
  wait_for_container_state "$GM_WEB_CONTAINER_NAME" running

  log "restoring prior Caddy snapshot when present"
  restore_caddy_snapshot

  cat >"$SUMMARY_FILE" <<EOF
Rollback apply succeeded.
Env file: $ENV_FILE
Manifest: $MANIFEST_FILE
Dump: $DUMP_FILE
Run dir: $RUN_DIR
Mode: apply
Rollback release tag: $ROLLBACK_RELEASE_TAG
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

[[ $# -eq 3 ]] || {
  usage
  exit 1
}

ENV_FILE=$1
MANIFEST_FILE=$2
DUMP_FILE=$3
ROLLBACK_MODE=${GM_ROLLBACK_MODE:-simulate}

require_file "$ENV_FILE"
require_file "$MANIFEST_FILE"
require_file "$DUMP_FILE"

require_cmd bash
require_cmd cp
require_cmd date
require_cmd docker
require_cmd grep
require_cmd mkdir
require_cmd pgrep
require_cmd python3

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
EVIDENCE_DIR="$REPO_ROOT/.sisyphus/evidence"
RUN_TIMESTAMP=$(date -u +%Y%m%dT%H%M%SZ)
RUN_DIR="$EVIDENCE_DIR/task-8-rollback-$RUN_TIMESTAMP"
SUMMARY_FILE="$EVIDENCE_DIR/task-8-rollback-sim-pass.txt"
mkdir -p "$RUN_DIR"

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

load_manifest_file "$MANIFEST_FILE"

require_var GM_DB_URL
require_var GM_RELEASE_TAG
require_var GM_API_PREVIOUS_IMAGE
require_var GM_WEB_PREVIOUS_IMAGE

COMPOSE_FILE=${DEPLOY_COMPOSE_FILE:-$REPO_ROOT/docker-compose.prod.yml}
COMPOSE_ENV_PATH=${DEPLOY_COMPOSE_ENV_PATH:-$REPO_ROOT/.env}
CADDY_BACKUP_METADATA_FILE=${CADDY_BACKUP_METADATA:-}
ROLLBACK_RELEASE_TAG="rollback-previous"
GM_API_CONTAINER_NAME=${GM_API_CONTAINER_NAME:-gm-api}
GM_WEB_CONTAINER_NAME=${GM_WEB_CONTAINER_NAME:-gm-web}

validate_release_tag "$GM_RELEASE_TAG"
validate_release_tag "$ROLLBACK_RELEASE_TAG"
validate_container_name "$GM_API_CONTAINER_NAME" "GM_API_CONTAINER_NAME"
validate_container_name "$GM_WEB_CONTAINER_NAME" "GM_WEB_CONTAINER_NAME"

require_file "$COMPOSE_FILE"

parse_db_url
discover_db_client_path
validate_dump || die "database dump '$DUMP_FILE' is not a valid custom-format dump"

case "$ROLLBACK_MODE" in
  simulate)
    simulate_plan
    ;;
  apply)
    apply_rollback
    ;;
  *)
    die "GM_ROLLBACK_MODE must be 'simulate' or 'apply'"
    ;;
esac

log "rollback flow complete -> $SUMMARY_FILE"
