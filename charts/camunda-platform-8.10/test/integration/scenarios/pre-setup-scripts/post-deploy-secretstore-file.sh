#!/bin/bash
# Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
# under one or more contributor license agreements. Licensed under a proprietary license.

set -euo pipefail

NAMESPACE="${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"
RELEASE="${RELEASE_NAME:-integration}"
RESOLVE_USER="${SECRET_STORE_RESOLVE_USER:-demo}"
RESOLVE_PASSWORD="${SECRET_STORE_RESOLVE_PASSWORD:-demo}"
EXPECTED_VALUE="secret-store-integration-value"
CONTEXT_ARGS=()
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  CONTEXT_ARGS=(--context "${KUBE_CONTEXT}")
fi

PORT_FORWARD_LOG="$(mktemp)"
kubectl "${CONTEXT_ARGS[@]}" -n "${NAMESPACE}" port-forward \
  "service/${RELEASE}-zeebe-gateway" ":8080" >"${PORT_FORWARD_LOG}" 2>&1 &
PORT_FORWARD_PID=$!
trap 'kill "${PORT_FORWARD_PID}" 2>/dev/null || true; rm -f "${PORT_FORWARD_LOG}"' EXIT

# kubectl picks the local port when it is left empty; read it back from its first line.
LOCAL_PORT=""
for _ in {1..30}; do
  LOCAL_PORT="$(sed -nE 's/^Forwarding from 127\.0\.0\.1:([0-9]+).*/\1/p' "${PORT_FORWARD_LOG}" | head -1)"
  [[ -n "${LOCAL_PORT}" ]] && break
  sleep 1
done
if [[ -z "${LOCAL_PORT}" ]]; then
  echo "Port-forward to ${RELEASE}-zeebe-gateway never reported a local port." >&2
  cat "${PORT_FORWARD_LOG}" >&2
  exit 1
fi

for attempt in {1..60}; do
  # kubectl port-forward exits with its target pod; without this the remaining attempts
  # curl a dead local port and the diagnostics blame the secret store.
  kill -0 "${PORT_FORWARD_PID}" 2>/dev/null || {
    echo "Port-forward to ${RELEASE}-zeebe-gateway died before the secret resolved." >&2
    break
  }

  # --max-time bounds a gateway that accepts the connection and never answers; without it
  # the retry budget does not hold because curl never returns.
  response="$(curl --silent --show-error --max-time 10 \
    --user "${RESOLVE_USER}:${RESOLVE_PASSWORD}" \
    --header 'Content-Type: application/json' \
    --data '{"references":["camunda.secrets.token"]}' \
    "http://127.0.0.1:${LOCAL_PORT}/orchestration/v2/secrets/resolve" 2>/dev/null)" || true

  if jq -e --arg want "${EXPECTED_VALUE}" \
      '.resolved == [{"reference":"camunda.secrets.token","value":$want}] and .errors == []' \
      <<<"${response}" >/dev/null 2>&1; then
    echo "Secret store resolved camunda.secrets.token from the mounted Secret."
    exit 0
  fi

  echo "Waiting for secret-store resolution (attempt ${attempt}/60)..."
  if (( attempt < 60 )); then
    sleep 5
  fi
done

echo "Secret store did not resolve camunda.secrets.token to the expected value." >&2
echo "Last response: ${response:-<none>}" >&2
kubectl "${CONTEXT_ARGS[@]}" logs -n "${NAMESPACE}" "statefulset/${RELEASE}-zeebe" --all-containers --since=10m >&2 || true
exit 1
