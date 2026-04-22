#!/usr/bin/env bash

set -euo pipefail

VERSION="1.0.0"
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
EVIDENCE_DIR="$REPO_ROOT/.sisyphus/evidence"
COMPOSE_FILE="$REPO_ROOT/docker-compose.prod.yml"
ROLLBACK_MANIFEST="$EVIDENCE_DIR/rollback-manifest.env"
BACKUP_ARTIFACT="$EVIDENCE_DIR/pre-deploy.dump"
COMPOSE_ENV_PATH="$REPO_ROOT/.env"

usage() {
  cat >&2 <<'EOF'
Usage: bash scripts/deploy_prod.sh <env-file>

Runs the production cutover sequence in strict order:
  1. preflight
  2. rollback manifest capture
  3. DB backup
  4. migrations
  5. build or pull tagged gm-api / gm-web images
  6. docker compose refresh
  7. Caddy install/reload if needed

Environment knobs:
  GM_DEPLOY_IMAGE_SOURCE=build|pull   (default: build)
  GM_DEPLOY_FORCE_CADDY_RELOAD=1      force Caddy install/reload even when config is unchanged
  GM_DEPLOY_CADDY_SOURCE_CONFIG=...   override repo-owned Caddyfile path

Required env contract comes from the supplied env file. Script is fail-fast and non-interactive.
EOF
}

version() {
  printf 'deploy_prod.sh %s\n' "$VERSION"
}

MAIN_LOG=

log() {
  if [[ -n "$MAIN_LOG" ]]; then
    printf 'deploy-prod: %s\n' "$*" | tee -a "$MAIN_LOG" >&2
  else
    printf 'deploy-prod: %s\n' "$*" >&2
  fi
}

die() {
  log "ERROR: $*"
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

load_env() {
  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
}

discover_caddy_runtime_config() {
  local -a argv=()
  local arg idx

  while IFS= read -r arg; do
    [[ -n "$arg" ]] && argv+=("$arg")
  done < <(docker inspect -f '{{range .Config.Entrypoint}}{{println .}}{{end}}{{range .Config.Cmd}}{{println .}}{{end}}' "$CADDY_CONTAINER_NAME")

  CADDY_RUN_CONFIG_PATH=/etc/caddy/Caddyfile
  CADDY_RUN_CONFIG_ADAPTER=caddyfile

  for ((idx = 0; idx < ${#argv[@]}; idx++)); do
    case "${argv[$idx]}" in
      --config)
        if (( idx + 1 < ${#argv[@]} )); then
          CADDY_RUN_CONFIG_PATH=${argv[$((idx + 1))]}
        fi
        ;;
      --adapter)
        if (( idx + 1 < ${#argv[@]} )); then
          CADDY_RUN_CONFIG_ADAPTER=${argv[$((idx + 1))]}
        fi
        ;;
    esac
  done
}

append_manifest_line() {
  local key=$1
  local value=$2
  printf '%s=%s\n' "$key" "$value" >>"$ROLLBACK_MANIFEST"
}

run_step() {
  local step_name=$1
  shift

  local step_log="$RUN_DIR/${step_name}.log"
  log "[$step_name] starting"

  if "$@" >"$step_log" 2>&1; then
    log "[$step_name] complete -> $step_log"
    return 0
  fi

  if [[ -s "$step_log" ]]; then
    log "[$step_name] failure output follows"
    cat "$step_log" >&2
  fi
  die "step '$step_name' failed; see $step_log"
}

caddy_reload_required() {
  local current_copy
  local import_probe_file

  if [[ "${GM_DEPLOY_FORCE_CADDY_RELOAD:-0}" == "1" ]]; then
    return 0
  fi

   import_probe_file="$RUN_DIR/caddy-root-import-check.txt"

  if ! docker exec \
    -e RUN_CONFIG_PATH="$CADDY_RUN_CONFIG_PATH" \
    -e TARGET_PATH="$CADDY_SITE_CONFIG_PATH" \
    "$CADDY_CONTAINER_NAME" \
    sh -ceu 'grep -Fq "import $TARGET_PATH" "$RUN_CONFIG_PATH"'; then
    cat >"$import_probe_file" <<EOF
missing-root-import=1
CADDY_CONTAINER_NAME=$CADDY_CONTAINER_NAME
CADDY_RUN_CONFIG_PATH=$CADDY_RUN_CONFIG_PATH
CADDY_SITE_CONFIG_PATH=$CADDY_SITE_CONFIG_PATH
EOF
    return 0
  fi

  cat >"$import_probe_file" <<EOF
missing-root-import=0
CADDY_CONTAINER_NAME=$CADDY_CONTAINER_NAME
CADDY_RUN_CONFIG_PATH=$CADDY_RUN_CONFIG_PATH
CADDY_SITE_CONFIG_PATH=$CADDY_SITE_CONFIG_PATH
EOF

  if ! docker exec -e TARGET_PATH="$CADDY_SITE_CONFIG_PATH" "$CADDY_CONTAINER_NAME" sh -ceu 'test -f "$TARGET_PATH"'; then
    return 0
  fi

  current_copy="$RUN_DIR/current-caddy.Caddyfile"
  docker cp "$CADDY_CONTAINER_NAME:$CADDY_SITE_CONFIG_PATH" "$current_copy" >/dev/null

  if cmp -s "$CADDY_SOURCE_CONFIG" "$current_copy"; then
    return 1
  fi

  return 0
}

prepare_release_images() {
  case "$IMAGE_SOURCE" in
    build)
      docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" build api web
      ;;
    pull)
      docker pull "gm-api:$GM_RELEASE_TAG"
      docker pull "gm-web:$GM_RELEASE_TAG"
      ;;
    *)
      die "GM_DEPLOY_IMAGE_SOURCE must be 'build' or 'pull' (got '$IMAGE_SOURCE')"
      ;;
  esac
}

wait_for_container_running() {
  local container_name=$1
  local deadline=$((SECONDS + 180))
  local state

  while (( SECONDS < deadline )); do
    state=$(docker inspect -f '{{.State.Status}}' "$container_name" 2>/dev/null || true)
    if [[ "$state" == "running" ]]; then
      return 0
    fi
    sleep 2
  done

  die "container '$container_name' did not reach running state"
}

wait_for_container_healthy() {
  local container_name=$1
  local deadline=$((SECONDS + 180))
  local state health

  while (( SECONDS < deadline )); do
    state=$(docker inspect -f '{{.State.Status}}' "$container_name" 2>/dev/null || true)
    health=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$container_name" 2>/dev/null || true)

    if [[ "$state" == "running" && "$health" == "healthy" ]]; then
      return 0
    fi

    if [[ "$state" == "exited" || "$health" == "unhealthy" ]]; then
      die "container '$container_name' is '$state' with health '$health'"
    fi

    sleep 2
  done

  die "container '$container_name' did not reach healthy state"
}

refresh_compose_services() {
  docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d --no-build --force-recreate api web
}

remove_target_container_if_present() {
  local container_name=$1

  if ! docker inspect "$container_name" >/dev/null 2>&1; then
    return 0
  fi

  docker rm -f "$container_name"
}

handoff_existing_app_containers() {
  remove_target_container_if_present "$WEB_CONTAINER_NAME"
  remove_target_container_if_present "$API_CONTAINER_NAME"
}

stage_compose_env() {
  mkdir -p "$RUN_DIR"

  if [[ -f "$COMPOSE_ENV_PATH" ]]; then
    COMPOSE_ENV_BACKUP="$RUN_DIR/compose-env.before"
    cp "$COMPOSE_ENV_PATH" "$COMPOSE_ENV_BACKUP"
  else
    COMPOSE_ENV_BACKUP=
  fi

  cp "$ENV_FILE" "$COMPOSE_ENV_PATH"
  chmod 600 "$COMPOSE_ENV_PATH"
}

latest_caddy_backup_metadata() {
  local latest
  latest=$(ls -1t "$EVIDENCE_DIR"/caddy-backups/*/backup-metadata.env 2>/dev/null | head -n 1 || true)
  printf '%s\n' "$latest"
}

record_running_state() {
  docker ps --filter "name=^/${API_CONTAINER_NAME}$" --filter "name=^/${WEB_CONTAINER_NAME}$" --format '{{.Names}} {{.Image}} {{.Status}}'
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ "${1:-}" == "--version" ]]; then
  version
  exit 0
fi

[[ $# -eq 1 ]] || {
  usage
  exit 1
}

ENV_FILE=$1
[[ -f "$ENV_FILE" ]] || die "env file '$ENV_FILE' does not exist"
[[ -f "$COMPOSE_FILE" ]] || die "compose file '$COMPOSE_FILE' does not exist"

require_cmd bash
require_cmd cmp
require_cmd cp
require_cmd date
require_cmd docker
require_cmd head
require_cmd ls
require_cmd mkdir
require_cmd tee

load_env

require_var GM_RELEASE_TAG
require_var CADDY_CONTAINER_NAME
require_var CADDY_SITE_CONFIG_PATH

IMAGE_SOURCE=${GM_DEPLOY_IMAGE_SOURCE:-build}
PUBLIC_HOSTNAME=${GM_DEPLOY_PUBLIC_HOSTNAME:-edda.subcult.tv}
CADDY_SOURCE_CONFIG=${GM_DEPLOY_CADDY_SOURCE_CONFIG:-$REPO_ROOT/deploy/caddy/edda.Caddyfile}
[[ -f "$CADDY_SOURCE_CONFIG" ]] || die "Caddy source config '$CADDY_SOURCE_CONFIG' does not exist"

API_CONTAINER_NAME=${GM_API_CONTAINER_NAME:-gm-api}
WEB_CONTAINER_NAME=${GM_WEB_CONTAINER_NAME:-gm-web}

RUN_TIMESTAMP=$(date -u +%Y%m%dT%H%M%SZ)
RUN_DIR="$EVIDENCE_DIR/deploy-prod-$RUN_TIMESTAMP"
mkdir -p "$RUN_DIR"
MAIN_LOG="$RUN_DIR/deploy.log"
touch "$MAIN_LOG"

discover_caddy_runtime_config

log "run dir: $RUN_DIR"
log "env file: $ENV_FILE"
log "release tag: $GM_RELEASE_TAG"
log "image source: $IMAGE_SOURCE"
log "compose file: $COMPOSE_FILE"
log "public hostname: $PUBLIC_HOSTNAME"

record_running_state >"$RUN_DIR/pre-cutover-state.txt" || true

run_step preflight bash "$SCRIPT_DIR/preflight_prod_deploy.sh" "$ENV_FILE" "$PUBLIC_HOSTNAME"

run_step capture-release env \
  GM_API_CURRENT_IMAGE="${GM_API_CURRENT_IMAGE:-}" \
  GM_WEB_CURRENT_IMAGE="${GM_WEB_CURRENT_IMAGE:-}" \
  bash "$SCRIPT_DIR/capture_prod_release.sh" "$GM_RELEASE_TAG" "$ROLLBACK_MANIFEST"

append_manifest_line DEPLOY_ENV_FILE "$ENV_FILE"
append_manifest_line DEPLOY_COMPOSE_FILE "$COMPOSE_FILE"
append_manifest_line DEPLOY_RUN_DIR "$RUN_DIR"
append_manifest_line DEPLOY_IMAGE_SOURCE "$IMAGE_SOURCE"
append_manifest_line DEPLOY_BACKUP_ARTIFACT "$BACKUP_ARTIFACT"
append_manifest_line DEPLOY_CADDY_SOURCE_CONFIG "$CADDY_SOURCE_CONFIG"
append_manifest_line DEPLOY_PUBLIC_HOSTNAME "$PUBLIC_HOSTNAME"
append_manifest_line DEPLOY_COMPOSE_ENV_PATH "$COMPOSE_ENV_PATH"
append_manifest_line CADDY_CONTAINER_NAME "$CADDY_CONTAINER_NAME"
append_manifest_line CADDY_SITE_CONFIG_PATH "$CADDY_SITE_CONFIG_PATH"
append_manifest_line CADDY_RUN_CONFIG_PATH "$CADDY_RUN_CONFIG_PATH"
append_manifest_line CADDY_RUN_CONFIG_ADAPTER "$CADDY_RUN_CONFIG_ADAPTER"
append_manifest_line DEPLOY_PRE_CUTOVER_STATE "$RUN_DIR/pre-cutover-state.txt"

run_step db-backup bash "$SCRIPT_DIR/db_backup.sh" "$ENV_FILE" "$BACKUP_ARTIFACT"
run_step db-migrate bash "$SCRIPT_DIR/run_prod_migrations.sh" "$ENV_FILE" "$RUN_DIR/goose-status.txt"
run_step image-prepare prepare_release_images
run_step stage-compose-env stage_compose_env
append_manifest_line DEPLOY_COMPOSE_ENV_STAGED_FROM "$ENV_FILE"
append_manifest_line DEPLOY_COMPOSE_ENV_BACKUP "${COMPOSE_ENV_BACKUP:-}"
run_step container-handoff handoff_existing_app_containers
run_step compose-refresh refresh_compose_services

wait_for_container_healthy "$API_CONTAINER_NAME"
wait_for_container_running "$WEB_CONTAINER_NAME"
docker inspect "$API_CONTAINER_NAME" "$WEB_CONTAINER_NAME" >"$RUN_DIR/post-compose-inspect.json"
record_running_state >"$RUN_DIR/post-cutover-state.txt"

append_manifest_line DEPLOY_POST_CUTOVER_STATE "$RUN_DIR/post-cutover-state.txt"
append_manifest_line DEPLOY_POST_COMPOSE_INSPECT "$RUN_DIR/post-compose-inspect.json"

if caddy_reload_required; then
  run_step caddy-install bash "$SCRIPT_DIR/install_caddy_site.sh" "$ENV_FILE" "$CADDY_SOURCE_CONFIG"
  CADDY_BACKUP_METADATA=$(latest_caddy_backup_metadata)
  if [[ -n "$CADDY_BACKUP_METADATA" ]]; then
    append_manifest_line CADDY_BACKUP_METADATA "$CADDY_BACKUP_METADATA"
  fi
else
  log "[caddy-install] skipped; deployed config already matches '$CADDY_SOURCE_CONFIG' and root import is present"
  printf 'skipped\n' >"$RUN_DIR/caddy-install.log"
fi

log "rollback manifest: $ROLLBACK_MANIFEST"
log "backup artifact: $BACKUP_ARTIFACT"
log "goose status artifact: $RUN_DIR/goose-status.txt"
log "deploy complete; services behind existing Caddy container '$CADDY_CONTAINER_NAME'"
