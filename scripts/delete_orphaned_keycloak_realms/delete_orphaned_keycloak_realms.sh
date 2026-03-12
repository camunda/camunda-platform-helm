#!/usr/bin/env bash
set -euo pipefail

# Delete orphaned Keycloak realms that no longer belong to any active CI namespace.
#
# Required environment variables:
#   KC_URL             - Keycloak base URL (e.g. https://keycloak-24-9-0.ci.distro.ultrawombat.com)
#   KC_ADMIN_USER      - Keycloak admin username
#   KC_ADMIN_PASSWORD  - Keycloak admin password
#
# Optional:
#   KC_CONTEXT_PATH    - Keycloak context path (default: /auth)
#   DRY_RUN            - Set to "true" to list orphaned realms without deleting (default: false)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

KC_URL="${KC_URL:?KC_URL must be set (e.g. https://keycloak-24-9-0.ci.distro.ultrawombat.com)}"
KC_ADMIN_USER="${KC_ADMIN_USER:?KC_ADMIN_USER must be set}"
KC_ADMIN_PASSWORD="${KC_ADMIN_PASSWORD:?KC_ADMIN_PASSWORD must be set}"
KC_CONTEXT_PATH="${KC_CONTEXT_PATH:-/auth}"
KC_CONTEXT_PATH="${KC_CONTEXT_PATH%/}"
DRY_RUN="${DRY_RUN:-false}"

# ----- Obtain admin access token -----
echo "[keycloak] Authenticating to ${KC_URL}${KC_CONTEXT_PATH} ..."
token_response=$(curl -sf -X POST \
  "${KC_URL}${KC_CONTEXT_PATH}/realms/master/protocol/openid-connect/token" \
  -d "grant_type=password&client_id=admin-cli&username=${KC_ADMIN_USER}&password=${KC_ADMIN_PASSWORD}" \
  -H "Content-Type: application/x-www-form-urlencoded")

KC_TOKEN=$(echo "$token_response" | jq -r '.access_token')
if [[ -z "$KC_TOKEN" || "$KC_TOKEN" == "null" ]]; then
  echo "[error] Failed to obtain Keycloak admin access token" >&2
  exit 1
fi
export KC_TOKEN
echo "[keycloak] Authenticated."

# ----- Find orphaned realms -----
echo "[keycloak] Finding orphaned realms ..."
orphaned_realms=$("${SCRIPT_DIR}/find-orphaned-keycloak-realms.sh")

if [[ -z "$orphaned_realms" ]]; then
  echo "[keycloak] No orphaned realms found."
  exit 0
fi

count=$(echo "$orphaned_realms" | wc -l | tr -d ' ')
echo "[keycloak] Found ${count} orphaned realm(s):"
echo "$orphaned_realms" | sed 's/^/  - /'

if [[ "$DRY_RUN" == "true" ]]; then
  echo "[keycloak] DRY_RUN=true — skipping deletion."
  exit 0
fi

# ----- Delete orphaned realms -----
echo "[keycloak] Deleting orphaned realms ..."
parallel --will-cite -j10 "${SCRIPT_DIR}/delete-realm.sh" ::: $orphaned_realms

echo "[keycloak] Done."
