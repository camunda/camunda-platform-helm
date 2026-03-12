#!/usr/bin/env bash

# Delete a single Keycloak realm via the admin REST API.
# Expects KC_URL, KC_CONTEXT_PATH, and KC_TOKEN to be set in the environment.
# Usage: delete-realm.sh <realm-name>

realm="$1"
if [[ -z "$realm" ]]; then
  echo "[error] Usage: $0 <realm-name>" >&2
  exit 1
fi

KC_URL="${KC_URL:?KC_URL must be set}"
KC_TOKEN="${KC_TOKEN:?KC_TOKEN must be set}"
KC_CONTEXT_PATH="${KC_CONTEXT_PATH:-/auth}"
KC_CONTEXT_PATH="${KC_CONTEXT_PATH%/}"

echo "Deleting realm: ${realm}"
response=$(curl -s -o /dev/null -w "%{http_code}" \
  -X DELETE \
  -H "Authorization: Bearer ${KC_TOKEN}" \
  "${KC_URL}${KC_CONTEXT_PATH}/admin/realms/${realm}")

if [[ "$response" -eq 204 || "$response" -eq 200 ]]; then
  echo "  -> Deleted successfully"
else
  echo "  -> FAILED (HTTP ${response})"
fi
