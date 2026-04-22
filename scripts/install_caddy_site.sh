#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: bash scripts/install_caddy_site.sh <env-file> <source-caddyfile>

Copies the repo-owned edda Caddy site config into the running Caddy container,
backs up the previous site config, validates the site + active Caddy config, and reloads Caddy.
EOF
}

log() {
  printf 'caddy-install: %s\n' "$*" >&2
}

die() {
  printf 'caddy-install: ERROR: %s\n' "$*" >&2
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

validate_local_source() {
  local source_dir source_base

  source_dir=$(cd "$(dirname "$SOURCE_CONFIG")" && pwd -P)
  source_base=$(basename "$SOURCE_CONFIG")

  docker run --rm \
    -v "$source_dir:/config:ro" \
    caddy:2 \
      caddy validate --config "/config/$source_base" --adapter caddyfile >/dev/null
}

root_config_has_import() {
  docker exec \
    -e RUN_CONFIG_PATH="$CADDY_RUN_CONFIG_PATH" \
    -e TARGET_PATH="$CADDY_SITE_CONFIG_PATH" \
    "$CADDY_CONTAINER_NAME" \
    sh -ceu 'grep -Fq "import $TARGET_PATH" "$RUN_CONFIG_PATH"'
}

discover_runtime_config() {
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

snapshot_previous_config() {
  local backup_state

  mkdir -p "$BACKUP_DIR"

  docker cp "$CADDY_CONTAINER_NAME:$CADDY_RUN_CONFIG_PATH" "$ACTIVE_CONFIG_BACKUP"

  if docker exec -e TARGET_PATH="$CADDY_SITE_CONFIG_PATH" "$CADDY_CONTAINER_NAME" sh -ceu 'test -f "$TARGET_PATH"'; then
    PREVIOUS_CONFIG_PRESENT=1
    docker cp "$CADDY_CONTAINER_NAME:$CADDY_SITE_CONFIG_PATH" "$BACKUP_FILE"
    backup_state=present
  else
    PREVIOUS_CONFIG_PRESENT=0
    : > "$ABSENT_MARKER"
    backup_state=absent
  fi

  cat >"$BACKUP_METADATA" <<EOF
CADDY_CONTAINER_NAME=$CADDY_CONTAINER_NAME
CADDY_SITE_CONFIG_PATH=$CADDY_SITE_CONFIG_PATH
CADDY_RUN_CONFIG_PATH=$CADDY_RUN_CONFIG_PATH
CADDY_RUN_CONFIG_ADAPTER=$CADDY_RUN_CONFIG_ADAPTER
CADDY_ROOT_IMPORT_PRESENT_BEFORE=$(( 1 - ACTIVE_CONFIG_MODIFIED ))
CADDY_SITE_BACKUP_STATE=$backup_state
CADDY_SITE_BACKUP_FILE=$BACKUP_FILE
CADDY_SITE_ABSENT_MARKER=$ABSENT_MARKER
ACTIVE_CADDY_CONFIG_BACKUP=$ACTIVE_CONFIG_BACKUP
ACTIVE_CADDY_CONFIG_MODIFIED=$ACTIVE_CONFIG_MODIFIED
EOF
}

ensure_active_config_import() {
  if root_config_has_import; then
    ACTIVE_CONFIG_MODIFIED=0
    return 0
  fi

  ACTIVE_CONFIG_MODIFIED=1

  docker exec \
    -e RUN_CONFIG_PATH="$CADDY_RUN_CONFIG_PATH" \
    -e TARGET_PATH="$CADDY_SITE_CONFIG_PATH" \
    "$CADDY_CONTAINER_NAME" \
    sh -ceu 'printf "\n# managed by scripts/install_caddy_site.sh for edda\nimport %s\n" "$TARGET_PATH" >> "$RUN_CONFIG_PATH"'
}

restore_previous_state() {
  if (( RESTORE_ON_ERROR == 0 )); then
    return 0
  fi

  log "restore triggered after failure"

  if (( PREVIOUS_CONFIG_PRESENT == 1 )); then
    docker cp "$BACKUP_FILE" "$CADDY_CONTAINER_NAME:$TMP_TARGET_PATH"
    docker exec \
      -e TMP_TARGET_PATH="$TMP_TARGET_PATH" \
      -e TARGET_PATH="$CADDY_SITE_CONFIG_PATH" \
      "$CADDY_CONTAINER_NAME" \
      sh -ceu 'mv "$TMP_TARGET_PATH" "$TARGET_PATH"'
  else
    docker exec -e TARGET_PATH="$CADDY_SITE_CONFIG_PATH" "$CADDY_CONTAINER_NAME" sh -ceu 'rm -f "$TARGET_PATH"'
  fi

  if (( ACTIVE_CONFIG_MODIFIED == 1 )); then
    docker cp "$ACTIVE_CONFIG_BACKUP" "$CADDY_CONTAINER_NAME:$CADDY_RUN_CONFIG_PATH"
  fi

  docker exec \
    -e RUN_CONFIG_PATH="$CADDY_RUN_CONFIG_PATH" \
    -e RUN_CONFIG_ADAPTER="$CADDY_RUN_CONFIG_ADAPTER" \
    "$CADDY_CONTAINER_NAME" \
    sh -ceu 'caddy reload --config "$RUN_CONFIG_PATH" --adapter "$RUN_CONFIG_ADAPTER" >/dev/null'
}

install_source_config() {
  local target_dir

  target_dir=$(dirname "$CADDY_SITE_CONFIG_PATH")
  docker exec -e TARGET_DIR="$target_dir" "$CADDY_CONTAINER_NAME" sh -ceu 'mkdir -p "$TARGET_DIR"'
  docker cp "$SOURCE_CONFIG" "$CADDY_CONTAINER_NAME:$TMP_TARGET_PATH"
  docker exec \
    -e TMP_TARGET_PATH="$TMP_TARGET_PATH" \
    -e TARGET_PATH="$CADDY_SITE_CONFIG_PATH" \
    "$CADDY_CONTAINER_NAME" \
    sh -ceu 'mv "$TMP_TARGET_PATH" "$TARGET_PATH"'
}

validate_installed_configs() {
  docker exec \
    -e TARGET_PATH="$CADDY_SITE_CONFIG_PATH" \
    "$CADDY_CONTAINER_NAME" \
    sh -ceu 'caddy validate --config "$TARGET_PATH" --adapter caddyfile >/dev/null'

  docker exec \
    -e RUN_CONFIG_PATH="$CADDY_RUN_CONFIG_PATH" \
    -e RUN_CONFIG_ADAPTER="$CADDY_RUN_CONFIG_ADAPTER" \
    "$CADDY_CONTAINER_NAME" \
    sh -ceu 'caddy validate --config "$RUN_CONFIG_PATH" --adapter "$RUN_CONFIG_ADAPTER" >/dev/null'
}

reload_caddy() {
  docker exec \
    -e RUN_CONFIG_PATH="$CADDY_RUN_CONFIG_PATH" \
    -e RUN_CONFIG_ADAPTER="$CADDY_RUN_CONFIG_ADAPTER" \
    "$CADDY_CONTAINER_NAME" \
    sh -ceu 'caddy reload --config "$RUN_CONFIG_PATH" --adapter "$RUN_CONFIG_ADAPTER" >/dev/null'
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
SOURCE_CONFIG=$2

[[ -f "$ENV_FILE" ]] || die "env file '$ENV_FILE' does not exist"
[[ -f "$SOURCE_CONFIG" ]] || die "source config '$SOURCE_CONFIG' does not exist"

require_cmd bash
require_cmd docker
require_cmd dirname
require_cmd date
require_cmd mkdir

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

require_var CADDY_CONTAINER_NAME
require_var CADDY_SITE_CONFIG_PATH

docker inspect "$CADDY_CONTAINER_NAME" >/dev/null 2>&1 || die "Caddy container '$CADDY_CONTAINER_NAME' does not exist"

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
TIMESTAMP=$(date -u +%Y%m%dT%H%M%SZ)
BACKUP_DIR="$REPO_ROOT/.sisyphus/evidence/caddy-backups/$TIMESTAMP"
BACKUP_FILE="$BACKUP_DIR/previous-site-config.Caddyfile"
ABSENT_MARKER="$BACKUP_DIR/previous-site-config.absent"
BACKUP_METADATA="$BACKUP_DIR/backup-metadata.env"
ACTIVE_CONFIG_BACKUP="$BACKUP_DIR/active-root-config.Caddyfile"
TMP_TARGET_PATH="${CADDY_SITE_CONFIG_PATH}.tmp.${TIMESTAMP}"
RESTORE_ON_ERROR=0
PREVIOUS_CONFIG_PRESENT=0

trap 'restore_previous_state' ERR

log "validating local source config '$SOURCE_CONFIG'"
validate_local_source

discover_runtime_config
log "detected active Caddy config '$CADDY_RUN_CONFIG_PATH' (adapter: $CADDY_RUN_CONFIG_ADAPTER)"

[[ "$CADDY_SITE_CONFIG_PATH" != "$CADDY_RUN_CONFIG_PATH" ]] || die "CADDY_SITE_CONFIG_PATH must point to a dedicated site snippet, not the active root config '$CADDY_RUN_CONFIG_PATH'"

ACTIVE_CONFIG_MODIFIED=0
if root_config_has_import; then
  log "active Caddy config already imports '$CADDY_SITE_CONFIG_PATH'"
else
  ACTIVE_CONFIG_MODIFIED=1
  log "active Caddy config will be updated to import '$CADDY_SITE_CONFIG_PATH'"
fi

log "snapshotting prior site config to '$BACKUP_DIR'"
snapshot_previous_config
RESTORE_ON_ERROR=1

log "installing repo-owned site config to '$CADDY_SITE_CONFIG_PATH'"
install_source_config

ensure_active_config_import

log "validating installed site config and active Caddy config"
validate_installed_configs

log "reloading Caddy"
reload_caddy
RESTORE_ON_ERROR=0

log "install complete; backup metadata: '$BACKUP_METADATA'"
