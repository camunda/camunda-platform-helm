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


set -euo pipefail

: "${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"

database_namespace="${HUB_NAMESPACE:-${TEST_NAMESPACE}}"
expected_cluster_name="authenticated-hub-ping"
if [[ -n "${HUB_NAMESPACE:-}" ]]; then
  expected_cluster_name="${TEST_NAMESPACE}"
fi

for attempt in {1..60}; do
  query_result="$(kubectl exec -n "${database_namespace}" statefulset/postgresql -- \
    sh -c 'psql --set ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname webmodeler --tuples-only --no-align --field-separator="|" --command "SELECT c.name, p.property_value FROM clusters c JOIN cluster_discovered_properties p ON p.cluster_id = c.id WHERE c.name = '\''$1'\'' AND p.property_key = '\''verification'\''"' sh "${expected_cluster_name}" 2>/dev/null)" || true
  if [[ "${query_result}" == "${expected_cluster_name}|inherited-oidc-credentials" ]]; then
    echo "Authenticated Hub ping registered the orchestration cluster."
    exit 0
  fi

  echo "Waiting for authenticated Hub ping registration (attempt ${attempt}/60)..."
  if (( attempt < 60 )); then
    sleep 5
  fi
done

echo "Authenticated Hub ping did not register the orchestration cluster." >&2
kubectl logs -n "${TEST_NAMESPACE}" statefulset/integration-zeebe --all-containers --since=10m >&2 || true
kubectl logs -n "${database_namespace}" deployment/integration-web-modeler-restapi --all-containers --since=10m >&2 || true
exit 1
