#!/bin/bash
# Copyright 2024 Camunda Services GmbH
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
# Pre-install script for the "gateway-cross-namespace" CI scenario.
# Called by deploy-camunda's PreInstallHook mechanism before helm install.
#
# Creates a shared Gateway namespace and a Gateway resource inside it so that
# Camunda Routes in the test namespace can reference a cross-namespace Gateway.
# The Gateway uses an HTTP listener (no TLS) so no cert copying is required.
#
# The Gateway name and namespace are fixed so that multiple parallel CI runs
# can share the resource — kubectl apply is idempotent and the spec is
# identical across runs.
#
# Required env vars (set by the matrix runner):
#   TEST_NAMESPACE  - target Kubernetes namespace for the Camunda chart
#   KUBE_CONTEXT    - kubectl context to target (optional)

set -euo pipefail

GATEWAY_NS="camunda-gateway-shared"
GATEWAY_NAME="shared-camunda-gateway"

CONTEXT_FLAG=""
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  CONTEXT_FLAG="--context ${KUBE_CONTEXT}"
fi

echo "[gateway-cross-ns] Creating shared gateway namespace '${GATEWAY_NS}'..."
kubectl ${CONTEXT_FLAG} create namespace "${GATEWAY_NS}" \
  --dry-run=client -o yaml | kubectl ${CONTEXT_FLAG} apply -f -

echo "[gateway-cross-ns] Applying Gateway '${GATEWAY_NAME}' in '${GATEWAY_NS}'..."
kubectl ${CONTEXT_FLAG} apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: ${GATEWAY_NAME}
  namespace: ${GATEWAY_NS}
spec:
  gatewayClassName: nginx
  listeners:
  - name: http
    port: 80
    protocol: HTTP
    allowedRoutes:
      namespaces:
        from: All
EOF

echo "[gateway-cross-ns] Done. Routes in '${TEST_NAMESPACE:-<unknown>}' will attach to '${GATEWAY_NAME}' in '${GATEWAY_NS}'."
