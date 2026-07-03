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
# Generates self-signed TLS material and creates K8s secrets in the target
# namespace (opensearch-tls-certs, opensearch-tls-ca, opensearch-jks).
#
# The Identity/WebModeler PostgreSQL database is provided by the `postgresql`
# companion profile (ci-test-config.yaml), not by this script.
#
# The OpenSearch companion chart is installed separately by the matrix
# runner via the `dependencies:` block in ci-test-config.yaml — it runs
# AFTER this pre-install hook, so the TLS secrets are guaranteed to exist
# when OpenSearch starts.
#
# Required env vars:
#   TEST_NAMESPACE  - target Kubernetes namespace
#   KUBE_CONTEXT    - kubectl context (optional)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Generate certs + create K8s secrets.
bash "${SCRIPT_DIR}/create-opensearch-tls-secrets.sh"

echo "[pre-install-opensearch-self-signed] TLS secrets created."
