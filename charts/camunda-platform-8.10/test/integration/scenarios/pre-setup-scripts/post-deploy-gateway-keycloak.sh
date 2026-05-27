#!/bin/bash
#
# Post-deploy hook for gateway-keycloak scenario.
# Combines:
#   1. Gateway ProxySettingsPolicy application (was fixtures: gateway-proxy-settings.yaml)
#   2. Zeebe-record alias creation for Optimize (see post-deploy-zeebe-aliases.sh)
#

set -euo pipefail

: "${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# --- Part 1: Apply gateway proxy settings ---
echo "[post-deploy-gateway-keycloak] Applying gateway-proxy-settings.yaml"
RESOURCES_DIR="$(dirname "${SCRIPT_DIR}")/common/resources"
kubectl apply -f "${RESOURCES_DIR}/gateway-proxy-settings.yaml" -n "${TEST_NAMESPACE}" --server-side

# --- Part 2: Create zeebe-record aliases (reuse shared script) ---
source "${SCRIPT_DIR}/post-deploy-zeebe-aliases.sh"
