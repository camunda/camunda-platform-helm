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

NAMESPACE="${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"
RELEASE="${RELEASE_NAME:-integration}"
RESOLVE_USER="${SECRET_STORE_RESOLVE_USER:-demo}"
RESOLVE_PASSWORD="${SECRET_STORE_RESOLVE_PASSWORD:-demo}"

# Owned by camunda/team-distribution:
# infrastructure/gcp/camunda-distribution/gke-distro-ci/secret-store/README.md
GCP_PROJECT="camunda-distribution"
GCP_PROJECT_NUMBER="922145893973"
SECRET_ID="camunda-ci-secretstore-token"
EXPECTED_VALUE="secret-store-gcp-integration-value"
WORKLOAD_POOL="${GCP_PROJECT}.svc.id.goog"

SERVICE_ACCOUNT="${RELEASE}-zeebe"
PRINCIPAL="principal://iam.googleapis.com/projects/${GCP_PROJECT_NUMBER}/locations/global/workloadIdentityPools/${WORKLOAD_POOL}/subject/ns/${NAMESPACE}/sa/${SERVICE_ACCOUNT}"

CONTEXT_ARGS=()
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  CONTEXT_ARGS=(--context "${KUBE_CONTEXT}")
fi

for tool in gcloud jq curl; do
  command -v "${tool}" >/dev/null 2>&1 || {
    echo "${tool} is required but not on PATH." >&2
    exit 1
  }
done

# The fixture carries no standing accessor binding, so the grant below is the only
# thing that lets this run read it. Revoked on exit so a failed run leaks nothing.
# shellcheck disable=SC2329 # invoked indirectly via trap
revoke_binding() {
  gcloud secrets remove-iam-policy-binding "${SECRET_ID}" \
    --project "${GCP_PROJECT}" \
    --role roles/secretmanager.secretAccessor \
    --member "${PRINCIPAL}" \
    --quiet >/dev/null 2>&1 || \
    echo "WARNING: could not revoke ${PRINCIPAL} from ${SECRET_ID}; check the fixture IAM policy." >&2
}

echo "Granting roles/secretmanager.secretAccessor on ${SECRET_ID} to the per-run principal."
gcloud secrets add-iam-policy-binding "${SECRET_ID}" \
  --project "${GCP_PROJECT}" \
  --role roles/secretmanager.secretAccessor \
  --member "${PRINCIPAL}" \
  --quiet >/dev/null
trap revoke_binding EXIT

# The chart must reach GCP as the workload principal granted above. A ServiceAccount
# annotation or a mounted key would mean the value resolved through some other identity
# and the wiring under test was never exercised.
if kubectl "${CONTEXT_ARGS[@]}" -n "${NAMESPACE}" get serviceaccount "${SERVICE_ACCOUNT}" \
    -o jsonpath='{.metadata.annotations}' 2>/dev/null | grep -q 'iam.gke.io/gcp-service-account'; then
  echo "ServiceAccount ${SERVICE_ACCOUNT} carries iam.gke.io/gcp-service-account; this scenario must not set gcpServiceAccount." >&2
  exit 1
fi

if kubectl "${CONTEXT_ARGS[@]}" -n "${NAMESPACE}" get statefulset "${RELEASE}-zeebe" \
    -o jsonpath='{.spec.template.spec.containers[*].env[*].name}' 2>/dev/null \
    | tr ' ' '\n' | grep -qx 'GOOGLE_APPLICATION_CREDENTIALS'; then
  echo "GOOGLE_APPLICATION_CREDENTIALS is set on the Orchestration container; static credentials would take precedence over workload identity." >&2
  exit 1
fi

PORT_FORWARD_LOG="$(mktemp)"
kubectl "${CONTEXT_ARGS[@]}" -n "${NAMESPACE}" port-forward \
  "service/${RELEASE}-zeebe-gateway" ":8080" >"${PORT_FORWARD_LOG}" 2>&1 &
PORT_FORWARD_PID=$!
trap 'kill "${PORT_FORWARD_PID}" 2>/dev/null || true; rm -f "${PORT_FORWARD_LOG}"; revoke_binding' EXIT

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

# The retry budget also absorbs IAM propagation on the binding granted above.
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
    --data "{\"references\":[\"camunda.secrets.${SECRET_ID}\"]}" \
    "http://127.0.0.1:${LOCAL_PORT}/orchestration/v2/secrets/resolve" 2>/dev/null)" || true

  if jq -e --arg want "${EXPECTED_VALUE}" --arg ref "camunda.secrets.${SECRET_ID}" \
      '.resolved == [{"reference":$ref,"value":$want}] and .errors == []' \
      <<<"${response}" >/dev/null 2>&1; then
    echo "Secret store resolved ${SECRET_ID} from GCP Secret Manager as the workload principal."
    exit 0
  fi

  echo "Waiting for secret-store resolution (attempt ${attempt}/60)..."
  if (( attempt < 60 )); then
    sleep 5
  fi
done

echo "Secret store did not resolve ${SECRET_ID} to the expected value." >&2
echo "Last response: ${response:-<none>}" >&2
kubectl "${CONTEXT_ARGS[@]}" logs -n "${NAMESPACE}" "statefulset/${RELEASE}-zeebe" --all-containers --since=10m >&2 || true
exit 1
