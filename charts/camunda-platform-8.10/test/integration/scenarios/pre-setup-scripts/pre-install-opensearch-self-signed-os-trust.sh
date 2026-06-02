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
# Pre-install script for the "opensearch-self-signed-os-trust" persistence
# layer. Validates that global.tls.caBundle (SSL_CERT_FILE / NODE_EXTRA_CA_CERTS)
# carries OS-level trust to a TLS-protected OpenSearch backend with a
# self-signed CA. The legacy JKS truststore path is also configured as a
# belt-and-braces fallback; the B2 PR (#6040) will remove the JKS dependency
# once the truststore-init container is in place.
#
# Applies the PostgreSQL fixture needed by Identity/WebModeler (the bundled
# PostgreSQL subchart was removed in 8.10), then generates self-signed TLS
# material and creates K8s secrets in the target namespace. Reuses
# create-opensearch-tls-secrets.sh.
#
# The OpenSearch companion chart is installed separately by the matrix
# runner via the `dependencies:` block in ci-test-config.yaml — it runs
# AFTER this pre-install hook, so the TLS secrets are guaranteed to exist
# when OpenSearch starts.
#
# Required env vars:
#   TEST_NAMESPACE  - target Kubernetes namespace
#   KUBE_CONTEXT    - kubectl context (optional)
#   RDBMS_POSTGRESQL_USERNAME / RDBMS_POSTGRESQL_PASSWORD

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESOURCE_DIR="$(cd "${SCRIPT_DIR}/../common/resources" && pwd)"
KUBECTL_FLAGS=()

if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  KUBECTL_FLAGS+=(--context="${KUBE_CONTEXT}")
fi

echo "[pre-install-opensearch-self-signed-os-trust] Applying PostgreSQL fixture..."
NAMESPACE="${TEST_NAMESPACE}" envsubst < "${RESOURCE_DIR}/postgresql-cluster.yaml" | \
  kubectl "${KUBECTL_FLAGS[@]}" apply --server-side --force-conflicts -f -
kubectl "${KUBECTL_FLAGS[@]}" wait --for=condition=Ready --timeout=300s \
  -n "${TEST_NAMESPACE}" cluster.postgresql.cnpg.io/postgresql-cluster
echo "[pre-install-opensearch-self-signed-os-trust] PostgreSQL fixture ready."

# Generate certs + create K8s secrets (same as opensearch-self-signed).
bash "${SCRIPT_DIR}/create-opensearch-tls-secrets.sh"

echo "[pre-install-opensearch-self-signed-os-trust] TLS secrets created."
