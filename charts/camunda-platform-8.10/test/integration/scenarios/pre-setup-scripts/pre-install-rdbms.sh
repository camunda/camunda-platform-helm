#!/usr/bin/env bash
# Scenario-specific pre-install hook for the rdbms persistence layer.
# Discovered automatically by versionmatrix.HasPreInstallScript and run by the
# matrix runner before `helm install`. Replaces the legacy Taskfile setup.pre
# kubectl-apply step.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RESOURCES_DIR="$(cd "$SCRIPT_DIR/../common/resources" && pwd)"

: "${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"
: "${RDBMS_POSTGRESQL_USERNAME:?RDBMS_POSTGRESQL_USERNAME must be set}"
: "${RDBMS_POSTGRESQL_PASSWORD:?RDBMS_POSTGRESQL_PASSWORD must be set}"

KUBECTL_ARGS=()
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  KUBECTL_ARGS+=(--context "$KUBE_CONTEXT")
fi

export NAMESPACE="$TEST_NAMESPACE"
export RELEASE_NAME="integration"

echo "Applying postgresql-cluster.yaml to namespace $NAMESPACE"
envsubst '$NAMESPACE $RELEASE_NAME $RDBMS_POSTGRESQL_PASSWORD $RDBMS_POSTGRESQL_USERNAME' \
  < "$RESOURCES_DIR/postgresql-cluster.yaml" \
  | kubectl "${KUBECTL_ARGS[@]}" apply -f -
