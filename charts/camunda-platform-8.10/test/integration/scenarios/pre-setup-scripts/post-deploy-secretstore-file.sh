#!/bin/bash
# Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
# under one or more contributor license agreements. Licensed under a proprietary license.

set -euo pipefail

NAMESPACE="${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"
RELEASE="${RELEASE_NAME:-integration}"
CONTEXT_ARGS=()
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  CONTEXT_ARGS=(--context "${KUBE_CONTEXT}")
fi

LOCAL_PORT=18080
kubectl "${CONTEXT_ARGS[@]}" -n "${NAMESPACE}" port-forward \
  "service/${RELEASE}-zeebe-gateway" "${LOCAL_PORT}:8080" >/dev/null 2>&1 &
PORT_FORWARD_PID=$!
trap 'kill "${PORT_FORWARD_PID}" 2>/dev/null || true' EXIT

for _ in {1..30}; do
  if curl --silent --output /dev/null "http://127.0.0.1:${LOCAL_PORT}/orchestration/v2/secrets/resolve"; then
    break
  fi
  sleep 1
done

response=$(curl --fail-with-body --silent --show-error \
  --user demo:demo \
  --header 'Content-Type: application/json' \
  --data '{"references":["camunda.secrets.token"]}' \
  "http://127.0.0.1:${LOCAL_PORT}/orchestration/v2/secrets/resolve")

jq -e '.resolved == [{"reference":"camunda.secrets.token","value":"secret-store-integration-value"}] and .errors == []' <<<"${response}"
