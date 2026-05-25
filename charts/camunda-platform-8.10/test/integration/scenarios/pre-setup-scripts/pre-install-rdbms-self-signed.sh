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
# Generates self-signed TLS material and creates K8s secrets in the target
# namespace (rdbms-tls-server, rdbms-tls-ca).
#
# The Bitnami PostgreSQL companion chart is installed separately by the
# matrix runner via the `dependencies:` block in ci-test-config.yaml — it
# runs AFTER this pre-install hook, so the TLS secrets are guaranteed to
# exist when PostgreSQL starts.
#
# Required env vars (set by the matrix runner):
#   TEST_NAMESPACE  - target Kubernetes namespace
#   KUBE_CONTEXT    - kubectl context (optional)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Generate certs + create K8s secrets.
bash "${SCRIPT_DIR}/create-rdbms-tls-secrets.sh"

echo "[pre-install-rdbms-self-signed] TLS secrets created."

# Create identity + webmodeler databases in the shared distribution-postgresql
# cluster. Identity and WebModeler need external databases when no bundled
# PostgreSQL is deployed (8.10+).
RESOURCE_PATH="${SCRIPT_DIR}/../common/resources/postgres-createdb-identity-webmodeler-job.yaml"
KUBECTL_FLAGS=()
[[ -n "${KUBE_CONTEXT:-}" ]] && KUBECTL_FLAGS+=(--context="${KUBE_CONTEXT}")

envsubst < "${RESOURCE_PATH}" | kubectl "${KUBECTL_FLAGS[@]}" apply -n "${TEST_NAMESPACE}" -f -
kubectl "${KUBECTL_FLAGS[@]}" wait --for=condition=complete --timeout=120s \
  -n "${TEST_NAMESPACE}" job/psql-create-db-identity-webmodeler

echo "[pre-install-rdbms-self-signed] identity + webmodeler DBs created."
