#!/usr/bin/env bash
#
# ci-cleanup-elasticsearch.sh — Clean up orphaned CI-created Elasticsearch
# indices and templates.
#
# Handles TWO naming conventions used by CI:
#
# Convention 1 — Hex-prefixed (6-char hex):
#   e.g. "abc123-operate-*". The hex is the last segment of the CI K8s
#   namespace. Safety: cross-reference against active namespaces.
#
# Convention 2 — Component-prefixed (8-char random):
#   e.g. "orch-elasticsearch-xnqzizh7-*". The 8-char ID is crypto/rand
#   and cannot be correlated to namespaces. Safety: age-based cleanup.
#
# Also cleans up:
#   - Broken indices with literal "$" (un-interpolated Helm vars)
#   - Sets replicas=0 on all non-system indices (CI cluster optimization)
#
# Usage:
#   # Local — with port-forward or direct access:
#   ./ci-cleanup-elasticsearch.sh \
#     --url https://localhost:9200 --user elastic --pass secret
#
#   # Local — auto-fetch password from K8s secret:
#   ./ci-cleanup-elasticsearch.sh \
#     --url https://localhost:9200 --user elastic \
#     --elasticsearch-namespace distribution-elasticsearch-21-6-3
#
#   # Dry run:
#   ./ci-cleanup-elasticsearch.sh \
#     --url https://localhost:9200 --user elastic --pass secret --dry-run
#
#   # In-cluster (CronJob) — uses env vars and service account token:
#   ES_PASSWORD=secret MAX_INDEX_AGE_HOURS=4 ./ci-cleanup-elasticsearch.sh
#

set -euo pipefail

log() { echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] $*"; }

usage() {
  cat << EOF
Usage: $0 [options]

Clean up orphaned CI-created Elasticsearch indices and templates.

Options:
  --url URL                        Elasticsearch base URL
                                   (default: \$ES_URL or https://localhost:9200)
  --user USER                      Username for basic auth
                                   (default: \$ES_USER or "elastic")
  --pass PASS                      Password for basic auth
                                   (default: \$ES_PASSWORD)
  --elasticsearch-namespace NS     K8s namespace to fetch password via
                                   scripts/get-credientials-from-cluster.sh
  --max-age-hours N                Max age in hours for component-prefixed
                                   resources (default: \$MAX_INDEX_AGE_HOURS or 4)
  --dry-run                        Show what would be deleted, do not delete
  --debug                          Verbose output
  -h, --help                       Show this help and exit

Environment variables (used as defaults when flags are not provided):
  ES_URL                 Elasticsearch URL (default: https://localhost:9200)
  ES_USER                Auth username (default: elastic)
  ES_PASSWORD            Auth password
  MAX_INDEX_AGE_HOURS    Age threshold for component-prefixed cleanup (default: 4)

In-cluster mode:
  When running inside a K8s pod with a service account, the script
  automatically uses the mounted token to list namespaces for
  hex-prefix orphan detection.

Examples:
  # One-shot local cleanup with port-forward running:
  $0 --url https://localhost:9200 --user elastic --pass secret

  # Dry run to see what would be cleaned:
  $0 --url https://localhost:9200 --user elastic --pass secret --dry-run

  # Auto-discover password from K8s:
  $0 --url https://localhost:9200 --elasticsearch-namespace distribution-elasticsearch-21-6-3
EOF
}

require_cmd() {
  command -v "$1" > /dev/null 2>&1 || {
    log "Required command not found: $1"
    exit 127
  }
}

# --- Defaults (from env vars, overridable by flags) ---
ES_URL="${ES_URL:-https://localhost:9200}"
ES_USER="${ES_USER:-elastic}"
ES_PASS="${ES_PASSWORD:-}"
ELASTICSEARCH_NAMESPACE=""
MAX_AGE_HOURS="${MAX_INDEX_AGE_HOURS:-1}"
DRY_RUN=0
DEBUG="${DEBUG:-0}"

# --- Argument parsing ---
while [[ $# -gt 0 ]]; do
  case "$1" in
    --url)
      ES_URL="$2"
      shift 2
      ;;
    --user)
      ES_USER="$2"
      shift 2
      ;;
    --pass)
      ES_PASS="$2"
      shift 2
      ;;
    --elasticsearch-namespace)
      ELASTICSEARCH_NAMESPACE="$2"
      shift 2
      ;;
    --max-age-hours)
      MAX_AGE_HOURS="$2"
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

# --- Credential resolution ---
if [[ -z "$ES_PASS" && -n "$ELASTICSEARCH_NAMESPACE" ]]; then
  SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
  CRED_SCRIPT="${SCRIPT_DIR}/get-credientials-from-cluster.sh"
  if [[ -x "$CRED_SCRIPT" ]]; then
    ES_PASS="$("$CRED_SCRIPT" --namespace "$ELASTICSEARCH_NAMESPACE")"
  else
    log "scripts/get-credientials-from-cluster.sh not found or not executable"
    exit 1
  fi
fi

if [[ -z "$ES_PASS" ]]; then
  log "ERROR: No password provided. Use --pass, --elasticsearch-namespace, or set ES_PASSWORD."
  exit 1
fi

MAX_AGE_SECS=$((MAX_AGE_HOURS * 3600))
ES_CURL="curl -sk -u ${ES_USER}:${ES_PASS}"

debug() { [[ "$DEBUG" == "1" ]] && log "[DEBUG] $*" || true; }

# --- Dry-run wrapper: skip mutations when --dry-run is set ---
es_delete() {
  local url="$1"
  if [ "${DRY_RUN}" -eq 1 ]; then
    log "  [dry-run] Would DELETE: ${url}"
    echo '{"acknowledged":true}'
    return 0
  fi
  ${ES_CURL} -s -XDELETE "${url}" 2> /dev/null || true
}

es_put() {
  local url="$1"
  shift
  if [ "${DRY_RUN}" -eq 1 ]; then
    log "  [dry-run] Would PUT: ${url}"
    echo '{"acknowledged":true}'
    return 0
  fi
  ${ES_CURL} -s -XPUT "${url}" "$@" 2> /dev/null || true
}

# ==================================================================
# Namespace fetching (for hex-prefix orphan detection)
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
      log "      Hex-prefix orphan detection will be SKIPPED."
      log "      Component-prefixed age-based cleanup will still run."
      echo "" > "${NS_FILE}"
      return 1
    fi
  fi

  if [[ ! -s "${NS_FILE}" ]]; then
    log "WARN: Namespace list is empty. Hex-prefix orphan detection will be SKIPPED."
    return 1
  fi

  local ns_count
  ns_count=$(wc -l < "${NS_FILE}" | tr -d ' ')
  log "  Found ${ns_count} namespaces"
  return 0
}

# Helper: check if a hex prefix belongs to an active CI job
is_active_prefix() {
  grep -qF "$1" "${NS_FILE}"
}

# ==================================================================
# Main
# ==================================================================
log "=== CI Resource Cleanup - $(date -u '+%Y-%m-%dT%H:%M:%SZ') ==="
log "  URL: ${ES_URL}"
log "  Max index age: ${MAX_AGE_HOURS}h"
if [ "${DRY_RUN}" -eq 1 ]; then
  log "  MODE: DRY RUN (no deletions will be performed)"
fi

# --- Verify ES connectivity ---
if ! ${ES_CURL} "${ES_URL}/_cluster/health" > /dev/null 2>&1; then
  log "ERROR: Cannot reach Elasticsearch at ${ES_URL}"
  exit 1
fi
log "Elasticsearch connectivity: OK"

# --- Fetch namespaces ---
echo ""
log "--- Fetching active K8s namespaces ---"
HAS_NAMESPACES=true
fetch_namespaces || HAS_NAMESPACES=false

# Track totals
total_indices_deleted=0
total_v2_deleted=0
total_comp_deleted=0
total_legacy_deleted=0

# ==============================================================
# PART A: Hex-prefixed resources (Convention 1)
#         Orphan detection via K8s namespace cross-reference.
#         Deletes BOTH indices and templates.
# ==============================================================
echo ""
echo "=========================================="
echo "PART A: Hex-prefixed resources ([0-9a-f]{6}-*)"
echo "=========================================="

if [[ "$HAS_NAMESPACES" == "false" ]]; then
  log "  SKIPPED — no namespace data available for orphan detection"
else

  # --- A1: Hex-prefixed INDICES ---
  echo ""
  log "--- A1: Hex-prefixed indices ---"

  HEX_IDX_FILE=$(mktemp)
  ${ES_CURL} -s "${ES_URL}/_cat/indices?h=index&format=json" 2> /dev/null \
    | jq -r '.[].index
             | select(test("^[0-9a-f]{6}-"))
             | select(startswith(".") | not)' \
      > "${HEX_IDX_FILE}" 2> /dev/null || true

  hex_idx_deleted=0
  hex_idx_skipped=0
  if [ -s "${HEX_IDX_FILE}" ]; then
    prefixes=$(cut -c1-6 "${HEX_IDX_FILE}" | sort -u)
    for prefix in $prefixes; do
      count=$(grep -c "^${prefix}-" "${HEX_IDX_FILE}" || true)
      if is_active_prefix "$prefix"; then
        log "  KEEP  '${prefix}': ${count} indices (namespace active)"
        hex_idx_skipped=$((hex_idx_skipped + count))
      else
        log "  DELETE '${prefix}': ${count} indices (orphaned)"
        resp=$(es_delete "${ES_URL}/${prefix}-*?expand_wildcards=open,closed")
        if echo "$resp" | grep -q '"acknowledged":true'; then
          hex_idx_deleted=$((hex_idx_deleted + count))
        else
          log "    WARN: ${resp}"
        fi
      fi
    done
  fi
  log "  Hex indices: deleted=${hex_idx_deleted} kept=${hex_idx_skipped}"
  total_indices_deleted=$((total_indices_deleted + hex_idx_deleted))
  rm -f "${HEX_IDX_FILE}"

  # --- A2: Hex-prefixed V2 index templates ---
  echo ""
  log "--- A2: Hex-prefixed V2 index templates ---"

  V2_FILE=$(mktemp)
  ${ES_CURL} -s "${ES_URL}/_index_template" 2> /dev/null \
    | jq -r '.index_templates[].name
             | select(test("^[0-9a-f]{6}-"))
             | select(startswith(".") | not)' \
      > "${V2_FILE}" 2> /dev/null || true

  v2_deleted=0
  v2_skipped=0
  if [ -s "${V2_FILE}" ]; then
    prefixes=$(cut -c1-6 "${V2_FILE}" | sort -u)
    for prefix in $prefixes; do
      count=$(grep -c "^${prefix}-" "${V2_FILE}" || true)
      if is_active_prefix "$prefix"; then
        log "  KEEP  '${prefix}': ${count} V2 templates (namespace active)"
        v2_skipped=$((v2_skipped + count))
      else
        log "  DELETE '${prefix}': ${count} V2 templates (orphaned)"
        resp=$(es_delete "${ES_URL}/_index_template/${prefix}-*")
        if echo "$resp" | grep -q '"acknowledged":true'; then
          v2_deleted=$((v2_deleted + count))
        else
          log "    WARN: ${resp}"
        fi
      fi
    done
  fi
  log "  Hex V2 templates: deleted=${v2_deleted} kept=${v2_skipped}"
  total_v2_deleted=$((total_v2_deleted + v2_deleted))
  rm -f "${V2_FILE}"

  # --- A3: Hex-prefixed component templates ---
  echo ""
  log "--- A3: Hex-prefixed component templates ---"

  COMP_FILE=$(mktemp)
  ${ES_CURL} -s "${ES_URL}/_component_template" 2> /dev/null \
    | jq -r '.component_templates[].name
             | select(test("^[0-9a-f]{6}-"))
             | select(startswith(".") | not)' \
      > "${COMP_FILE}" 2> /dev/null || true

  comp_deleted=0
  comp_skipped=0
  if [ -s "${COMP_FILE}" ]; then
    prefixes=$(cut -c1-6 "${COMP_FILE}" | sort -u)
    for prefix in $prefixes; do
      count=$(grep -c "^${prefix}-" "${COMP_FILE}" || true)
      if is_active_prefix "$prefix"; then
        log "  KEEP  '${prefix}': ${count} component templates (namespace active)"
        comp_skipped=$((comp_skipped + count))
      else
        log "  DELETE '${prefix}': ${count} component templates (orphaned)"
        resp=$(es_delete "${ES_URL}/_component_template/${prefix}-*")
        if echo "$resp" | grep -q '"acknowledged":true'; then
          comp_deleted=$((comp_deleted + count))
        else
          log "    WARN: ${resp}"
        fi
      fi
    done
  fi
  log "  Hex component templates: deleted=${comp_deleted} kept=${comp_skipped}"
  total_comp_deleted=$((total_comp_deleted + comp_deleted))
  rm -f "${COMP_FILE}"

  # --- A4: Hex-prefixed legacy (v1) templates ---
  echo ""
  log "--- A4: Hex-prefixed legacy templates ---"

  LEGACY_FILE=$(mktemp)
  ${ES_CURL} -s "${ES_URL}/_template" 2> /dev/null \
    | jq -r 'keys[]
             | select(test("^[0-9a-f]{6}-"))
             | select(startswith(".") | not)' \
      > "${LEGACY_FILE}" 2> /dev/null || true

  legacy_deleted=0
  legacy_skipped=0
  if [ -s "${LEGACY_FILE}" ]; then
    prefixes=$(cut -c1-6 "${LEGACY_FILE}" | sort -u)
    for prefix in $prefixes; do
      count=$(grep -c "^${prefix}-" "${LEGACY_FILE}" || true)
      if is_active_prefix "$prefix"; then
        log "  KEEP  '${prefix}': ${count} legacy templates (namespace active)"
        legacy_skipped=$((legacy_skipped + count))
      else
        log "  DELETE '${prefix}': ${count} legacy templates (orphaned)"
        resp=$(es_delete "${ES_URL}/_template/${prefix}-*")
        if echo "$resp" | grep -q '"acknowledged":true'; then
          legacy_deleted=$((legacy_deleted + count))
        else
          log "    WARN: ${resp}"
        fi
      fi
    done
  fi
  log "  Hex legacy templates: deleted=${legacy_deleted} kept=${legacy_skipped}"
  total_legacy_deleted=$((total_legacy_deleted + legacy_deleted))
  rm -f "${LEGACY_FILE}"

fi # HAS_NAMESPACES

# ==============================================================
# PART B: Component-prefixed resources (Convention 2)
#         Age-based cleanup. The 8-char random ID cannot be
#         correlated to K8s namespaces, so we delete any
#         resources older than MAX_AGE_HOURS.
#
#         Known component prefixes from deploy-camunda Go CLI:
#           orch-*   (orchestration)
#           opt-*    (optimize)
#           task-*   (tasklist)
#           op-*     (operate)
#
#         Also catches bare operate-* and tasklist-* indices.
# ==============================================================
echo ""
echo "=========================================="
echo "PART B: Component-prefixed resources (age > ${MAX_AGE_HOURS}h)"
echo "=========================================="

NOW_EPOCH=$(date +%s)
THRESHOLD_MS=$(((NOW_EPOCH - MAX_AGE_SECS) * 1000))

# The jq regex matches Convention 2 index names:
#   orch-*  opt-*  task-*  op-*
# Plus bare component names without CI prefix:
#   operate-*  tasklist-*  optimize-*
# Excludes system indices (starting with .).
CI_PREFIX_REGEX="^(orch|opt|task|op|operate|tasklist|optimize)-"

# --- B1: Component-prefixed INDICES (age-based) ---
echo ""
log "--- B1: Component-prefixed indices older than ${MAX_AGE_HOURS}h ---"

# Fetch all non-system index names matching component prefixes
CI_IDX_NAMES=$(${ES_CURL} -s "${ES_URL}/_cat/indices?h=index&format=json" 2> /dev/null \
  | jq -r --arg RE "$CI_PREFIX_REGEX" \
    '.[].index
     | select(startswith(".") | not)
     | select(test($RE))' 2> /dev/null || true)

ci_idx_deleted=0
ci_idx_skipped=0
if [ -n "$CI_IDX_NAMES" ]; then
  ci_idx_total=$(echo "$CI_IDX_NAMES" | wc -l | tr -d ' ')
  log "  Found ${ci_idx_total} component-prefixed indices"

  # Bulk-fetch creation_date for all matching indices using wildcards.
  SETTINGS_JSON=$(${ES_CURL} -s \
    "${ES_URL}/orch-*,opt-*,task-*,op-*,operate-*,tasklist-*,optimize-*/_settings?filter_path=*.settings.index.creation_date&expand_wildcards=open,closed" \
    2> /dev/null || echo "{}")

  # Extract index names that are older than the threshold
  EXPIRED_INDICES=$(echo "$SETTINGS_JSON" | jq -r --argjson TH "$THRESHOLD_MS" '
    to_entries
    | map(select(
        (.value.settings.index.creation_date | tonumber) < $TH
      ))
    | .[].key' 2> /dev/null || true)

  if [ -n "$EXPIRED_INDICES" ]; then
    expired_count=$(echo "$EXPIRED_INDICES" | wc -l | tr -d ' ')
    log "  ${expired_count} indices older than ${MAX_AGE_HOURS}h — deleting"

    # Delete in batches of 50 to avoid URL length limits
    batch=""
    batch_count=0
    while IFS= read -r idx; do
      [ -z "$idx" ] && continue
      if [ -n "$batch" ]; then
        batch="${batch},${idx}"
      else
        batch="${idx}"
      fi
      batch_count=$((batch_count + 1))
      if [ "$batch_count" -ge 50 ]; then
        resp=$(es_delete "${ES_URL}/${batch}?expand_wildcards=open,closed")
        if echo "$resp" | grep -q '"acknowledged":true'; then
          ci_idx_deleted=$((ci_idx_deleted + batch_count))
        else
          log "    WARN: batch delete: ${resp}"
        fi
        batch=""
        batch_count=0
      fi
    done <<< "$EXPIRED_INDICES"
    # Flush remaining batch
    if [ -n "$batch" ]; then
      resp=$(es_delete "${ES_URL}/${batch}?expand_wildcards=open,closed")
      if echo "$resp" | grep -q '"acknowledged":true'; then
        ci_idx_deleted=$((ci_idx_deleted + batch_count))
      else
        log "    WARN: batch delete: ${resp}"
      fi
    fi
  else
    ci_idx_skipped=$ci_idx_total
    log "  All component-prefixed indices are younger than ${MAX_AGE_HOURS}h"
  fi
else
  log "  No component-prefixed indices found"
fi
log "  Component indices: deleted=${ci_idx_deleted} kept=${ci_idx_skipped}"
total_indices_deleted=$((total_indices_deleted + ci_idx_deleted))

# --- B2: Component-prefixed V2 index templates (age-based) ---
#
# Templates don't have creation_date. Strategy: extract the unique
# group prefix (e.g. "orch-elasticsearch-xnqzizh7") from each
# template name and check whether ANY indices still exist for that
# group. If no indices remain (they were either just deleted above
# or never existed), the templates are orphaned.
#
# For groups that DO still have indices, skip them — Part B1 will
# catch those indices once they age out.
echo ""
log "--- B2: Component-prefixed V2 index templates ---"

CI_V2_FILE=$(mktemp)
${ES_CURL} -s "${ES_URL}/_index_template" 2> /dev/null \
  | jq -r --arg RE "$CI_PREFIX_REGEX" \
    '.index_templates[].name
     | select(startswith(".") | not)
     | select(test($RE))' \
    > "${CI_V2_FILE}" 2> /dev/null || true

ci_v2_deleted=0
ci_v2_skipped=0
if [ -s "${CI_V2_FILE}" ]; then
  # Extract unique group prefixes. Convention 2 template names look like:
  #   orch-elasticsearch-xnqzizh7-install-zeebe
  #   opt-keycloak-original-zzj93rze-install-optimize-index
  # The group is everything up to and including the 8-char random ID.
  # The flow suffix (-install, -upgrade-patch, etc.) always follows the 8-char ID.
  TPL_GROUPS=$(sed -E 's/-(install|upgrade-patch|upgrade-minor|modular-upgrade-minor)([-_].*)?$//' \
    < "${CI_V2_FILE}" | sort -u || true)

  if [ -n "$TPL_GROUPS" ]; then
    for group in $TPL_GROUPS; do
      tpl_count=$(grep -c "^${group}" "${CI_V2_FILE}" || true)
      # Check if any indices still exist for this group
      resp=$(${ES_CURL} -s "${ES_URL}/_cat/indices/${group}-*?h=index&format=json" 2> /dev/null || echo "[]")
      idx_remaining=$(echo "$resp" | jq -r 'if type == "array" then length else 0 end' 2> /dev/null || echo "0")
      if [ "$idx_remaining" -eq 0 ] || echo "$resp" | grep -q '"status":404' 2> /dev/null; then
        log "  DELETE '${group}': ${tpl_count} V2 templates (no indices remain)"
        resp=$(es_delete "${ES_URL}/_index_template/${group}-*")
        if echo "$resp" | grep -q '"acknowledged":true'; then
          ci_v2_deleted=$((ci_v2_deleted + tpl_count))
        else
          log "    WARN: ${resp}"
        fi
      else
        log "  KEEP  '${group}': ${tpl_count} V2 templates (${idx_remaining} indices still active)"
        ci_v2_skipped=$((ci_v2_skipped + tpl_count))
      fi
    done
  fi
fi
log "  Component V2 templates: deleted=${ci_v2_deleted} kept=${ci_v2_skipped}"
total_v2_deleted=$((total_v2_deleted + ci_v2_deleted))
rm -f "${CI_V2_FILE}"

# --- B3: Component-prefixed component templates ---
echo ""
log "--- B3: Component-prefixed component templates ---"

CI_COMP_FILE=$(mktemp)
${ES_CURL} -s "${ES_URL}/_component_template" 2> /dev/null \
  | jq -r --arg RE "$CI_PREFIX_REGEX" \
    '.component_templates[].name
     | select(startswith(".") | not)
     | select(test($RE))' \
    > "${CI_COMP_FILE}" 2> /dev/null || true

ci_comp_deleted=0
ci_comp_skipped=0
if [ -s "${CI_COMP_FILE}" ]; then
  TPL_GROUPS=$(sed -E 's/-(install|upgrade-patch|upgrade-minor|modular-upgrade-minor)([-_].*)?$//' \
    < "${CI_COMP_FILE}" | sort -u || true)

  if [ -n "$TPL_GROUPS" ]; then
    for group in $TPL_GROUPS; do
      tpl_count=$(grep -c "^${group}" "${CI_COMP_FILE}" || true)
      resp=$(${ES_CURL} -s "${ES_URL}/_cat/indices/${group}-*?h=index&format=json" 2> /dev/null || echo "[]")
      idx_remaining=$(echo "$resp" | jq -r 'if type == "array" then length else 0 end' 2> /dev/null || echo "0")
      if [ "$idx_remaining" -eq 0 ] || echo "$resp" | grep -q '"status":404' 2> /dev/null; then
        log "  DELETE '${group}': ${tpl_count} component templates (no indices remain)"
        resp=$(es_delete "${ES_URL}/_component_template/${group}-*")
        if echo "$resp" | grep -q '"acknowledged":true'; then
          ci_comp_deleted=$((ci_comp_deleted + tpl_count))
        else
          log "    WARN: ${resp}"
        fi
      else
        log "  KEEP  '${group}': ${tpl_count} component templates (${idx_remaining} indices still active)"
        ci_comp_skipped=$((ci_comp_skipped + tpl_count))
      fi
    done
  fi
fi
log "  Component comp templates: deleted=${ci_comp_deleted} kept=${ci_comp_skipped}"
total_comp_deleted=$((total_comp_deleted + ci_comp_deleted))
rm -f "${CI_COMP_FILE}"

# --- B4: Component-prefixed legacy (v1) templates ---
echo ""
log "--- B4: Component-prefixed legacy templates ---"

CI_LEGACY_FILE=$(mktemp)
${ES_CURL} -s "${ES_URL}/_template" 2> /dev/null \
  | jq -r --arg RE "$CI_PREFIX_REGEX" \
    'keys[]
     | select(startswith(".") | not)
     | select(test($RE))' \
    > "${CI_LEGACY_FILE}" 2> /dev/null || true

ci_legacy_deleted=0
ci_legacy_skipped=0
if [ -s "${CI_LEGACY_FILE}" ]; then
  TPL_GROUPS=$(sed -E 's/-(install|upgrade-patch|upgrade-minor|modular-upgrade-minor)([-_].*)?$//' \
    < "${CI_LEGACY_FILE}" | sort -u || true)

  if [ -n "$TPL_GROUPS" ]; then
    for group in $TPL_GROUPS; do
      tpl_count=$(grep -c "^${group}" "${CI_LEGACY_FILE}" || true)
      resp=$(${ES_CURL} -s "${ES_URL}/_cat/indices/${group}-*?h=index&format=json" 2> /dev/null || echo "[]")
      idx_remaining=$(echo "$resp" | jq -r 'if type == "array" then length else 0 end' 2> /dev/null || echo "0")
      if [ "$idx_remaining" -eq 0 ] || echo "$resp" | grep -q '"status":404' 2> /dev/null; then
        log "  DELETE '${group}': ${tpl_count} legacy templates (no indices remain)"
        resp=$(es_delete "${ES_URL}/_template/${group}-*")
        if echo "$resp" | grep -q '"acknowledged":true'; then
          ci_legacy_deleted=$((ci_legacy_deleted + tpl_count))
        else
          log "    WARN: ${resp}"
        fi
      else
        log "  KEEP  '${group}': ${tpl_count} legacy templates (${idx_remaining} indices still active)"
        ci_legacy_skipped=$((ci_legacy_skipped + tpl_count))
      fi
    done
  fi
fi
log "  Component legacy templates: deleted=${ci_legacy_deleted} kept=${ci_legacy_skipped}"
total_legacy_deleted=$((total_legacy_deleted + ci_legacy_deleted))
rm -f "${CI_LEGACY_FILE}"

# ==============================================================
# PART C: Garbage cleanup
#         Broken indices with literal "$" from un-interpolated
#         Helm template variables. Always safe to delete.
# ==============================================================
echo ""
echo "=========================================="
echo "PART C: Garbage indices (literal \$ in name)"
echo "=========================================="

GARBAGE_INDICES=$(${ES_CURL} -s "${ES_URL}/_cat/indices?h=index&format=json" 2> /dev/null \
  | jq -r '.[].index | select(contains("$"))' 2> /dev/null || true)

garbage_deleted=0
if [ -n "$GARBAGE_INDICES" ]; then
  garbage_count=$(echo "$GARBAGE_INDICES" | wc -l | tr -d ' ')
  log "  Found ${garbage_count} indices with literal \$ in name"
  while IFS= read -r idx; do
    [ -z "$idx" ] && continue
    encoded_idx=$(echo "$idx" | sed 's/\$/%24/g')
    resp=$(es_delete "${ES_URL}/${encoded_idx}")
    if echo "$resp" | grep -q '"acknowledged":true'; then
      log "  DELETED: ${idx}"
      garbage_deleted=$((garbage_deleted + 1))
    else
      log "  WARN: failed to delete '${idx}': ${resp}"
    fi
  done <<< "$GARBAGE_INDICES"
else
  log "  No garbage indices found"
fi
log "  Garbage indices: deleted=${garbage_deleted}"
total_indices_deleted=$((total_indices_deleted + garbage_deleted))

# ==============================================================
# PART D: Set replicas=0 on all non-system indices
#         No value in replica shards on a CI cluster.
# ==============================================================
echo ""
echo "=========================================="
echo "PART D: Replicas enforcement"
echo "=========================================="

indices_to_fix=$(${ES_CURL} -s "${ES_URL}/_cat/indices?h=index,rep&format=json" 2> /dev/null \
  | jq -r '.[]
           | select(.index | startswith(".") | not)
           | select(.rep != "0")
           | .index' 2> /dev/null || true)

if [ -n "$indices_to_fix" ]; then
  idx_count=$(echo "$indices_to_fix" | wc -l | tr -d ' ')
  log "  Found ${idx_count} non-system indices with replicas > 0"
  resp=$(es_put "${ES_URL}/*,-.*/_settings" \
    -H "Content-Type: application/json" \
    -d '{"index":{"number_of_replicas":0}}')
  if echo "$resp" | grep -q '"acknowledged":true'; then
    log "  Set replicas=0 on all non-system indices"
  else
    log "  WARN: Set replicas response: ${resp}"
  fi
else
  log "  All non-system indices already have replicas=0"
fi

# --- Cleanup temp files ---
rm -f "${NS_FILE:-}"

# --- Summary ---
echo ""
log "=== Cleanup complete ==="
log "  Indices deleted:          ${total_indices_deleted}"
log "  V2 index templates:       ${total_v2_deleted}"
log "  Component templates:      ${total_comp_deleted}"
log "  Legacy templates:         ${total_legacy_deleted}"
total_resources=$((total_indices_deleted + total_v2_deleted + total_comp_deleted + total_legacy_deleted))
log "  Total resources removed:  ${total_resources}"
if [ "${DRY_RUN}" -eq 1 ]; then
  log "  (dry-run mode — nothing was actually deleted)"
fi
