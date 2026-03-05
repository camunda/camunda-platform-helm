#!/usr/bin/env bash
#
# ci-cleanup-keycloak-realms.sh — Clean up orphaned CI-created Keycloak realms.
#
# Two realm naming conventions exist in CI:
#
#   1. Go CLI (generateCompactRealmName in deploy-camunda):
#      {scenario}-{8-char-random-suffix}
#      e.g. "keycloak-mt-a8x9z3k1"
#
#   2. GitHub Actions workflow (workflow-vars action):
#      {6-hex-job-id}-realm
#      e.g. "d6da2c-realm", "82ceea-realm"
#      Generated from: GITHUB_WORKFLOW_JOB_ID=$(sha256sum | head -c 6)
#      The same 6-hex ID appears in the namespace name, so namespace
#      cross-reference works by grepping for the hex prefix.
#
# Two-phase cleanup:
#   Phase 1 — Identify orphans using local-only filtering:
#     - Get all realm names + enabled status (single API call)
#     - Get all active K8s namespaces (single K8s call)
#     - A CI-pattern realm is orphaned if its identifier does not appear
#       in any active namespace name.
#     No per-realm API calls (sessions, users) — too slow at scale.
#
#   Phase 2 — Disable-then-delete:
#     a) DISABLE all enabled orphans (~0.5s each, parallelized).
#        Disabled realms return "403 Realm not enabled" which immediately
#        stops CLIENT_LOGIN_ERROR spam from stale CI job credentials.
#     b) DELETE already-disabled orphans in small batches (DELETE_BATCH_SIZE
#        per run, default 20). Deletion is slow (~20-30s each) but orphans
#        are inert once disabled, so there is no urgency.
#
# The "master" realm is always protected.
#
# Usage:
#   ./ci-cleanup-keycloak-realms.sh \
#     --url http://localhost:8080 --user admin --pass secret
#
#   # Dry run:
#   ./ci-cleanup-keycloak-realms.sh \
#     --url http://localhost:8080 --user admin --pass secret --dry-run
#
#   # In-cluster (CronJob):
#   KEYCLOAK_PASSWORD=secret ./ci-cleanup-keycloak-realms.sh
#

set -euo pipefail

log() { echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] $*"; }

usage() {
  cat << EOF
Usage: $0 [options]

Clean up orphaned CI-created Keycloak realms.

Options:
  --url URL             Keycloak base URL
                        (default: \$KEYCLOAK_URL or http://localhost:8080)
  --user USER           Admin username
                        (default: \$KEYCLOAK_USER or "admin")
  --pass PASS           Admin password
                        (default: \$KEYCLOAK_PASSWORD)
  --disable-parallel N  Parallel disable requests (default: 10)
  --delete-batch N      Max realms to DELETE per run (default: 20)
  --delete-parallel N   Parallel delete requests (default: 2)
  --dry-run             Show what would be done, do not mutate
  --debug               Verbose output
  -h, --help            Show this help and exit

Environment variables (used as defaults when flags are not provided):
  KEYCLOAK_URL       Keycloak base URL (default: http://localhost:8080)
  KEYCLOAK_USER      Admin username (default: admin)
  KEYCLOAK_PASSWORD  Admin password
EOF
}

require_cmd() {
  command -v "$1" > /dev/null 2>&1 || {
    log "Required command not found: $1"
    exit 127
  }
}

# --- Defaults ---
KC_URL="${KEYCLOAK_URL:-http://localhost:8080}"
KC_USER="${KEYCLOAK_USER:-admin}"
KC_PASS="${KEYCLOAK_PASSWORD:-}"
DISABLE_PARALLEL=10
DELETE_BATCH_SIZE=20
DELETE_PARALLEL=2
DRY_RUN=0
DEBUG="${DEBUG:-0}"

# --- Argument parsing ---
while [[ $# -gt 0 ]]; do
  case "$1" in
    --url)              KC_URL="$2";           shift 2 ;;
    --user)             KC_USER="$2";          shift 2 ;;
    --pass)             KC_PASS="$2";          shift 2 ;;
    --disable-parallel) DISABLE_PARALLEL="$2"; shift 2 ;;
    --delete-batch)     DELETE_BATCH_SIZE="$2"; shift 2 ;;
    --delete-parallel)  DELETE_PARALLEL="$2";  shift 2 ;;
    --dry-run)          DRY_RUN=1;             shift   ;;
    --debug)            DEBUG=1;               shift   ;;
    -h|--help)          usage; exit 0                  ;;
    *)                  log "Unknown option: $1"; usage; exit 1 ;;
  esac
done

require_cmd curl
require_cmd jq

if [[ -z "$KC_PASS" ]]; then
  log "ERROR: No password provided. Use --pass or set KEYCLOAK_PASSWORD."
  exit 1
fi

KC_URL="${KC_URL%/}"

debug() { [[ "$DEBUG" == "1" ]] && log "[DEBUG] $*" || true; }

# --- Token management ---
TOKEN=""
TOKEN_ACQUIRED_AT=0
TOKEN_REFRESH_INTERVAL=45

acquire_token() {
  local response
  response=$(curl -sk -X POST \
    "${KC_URL}/auth/realms/master/protocol/openid-connect/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "grant_type=password&client_id=admin-cli&username=${KC_USER}&password=${KC_PASS}" \
    2>/dev/null)
  local new_token
  new_token=$(echo "$response" | jq -r '.access_token // empty')
  if [[ -z "$new_token" ]]; then
    log "ERROR: Failed to acquire/refresh admin token."
    debug "Response: ${response}"
    return 1
  fi
  TOKEN="$new_token"
  TOKEN_ACQUIRED_AT=$(date +%s)
  debug "Token acquired/refreshed"
}

ensure_token() {
  local now elapsed
  now=$(date +%s)
  elapsed=$(( now - TOKEN_ACQUIRED_AT ))
  if [[ "$elapsed" -ge "$TOKEN_REFRESH_INTERVAL" ]]; then
    debug "Token age ${elapsed}s, refreshing..."
    acquire_token || { log "ERROR: Token refresh failed."; exit 1; }
  fi
}

# --- Namespace fetching ---
fetch_namespaces() {
  NS_FILE=$(mktemp)
  local K8S_TOKEN_FILE="/var/run/secrets/kubernetes.io/serviceaccount/token"
  local K8S_CA="/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
  local K8S_API="https://kubernetes.default.svc"

  if [[ -f "$K8S_TOKEN_FILE" ]]; then
    debug "Using in-cluster service account"
    local K8S_TOKEN
    K8S_TOKEN="$(cat "$K8S_TOKEN_FILE")"
    curl -s --cacert "${K8S_CA}" -H "Authorization: Bearer ${K8S_TOKEN}" \
      "${K8S_API}/api/v1/namespaces" \
      | jq -r '.items[].metadata.name' > "${NS_FILE}" 2>/dev/null
  elif command -v kubectl > /dev/null 2>&1; then
    debug "Using kubectl"
    kubectl get namespaces -o jsonpath='{.items[*].metadata.name}' \
      | tr ' ' '\n' > "${NS_FILE}" 2>/dev/null
  else
    log "ERROR: No K8s access. Cannot safely identify orphans."
    return 1
  fi

  if [[ ! -s "${NS_FILE}" ]]; then
    log "ERROR: Namespace list is empty. Aborting."
    return 1
  fi

  local ns_count
  ns_count=$(wc -l < "${NS_FILE}" | tr -d ' ')
  log "  Found ${ns_count} namespaces"
}

# ==================================================================
# Main
# ==================================================================
log "=== CI Keycloak Realm Cleanup ==="
log "  URL:              ${KC_URL}"
log "  Disable parallel: ${DISABLE_PARALLEL}"
log "  Delete batch:     ${DELETE_BATCH_SIZE} per run"
log "  Delete parallel:  ${DELETE_PARALLEL}"
[[ "${DRY_RUN}" -eq 1 ]] && log "  MODE: DRY RUN"

# --- Acquire admin token ---
log "Acquiring admin token..."
acquire_token || exit 1
log "Admin token acquired."

# --- Fetch namespaces ---
echo ""
log "--- Fetching active K8s namespaces ---"
fetch_namespaces || exit 1

# --- List realms (single API call) ---
echo ""
log "--- Listing Keycloak realms ---"
ensure_token
# Get realm name + enabled status in one call
REALM_JSON=$(curl -sk -H "Authorization: Bearer ${TOKEN}" \
  "${KC_URL}/auth/admin/realms" 2>/dev/null || echo "[]")
REALM_DATA=$(echo "$REALM_JSON" | jq -r '.[] | "\(.realm)\t\(.enabled)"' 2>/dev/null || true)

if [[ -z "$REALM_DATA" ]]; then
  log "No realms found (or failed to list)."
  rm -f "${NS_FILE:-}"
  exit 0
fi

total=$(echo "$REALM_DATA" | wc -l | tr -d ' ')
log "  Found ${total} realms"

# ==================================================================
# Phase 1: Identify orphans (local filtering — no per-realm API calls)
# ==================================================================
CI_REALM_REGEX_GO='^.+-[a-z0-9]{8}$'
CI_REALM_REGEX_GHA='^[a-f0-9]{6}-realm$'

# Output files: orphans that need disabling, orphans already disabled (ready to delete)
DISABLE_FILE=$(mktemp)
DELETE_FILE=$(mktemp)
kept_namespace=0
kept_not_ci=0

echo ""
log "--- Phase 1: Identifying orphans ---"
while IFS=$'\t' read -r realm enabled; do
  [[ -z "$realm" ]] && continue

  # Protect master
  if [[ "$realm" == "master" ]]; then
    kept_not_ci=$((kept_not_ci + 1))
    continue
  fi

  # Match CI patterns and extract namespace search key
  ns_search_key=""
  if echo "$realm" | grep -qE "$CI_REALM_REGEX_GHA"; then
    ns_search_key="${realm%-realm}"
  elif echo "$realm" | grep -qE "$CI_REALM_REGEX_GO"; then
    ns_search_key="${realm%-????????}"
  else
    kept_not_ci=$((kept_not_ci + 1))
    continue
  fi

  # Namespace cross-reference
  if grep -qF "$ns_search_key" "${NS_FILE}"; then
    kept_namespace=$((kept_namespace + 1))
    continue
  fi

  # Orphaned — categorize by enabled status
  if [[ "$enabled" == "true" ]]; then
    echo "$realm" >> "${DISABLE_FILE}"
  else
    echo "$realm" >> "${DELETE_FILE}"
  fi

done <<< "$REALM_DATA"

disable_count=0; [[ -s "${DISABLE_FILE}" ]] && disable_count=$(wc -l < "${DISABLE_FILE}" | tr -d ' ')
delete_ready=0;  [[ -s "${DELETE_FILE}" ]]  && delete_ready=$(wc -l < "${DELETE_FILE}" | tr -d ' ')
orphan_total=$((disable_count + delete_ready))
kept_total=$((kept_namespace + kept_not_ci))

log "  Orphans:          ${orphan_total} (${disable_count} enabled, ${delete_ready} already disabled)"
log "  Kept:             ${kept_total} (${kept_namespace} namespace match, ${kept_not_ci} non-CI/protected)"

# ==================================================================
# Phase 2a: Disable enabled orphans (fast — ~0.5s each)
# ==================================================================
echo ""
log "--- Phase 2a: Disabling ${disable_count} enabled orphans (${DISABLE_PARALLEL} parallel) ---"

disabled_ok=0
disabled_fail=0

if [[ "$disable_count" -gt 0 ]]; then
  DISABLE_RESULTS=$(mktemp -d)
  batch_count=0
  while IFS= read -r realm; do
    [[ -z "$realm" ]] && continue

    if [[ "${DRY_RUN}" -eq 1 ]]; then
      log "  [dry-run] Would DISABLE: ${realm}"
      disabled_ok=$((disabled_ok + 1))
      continue
    fi

    if [[ "$batch_count" -eq 0 ]]; then
      ensure_token
    fi

    (
      http_code=$(curl -sk -o /dev/null -w '%{http_code}' \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -X PUT "${KC_URL}/auth/admin/realms/${realm}" \
        -d "{\"realm\":\"${realm}\",\"enabled\":false}" 2>/dev/null)
      echo "${http_code}" > "${DISABLE_RESULTS}/${realm}"
    ) &

    batch_count=$((batch_count + 1))
    if [[ "$batch_count" -ge "$DISABLE_PARALLEL" ]]; then
      wait
      batch_count=0
    fi
  done < "${DISABLE_FILE}"
  wait

  # Tally
  if [[ "${DRY_RUN}" -eq 0 ]]; then
    for f in "${DISABLE_RESULTS}"/*; do
      [[ -f "$f" ]] || continue
      code=$(cat "$f")
      if [[ "$code" == "204" || "$code" == "200" ]]; then
        disabled_ok=$((disabled_ok + 1))
      else
        disabled_fail=$((disabled_fail + 1))
        log "  WARN: DISABLE '$(basename "$f")' → HTTP ${code}"
      fi
    done
  fi
  rm -rf "${DISABLE_RESULTS}"
fi

log "  Disabled: ${disabled_ok} OK, ${disabled_fail} failed"

# ==================================================================
# Phase 2b: Delete already-disabled orphans (slow — batched)
# ==================================================================
# Cap at DELETE_BATCH_SIZE per run to avoid hitting deadline.
# Disabled realms are inert (403 on all auth), so no urgency.
delete_target=$delete_ready
if [[ "$delete_target" -gt "$DELETE_BATCH_SIZE" ]]; then
  delete_target=$DELETE_BATCH_SIZE
fi

echo ""
log "--- Phase 2b: Deleting ${delete_target} of ${delete_ready} disabled orphans (${DELETE_PARALLEL} parallel) ---"

deleted_ok=0
deleted_fail=0

if [[ "$delete_target" -gt 0 ]]; then
  DELETE_RESULTS=$(mktemp -d)
  batch_count=0
  lines_read=0
  while IFS= read -r realm; do
    [[ -z "$realm" ]] && continue
    lines_read=$((lines_read + 1))
    [[ "$lines_read" -gt "$delete_target" ]] && break

    if [[ "${DRY_RUN}" -eq 1 ]]; then
      log "  [dry-run] Would DELETE: ${realm}"
      deleted_ok=$((deleted_ok + 1))
      continue
    fi

    if [[ "$batch_count" -eq 0 ]]; then
      ensure_token
    fi

    (
      http_code=$(curl -sk -o /dev/null -w '%{http_code}' \
        -H "Authorization: Bearer ${TOKEN}" \
        -X DELETE "${KC_URL}/auth/admin/realms/${realm}" 2>/dev/null)
      echo "${http_code}" > "${DELETE_RESULTS}/${realm}"
    ) &

    batch_count=$((batch_count + 1))
    if [[ "$batch_count" -ge "$DELETE_PARALLEL" ]]; then
      wait
      batch_count=0
    fi
  done < "${DELETE_FILE}"
  wait

  # Tally
  if [[ "${DRY_RUN}" -eq 0 ]]; then
    for f in "${DELETE_RESULTS}"/*; do
      [[ -f "$f" ]] || continue
      code=$(cat "$f")
      if [[ "$code" == "204" || "$code" == "200" ]]; then
        deleted_ok=$((deleted_ok + 1))
      else
        deleted_fail=$((deleted_fail + 1))
        log "  WARN: DELETE '$(basename "$f")' → HTTP ${code}"
      fi
    done
  fi
  rm -rf "${DELETE_RESULTS}"
fi

log "  Deleted: ${deleted_ok} OK, ${deleted_fail} failed"
if [[ "$delete_ready" -gt "$DELETE_BATCH_SIZE" ]]; then
  log "  Remaining: $((delete_ready - delete_target)) disabled orphans (will be deleted in future runs)"
fi

# --- Cleanup ---
rm -f "${NS_FILE:-}" "${DISABLE_FILE:-}" "${DELETE_FILE:-}"

# --- Summary ---
echo ""
log "=== Cleanup complete ==="
log "  Total realms:       ${total}"
log "  Orphans:            ${orphan_total}"
log "    Disabled (this run): ${disabled_ok} OK / ${disabled_fail} fail"
log "    Deleted (this run):  ${deleted_ok} OK / ${deleted_fail} fail"
log "  Kept:               ${kept_total}"
log "    Namespace match:   ${kept_namespace}"
log "    Non-CI/protected:  ${kept_not_ci}"
[[ "${DRY_RUN}" -eq 1 ]] && log "  (dry-run — nothing was actually changed)"
