#!/bin/bash
#
# Post-deploy hook for gateway-keycloak scenario:
#   Applies the NGINX ProxySettingsPolicy for large auth headers.
#   Required because the Gateway API CRD is only registered by the chart itself,
#   so this resource can only be applied after helm install.
#
# Environment:
#   TEST_NAMESPACE — target K8s namespace (set by lifecycle hook runner)
#

set -euo pipefail

: "${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESOURCES_DIR="${SCRIPT_DIR}/../common/resources"
RELEASE_NAME="${RELEASE_NAME:-integration}"
export NAMESPACE="${TEST_NAMESPACE}"
export RELEASE_NAME

echo "[post-deploy-gateway-keycloak] Applying gateway-proxy-settings.yaml..."
# The fixture YAML uses $NAMESPACE / $RELEASE_NAME placeholders (same convention
# as the fixtures: list in ci-test-config.yaml). The lifecycle hook runner sets
# both env vars; substitute them before applying.
envsubst < "${RESOURCES_DIR}/gateway-proxy-settings.yaml" | kubectl apply -n "${TEST_NAMESPACE}" -f - --server-side --force-conflicts
echo "[post-deploy-gateway-keycloak] Done"
