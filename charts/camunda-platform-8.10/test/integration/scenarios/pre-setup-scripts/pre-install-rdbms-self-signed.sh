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
# Provisions a CloudNativePG `Cluster` in the scenario namespace with the
# `orchestration` database for RDBMS secondary storage plus `identity` and
# `webmodeler` databases for the Identity + WebModeler external DBs. CNPG
# generates a self-signed CA + server cert by default; the CA is exposed via
# the `postgresql-cluster-ca` secret and consumed by Camunda for
# `sslmode=verify-full` validation.
#
# Required env vars (set by the matrix runner):
#   TEST_NAMESPACE             - target Kubernetes namespace
#   KUBE_CONTEXT               - kubectl context (optional)
#   RDBMS_POSTGRESQL_USERNAME  - app-role username (passed to CNPG)
#   RDBMS_POSTGRESQL_PASSWORD  - app-role password (passed to CNPG)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESOURCE_PATH="${SCRIPT_DIR}/../common/resources/postgresql-cluster-tls.yaml"

KUBECTL_FLAGS=()
[[ -n "${KUBE_CONTEXT:-}" ]] && KUBECTL_FLAGS+=(--context="${KUBE_CONTEXT}")

NAMESPACE="${TEST_NAMESPACE}" envsubst < "${RESOURCE_PATH}" | \
  kubectl "${KUBECTL_FLAGS[@]}" apply -n "${TEST_NAMESPACE}" -f -
kubectl "${KUBECTL_FLAGS[@]}" wait --for=condition=Ready --timeout=300s \
  -n "${TEST_NAMESPACE}" cluster.postgresql.cnpg.io/postgresql-cluster

echo "[pre-install-rdbms-self-signed] CNPG TLS cluster ready."
