#!/usr/bin/env bash
set -euo pipefail

# --- Logging ---------------------------------------------------------------
# Usage: set LOG_LEVEL (DEBUG, INFO, WARN, ERROR), LOG_FILE, NO_COLOR envs or flags
LOG_LEVEL="${LOG_LEVEL:-INFO}"
LOG_FILE="${LOG_FILE:-}"
NO_COLOR="${NO_COLOR:-}"

_log_level_to_num() {
  case "${1:-INFO}" in
  DEBUG) echo 0 ;;
  INFO) echo 1 ;;
  WARN) echo 2 ;;
  ERROR) echo 3 ;;
  *) echo 1 ;;
  esac
}

_LOG_LEVEL_NUM=$(_log_level_to_num "$LOG_LEVEL")

if [[ -t 1 && -z "$NO_COLOR" ]]; then
  _C_RESET="\033[0m"
  _C_DEBUG="\033[35m" # magenta
  _C_INFO="\033[36m"  # cyan
  _C_WARN="\033[33m"  # yellow
  _C_ERROR="\033[31m" # red
else
  _C_RESET=""
  _C_DEBUG=""
  _C_INFO=""
  _C_WARN=""
  _C_ERROR=""
fi

_now_ts() { date '+%Y-%m-%d %H:%M:%S'; }

_log() {
  local level="$1"
  shift || true
  local level_num=$(_log_level_to_num "$level")
  ((level_num < _LOG_LEVEL_NUM)) && return 0
  local color prefix
  case "$level" in
  DEBUG)
    color="$_C_DEBUG"
    prefix="DBG"
    ;;
  INFO)
    color="$_C_INFO"
    prefix="INF"
    ;;
  WARN)
    color="$_C_WARN"
    prefix="WRN"
    ;;
  ERROR)
    color="$_C_ERROR"
    prefix="ERR"
    ;;
  esac
  local line
  line="[$(_now_ts)] [$prefix] $*"
  if [[ "$level" == "ERROR" || "$level" == "WARN" ]]; then
    printf "%b%s%b\n" "$color" "$line" "$_C_RESET" >&2
  else
    printf "%b%s%b\n" "$color" "$line" "$_C_RESET"
  fi
  if [[ -n "$LOG_FILE" ]]; then
    printf "%s\n" "$line" >>"$LOG_FILE"
  fi
}

log_debug() { _log DEBUG "$@"; }
log_info() { _log INFO "$@"; }
log_warn() { _log WARN "$@"; }
log_error() { _log ERROR "$@"; }

# Flags: -q/--quiet, -v/--verbose, -d/--debug, --log-file FILE, --no-color
# Also supports: --url, --scheme/--protocol, --host, --port, --base-path,
#                --default-client-root-url, --post-logout-uris, --insecure, --timeout SECONDS,
#                --realm NAME, --retries N, --retry-delay SECONDS, --dry-run, -h/--help
print_help() {
  cat <<'USAGE'
Usage: create-keycloak-realm.sh [options]

Purpose:
  Create a Keycloak realm if missing, provision clients, and ensure required roles
  and default client scopes. Safe to run multiple times (idempotent).

Core options:
  --realm NAME                   Realm to manage (default: REALM_NAME env or "ci-realm")
  --url URL                      Full base URL to Keycloak; overrides scheme/host/port/base-path
  --scheme|--protocol SCHEME     URL scheme when building a URL (default: http)
  --host HOST                    Hostname (default: localhost)
  --port PORT                    Port (default: 8080)
  --base-path PATH               Optional base path (e.g. auth). Omit if unused.

Client configuration (via CLIENTS array in this script):
  Format: clientId,name,redirectUri,confidential[,rootUrl]
    - clientId                   Unique client identifier
    - name                       Display name (optional in 3-field form)
    - redirectUri                Optional. If omitted, standard (auth code) flow is disabled
    - confidential               true|false (confidential vs public)
    - rootUrl                    Optional. Used when redirectUri is relative (e.g., "/callback").
                                  If omitted and --default-client-root-url is provided, that is used.

Defaults per client:
  - publicClient=false (client authentication enabled)
  - authorizationServicesEnabled=true
  - standardFlowEnabled=true only when a redirectUri is supplied; otherwise false
  - serviceAccountsEnabled=true
  - Roles ensured: read, read:users, write
  - Default client scopes ensured: camunda-identity, roles, service_account, web-origins

Additional options:
  --default-client-root-url URL  Fallback rootUrl for clients with relative redirectUri
  --dry-run                      Print intended requests/payloads; do not call Keycloak
  --insecure                     Disable TLS verification for HTTPS (curl -k)
  --timeout SECONDS              Curl connect and overall timeouts
  --retries N                    Retry count for network errors (default: 3)
  --retry-delay SECONDS          Delay between retries (default: 2)

Logging:
  -q, --quiet                    Only warnings and errors
  -v, --verbose                  Info-level logs (default)
  -d, --debug                    Debug-level logs (shows payload snippets)
      --log-file FILE            Append logs to FILE
      --no-color                 Disable ANSI colors in output

Environment variables:
  REALM_NAME, KEYCLOAK_URL, KEYCLOAK_SCHEME/KEYCLOAK_PROTOCOL, KEYCLOAK_HOST, KEYCLOAK_PORT,
  KEYCLOAK_BASE_PATH, DEFAULT_CLIENT_ROOT_URL, LOG_LEVEL, LOG_FILE, NO_COLOR

Exit codes:
  0   Success
  1   Validation/create failed and the resource does not exist

Examples:
  # Build URL from components
  ./create-keycloak-realm.sh --scheme https --host kc.example.com --port 8443 --realm staging

  # Use a full URL (overrides scheme/host/port/base-path)
  ./create-keycloak-realm.sh --url https://kc.example.com/auth --realm prod

  # Clients with relative redirectUri; provide global root
  ./create-keycloak-realm.sh --default-client-root-url http://localhost:3000

  # Dry-run to inspect payloads
  ./create-keycloak-realm.sh --dry-run --debug --realm test
USAGE
}
while [[ ${1:-} ]]; do
  case "$1" in
  -q | --quiet)
    LOG_LEVEL=WARN
    _LOG_LEVEL_NUM=2
    shift
    ;;
  -v | --verbose)
    LOG_LEVEL=INFO
    _LOG_LEVEL_NUM=1
    shift
    ;;
  -d | --debug)
    LOG_LEVEL=DEBUG
    _LOG_LEVEL_NUM=0
    shift
    ;;
  -h | --help)
    print_help
    exit 0
    ;;
  --url)
    URL_OPT="$2"
    shift 2
    ;;
  --scheme | --protocol)
    SCHEME_OPT="$2"
    shift 2
    ;;
  --host)
    HOST_OPT="$2"
    shift 2
    ;;
  --port)
    PORT_OPT="$2"
    shift 2
    ;;
  --base-path)
    BASE_PATH_OPT="$2"
    shift 2
    ;;
  --realm|--realm-name)
    REALM_NAME="$2"
    shift 2
    ;;
  --default-client-root-url)
    DEFAULT_CLIENT_ROOT_URL="$2"
    shift 2
    ;;
  --post-logout-uris)
    POST_LOGOUT_URIS_OPT="$2"
    shift 2
    ;;
  --insecure)
    INSECURE=1
    shift
    ;;
  --timeout)
    CURL_TIMEOUT="$2"
    shift 2
    ;;
  --retries)
    RETRIES="$2"
    shift 2
    ;;
  --retry-delay)
    RETRY_DELAY="$2"
    shift 2
    ;;
  --dry-run)
    DRY_RUN=1
    shift
    ;;
  --log-file)
    LOG_FILE="$2"
    shift 2
    ;;
  --no-color)
    NO_COLOR=1
    shift
    ;;
  *)
    log_warn "Unknown argument: $1 (use --help)"
    shift
    ;;
  esac
done

set -o errtrace
trap 'rc=$?; log_error "Command failed (exit $rc) at line $LINENO: $BASH_COMMAND"; exit $rc' ERR

# --- Configuration ---
# You can specify a full URL via KEYCLOAK_URL or components via KEYCLOAK_SCHEME/KEYCLOAK_HOST/KEYCLOAK_PORT/KEYCLOAK_BASE_PATH
# Example components: http, localhost, 8080, (empty or "auth" for older versions)
KEYCLOAK_URL="${KEYCLOAK_URL:-}"
KEYCLOAK_SCHEME="${KEYCLOAK_SCHEME:-${KEYCLOAK_PROTOCOL:-}}"
KEYCLOAK_HOST="${KEYCLOAK_HOST:-}"
KEYCLOAK_PORT="${KEYCLOAK_PORT:-}"
KEYCLOAK_BASE_PATH="${KEYCLOAK_BASE_PATH:-}"
KEYCLOAK_USER="${KEYCLOAK_USER:-admin}"
KEYCLOAK_PASSWORD="${KEYCLOAK_PASSWORD:-admin}"
REALM_NAME="${REALM_NAME:-ci-realm}"
DEFAULT_CLIENT_ROOT_URL="${DEFAULT_CLIENT_ROOT_URL:-}"
# Resolve post-logout URIs allowing empty values from flags/env
if [[ ${POST_LOGOUT_URIS_OPT+x} ]]; then
  POST_LOGOUT_URIS="$POST_LOGOUT_URIS_OPT"
elif [[ ${POST_LOGOUT_URIS+x} ]]; then
  POST_LOGOUT_URIS="$POST_LOGOUT_URIS"
else
  POST_LOGOUT_URIS="+"
fi
# --- Resolve Keycloak URL ---
if [[ -n "${URL_OPT:-}" ]]; then
  KEYCLOAK_URL="$URL_OPT"
else
  # Prefer component flags/envs when any are provided, otherwise fall back to existing KEYCLOAK_URL
  if [[ -n "${SCHEME_OPT:-}${HOST_OPT:-}${PORT_OPT:-}${BASE_PATH_OPT:-}${KEYCLOAK_SCHEME:-}${KEYCLOAK_HOST:-}${KEYCLOAK_PORT:-}${KEYCLOAK_BASE_PATH:-}" || -z "${KEYCLOAK_URL:-}" ]]; then
    _scheme="${SCHEME_OPT:-${KEYCLOAK_SCHEME:-http}}"
    _host="${HOST_OPT:-${KEYCLOAK_HOST:-localhost}}"
    _port="${PORT_OPT:-${KEYCLOAK_PORT:-8080}}"
    _base="${BASE_PATH_OPT:-${KEYCLOAK_BASE_PATH:-}}"
    # Normalize base path
    _base="${_base#/}"
    _base="${_base%/}"
    if [[ -n "$_base" ]]; then
      _base="/$_base"
    fi
    KEYCLOAK_URL="${_scheme}://${_host}:${_port}${_base}"
  fi
fi

# Normalize URL (strip trailing slash)
KEYCLOAK_URL="${KEYCLOAK_URL%/}"

# Configure curl options
CURL_OPTS=()
if [[ -n "${INSECURE:-}" ]]; then
  CURL_OPTS+=(-k)
fi
if [[ -n "${CURL_TIMEOUT:-}" ]]; then
  CURL_OPTS+=(--connect-timeout "$CURL_TIMEOUT" --max-time "$CURL_TIMEOUT")
fi

# Retry defaults
RETRIES="${RETRIES:-3}"
RETRY_DELAY="${RETRY_DELAY:-2}"

# Curl helper with retries; sets globals: CURL_STATUS, CURL_BODY_FILE, CURL_HDR_FILE
curl_with_retries() {
  local attempt=1
  CURL_BODY_FILE="$(mktemp)"
  CURL_HDR_FILE="$(mktemp)"
  while :; do
    set +e
    local status
    status=$(curl -s "${CURL_OPTS[@]}" "$@" -o "$CURL_BODY_FILE" -D "$CURL_HDR_FILE" -w "%{http_code}")
    local rc=$?
    set -e
    if [[ $rc -eq 0 ]]; then
      CURL_STATUS="$status"
      return 0
    fi
    if ((attempt >= RETRIES)); then
      CURL_STATUS="$status"
      return $rc
    fi
    log_warn "HTTP call failed (exit $rc). Retry $attempt/$RETRIES in ${RETRY_DELAY}s..."
    sleep "$RETRY_DELAY"
    attempt=$((attempt + 1))
  done
}

# Clients to create
# Supported formats (CSV per line):
# - clientId,name,redirectUri,confidential[,rootUrl]
# - clientId,redirectUri,confidential           # name defaults to clientId
CLIENTS=(
  "camunda-identity-resource-server,Camunda Management Identity Resource Server,/auth/login-callback,false"
)

require_deps() {
  for _dep in curl jq; do
    if ! command -v "$_dep" >/dev/null 2>&1; then
      log_error "Required dependency not found: $_dep"
      exit 1
    fi
  done
}

log_start_info() {
  log_info "Starting Keycloak realm setup"
  log_info "Keycloak URL: $KEYCLOAK_URL"
  log_info "Realm: $REALM_NAME"
  if [[ -n "${DRY_RUN:-}" ]]; then log_warn "DRY RUN enabled - no changes will be made"; fi
  if [[ -n "${INSECURE:-}" ]]; then log_warn "TLS verification disabled (--insecure)"; fi
  if [[ -n "${CURL_TIMEOUT:-}" ]]; then log_info "HTTP timeout: ${CURL_TIMEOUT}s"; fi
  log_debug "Clients count: ${#CLIENTS[@]}"
}

get_access_token() {
  log_info "Obtaining admin access token..."
  if [[ -n "${DRY_RUN:-}" ]]; then
    ACCESS_TOKEN="dry-run-token"
    log_info "DRY-RUN: would POST $KEYCLOAK_URL/realms/master/protocol/openid-connect/token"
    return 0
  fi
  local ctype
  if ! curl_with_retries -X POST "$KEYCLOAK_URL/realms/master/protocol/openid-connect/token" \
      -H "Content-Type: application/x-www-form-urlencoded" \
      --data-urlencode "username=$KEYCLOAK_USER" \
      --data-urlencode "password=$KEYCLOAK_PASSWORD" \
      --data-urlencode 'grant_type=password' \
      --data-urlencode 'client_id=admin-cli'; then
    log_error "Token request failed (network error). Check connectivity to $KEYCLOAK_URL"
    rm -f "$CURL_BODY_FILE" "$CURL_HDR_FILE"
    exit 1
  fi
  ctype="$(awk -F': ' 'BEGIN{IGNORECASE=1}/^Content-Type:/{print $2}' "$CURL_HDR_FILE" | tr -d '\r')"
  if [[ "$CURL_STATUS" != "200" ]]; then
    log_error "Token request failed (HTTP $CURL_STATUS). Response: $(head -c 400 "$CURL_BODY_FILE")"
    rm -f "$CURL_BODY_FILE" "$CURL_HDR_FILE"
    exit 1
  fi
  if [[ -n "$ctype" && "${ctype,,}" != application/json* ]]; then
    log_warn "Unexpected Content-Type for token response: ${ctype}"
  fi
  ACCESS_TOKEN="$(jq -r '.access_token // empty' <"$CURL_BODY_FILE" 2>/dev/null || true)"
  rm -f "$CURL_BODY_FILE" "$CURL_HDR_FILE"
  if [[ -z "$ACCESS_TOKEN" ]]; then
    log_error "Failed to parse access token from response"
    exit 1
  fi
  log_info "Authenticated as '$KEYCLOAK_USER'"
}

ensure_realm_exists() {
  log_info "Ensuring realm exists: $REALM_NAME"
  if [[ -n "${DRY_RUN:-}" ]]; then
    log_info "DRY-RUN: would POST $KEYCLOAK_URL/admin/realms with {\"realm\":\"$REALM_NAME\",\"enabled\":true}"
    return 0
  fi
  local status check
  status=$(
    curl -s "${CURL_OPTS[@]}" -o /dev/null -w "%{http_code}" \
      -X POST "$KEYCLOAK_URL/admin/realms" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $ACCESS_TOKEN" \
      -d "{\"realm\": \"$REALM_NAME\", \"enabled\": true}"
  )
  case "$status" in
  200 | 201 | 204) log_info "Realm '$REALM_NAME' created or active (HTTP $status)" ;;
  409) log_warn "Realm '$REALM_NAME' already exists (HTTP 409)" ;;
  *)
    check=$(
      curl -s "${CURL_OPTS[@]}" -o /dev/null -w "%{http_code}" \
        -X GET "$KEYCLOAK_URL/admin/realms/$REALM_NAME" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    )
    if [[ "$check" == "200" ]]; then
      log_warn "Realm '$REALM_NAME' appears to exist despite create failure (HTTP $status)"
    else
      log_error "Realm create failed (HTTP $status) and realm not found (GET status $check). Aborting."
      exit 1
    fi
    ;;
  esac
}

parse_client_line() {
  local line="$1"
  local id name uri confidential root
  IFS=',' read -r id name uri confidential root <<<"$line"
  if [[ -z "${confidential:-}" && -n "${uri:-}" ]]; then
    confidential="$uri"
    uri="$name"
    name="$id"
  fi
  name="${name:-$id}"
  root="${root:-}"
  if [[ "$uri" == /* && -z "$root" && -n "$DEFAULT_CLIENT_ROOT_URL" ]]; then
    root="$DEFAULT_CLIENT_ROOT_URL"
  fi
  printf '%s|%s|%s|%s|%s' "$id" "$name" "$uri" "$confidential" "$root"
}

build_client_payload() {
  local id="$1" name="$2" uri="$3" confidential="$4" root="$5"
  local has_root has_redirect
  if [[ -n "$root" ]]; then has_root=true; else has_root=false; fi
  if [[ -n "$uri" ]]; then has_redirect=true; else has_redirect=false; fi
  jq -n \
    --arg id "$id" \
    --arg name "$name" \
    --arg root "$root" \
    --argjson hasRoot "$has_root" \
    --argjson hasRedirect "$has_redirect" \
    --argjson confidential "$confidential" \
    '{
      clientId: $id,
      name: $name,
      protocol: "openid-connect",
      enabled: true,
      publicClient: false,
      authorizationServicesEnabled: true,
      standardFlowEnabled: $hasRedirect,
      serviceAccountsEnabled: true
    } | if $hasRoot then . + {rootUrl: $root} else . end'
}

# Resolve client UUID and ensure roles exist
get_client_uuid() {
  local clientId="$1"
  if [[ -n "${DRY_RUN:-}" ]]; then
    printf '%s' "uuid-dryrun-$clientId"
    return 0
  fi
  local list status uuid
  list="$(mktemp)"
  status=$(
    curl -s "${CURL_OPTS[@]}" -o "$list" -w "%{http_code}" \
      -G "$KEYCLOAK_URL/admin/realms/$REALM_NAME/clients" \
      -H "Authorization: Bearer $ACCESS_TOKEN" \
      --data-urlencode "clientId=$clientId"
  )
  if [[ "$status" != "200" ]]; then
    log_error "Failed to fetch client '$clientId' (HTTP $status)"
    rm -f "$list"
    return 1
  fi
  uuid="$(jq -r '.[0].id // empty' <"$list")"
  rm -f "$list"
  if [[ -z "$uuid" ]]; then
    log_error "Client '$clientId' not found when resolving UUID"
    return 1
  fi
  printf '%s' "$uuid"
}

ensure_client_roles() {
  local clientId="$1"; shift
  local uuid role check_status create_status
  uuid="$(get_client_uuid "$clientId")" || return 1
  log_info "Ensuring roles for client '$clientId': $*"
  for role in "$@"; do
    if [[ -n "${DRY_RUN:-}" ]]; then
      log_info "DRY-RUN: would ensure role '$role' for client '$clientId'"
      continue
    fi
    check_status=$(
      curl -s "${CURL_OPTS[@]}" -o /dev/null -w "%{http_code}" \
        -X GET "$KEYCLOAK_URL/admin/realms/$REALM_NAME/clients/$uuid/roles/$role" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    )
    case "$check_status" in
      200)
        log_debug "Role '$role' already exists for client '$clientId'" ;;
      404)
        create_status=$(
          curl -s "${CURL_OPTS[@]}" -o /dev/null -w "%{http_code}" \
            -X POST "$KEYCLOAK_URL/admin/realms/$REALM_NAME/clients/$uuid/roles" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $ACCESS_TOKEN" \
            -d "{\"name\":\"$role\"}"
        )
        case "$create_status" in
          200|201|204) log_info "Created role '$role' for client '$clientId'" ;;
          409) log_warn "Role '$role' already exists for client '$clientId' (409)" ;;
          *)
            log_error "Failed to create role '$role' for client '$clientId' (HTTP $create_status)"
            return 1 ;;
        esac ;;
      *)
        log_error "Failed to check role '$role' for client '$clientId' (HTTP $check_status)"
        return 1 ;;
    esac
  done
}

# Lookup a client scope ID by name; echo ID or empty
get_client_scope_id() {
  local scope_name="$1"
  if [[ -n "${DRY_RUN:-}" ]]; then
    printf '%s' "scope-dryrun-$scope_name"
    return 0
  fi
  local list status scope_id
  list="$(mktemp)"
  status=$(
    curl -s "${CURL_OPTS[@]}" -o "$list" -w "%{http_code}" \
      -X GET "$KEYCLOAK_URL/admin/realms/$REALM_NAME/client-scopes" \
      -H "Authorization: Bearer $ACCESS_TOKEN"
  )
  if [[ "$status" != "200" ]]; then
    log_error "Failed to list client scopes (HTTP $status)"
    rm -f "$list"
    return 1
  fi
  scope_id="$(jq -r --arg n "$scope_name" '.[] | select(.name == $n) | .id' <"$list" | head -n1)"
  rm -f "$list"
  printf '%s' "$scope_id"
}

# Ensure client scopes exist and are assigned as default to a client
ensure_client_scopes() {
  local clientId="$1"; shift
  local uuid scope scope_id create_status assign_status
  uuid="$(get_client_uuid "$clientId")" || return 1
  log_info "Ensuring default client scopes for '$clientId': $*"
  for scope in "$@"; do
    if [[ -n "${DRY_RUN:-}" ]]; then
      log_info "DRY-RUN: would ensure scope '$scope' exists and is assigned to '$clientId'"
      continue
    fi
    scope_id="$(get_client_scope_id "$scope")" || return 1
    if [[ -z "$scope_id" ]]; then
      create_status=$(
        curl -s "${CURL_OPTS[@]}" -o /dev/null -w "%{http_code}" \
          -X POST "$KEYCLOAK_URL/admin/realms/$REALM_NAME/client-scopes" \
          -H "Content-Type: application/json" \
          -H "Authorization: Bearer $ACCESS_TOKEN" \
          -d "{\"name\":\"$scope\",\"protocol\":\"openid-connect\"}"
      )
      case "$create_status" in
        200|201|204)
          log_info "Created client scope '$scope'" ;;
        409)
          log_warn "Client scope '$scope' already exists (409)" ;;
        *)
          log_error "Failed to create client scope '$scope' (HTTP $create_status)"
          return 1 ;;
      esac
      scope_id="$(get_client_scope_id "$scope")" || return 1
      if [[ -z "$scope_id" ]]; then
        log_error "Unable to resolve client scope id for '$scope' after creation"
        return 1
      fi
    fi
    assign_status=$(
      curl -s "${CURL_OPTS[@]}" -o /dev/null -w "%{http_code}" \
        -X PUT "$KEYCLOAK_URL/admin/realms/$REALM_NAME/clients/$uuid/default-client-scopes/$scope_id" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    )
    case "$assign_status" in
      204)
        log_info "Assigned default client scope '$scope' to '$clientId'" ;;
      409)
        log_warn "Default client scope '$scope' already assigned to '$clientId' (409)" ;;
      404)
        log_error "Assignment failed: client or scope not found (404) for scope '$scope'"
        return 1 ;;
      *)
        log_error "Failed to assign client scope '$scope' to '$clientId' (HTTP $assign_status)"
        return 1 ;;
    esac
  done
}

create_or_verify_client() {
  local id="$1" name="$2" uri="$3" confidential="$4" root="$5"
  log_info "Ensuring client exists: $id ($name)"
  log_debug "Client name=$name redirectUri=$uri confidential=$confidential rootUrl=${root:-}"
  if [[ -z "$uri" ]]; then
    log_warn "No redirect URI provided; standard (authorization code) flow will be disabled for client '$id'"
  fi
  local payload
  payload="$(build_client_payload "$id" "$name" "$uri" "$confidential" "$root")"
  if [[ -n "${DRY_RUN:-}" ]]; then
    log_info "DRY-RUN: would POST $KEYCLOAK_URL/admin/realms/$REALM_NAME/clients"
    log_debug "DRY-RUN payload: $(echo "$payload" | jq -c . 2>/dev/null || echo "$payload")"
    # Simulate successful existence checks for roles/scopes
    if ! ensure_client_roles "$id" read "read:users" write; then
      log_error "DRY-RUN: roles ensure failed for '$id'"
      return 1
    fi
    if ! ensure_client_scopes "$id" camunda-identity roles service_account web-origins; then
      log_error "DRY-RUN: scopes ensure failed for '$id'"
      return 1
    fi
    return 0
  fi
  local body status
  body="$(mktemp)"
  status=$(
    curl -s "${CURL_OPTS[@]}" -o "$body" -w "%{http_code}" \
      -X POST "$KEYCLOAK_URL/admin/realms/$REALM_NAME/clients" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $ACCESS_TOKEN" \
      -d "$payload"
  )
  case "$status" in
  200 | 201 | 204) log_info "Client '$id' created or active (HTTP $status)" ;;
  409) log_warn "Client '$id' already exists (HTTP 409)" ;;
  *)
    log_debug "Client '$id' create failed. Payload (truncated): $(echo "$payload" | head -c 400)"
    local list json_status
    list="$(mktemp)"
    json_status=$(
      curl -s "${CURL_OPTS[@]}" -o "$list" -w "%{http_code}" \
        -G "$KEYCLOAK_URL/admin/realms/$REALM_NAME/clients" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        --data-urlencode "clientId=$id"
    )
    if [[ "$json_status" == "200" ]] && jq -e 'type=="array" and length>0' <"$list" >/dev/null 2>&1; then
      log_warn "Client '$id' appears to exist despite create failure (HTTP $status)"
    else
      log_error "Client '$id' create failed (HTTP $status). Response: $(head -c 400 "$body"). Client not found (GET status $json_status). Aborting."
      rm -f "$list" "$body"
      exit 1
    fi
    rm -f "$list"
    ;;
  esac
  # Ensure default roles on the client
  if ! ensure_client_roles "$id" read "read:users" write; then
    log_error "Failed to ensure roles for client '$id'"
    rm -f "$body"
    exit 1
  fi
  # Ensure default client scopes on the client
  if ! ensure_client_scopes "$id" camunda-identity roles service_account web-origins; then
    log_error "Failed to ensure client scopes for client '$id'"
    rm -f "$body"
    exit 1
  fi
  rm -f "$body"
}

main() {
  require_deps
  log_start_info
  get_access_token
  ensure_realm_exists
  local parsed id name uri confidential root
  for client in "${CLIENTS[@]}"; do
    parsed="$(parse_client_line "$client")"
    IFS='|' read -r id name uri confidential root <<<"$parsed"
    create_or_verify_client "$id" "$name" "$uri" "$confidential" "$root"
  done
  log_info "✅ Realm '$REALM_NAME' setup complete"
}

main "$@"
