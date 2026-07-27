#!/bin/bash

set -euo pipefail

: "${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"

database_namespace="${MGMT_NAMESPACE:-${TEST_NAMESPACE}}"
expected_cluster_name="authenticated-hub-ping"
if [[ -n "${MGMT_NAMESPACE:-}" ]]; then
  expected_cluster_name="${TEST_NAMESPACE}"
fi

for attempt in {1..60}; do
  query_result="$(kubectl exec -n "${database_namespace}" deployment/postgresql -- \
    sh -c 'psql --set ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname webmodeler --tuples-only --no-align --field-separator="|" --command "SELECT c.name, p.property_value FROM clusters c JOIN cluster_discovered_properties p ON p.cluster_id = c.id WHERE c.name = '\''$1'\'' AND p.property_key = '\''verification'\''"' sh "${expected_cluster_name}")" || {
      echo "Failed to query Hub registration state in namespace ${database_namespace}." >&2
      exit 1
    }
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
