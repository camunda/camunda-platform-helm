#!/usr/bin/env bash
set -euo pipefail

# Find Keycloak realms that do not belong to any active CI namespace.
#
# Realm naming conventions created by CI:
#   Legacy (workflow-vars):   {6-char-hex-job-id}-realm       (e.g. 189454-realm)
#   deploy-camunda.sh:        {namespace}-{8-char-random}      (e.g. camunda-pr-123-a8x9z3k1)
#   deploy-camunda Go tool:   {scenario}-{8-char-suffix}       (e.g. keycloak-mt-a8x9z3k1)
#
# Detection strategy:
#   1. Collect active github-job-id labels from namespaces (for legacy realms).
#   2. Collect active namespace names with the github-run-id label (for newer realms).
#   3. A realm is orphaned when:
#      a. Its prefix (text before the first '-') does NOT match any active job ID, AND
#      b. No active namespace name is a prefix of the realm name.
#   4. Built-in realms ("master") are always skipped.
#
# Required environment variables:
#   KC_URL             - Keycloak base URL (e.g. https://keycloak-24-9-0.ci.distro.ultrawombat.com)
#   KC_CONTEXT_PATH    - Keycloak context path (default: /auth)
#
# Authentication (one of):
#   KC_TOKEN           - Pre-obtained admin access token
#   KC_ADMIN_USER + KC_ADMIN_PASSWORD - Credentials to obtain a token

KC_URL="${KC_URL:?KC_URL must be set (e.g. https://keycloak-24-9-0.ci.distro.ultrawombat.com)}"
KC_CONTEXT_PATH="${KC_CONTEXT_PATH:-/auth}"

# Remove trailing slash from context path for consistent URL building.
KC_CONTEXT_PATH="${KC_CONTEXT_PATH%/}"

# ----- Obtain admin access token (if not already provided) -----
if [[ -z "${KC_TOKEN:-}" ]]; then
  KC_ADMIN_USER="${KC_ADMIN_USER:?KC_ADMIN_USER must be set (or provide KC_TOKEN)}"
  KC_ADMIN_PASSWORD="${KC_ADMIN_PASSWORD:?KC_ADMIN_PASSWORD must be set (or provide KC_TOKEN)}"

  token_response=$(curl -sf -X POST \
    "${KC_URL}${KC_CONTEXT_PATH}/realms/master/protocol/openid-connect/token" \
    -d "grant_type=password&client_id=admin-cli&username=${KC_ADMIN_USER}&password=${KC_ADMIN_PASSWORD}" \
    -H "Content-Type: application/x-www-form-urlencoded")

  KC_TOKEN=$(echo "$token_response" | jq -r '.access_token')
  if [[ -z "$KC_TOKEN" || "$KC_TOKEN" == "null" ]]; then
    echo "[error] Failed to obtain Keycloak admin access token" >&2
    exit 1
  fi
fi

# ----- Collect active identifiers from Kubernetes -----
# Active job IDs (for legacy {job-id}-realm naming).
active_job_ids=$(kubectl get ns \
  -o custom-columns=:metadata.labels.github-job-id \
  -l 'github-run-id' --no-headers 2>/dev/null | sort -u)

# Active namespace names (for newer {namespace}-{random} naming).
active_namespaces=$(kubectl get ns \
  -o custom-columns=:metadata.name \
  -l 'github-run-id' --no-headers 2>/dev/null | sort -u)

# ----- Fetch all realms from Keycloak -----
all_realms=$(curl -sf \
  -H "Authorization: Bearer ${KC_TOKEN}" \
  "${KC_URL}${KC_CONTEXT_PATH}/admin/realms" | jq -r '.[].realm')

# Built-in realms that must never be deleted.
SKIP_REALMS="master"

# ----- Identify orphaned realms -----
while IFS= read -r realm; do
  [[ -z "$realm" ]] && continue

  # Skip built-in realms.
  if echo "$SKIP_REALMS" | grep -qwF "$realm"; then
    continue
  fi

  # Check 1: legacy naming — prefix before the first '-' matches an active job ID.
  prefix="${realm%%-*}"
  if [[ -n "$active_job_ids" ]] && grep -qxF "$prefix" <<< "$active_job_ids"; then
    continue
  fi

  # Check 2: newer naming — an active namespace name is a prefix of the realm.
  matched=false
  if [[ -n "$active_namespaces" ]]; then
    while IFS= read -r ns; do
      [[ -z "$ns" ]] && continue
      if [[ "$realm" == "${ns}-"* ]]; then
        matched=true
        break
      fi
    done <<< "$active_namespaces"
  fi
  if $matched; then
    continue
  fi

  echo "$realm"
done <<< "$all_realms"
