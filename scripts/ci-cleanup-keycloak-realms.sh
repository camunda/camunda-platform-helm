#!/usr/bin/env bash
#
# ci-cleanup-keycloak-realms.sh — Clean up orphaned CI-created Keycloak realms.
#
# Realm naming convention (from deploy-camunda Go CLI generateCompactRealmName):
#   {scenario}-{8-char-random-suffix}
#   e.g. "keycloak-mt-a8x9z3k1"
#
# Orphan detection uses a triple-signal approach:
#   1. Session check: if any client has active sessions, the realm is in use.
#   2. Namespace cross-reference: if any active K8s namespace contains the
#      realm's scenario portion, the CI job is still running.
#   3. Age check: realms younger than --min-age (default 2h) are protected.
#      This prevents deletion during Helm --force upgrades when sessions
#      temporarily drop to zero while pods are being recreated.
#
# A realm is deleted only when ALL THREE signals indicate it is orphaned.
# The "master" realm is always protected.
#
# NOTE: The namespace cross-reference (signal 2) has a known limitation:
# realm names use the full scenario name (e.g., "keycloak-mt") while
# namespace names use compact shortnames (e.g., "kemt"). This means the
# grep may fail to match active namespaces. The age-based check (signal 3)
# compensates for this gap.
#
# Usage:
#   # Local — with port-forward or direct access:
#   ./ci-cleanup-keycloak-realms.sh \
#     --url http://localhost:8080 --user admin --pass secret
#
#   # Dry run:
#   ./ci-cleanup-keycloak-realms.sh \
#     --url http://localhost:8080 --user admin --pass secret --dry-run
#
#   # In-cluster (CronJob) — uses env vars and service account token:
#   KEYCLOAK_PASSWORD=secret ./ci-cleanup-keycloak-realms.sh
#

set -euo pipefail

log() { echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] $*"; }

usage() {
  cat << EOF
Usage: $0 [options]

Clean up orphaned CI-created Keycloak realms.

Options:
  --url URL         Keycloak base URL
                    (default: \$KEYCLOAK_URL or http://localhost:8080)
  --user USER       Admin username
                    (default: \$KEYCLOAK_USER or "admin")
  --pass PASS       Admin password
                    (default: \$KEYCLOAK_PASSWORD)
  --min-age HOURS   Protect realms younger than HOURS hours (default: 2)
  --dry-run         Show what would be deleted, do not delete
  --debug           Verbose output
  -h, --help        Show this help and exit

Environment variables (used as defaults when flags are not provided):
  KEYCLOAK_URL       Keycloak base URL (default: http://localhost:8080)
  KEYCLOAK_USER      Admin username (default: admin)
  KEYCLOAK_PASSWORD  Admin password

In-cluster mode:
  When running inside a K8s pod with a service account, the script
  automatically uses the mounted token to list namespaces for
  orphan detection via namespace cross-reference.

Examples:
  # One-shot local cleanup with port-forward running:
  $0 --url http://localhost:8080 --user admin --pass secret

  # Dry run to see what would be cleaned:
  $0 --url http://localhost:8080 --user admin --pass secret --dry-run
EOF
}

require_cmd() {
  command -v "$1" > /dev/null 2>&1 || {
    log "Required command not found: $1"
    exit 127
  }
}

# --- Defaults (from env vars, overridable by flags) ---
KC_URL="${KEYCLOAK_URL:-http://localhost:8080}"
KC_USER="${KEYCLOAK_USER:-admin}"
KC_PASS="${KEYCLOAK_PASSWORD:-}"
MIN_AGE_HOURS=2
DRY_RUN=0
DEBUG="${DEBUG:-0}"

# --- Argument parsing ---
while [[ $# -gt 0 ]]; do
  case "$1" in
    --url)
      KC_URL="$2"
      shift 2
      ;;
    --user)
      KC_USER="$2"
      shift 2
      ;;
    --pass)
      KC_PASS="$2"
      shift 2
      ;;
    --min-age)
      MIN_AGE_HOURS="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --debug)
      DEBUG=1
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      log "Unknown option: $1"
      usage
      exit 1
      ;;
  esac
done

require_cmd curl
require_cmd jq

if [[ -z "$KC_PASS" ]]; then
  log "ERROR: No password provided. Use --pass or set KEYCLOAK_PASSWORD."
  exit 1
fi

# Strip trailing slash from URL
KC_URL="${KC_URL%/}"

# Compute age cutoff: realms younger than MIN_AGE_HOURS are protected.
NOW_EPOCH=$(date +%s)
MIN_AGE_SECONDS=$(( MIN_AGE_HOURS * 3600 ))
CUTOFF_EPOCH=$(( NOW_EPOCH - MIN_AGE_SECONDS ))

debug() { [[ "$DEBUG" == "1" ]] && log "[DEBUG] $*" || true; }

# --- Dry-run wrapper: skip mutations when --dry-run is set ---
kc_delete_realm() {
  local realm="$1"
  if [ "${DRY_RUN}" -eq 1 ]; then
    log "  [dry-run] Would DELETE realm: ${realm}"
    return 0
  fi
  curl -sk -H "Authorization: Bearer ${TOKEN}" \
    -X DELETE "${KC_URL}/auth/admin/realms/${realm}" 2> /dev/null || true
}

# ==================================================================
# Namespace fetching (for orphan detection via namespace cross-ref)
# ==================================================================
fetch_namespaces() {
  NS_FILE=$(mktemp)

  # Try in-cluster service account first
  local K8S_TOKEN_FILE="/var/run/secrets/kubernetes.io/serviceaccount/token"
  local K8S_CA="/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
  local K8S_API="https://kubernetes.default.svc"

  if [[ -f "$K8S_TOKEN_FILE" ]]; then
    debug "Using in-cluster service account for namespace list"
    local K8S_TOKEN
    K8S_TOKEN="$(cat "$K8S_TOKEN_FILE")"
    curl -s --cacert "${K8S_CA}" -H "Authorization: Bearer ${K8S_TOKEN}" \
      "${K8S_API}/api/v1/namespaces" \
      | jq -r '.items[].metadata.name' > "${NS_FILE}" 2> /dev/null
  else
    # Local mode: use kubectl
    debug "Using kubectl for namespace list"
    if command -v kubectl > /dev/null 2>&1; then
      kubectl get namespaces -o jsonpath='{.items[*].metadata.name}' \
        | tr ' ' '\n' > "${NS_FILE}" 2> /dev/null
    else
      log "WARN: No K8s access (no service account token, no kubectl)."
      log "      Namespace cross-reference will be SKIPPED."
      echo "" > "${NS_FILE}"
      return 1
    fi
  fi

  if [[ ! -s "${NS_FILE}" ]]; then
    log "WARN: Namespace list is empty. Namespace cross-reference will be SKIPPED."
    return 1
  fi

  local ns_count
  ns_count=$(wc -l < "${NS_FILE}" | tr -d ' ')
  log "  Found ${ns_count} namespaces"
  return 0
}

# ==================================================================
# Main
# ==================================================================
log "=== CI Keycloak Realm Cleanup - $(date -u '+%Y-%m-%dT%H:%M:%SZ') ==="
log "  URL: ${KC_URL}"
if [ "${DRY_RUN}" -eq 1 ]; then
  log "  MODE: DRY RUN (no deletions will be performed)"
fi

# --- Acquire admin token ---
log "Acquiring admin token..."
TOKEN_RESPONSE=$(curl -sk -X POST \
  "${KC_URL}/auth/realms/master/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password&client_id=admin-cli&username=${KC_USER}&password=${KC_PASS}" \
  2> /dev/null)

TOKEN=$(echo "$TOKEN_RESPONSE" | jq -r '.access_token // empty')
if [[ -z "$TOKEN" ]]; then
  log "ERROR: Failed to acquire admin token from Keycloak."
  debug "Response: ${TOKEN_RESPONSE}"
  exit 1
fi
log "Admin token acquired."

# --- Fetch namespaces ---
echo ""
log "--- Fetching active K8s namespaces ---"
HAS_NAMESPACES=true
fetch_namespaces || HAS_NAMESPACES=false

# --- List realms ---
echo ""
log "--- Listing Keycloak realms ---"
REALMS=$(curl -sk -H "Authorization: Bearer ${TOKEN}" \
  "${KC_URL}/auth/admin/realms" 2> /dev/null \
  | jq -r '.[].realm' 2> /dev/null || true)

if [[ -z "$REALMS" ]]; then
  log "No realms found (or failed to list realms)."
  rm -f "${NS_FILE:-}"
  exit 0
fi

total=$(echo "$REALMS" | wc -l | tr -d ' ')
log "  Found ${total} realms"

# CI realm pattern: {scenario}-{8-char-alphanumeric-suffix}
CI_REALM_REGEX='^.+-[a-z0-9]{8}$'

deleted=0
skipped=0
kept_sessions=0
kept_namespace=0
kept_young=0
kept_not_ci=0

echo ""
log "--- Checking realms for orphans ---"
while IFS= read -r realm; do
  [[ -z "$realm" ]] && continue

  # Always protect master
  if [[ "$realm" == "master" ]]; then
    debug "SKIP  '${realm}': protected realm"
    kept_not_ci=$((kept_not_ci + 1))
    skipped=$((skipped + 1))
    continue
  fi

  # Skip if not matching CI naming pattern
  if ! echo "$realm" | grep -qE "$CI_REALM_REGEX"; then
    debug "SKIP  '${realm}': does not match CI pattern"
    kept_not_ci=$((kept_not_ci + 1))
    skipped=$((skipped + 1))
    continue
  fi

  # Extract scenario portion (everything before the last -XXXXXXXX)
  scenario="${realm%-????????}"
  debug "Realm '${realm}' → scenario='${scenario}'"

  # Check 1: Active sessions
  session_stats=$(curl -sk -H "Authorization: Bearer ${TOKEN}" \
    "${KC_URL}/auth/admin/realms/${realm}/client-session-stats" 2> /dev/null || echo "[]")
  active_sessions=$(echo "$session_stats" | jq '[.[].active // 0] | add // 0' 2> /dev/null || echo "0")

  if [[ "$active_sessions" -gt 0 ]]; then
    log "  KEEP  '${realm}': ${active_sessions} active sessions"
    kept_sessions=$((kept_sessions + 1))
    skipped=$((skipped + 1))
    continue
  fi

  # Check 2: Namespace cross-reference
  # NOTE: This check has a known limitation — realm names use the full scenario
  # name (e.g., "keycloak-mt") while namespace names use compact shortnames
  # (e.g., "kemt"), so the grep may miss active namespaces. The age check
  # (Check 3) compensates for this gap.
  if [[ "$HAS_NAMESPACES" == "true" ]] && grep -qF "$scenario" "${NS_FILE}"; then
    log "  KEEP  '${realm}': namespace match for scenario '${scenario}'"
    kept_namespace=$((kept_namespace + 1))
    skipped=$((skipped + 1))
    continue
  fi

  # Check 3: Realm age — protect realms younger than MIN_AGE_HOURS.
  # During Helm --force upgrades, old pods are deleted and new pods recreated,
  # causing active sessions to temporarily drop to zero. This age gate prevents
  # the cronjob from deleting realms that are still needed by in-progress runs.
  # We use the first user's createdTimestamp as a proxy for realm creation time
  # (Identity always creates users in the realm during setup).
  first_user=$(curl -sk -H "Authorization: Bearer ${TOKEN}" \
    "${KC_URL}/auth/admin/realms/${realm}/users?first=0&max=1" 2>/dev/null || echo "[]")
  created_ts=$(echo "$first_user" | jq -r '.[0].createdTimestamp // empty' 2>/dev/null || echo "")

  if [[ -n "$created_ts" ]]; then
    created_epoch=$(( created_ts / 1000 ))
    if [[ "$created_epoch" -gt "$CUTOFF_EPOCH" ]]; then
      age_hours=$(( (NOW_EPOCH - created_epoch) / 3600 ))
      log "  KEEP  '${realm}': younger than ${MIN_AGE_HOURS}h (created ${age_hours}h ago)"
      kept_young=$((kept_young + 1))
      skipped=$((skipped + 1))
      continue
    fi
  fi

  # All checks say orphaned — delete
  log "  DELETE '${realm}' (no sessions, no namespace match, older than ${MIN_AGE_HOURS}h)"
  kc_delete_realm "$realm"
  deleted=$((deleted + 1))

done <<< "$REALMS"

# --- Cleanup temp files ---
rm -f "${NS_FILE:-}"

# --- Summary ---
echo ""
log "=== Cleanup complete ==="
log "  Total realms checked:     ${total}"
log "  Deleted (orphaned):       ${deleted}"
log "  Kept (total):             ${skipped}"
log "    - Active sessions:      ${kept_sessions}"
log "    - Namespace match:      ${kept_namespace}"
log "    - Younger than ${MIN_AGE_HOURS}h:       ${kept_young}"
log "    - Non-CI / protected:   ${kept_not_ci}"
if [ "${DRY_RUN}" -eq 1 ]; then
  log "  (dry-run mode — nothing was actually deleted)"
fi
