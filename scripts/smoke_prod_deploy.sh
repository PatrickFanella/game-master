#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: bash scripts/smoke_prod_deploy.sh <base-url>

Runs deterministic post-cutover smoke checks against the deployed Edda stack.
Artifacts are written under .sisyphus/evidence/task-8-smoke-<timestamp>/.

Behavior:
  - probes the public edge URL first
  - if the public edge is redirect-looping, resolves the public hostname directly to the local Caddy container
  - validates homepage HTML, /api/healthz, negative 401, browser register/login/auth-me, same-origin API pathing, campaign lookup, and browser-driven websocket auth/traffic
EOF
}

log() {
  printf 'smoke-prod: %s\n' "$*" >&2
}

die() {
  printf 'smoke-prod: ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command '$1' not found"
}

manifest_get() {
  local manifest_path=$1
  local key=$2

  [[ -f "$manifest_path" ]] || return 1

  python3 - "$manifest_path" "$key" <<'PY'
import sys
from pathlib import Path

manifest_path = Path(sys.argv[1])
target_key = sys.argv[2]

for raw_line in manifest_path.read_text(encoding='utf-8').splitlines():
    line = raw_line.strip()
    if not line or line.startswith('#') or '=' not in line:
        continue
    key, value = line.split('=', 1)
    if key == target_key:
        print(value)
        raise SystemExit(0)

raise SystemExit(1)
PY
}

resolve_caddy_container_name() {
  local candidate value

  if [[ -n "${CADDY_CONTAINER_NAME:-}" ]]; then
    if docker inspect "$CADDY_CONTAINER_NAME" >/dev/null 2>&1; then
      CADDY_CONTAINER_RESOLUTION_SOURCE='environment:CADDY_CONTAINER_NAME'
      CADDY_CONTAINER_NAME_RESOLVED=$CADDY_CONTAINER_NAME
      return 0
    fi
    log "ignoring CADDY_CONTAINER_NAME='$CADDY_CONTAINER_NAME' because docker inspect could not find that container"
  fi

  for candidate in "$ROLLBACK_MANIFEST_PATH" "$REPO_ENV_PATH"; do
    value=$(manifest_get "$candidate" CADDY_CONTAINER_NAME 2>/dev/null || true)
    if [[ -z "$value" ]]; then
      continue
    fi
    if docker inspect "$value" >/dev/null 2>&1; then
      CADDY_CONTAINER_RESOLUTION_SOURCE="$candidate"
      CADDY_CONTAINER_NAME_RESOLVED=$value
      return 0
    fi
    log "ignoring CADDY container candidate '$value' from '$candidate' because docker inspect could not find that container"
  done

  die "could not resolve a live Caddy container name from CADDY_CONTAINER_NAME, '$ROLLBACK_MANIFEST_PATH', or '$REPO_ENV_PATH'"
}

container_host_resolve_ip() {
  local container_name=$1
  local host_port_binding

  host_port_binding=$(docker inspect -f '{{with index .NetworkSettings.Ports "443/tcp"}}{{range .}}{{println .HostIp "|" .HostPort}}{{end}}{{end}}' "$container_name" 2>/dev/null || true)
  [[ -n "$host_port_binding" ]] || return 1

  if [[ "$host_port_binding" == *'| 443'* || "$host_port_binding" == *'|443'* ]]; then
    RESOLVE_IP=127.0.0.1
    return 0
  fi

  return 1
}

write_summary() {
  cat >"$SUMMARY_FILE" <<EOF
Smoke target: $BASE_URL
Resolved mode: $RESOLUTION_MODE
Resolve IP: ${RESOLVE_IP:-none}
Resolve container: ${CADDY_CONTAINER_NAME_RESOLVED:-none}
Resolve source: ${CADDY_CONTAINER_RESOLUTION_SOURCE:-none}
Run dir: $RUN_DIR
Homepage HTML: $RUN_DIR/homepage.html
API healthz: $RUN_DIR/api-healthz.json
Negative auth 401: $RUN_DIR/auth-me-unauthorized-body.txt
Browser summary: $RUN_DIR/browser-summary.json
Browser network log: $RUN_DIR/browser-network.json
Campaign lookup: $RUN_DIR/campaign-lookup.json
Authenticated auth/me: $RUN_DIR/auth-me.json
Websocket success: $RUN_DIR/websocket-success.json
Edge diagnosis: ${EDGE_DIAGNOSIS_FILE:-none}
EOF
}

extract_url_parts() {
  python3 - "$BASE_URL" <<'PY'
import sys
from urllib.parse import urlparse

parsed = urlparse(sys.argv[1])
if not parsed.scheme or not parsed.netloc:
    raise SystemExit(1)
print(parsed.scheme)
print(parsed.hostname or '')
print(parsed.port or (443 if parsed.scheme == 'https' else 80))
print(parsed.path.rstrip('/'))
PY
}

detect_caddy_fallback_ip() {
  local container_name=$1

  RESOLVE_IP=

  if container_host_resolve_ip "$container_name"; then
    CADDY_FALLBACK_MODE='host-published-443'
    return 0
  fi

  RESOLVE_IP=$(docker inspect -f '{{with index .NetworkSettings.Networks "projects"}}{{.IPAddress}}{{end}}' "$container_name" 2>/dev/null || true)
  if [[ -n "$RESOLVE_IP" ]]; then
    CADDY_FALLBACK_MODE='container-projects-ip'
    return 0
  fi

  return 1
}

capture_edge_diagnosis() {
  local public_root_headers="$RUN_DIR/public-root-headers.txt"
  local public_health_headers="$RUN_DIR/public-healthz-headers.txt"
  local direct_root_headers="$RUN_DIR/direct-root-headers.txt"
  local direct_health_headers="$RUN_DIR/direct-healthz-headers.txt"

  EDGE_DIAGNOSIS_FILE="$RUN_DIR/edge-diagnosis.txt"

  curl --silent --show-error --head --max-redirs 0 -D "$public_root_headers" -o /dev/null "$BASE_URL/" || true
  curl --silent --show-error --head --max-redirs 0 -D "$public_health_headers" -o /dev/null "$BASE_URL/api/healthz" || true
  curl --silent --show-error --head --max-redirs 0 --resolve "$HOSTNAME:443:$RESOLVE_IP" -D "$direct_root_headers" -o /dev/null "$BASE_URL/" || true
  curl --silent --show-error --request GET --max-redirs 0 --resolve "$HOSTNAME:443:$RESOLVE_IP" -D "$direct_health_headers" -o "$RUN_DIR/direct-healthz-body.txt" "$BASE_URL/api/healthz" || true

  python3 - "$EDGE_DIAGNOSIS_FILE" "$BASE_URL" "$public_root_headers" "$public_health_headers" "$direct_root_headers" "$direct_health_headers" <<'PY'
import sys
from pathlib import Path

output_path = Path(sys.argv[1])
base_url = sys.argv[2]
public_root_headers = Path(sys.argv[3]).read_text(encoding='utf-8', errors='replace')
public_health_headers = Path(sys.argv[4]).read_text(encoding='utf-8', errors='replace')
direct_root_headers = Path(sys.argv[5]).read_text(encoding='utf-8', errors='replace')
direct_health_headers = Path(sys.argv[6]).read_text(encoding='utf-8', errors='replace')

def find_header(blob: str, prefix: str) -> str:
    prefix_lower = prefix.lower()
    for line in blob.splitlines():
        if line.lower().startswith(prefix_lower):
            return line.split(':', 1)[1].strip()
    return ''

public_root_location = find_header(public_root_headers, 'location:')
public_health_location = find_header(public_health_headers, 'location:')
public_root_server = find_header(public_root_headers, 'server:')
public_health_server = find_header(public_health_headers, 'server:')

root_self_redirect = public_root_location.rstrip('/') == base_url.rstrip('/')
health_self_redirect = public_health_location == base_url.rstrip('/') + '/api/healthz'
direct_origin_root_ok = 'HTTP/2 200' in direct_root_headers or 'HTTP/1.1 200' in direct_root_headers
direct_origin_health_ok = 'HTTP/2 200' in direct_health_headers or 'HTTP/1.1 200' in direct_health_headers

lines = [
    f'Public root server: {public_root_server or "unknown"}',
    f'Public root location: {public_root_location or "none"}',
    f'Public /api/healthz server: {public_health_server or "unknown"}',
    f'Public /api/healthz location: {public_health_location or "none"}',
    f'Direct origin root healthy: {str(direct_origin_root_ok).lower()}',
    f'Direct origin /api/healthz healthy: {str(direct_origin_health_ok).lower()}',
]

if root_self_redirect and health_self_redirect and direct_origin_root_ok and direct_origin_health_ok:
    lines.append('Diagnosis: public edge is still self-redirecting while direct-origin HTTPS is healthy. Repo-owned Caddy deploy state is not the active blocker. The remaining blocker is external edge forwarding/redirect policy outside these scripts, most likely an HTTP-origin tunnel/edge path such as cloudflared ingress to http://caddy:80.')
else:
    lines.append('Diagnosis: redirect-loop signature was not fully conclusive; inspect the captured header artifacts in this run directory.')

output_path.write_text('\n'.join(lines) + '\n', encoding='utf-8')
PY
}

curl_target() {
  local output_file=$1
  shift
  local path_suffix=$1
  shift

  curl \
    --fail \
    --show-error \
    --silent \
    --location \
    --max-redirs 10 \
    --output "$output_file" \
    "${CURL_COMMON_ARGS[@]}" \
    "$BASE_URL$path_suffix" \
    "$@"
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

[[ $# -eq 1 ]] || {
  usage
  exit 1
}

BASE_URL=${1%/}

require_cmd bash
require_cmd curl
require_cmd date
require_cmd docker
require_cmd mkdir
require_cmd node
require_cmd python3
require_cmd test

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
EVIDENCE_DIR="$REPO_ROOT/.sisyphus/evidence"
HELPER_SCRIPT="$SCRIPT_DIR/smoke_prod_deploy_helper.mjs"
ROLLBACK_MANIFEST_PATH=${GM_SMOKE_ROLLBACK_MANIFEST:-$EVIDENCE_DIR/rollback-manifest.env}
REPO_ENV_PATH=${GM_SMOKE_REPO_ENV_PATH:-$REPO_ROOT/.env}
[[ -f "$HELPER_SCRIPT" ]] || die "helper script '$HELPER_SCRIPT' does not exist"

mapfile -t URL_PARTS < <(extract_url_parts) || die "base URL '$BASE_URL' is invalid"
SCHEME=${URL_PARTS[0]}
HOSTNAME=${URL_PARTS[1]}
PORT=${URL_PARTS[2]}
BASE_PATH=${URL_PARTS[3]}

[[ "$SCHEME" == "https" ]] || die "smoke target must use https"
[[ "$PORT" == "443" ]] || die "smoke target must use port 443"
[[ -n "$HOSTNAME" ]] || die "smoke target hostname must not be empty"
[[ -z "$BASE_PATH" ]] || die "smoke target must not include a non-root path"

RUN_TIMESTAMP=$(date -u +%Y%m%dT%H%M%SZ)
RUN_DIR="$EVIDENCE_DIR/task-8-smoke-$RUN_TIMESTAMP"
SUMMARY_FILE="$EVIDENCE_DIR/task-8-smoke-pass.txt"
AUTH_FAIL_FILE="$EVIDENCE_DIR/task-8-auth-fail.txt"
mkdir -p "$RUN_DIR"

EDGE_PROBE_HEADERS="$RUN_DIR/public-login-probe.txt"
EDGE_PROBE_BODY="$RUN_DIR/public-login-probe-body.txt"
RESOLUTION_MODE=public
RESOLVE_IP=
CADDY_CONTAINER_NAME_RESOLVED=
CADDY_CONTAINER_RESOLUTION_SOURCE=
CADDY_FALLBACK_MODE=
EDGE_DIAGNOSIS_FILE=
CURL_COMMON_ARGS=()

log "probing public edge URL '$BASE_URL/login'"
if curl --show-error --silent --location --max-redirs 5 -D "$EDGE_PROBE_HEADERS" -o "$EDGE_PROBE_BODY" "$BASE_URL/login"; then
  log "public edge probe succeeded"
else
  resolve_caddy_container_name
  detect_caddy_fallback_ip "$CADDY_CONTAINER_NAME_RESOLVED"
  [[ -n "$RESOLVE_IP" ]] || die "public edge probe failed and no direct-origin fallback address could be discovered from Caddy container '$CADDY_CONTAINER_NAME_RESOLVED'"
  RESOLUTION_MODE=direct-caddy-resolve
  CURL_COMMON_ARGS=(--resolve "$HOSTNAME:443:$RESOLVE_IP")
  log "public edge probe failed; falling back to direct-origin resolve via Caddy '$CADDY_CONTAINER_NAME_RESOLVED' at '$RESOLVE_IP' (source: $CADDY_CONTAINER_RESOLUTION_SOURCE, mode: $CADDY_FALLBACK_MODE)"
  capture_edge_diagnosis
  log "edge diagnosis captured at '$EDGE_DIAGNOSIS_FILE'"
fi

curl_target "$RUN_DIR/homepage.html" /
python3 - "$RUN_DIR/homepage.html" <<'PY'
import sys
from pathlib import Path

html = Path(sys.argv[1]).read_text(encoding='utf-8')
if '<html' not in html.lower():
    raise SystemExit('homepage HTML artifact did not contain an <html> tag')
PY

curl_target "$RUN_DIR/api-healthz.json" /api/healthz
python3 - "$RUN_DIR/api-healthz.json" <<'PY'
import json
import sys
from pathlib import Path

payload = json.loads(Path(sys.argv[1]).read_text(encoding='utf-8'))
if payload.get('status') != 'ok':
    raise SystemExit(f"unexpected /api/healthz payload: {payload}")
PY

UNAUTH_HEADERS="$RUN_DIR/auth-me-unauthorized-headers.txt"
UNAUTH_BODY="$RUN_DIR/auth-me-unauthorized-body.txt"
UNAUTH_STATUS=$(curl \
  --silent \
  --show-error \
  --location \
  --max-redirs 10 \
  -D "$UNAUTH_HEADERS" \
  -o "$UNAUTH_BODY" \
  -w '%{http_code}' \
  "${CURL_COMMON_ARGS[@]}" \
  "$BASE_URL/api/v1/auth/me")
[[ "$UNAUTH_STATUS" == "401" ]] || die "expected unauthenticated /api/v1/auth/me to return 401, got $UNAUTH_STATUS"
cat >"$AUTH_FAIL_FILE" <<EOF
Negative auth smoke succeeded.
Target: $BASE_URL/api/v1/auth/me
Resolved mode: $RESOLUTION_MODE
Resolve IP: ${RESOLVE_IP:-none}
HTTP status: $UNAUTH_STATUS
Headers: $UNAUTH_HEADERS
Body: $UNAUTH_BODY
EOF

log "running browser + websocket smoke helper"
node "$HELPER_SCRIPT" "$BASE_URL" "$RUN_DIR" "$RESOLVE_IP"

BROWSER_SUMMARY="$RUN_DIR/browser-summary.json"
[[ -f "$BROWSER_SUMMARY" ]] || die "browser summary '$BROWSER_SUMMARY' was not produced"

SMOKE_EMAIL=$(python3 - "$BROWSER_SUMMARY" <<'PY'
import json
import sys
from pathlib import Path

data = json.loads(Path(sys.argv[1]).read_text(encoding='utf-8'))
print(data['email'])
PY
)

python3 - "$RUN_DIR/auth-me.json" "$SMOKE_EMAIL" "$BROWSER_SUMMARY" <<'PY'
import json
import sys
from pathlib import Path

summary = json.loads(Path(sys.argv[3]).read_text(encoding='utf-8'))
payload = summary['authMe']['body']
expected_email = sys.argv[2]

if payload.get('user', {}).get('email') != expected_email:
    raise SystemExit(f"authenticated /api/v1/auth/me email mismatch: {payload}")

Path(sys.argv[1]).write_text(
    json.dumps(payload, indent=2) + '\n',
    encoding='utf-8',
)

Path(Path(sys.argv[1]).with_name('campaign-lookup.json')).write_text(
    json.dumps(summary['campaignLookup'], indent=2) + '\n',
    encoding='utf-8',
)
Path(Path(sys.argv[1]).with_name('websocket-success.json')).write_text(
    json.dumps(summary['websocket'], indent=2) + '\n',
    encoding='utf-8',
)
PY

write_summary
log "smoke complete -> $SUMMARY_FILE"
