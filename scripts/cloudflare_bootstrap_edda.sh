#!/usr/bin/env bash

set -euo pipefail

VERSION="1.0.0"
SCRIPT_NAME=$(basename "$0")

usage() {
  cat >&2 <<'EOF'
Usage: bash scripts/cloudflare_bootstrap_edda.sh <env-file> <hostname>

Bootstraps first-time Cloudflare edge config for a same-origin app hostname.

Reads these env vars from the supplied env file:
  - CLOUDFLARE_API_TOKEN
  - CLOUDFLARE_ZONE_ID
  - CLOUDFLARE_RECORD_TARGET

Optional env var for test/mocking:
  - CLOUDFLARE_API_BASE (defaults to https://api.cloudflare.com/client/v4)

It will:
  - create or update a proxied A or CNAME record for the hostname
  - fail if conflicting DNS layout makes routing unsafe
  - set SSL mode to Full (strict)
  - ensure WebSockets support is enabled
  - ensure /api/* and websocket paths bypass cache via Cache Rules
EOF
}

version() {
  printf '%s %s\n' "$SCRIPT_NAME" "$VERSION"
}

log() {
  printf 'cloudflare-bootstrap: %s\n' "$*" >&2
}

die() {
  printf 'cloudflare-bootstrap: ERROR: %s\n' "$*" >&2
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

cf_api() {
  local method=$1
  local path=$2
  local body=${3:-}
  local tmp_body tmp_meta http_code result_path

  tmp_body=$(mktemp)
  tmp_meta=$(mktemp)

  if [[ -n "$body" ]]; then
    http_code=$(curl -sS -X "$method" "$API_BASE$path" \
      -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
      -H 'Content-Type: application/json' \
      --data "$body" \
      -o "$tmp_body" \
      -w '%{http_code}' \
      2>"$tmp_meta") || {
      local curl_stderr
      curl_stderr=$(<"$tmp_meta")
      rm -f "$tmp_body" "$tmp_meta"
      die "Cloudflare API request failed for $method $path: ${curl_stderr:-curl error}"
    }
  else
    http_code=$(curl -sS -X "$method" "$API_BASE$path" \
      -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
      -o "$tmp_body" \
      -w '%{http_code}' \
      2>"$tmp_meta") || {
      local curl_stderr
      curl_stderr=$(<"$tmp_meta")
      rm -f "$tmp_body" "$tmp_meta"
      die "Cloudflare API request failed for $method $path: ${curl_stderr:-curl error}"
    }
  fi

  jq -e . >/dev/null 2>&1 <"$tmp_body" || {
    local raw_body
    raw_body=$(<"$tmp_body")
    rm -f "$tmp_body" "$tmp_meta"
    die "Cloudflare API returned non-JSON for $method $path (HTTP $http_code): $raw_body"
  }

  if [[ ! "$http_code" =~ ^2 ]]; then
    local errors messages
    errors=$(jq -cr '[.errors[]?.message] | join("; ")' <"$tmp_body")
    messages=$(jq -cr '[.messages[]?.message] | join("; ")' <"$tmp_body")
    rm -f "$tmp_meta"
    result_path="$tmp_body"
    die "Cloudflare API error for $method $path (HTTP $http_code): $(format_cf_api_error "$http_code" "$errors" "$messages"). Response body: $result_path"
  fi

  if ! jq -e '.success == true' >/dev/null 2>&1 <"$tmp_body"; then
    local errors messages
    errors=$(jq -cr '[.errors[]?.message] | join("; ")' <"$tmp_body")
    messages=$(jq -cr '[.messages[]?.message] | join("; ")' <"$tmp_body")
    rm -f "$tmp_meta"
    result_path="$tmp_body"
    die "Cloudflare API reported failure for $method $path: $(format_cf_api_error 200 "$errors" "$messages"). Response body: $result_path"
  fi

  rm -f "$tmp_meta"
  printf '%s\n' "$tmp_body"
}

cleanup_cf_api_files() {
  if (( $# == 0 )); then
    return 0
  fi
  rm -f "$@"
}

format_cf_api_error() {
  local http_code=$1
  local errors=$2
  local messages=$3
  local combined

  combined=$errors
  if [[ -n "$messages" ]]; then
    if [[ -n "$combined" ]]; then
      combined+="; $messages"
    else
      combined=$messages
    fi
  fi

  if [[ -z "$combined" ]]; then
    combined='unknown error'
  fi

  case "$http_code:$combined" in
    400:*Invalid\ request\ headers*|401:*|403:*)
      printf 'Cloudflare auth/scope failure: %s. Verify CLOUDFLARE_API_TOKEN is valid and has Zone DNS Edit, Zone Settings Edit, and Zone Cache Rules/Rulesets Edit for zone %s.' "$combined" "$CLOUDFLARE_ZONE_ID"
      ;;
    *)
      printf '%s' "$combined"
      ;;
  esac
}

cf_get_allow_404() {
  local path=$1
  local tmp_body tmp_meta http_code

  tmp_body=$(mktemp)
  tmp_meta=$(mktemp)

  http_code=$(curl -sS -X GET "$API_BASE$path" \
    -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
    -o "$tmp_body" \
    -w '%{http_code}' \
    2>"$tmp_meta") || {
    local curl_stderr
    curl_stderr=$(<"$tmp_meta")
    rm -f "$tmp_body" "$tmp_meta"
    die "Cloudflare API request failed for GET $path: ${curl_stderr:-curl error}"
  }

  jq -e . >/dev/null 2>&1 <"$tmp_body" || {
    local raw_body
    raw_body=$(<"$tmp_body")
    rm -f "$tmp_body" "$tmp_meta"
    die "Cloudflare API returned non-JSON for GET $path (HTTP $http_code): $raw_body"
  }

  rm -f "$tmp_meta"
  printf '%s %s\n' "$http_code" "$tmp_body"
}

record_type_for_target() {
  local target=$1
  if [[ "$target" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
    printf 'A\n'
    return 0
  fi

  if [[ "$target" == *:* ]]; then
    die "CLOUDFLARE_RECORD_TARGET='$target' looks like IPv6. Refusing because this bootstrap must verify no AAAA routing conflicts for edda.subcult.tv. Use an IPv4 origin or hostname target instead."
  fi

  if [[ "$target" =~ ^[A-Za-z0-9.-]+\.?$ ]]; then
    printf 'CNAME\n'
    return 0
  fi

  die "CLOUDFLARE_RECORD_TARGET='$target' is neither an IPv4 address nor a hostname"
}

normalize_hostname() {
  local host=$1
  host=${host%.}
  printf '%s\n' "$host"
}

ensure_hostname_in_zone() {
  [[ "$HOSTNAME" == *.* ]] || die "hostname '$HOSTNAME' must be a fully-qualified domain name"
}

load_env() {
  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
}

fetch_zone_details() {
  local response_path zone_name zone_status
  response_path=$(cf_api GET "/zones/$CLOUDFLARE_ZONE_ID")
  zone_name=$(jq -r '.result.name // empty' <"$response_path")
  zone_status=$(jq -r '.result.status // empty' <"$response_path")

  [[ -n "$zone_name" ]] || {
    cleanup_cf_api_files "$response_path"
    die "unable to determine Cloudflare zone name for zone '$CLOUDFLARE_ZONE_ID'"
  }

  if [[ "$HOSTNAME" != "$zone_name" && "$HOSTNAME" != *".$zone_name" ]]; then
    cleanup_cf_api_files "$response_path"
    die "hostname '$HOSTNAME' is not inside Cloudflare zone '$zone_name'"
  fi

  log "resolved zone '$zone_name' (status: ${zone_status:-unknown})"
  ZONE_NAME=$zone_name
  cleanup_cf_api_files "$response_path"
}

fetch_hostname_records() {
  local response_path
  response_path=$(cf_api GET "/zones/$CLOUDFLARE_ZONE_ID/dns_records?name=$HOSTNAME&per_page=100")
  DNS_RECORDS_JSON=$(jq -c '.result // []' <"$response_path")
  cleanup_cf_api_files "$response_path"
}

validate_dns_state() {
  local cname_count aaaa_count unsupported_count exact_target_count target_mismatch_count

  cname_count=$(jq '[.[] | select(.type == "CNAME")] | length' <<<"$DNS_RECORDS_JSON")
  aaaa_count=$(jq '[.[] | select(.type == "AAAA")] | length' <<<"$DNS_RECORDS_JSON")
  unsupported_count=$(jq '[.[] | select(.type != "A" and .type != "AAAA" and .type != "CNAME")] | length' <<<"$DNS_RECORDS_JSON")
  exact_target_count=$(jq --arg desired_type "$DESIRED_RECORD_TYPE" --arg desired_content "$CLOUDFLARE_RECORD_TARGET" '[.[] | select(.type == $desired_type and .content == $desired_content)] | length' <<<"$DNS_RECORDS_JSON")
  target_mismatch_count=$(jq --arg desired_type "$DESIRED_RECORD_TYPE" --arg desired_content "$CLOUDFLARE_RECORD_TARGET" '[.[] | select(.type == $desired_type and .content != $desired_content)] | length' <<<"$DNS_RECORDS_JSON")

  if (( unsupported_count > 0 )); then
    die "hostname '$HOSTNAME' has unsupported DNS record types at Cloudflare; refusing to mutate mixed routing state"
  fi

  if (( aaaa_count > 0 )); then
    die "hostname '$HOSTNAME' already has AAAA record(s). Refusing automation because IPv6 + proxied app cutover could route traffic to the wrong origin. Remove or intentionally manage AAAA first."
  fi

  if [[ "$DESIRED_RECORD_TYPE" == "A" ]] && (( cname_count > 0 )); then
    die "hostname '$HOSTNAME' already has a CNAME record; refusing to replace it with an A record automatically"
  fi

  if [[ "$DESIRED_RECORD_TYPE" == "CNAME" ]]; then
    local non_cname_count
    non_cname_count=$(jq '[.[] | select(.type != "CNAME")] | length' <<<"$DNS_RECORDS_JSON")
    if (( non_cname_count > 0 )); then
      die "hostname '$HOSTNAME' already has non-CNAME record(s); refusing to replace them with a CNAME automatically"
    fi
  fi

  if (( target_mismatch_count > 0 )); then
    die "hostname '$HOSTNAME' already has $DESIRED_RECORD_TYPE record(s) with different target(s); refusing to guess which origin should win"
  fi

  if (( exact_target_count > 1 )); then
    die "hostname '$HOSTNAME' has duplicate $DESIRED_RECORD_TYPE records for target '$CLOUDFLARE_RECORD_TARGET'; clean up duplicates first"
  fi
}

upsert_dns_record() {
  local existing_id response_path body proxied_value record_id record_type record_content record_proxied
  existing_id=$(jq -r --arg desired_type "$DESIRED_RECORD_TYPE" --arg desired_content "$CLOUDFLARE_RECORD_TARGET" '.[] | select(.type == $desired_type and .content == $desired_content) | .id' <<<"$DNS_RECORDS_JSON")
  proxied_value=true
  body=$(jq -cn \
    --arg type "$DESIRED_RECORD_TYPE" \
    --arg name "$HOSTNAME" \
    --arg content "$CLOUDFLARE_RECORD_TARGET" \
    --argjson proxied "$proxied_value" \
    '{type:$type,name:$name,content:$content,proxied:$proxied,ttl:1}')

  if [[ -n "$existing_id" ]]; then
    log "updating existing $DESIRED_RECORD_TYPE record for '$HOSTNAME'"
    response_path=$(cf_api PUT "/zones/$CLOUDFLARE_ZONE_ID/dns_records/$existing_id" "$body")
  else
    log "creating proxied $DESIRED_RECORD_TYPE record for '$HOSTNAME'"
    response_path=$(cf_api POST "/zones/$CLOUDFLARE_ZONE_ID/dns_records" "$body")
  fi

  record_id=$(jq -r '.result.id // empty' <"$response_path")
  record_type=$(jq -r '.result.type // empty' <"$response_path")
  record_content=$(jq -r '.result.content // empty' <"$response_path")
  record_proxied=$(jq -r '.result.proxied' <"$response_path")

  [[ -n "$record_id" ]] || {
    cleanup_cf_api_files "$response_path"
    die "Cloudflare did not return a DNS record ID after upsert"
  }

  [[ "$record_type" == "$DESIRED_RECORD_TYPE" ]] || {
    cleanup_cf_api_files "$response_path"
    die "Cloudflare returned record type '$record_type', expected '$DESIRED_RECORD_TYPE'"
  }

  [[ "$record_content" == "$CLOUDFLARE_RECORD_TARGET" ]] || {
    cleanup_cf_api_files "$response_path"
    die "Cloudflare returned record content '$record_content', expected '$CLOUDFLARE_RECORD_TARGET'"
  }

  [[ "$record_proxied" == "true" ]] || {
    cleanup_cf_api_files "$response_path"
    die "Cloudflare returned proxied='$record_proxied' for '$HOSTNAME'; expected true"
  }

  DNS_RECORD_ID=$record_id
  cleanup_cf_api_files "$response_path"
}

set_zone_setting() {
  local setting_name=$1
  local expected_value=$2
  local body response_path actual_value
  body=$(jq -cn --arg value "$expected_value" '{value:$value}')
  response_path=$(cf_api PATCH "/zones/$CLOUDFLARE_ZONE_ID/settings/$setting_name" "$body")
  actual_value=$(jq -r '.result.value // empty' <"$response_path")
  [[ "$actual_value" == "$expected_value" ]] || {
    cleanup_cf_api_files "$response_path"
    die "Cloudflare setting '$setting_name' is '$actual_value', expected '$expected_value'"
  }
  cleanup_cf_api_files "$response_path"
}

build_guardrail_rule() {
  jq -cn \
    --arg host "$HOSTNAME" \
    '{
      description: "Bypass cache for edda same-origin API and websocket traffic",
      expression: "(http.host eq \"" + $host + "\" and (starts_with(http.request.uri.path, \"/api/\") or starts_with(http.request.uri.path, \"/api/v1/campaigns/\")))",
      action: "set_cache_settings",
      action_parameters: {
        cache: false,
        browser_ttl: {
          mode: "bypass"
        }
      },
      enabled: true
    }'
}

purge_cache_files() {
  local files_json response_path purge_count
  if (( $# == 0 )); then
    return 0
  fi

  files_json=$(printf '%s\n' "$@" | jq -R . | jq -cs '{files: .}')
  response_path=$(cf_api POST "/zones/$CLOUDFLARE_ZONE_ID/purge_cache" "$files_json")
  purge_count=$(jq '.result.id? // empty | length' <"$response_path")
  [[ "$purge_count" -gt 0 ]] || {
    cleanup_cf_api_files "$response_path"
    die "Cloudflare cache purge did not return a purge operation id"
  }
  cleanup_cf_api_files "$response_path"
}

ensure_cache_guardrail() {
  local entrypoint_path current_rules response_path updated_payload verify_rule_count
  local marker_description
  local get_result get_status

  marker_description='Bypass cache for edda same-origin API and websocket traffic'
  entrypoint_path="/zones/$CLOUDFLARE_ZONE_ID/rulesets/phases/http_request_cache_settings/entrypoint"

  get_result=$(cf_get_allow_404 "$entrypoint_path")
  get_status=${get_result%% *}
  response_path=${get_result#* }

  case "$get_status" in
    200)
      current_rules=$(jq -c --arg desc "$marker_description" '.result.rules // [] | map(select(.description != $desc))' <"$response_path")
      ;;
    404)
      current_rules='[]'
      ;;
    *)
      local errors messages
      errors=$(jq -cr '[.errors[]?.message] | join("; ")' <"$response_path")
      messages=$(jq -cr '[.messages[]?.message] | join("; ")' <"$response_path")
      cleanup_cf_api_files "$response_path"
      die "Cloudflare API error for GET $entrypoint_path (HTTP $get_status): $(format_cf_api_error "$get_status" "$errors" "$messages")"
      ;;
  esac
  cleanup_cf_api_files "$response_path"

  updated_payload=$(jq -cn \
    --arg name 'Default phase entry point ruleset' \
    --arg description 'Managed by scripts/cloudflare_bootstrap_edda.sh' \
    --argjson existing_rules "$current_rules" \
    --argjson guardrail_rule "$(build_guardrail_rule)" \
    '{name:$name, description:$description, rules: ($existing_rules + [$guardrail_rule])}')

  response_path=$(cf_api PUT "$entrypoint_path" "$updated_payload")
  verify_rule_count=$(jq --arg desc "$marker_description" '[.result.rules[]? | select(.description == $desc and .action == "set_cache_settings" and .enabled == true and .action_parameters.cache == false and .action_parameters.browser_ttl.mode == "bypass")] | length' <"$response_path")
  [[ "$verify_rule_count" == "1" ]] || {
    cleanup_cf_api_files "$response_path"
    die "Cloudflare cache guardrail rule for '/api/*' was not applied as expected"
  }
  cleanup_cf_api_files "$response_path"

  log "purging stale Cloudflare cache objects for API guardrail verification"
  purge_cache_files \
    "https://$HOSTNAME/api/healthz" \
    "https://$HOSTNAME/api/healthz?repeat=probe"
}

verify_final_state() {
  local dns_path ssl_path ws_path rules_path dns_verify ssl_verify ws_verify rule_verify

  dns_path=$(cf_api GET "/zones/$CLOUDFLARE_ZONE_ID/dns_records/$DNS_RECORD_ID")
  ssl_path=$(cf_api GET "/zones/$CLOUDFLARE_ZONE_ID/settings/ssl")
  ws_path=$(cf_api GET "/zones/$CLOUDFLARE_ZONE_ID/settings/websockets")
  rules_path=$(cf_api GET "/zones/$CLOUDFLARE_ZONE_ID/rulesets/phases/http_request_cache_settings/entrypoint")

  dns_verify=$(jq -r '.result.proxied' <"$dns_path")
  ssl_verify=$(jq -r '.result.value // empty' <"$ssl_path")
  ws_verify=$(jq -r '.result.value // empty' <"$ws_path")
  rule_verify=$(jq '[.result.rules[]? | select(.description == "Bypass cache for edda same-origin API and websocket traffic" and .action == "set_cache_settings" and .enabled == true and .action_parameters.browser_ttl.mode == "bypass")] | length' <"$rules_path")

  [[ "$dns_verify" == "true" ]] || die "post-verify failed: DNS record '$HOSTNAME' is not proxied"
  [[ "$ssl_verify" == "strict" ]] || die "post-verify failed: SSL mode is '$ssl_verify', expected 'strict'"
  [[ "$ws_verify" == "on" ]] || die "post-verify failed: WebSockets setting is '$ws_verify', expected 'on'"
  rule_verify=$(jq '[.result.rules[]? | select(.description == "Bypass cache for edda same-origin API and websocket traffic" and .action == "set_cache_settings" and .enabled == true and .action_parameters.cache == false and .action_parameters.browser_ttl.mode == "bypass")] | length' <"$rules_path")

  [[ "$rule_verify" == "1" ]] || die "post-verify failed: cache guardrail rule missing or malformed"

  printf 'verified_host=%s\n' "$HOSTNAME"
  printf 'verified_zone=%s\n' "$ZONE_NAME"
  printf 'verified_record_id=%s\n' "$DNS_RECORD_ID"
  printf 'verified_record_type=%s\n' "$DESIRED_RECORD_TYPE"
  printf 'verified_record_target=%s\n' "$CLOUDFLARE_RECORD_TARGET"
  printf 'verified_record_proxied=true\n'
  printf 'verified_ssl_mode=strict\n'
  printf 'verified_websockets=on\n'
  printf 'verified_cache_guardrail=api-bypass\n'

  cleanup_cf_api_files "$dns_path" "$ssl_path" "$ws_path" "$rules_path"
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ "${1:-}" == "--version" ]]; then
  version
  exit 0
fi

[[ $# -eq 2 ]] || {
  usage
  exit 1
}

ENV_FILE=$1
HOSTNAME=$(normalize_hostname "$2")
API_BASE='https://api.cloudflare.com/client/v4'

[[ -f "$ENV_FILE" ]] || die "env file '$ENV_FILE' does not exist"

require_cmd bash
require_cmd curl
require_cmd jq
require_cmd mktemp

load_env
API_BASE=${CLOUDFLARE_API_BASE:-$API_BASE}

require_var CLOUDFLARE_API_TOKEN
require_var CLOUDFLARE_ZONE_ID
require_var CLOUDFLARE_RECORD_TARGET

ensure_hostname_in_zone
DESIRED_RECORD_TYPE=$(record_type_for_target "$CLOUDFLARE_RECORD_TARGET")

fetch_zone_details
fetch_hostname_records
validate_dns_state
upsert_dns_record

log "setting SSL/TLS mode to Full (strict)"
set_zone_setting ssl strict

log "ensuring Cloudflare WebSockets support is enabled"
set_zone_setting websockets on

log "ensuring API/websocket cache bypass guardrail"
ensure_cache_guardrail

log "verifying final Cloudflare state"
verify_final_state

log "bootstrap complete for '$HOSTNAME'"
