#!/usr/bin/env bash
set -euo pipefail

# --- Logging helpers ---
timestamp() { date +"%Y-%m-%dT%H:%M:%S%z"; }

is_tty=0
if [[ -t 1 ]]; then
  is_tty=1
fi

if [[ "$is_tty" -eq 1 ]] && command -v tput >/dev/null 2>&1; then
  COLOR_RED="$(tput setaf 1)"
  COLOR_YELLOW="$(tput setaf 3)"
  COLOR_GREEN="$(tput setaf 2)"
  COLOR_BLUE="$(tput setaf 4)"
  COLOR_RESET="$(tput sgr0)"
else
  COLOR_RED=""
  COLOR_YELLOW=""
  COLOR_GREEN=""
  COLOR_BLUE=""
  COLOR_RESET=""
fi

log() {
  local level="$1"; shift
  local color=""
  case "$level" in
    INFO) color="$COLOR_BLUE" ;;
    WARN) color="$COLOR_YELLOW" ;;
    ERROR) color="$COLOR_RED" ;;
    OK) color="$COLOR_GREEN" ;;
  esac
  printf "%s %s[%s]%s %s\n" "$(timestamp)" "$color" "$level" "$COLOR_RESET" "$*" >&2
}

info() { log INFO "$@"; }
warn() { log WARN "$@"; }
error() { log ERROR "$@"; }
success() { log OK "$@"; }

DEBUG="${DEBUG:-0}"
debug() { [[ "$DEBUG" == "1" ]] && log INFO "[debug] $*" || true; }

on_error() {
  local exit_code=$?
  local line_no=${1:-unknown}
  error "Failed at line ${line_no} with exit code ${exit_code}."
}
trap 'on_error ${LINENO}' ERR

cleanup() {
  [[ -f "${tmp_realm_json:-}" ]] && rm -f "$tmp_realm_json" || true
}
trap cleanup EXIT

# --- Inputs ---
CHART_PATH="${CHART_PATH:-$1}"         # e.g. path/to/chart
REALM_NAME="${REALM_NAME:-$2}"         # e.g. my-realm
TEST_NAMESPACE="${TEST_NAMESPACE:-$3}" # e.g. qa-namespace

# --- Derived paths ---
integration_dir="${CHART_PATH}/test/integration"
realm_json="${integration_dir}/realm.json"
tmp_realm_json="${integration_dir}/realm.tmp.json"

# --- Validate inputs ---
print_usage() {
  {
    echo "Usage: CHART_PATH=... REALM_NAME=... TEST_NAMESPACE=... $0"
    echo "   or: $0 <CHART_PATH> <REALM_NAME> <TEST_NAMESPACE>"
  } >&2
}

if [[ -z "${CHART_PATH}" || -z "${REALM_NAME}" || -z "${TEST_NAMESPACE}" ]]; then
  print_usage
  exit 1
fi

# --- Pre-flight checks ---
for dep in jq kubectl; do
  if ! command -v "$dep" >/dev/null 2>&1; then
    error "Required dependency not found: $dep"
    exit 1
  fi
done

if [[ ! -f "$realm_json" ]]; then
  error "Realm JSON not found at: $realm_json"
  exit 1
fi

info "Preparing to load Keycloak realm '${REALM_NAME}' from chart '${CHART_PATH}' into namespace '${TEST_NAMESPACE}'."

# --- Modify realm.json ---
info "Updating realm.json for realm '${REALM_NAME}'..."
jq --arg realm "$REALM_NAME" '
  .id = $realm |
  .realm = $realm |
  (.roles.realm[]? | .containerId) = $realm |
  (.defaultRole.containerId) = $realm
' "$realm_json" >"$tmp_realm_json"

# --- Create ConfigMap ---
info "Creating ConfigMap 'realm-json' in namespace '${TEST_NAMESPACE}'..."
kubectl create configmap realm-json \
  -n "$TEST_NAMESPACE" \
  --from-file=realm.json="$tmp_realm_json" \
  --dry-run=client -o yaml | kubectl apply -f -

success "ConfigMap 'realm-json' applied successfully."
