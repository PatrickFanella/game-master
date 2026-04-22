#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: bash scripts/capture_prod_release.sh <release-tag> <artifact-path>

Captures the current gm-api and gm-web image tags before cutover and records the next release tag.
EOF
}

log() {
  printf 'release-capture: %s\n' "$*" >&2
}

die() {
  printf 'release-capture: ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command '$1' not found"
}

container_has_network() {
  local container_name=$1
  local network_name=$2
  docker inspect -f '{{range $name, $_ := .NetworkSettings.Networks}}{{println $name}}{{end}}' "$container_name" 2>/dev/null | grep -Fxq "$network_name"
}

validate_release_tag() {
  [[ "$RELEASE_TAG" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]] || die "release tag '$RELEASE_TAG' must match Docker tag charset [A-Za-z0-9_.-] and be <=128 chars"
}

validate_container_name() {
  local container_name=$1
  local label=$2
  [[ "$container_name" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || die "$label '$container_name' contains unsupported characters"
}

validate_target_container() {
  local container_name=$1
  local expected_service=$2

  docker inspect "$container_name" >/dev/null 2>&1 || die "container '$container_name' does not exist"
  container_has_network "$container_name" projects || die "container '$container_name' is not attached to Docker network 'projects'"

  local compose_service
  compose_service=$(docker inspect -f '{{ index .Config.Labels "com.docker.compose.service" }}' "$container_name" 2>/dev/null || true)
  [[ "$compose_service" == "$expected_service" ]] || die "container '$container_name' belongs to compose service '$compose_service', expected '$expected_service'"
}

image_ref() {
  local container_name=$1
  docker inspect -f '{{.Config.Image}}' "$container_name" 2>/dev/null || true
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

[[ $# -eq 2 ]] || {
  usage
  exit 1
}

API_CONTAINER_NAME=${GM_API_CONTAINER_NAME:-gm-api}
WEB_CONTAINER_NAME=${GM_WEB_CONTAINER_NAME:-gm-web}

RELEASE_TAG=$1
ARTIFACT_PATH=$2

[[ -n "$RELEASE_TAG" ]] || die "release tag must not be empty"
[[ -n "$ARTIFACT_PATH" ]] || die "artifact path must not be empty"

require_cmd docker
require_cmd grep

validate_release_tag
validate_container_name "$API_CONTAINER_NAME" "GM_API_CONTAINER_NAME"
validate_container_name "$WEB_CONTAINER_NAME" "GM_WEB_CONTAINER_NAME"
validate_target_container "$API_CONTAINER_NAME" api
validate_target_container "$WEB_CONTAINER_NAME" web

API_CURRENT_IMAGE=${GM_API_CURRENT_IMAGE:-$(image_ref "$API_CONTAINER_NAME")}
WEB_CURRENT_IMAGE=${GM_WEB_CURRENT_IMAGE:-$(image_ref "$WEB_CONTAINER_NAME")}

[[ -n "$API_CURRENT_IMAGE" ]] || die "API current image is missing; set GM_API_CURRENT_IMAGE or ensure container '$API_CONTAINER_NAME' is running"
[[ -n "$WEB_CURRENT_IMAGE" ]] || die "web current image is missing; set GM_WEB_CURRENT_IMAGE or ensure container '$WEB_CONTAINER_NAME' is running"

mkdir -p "$(dirname "$ARTIFACT_PATH")"

cat >"$ARTIFACT_PATH" <<EOF
GM_RELEASE_TAG=$RELEASE_TAG
GM_API_PREVIOUS_IMAGE=$API_CURRENT_IMAGE
GM_WEB_PREVIOUS_IMAGE=$WEB_CURRENT_IMAGE
GM_API_TARGET_IMAGE=gm-api:$RELEASE_TAG
GM_WEB_TARGET_IMAGE=gm-web:$RELEASE_TAG
EOF

log "captured rollback manifest at '$ARTIFACT_PATH'"
log "previous: api=$API_CURRENT_IMAGE web=$WEB_CURRENT_IMAGE"
log "target: api=gm-api:$RELEASE_TAG web=gm-web:$RELEASE_TAG"
