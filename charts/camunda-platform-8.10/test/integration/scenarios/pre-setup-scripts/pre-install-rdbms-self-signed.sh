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
# Pre-install script for the "rdbms-self-signed" persistence layer.
# Two responsibilities, in order:
#   1. Generate self-signed TLS material and create K8s secrets in the
#      target namespace (rdbms-tls-server, rdbms-tls-ca).
#   2. Install the Bitnami PostgreSQL companion chart with TLS enabled,
#      consuming the cert secrets from step 1.
#
# This is the CI equivalent of the matrix runner's `dependencies:` block
# in ci-test-config.yaml — the runner-Go path processes that block via
# `flags.CompanionCharts`, but the Taskfile-driven CI path used by
# integration test workflows does not, so we deploy the companion chart
# inline here.
#
# Required env vars (set by the matrix runner / Taskfile):
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
bash "${SCRIPT_DIR}/create-rdbms-tls-secrets.sh"

# 2. Deploy the companion PostgreSQL chart.
echo "[pre-install-rdbms-self-signed] Installing Bitnami PostgreSQL companion chart..."
helm repo add bitnami https://charts.bitnami.com/bitnami --force-update >/dev/null
helm repo update bitnami >/dev/null
helm upgrade --install postgres-tls bitnami/postgresql \
  --version 18.6.2 \
  --namespace "$NAMESPACE" \
  "${HELM_CTX_FLAG[@]}" \
  -f "${REPO_ROOT}/test/integration/companion-values/postgresql-tls.yaml" \
  --wait --timeout 5m

echo "[pre-install-rdbms-self-signed] PostgreSQL companion chart installed."
