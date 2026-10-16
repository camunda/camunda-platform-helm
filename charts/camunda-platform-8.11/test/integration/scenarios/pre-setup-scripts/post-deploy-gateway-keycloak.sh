#!/bin/bash
# Copyright 2026 Camunda Services GmbH
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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
