#!/bin/bash
# Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
# under one or more contributor license agreements. Licensed under a proprietary license.

set -euo pipefail

NAMESPACE="${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"
CONTEXT_ARGS=()
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  CONTEXT_ARGS=(--context "${KUBE_CONTEXT}")
fi

kubectl "${CONTEXT_ARGS[@]}" -n "${NAMESPACE}" create secret generic camunda-secret-store-test \
  --from-literal=token=secret-store-integration-value \
  --dry-run=client -o yaml | kubectl "${CONTEXT_ARGS[@]}" -n "${NAMESPACE}" apply -f -
