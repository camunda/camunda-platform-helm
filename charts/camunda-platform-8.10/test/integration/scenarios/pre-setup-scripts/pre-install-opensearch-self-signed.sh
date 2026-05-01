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
# Pre-install script for the "opensearch-self-signed" persistence layer.
# Two responsibilities, in order:
#   1. Generate self-signed TLS material and create K8s secrets in the
#      target namespace (opensearch-tls-certs, opensearch-tls-ca,
#      opensearch-jks).
#   2. Install the OpenSearch companion chart with TLS enabled, consuming
#      the cert secrets from step 1.
#
# This is the CI equivalent of the matrix runner's `dependencies:` block
# in ci-test-config.yaml — the runner-Go path processes that block via
# `flags.CompanionCharts`, but the Taskfile-driven CI path used by
# integration test workflows does not, so we deploy the companion chart
# inline here.
#
# Required env vars:
#   TEST_NAMESPACE  - target Kubernetes namespace
#   KUBE_CONTEXT    - kubectl context (optional)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../../../.." && pwd)"

NAMESPACE="${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"
HELM_CTX_FLAG=()
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  HELM_CTX_FLAG=(--kube-context "$KUBE_CONTEXT")
fi

# 1. Generate certs + create K8s secrets.
bash "${SCRIPT_DIR}/create-opensearch-tls-secrets.sh"

# 2. Deploy the companion OpenSearch chart.
echo "[pre-install-opensearch-self-signed] Installing OpenSearch companion chart..."
helm repo add opensearch https://opensearch-project.github.io/helm-charts/ --force-update >/dev/null
helm repo update opensearch >/dev/null
helm upgrade --install opensearch opensearch/opensearch \
  --version 3.6.0 \
  --namespace "$NAMESPACE" \
  "${HELM_CTX_FLAG[@]}" \
  -f "${REPO_ROOT}/test/integration/companion-values/opensearch-tls.yaml" \
  --wait --timeout 10m

echo "[pre-install-opensearch-self-signed] OpenSearch companion chart installed."
