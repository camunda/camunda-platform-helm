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

# The lifecycle runner invokes hooks under `bash -x`, which traces the expansion below
# into the job log. Drop xtrace across the credential read and restore the caller's state.
xtrace_was_on=0
case $- in *x*) xtrace_was_on=1 ;; esac
{ set +x; } 2>/dev/null
RESOLVE_PASSWORD="${SECRET_STORE_RESOLVE_PASSWORD:-demo}"
if (( xtrace_was_on )); then set -x; fi

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
# thing that lets this run read it. The expiry condition bounds the binding on the
# kill paths that never reach the trap, and must be byte-identical on add and remove
# or gcloud matches no binding.
GRANT_EXPIRY="$(date -u -d '+2 hours' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null \
  || date -u -v+2H +%Y-%m-%dT%H:%M:%SZ)"
GRANT_CONDITION="expression=request.time < timestamp(\"${GRANT_EXPIRY}\"),title=ci-secretstore-run"

# shellcheck disable=SC2329 # invoked indirectly via trap
revoke_binding() {
  local rc="${1:-0}"
  local policy
  gcloud secrets remove-iam-policy-binding "${SECRET_ID}" \
    --project "${GCP_PROJECT}" \
    --role roles/secretmanager.secretAccessor \
    --member "${PRINCIPAL}" \
    --condition "${GRANT_CONDITION}" \
    --quiet >/dev/null 2>&1 || true
  # The post-condition, not the remove exit code, decides: remove also fails when the
  # grant never landed. An unreadable policy fails closed rather than assuming success.
  if ! policy="$(gcloud secrets get-iam-policy "${SECRET_ID}" \
      --project "${GCP_PROJECT}" \
      --flatten 'bindings[].members' \
      --format 'value(bindings.members)' 2>/dev/null)"; then
    echo "ERROR: could not read the IAM policy of ${SECRET_ID} to confirm revoke; any binding created by this run expires at ${GRANT_EXPIRY}." >&2
    rc=1
  elif grep -qF "${PRINCIPAL}" <<<"${policy}"; then
    echo "ERROR: ${PRINCIPAL} still holds a binding on ${SECRET_ID} after revoke; it expires at ${GRANT_EXPIRY}." >&2
    rc=1
  fi
  exit "${rc}"
}

# Installed before the grant so a signal arriving mid-gcloud still revokes. INT and
# TERM exit instead of resuming, which is what lets the EXIT trap run on cancellation.
trap 'revoke_binding $?' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

echo "Granting roles/secretmanager.secretAccessor on ${SECRET_ID} to the per-run principal until ${GRANT_EXPIRY}."
gcloud secrets add-iam-policy-binding "${SECRET_ID}" \
  --project "${GCP_PROJECT}" \
  --role roles/secretmanager.secretAccessor \
  --member "${PRINCIPAL}" \
  --condition "${GRANT_CONDITION}" \
  --quiet >/dev/null

# The chart must reach GCP as the workload principal granted above. A ServiceAccount
# annotation or a mounted key would mean the value resolved through some other identity
# and the wiring under test was never exercised. Both reads fail closed: a discarded
# kubectl error would otherwise look exactly like an absent key.
if ! SA_ANNOTATIONS="$(kubectl "${CONTEXT_ARGS[@]}" -n "${NAMESPACE}" get serviceaccount "${SERVICE_ACCOUNT}" \
    -o jsonpath='{.metadata.annotations}')"; then
  echo "Could not read ServiceAccount ${SERVICE_ACCOUNT}; the workload-identity guard cannot be evaluated." >&2
  exit 1
fi
if grep -q 'iam.gke.io/gcp-service-account' <<<"${SA_ANNOTATIONS}"; then
  echo "ServiceAccount ${SERVICE_ACCOUNT} carries iam.gke.io/gcp-service-account; this scenario must not set gcpServiceAccount." >&2
  exit 1
fi

if ! CONTAINER_ENV_NAMES="$(kubectl "${CONTEXT_ARGS[@]}" -n "${NAMESPACE}" get statefulset "${RELEASE}-zeebe" \
    -o jsonpath='{.spec.template.spec.containers[*].env[*].name}')"; then
  echo "Could not read StatefulSet ${RELEASE}-zeebe; the ambient-credential guard cannot be evaluated." >&2
  exit 1
fi
if tr ' ' '\n' <<<"${CONTAINER_ENV_NAMES}" | grep -qx 'GOOGLE_APPLICATION_CREDENTIALS'; then
  echo "GOOGLE_APPLICATION_CREDENTIALS is set on the Orchestration container; static credentials would take precedence over workload identity." >&2
  exit 1
fi

PORT_FORWARD_LOG="$(mktemp)"
kubectl "${CONTEXT_ARGS[@]}" -n "${NAMESPACE}" port-forward \
  "service/${RELEASE}-zeebe-gateway" ":8080" >"${PORT_FORWARD_LOG}" 2>&1 &
PORT_FORWARD_PID=$!
trap 'rc=$?; kill "${PORT_FORWARD_PID}" 2>/dev/null || true; rm -f "${PORT_FORWARD_LOG}"; revoke_binding "${rc}"' EXIT

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
  # the retry budget does not hold because curl never returns. xtrace is dropped across the
  # whole exchange so `bash -x` traces neither --user nor the resolved payload.
  { set +x; } 2>/dev/null
  response="$(curl --silent --show-error --max-time 10 \
    --user "${RESOLVE_USER}:${RESOLVE_PASSWORD}" \
    --header 'Content-Type: application/json' \
    --data "{\"references\":[\"camunda.secrets.${SECRET_ID}\"]}" \
    "http://127.0.0.1:${LOCAL_PORT}/orchestration/v2/secrets/resolve" 2>/dev/null)" || true

  if jq -e --arg want "${EXPECTED_VALUE}" --arg ref "camunda.secrets.${SECRET_ID}" \
      '.resolved == [{"reference":$ref,"value":$want}] and .errors == []' \
      <<<"${response}" >/dev/null 2>&1; then
    if (( xtrace_was_on )); then set -x; fi
    echo "Secret store resolved ${SECRET_ID} from GCP Secret Manager as the workload principal."
    exit 0
  fi

  # The payload carries resolved secret values, so only reference names and errors survive
  # into the diagnostics.
  response_summary="$(jq -c '{references: [.resolved[]?.reference], errors: .errors}' \
    <<<"${response}" 2>/dev/null)" || response_summary='<unparseable response>'
  if (( xtrace_was_on )); then set -x; fi

  echo "Waiting for secret-store resolution (attempt ${attempt}/60)..."
  if (( attempt < 60 )); then
    sleep 5
  fi
done

echo "Secret store did not resolve ${SECRET_ID} to the expected value." >&2
echo "Last response (values redacted): ${response_summary:-<none>}" >&2
kubectl "${CONTEXT_ARGS[@]}" logs -n "${NAMESPACE}" "statefulset/${RELEASE}-zeebe" --all-containers --since=10m >&2 || true
exit 1
