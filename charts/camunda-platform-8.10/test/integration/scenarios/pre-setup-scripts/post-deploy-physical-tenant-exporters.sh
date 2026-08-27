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
HUB_NAMESPACE="${HUB_NAMESPACE:?HUB_NAMESPACE must be set}"
HUB_HOST="${HUB_HOST:?HUB_HOST must be set}"
ORCH_HOST="${ORCH_HOST:?ORCH_HOST must be set}"
OPTDEFAULT_OPTIMIZE_CONTEXT_PATH="${OPTDEFAULT_OPTIMIZE_CONTEXT_PATH:?OPTDEFAULT_OPTIMIZE_CONTEXT_PATH must be set}"
OPTTA_OPTIMIZE_CONTEXT_PATH="${OPTTA_OPTIMIZE_CONTEXT_PATH:?OPTTA_OPTIMIZE_CONTEXT_PATH must be set}"
OPTTB_OPTIMIZE_CONTEXT_PATH="${OPTTB_OPTIMIZE_CONTEXT_PATH:?OPTTB_OPTIMIZE_CONTEXT_PATH must be set}"
CONTEXT_ARGS=()
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  CONTEXT_ARGS=(--context "${KUBE_CONTEXT}")
fi

fail() {
  echo "$1" >&2
  echo "--- partitions ---" >&2
  echo "${partitions:-<none>}" >&2
  echo "--- exporter log lines ---" >&2
  grep -E 'broker\.exporter\.(elasticsearch|opensearch)|BlockingExporter' <<<"${broker_log:-}" >&2 || true
  exit 1
}

partitions_are_healthy() {
  jq -e '
    (keys | sort) == ["default", "tenanta", "tenantb"]
    and all(.default, .tenanta, .tenantb;
      type == "array"
      and length > 0
      and all(.[]; .exporterPhase == "EXPORTING" and .exportedPosition >= 0))
  ' >/dev/null 2>&1
}

jwt_payload() {
  local payload padding=""
  payload="$(cut -d. -f2 <<<"$1" | tr '_-' '/+')"
  case $((${#payload} % 4)) in
    2) padding="==" ;;
    3) padding="=" ;;
  esac
  printf '%s%s' "${payload}" "${padding}" | base64 --decode 2>/dev/null
}

token_has_expected_audience() {
  local token="$1" expected="$2"
  shift 2
  local payload forbidden
  payload="$(jwt_payload "${token}")"
  jq -e --arg expected "${expected}" '
    (.aud | if type == "array" then . else [.] end) | index($expected) != null
  ' <<<"${payload}" >/dev/null || return 1
  for forbidden in "$@"; do
    jq -e --arg forbidden "${forbidden}" '
      (.aud | if type == "array" then . else [.] end) | index($forbidden) == null
    ' <<<"${payload}" >/dev/null || return 1
  done
}

definitions_contain() {
  local definitions="$1" process_id="$2"
  jq -e --arg process_id "${process_id}" 'any(.[]; .key == $process_id or .name == $process_id)' \
    <<<"${definitions}" >/dev/null
}

tenant_api_path() {
  if [[ "$1" == "default" ]]; then
    printf '/orchestration/v2'
  else
    printf '/orchestration/physical-tenants/%s/v2' "$1"
  fi
}

secret_value() {
  kubectl "${CONTEXT_ARGS[@]}" -n "${HUB_NAMESPACE}" get secret integration-test-credentials \
    -o "jsonpath={.data.$1}" | base64 --decode
}

TEMP_DIR="$(mktemp -d)"
secure_temp_file() {
  local file
  file="$(mktemp "${TEMP_DIR}/acceptance.XXXXXX")"
  chmod 600 "${file}"
  printf '%s' "${file}"
}

curl_config() {
  local url="$1" token="${2:-}" config
  config="$(secure_temp_file)"
  {
    printf 'url = "%s"\n' "${url}"
    printf 'silent\nshow-error\nconnect-timeout = 10\nmax-time = 30\n'
    [[ -n "${token}" ]] && printf 'header = "Authorization: Bearer %s"\n' "${token}"
  } >"${config}"
  printf '%s' "${config}"
}

client_token() {
  local client_id="$1" secret="$2" config
  config="$(curl_config "https://${HUB_HOST}/auth/realms/camunda-platform/protocol/openid-connect/token")"
  {
    printf 'fail\nrequest = "POST"\n'
    printf 'data-urlencode = "grant_type=client_credentials"\n'
    printf 'data-urlencode = "client_id=%s"\n' "${client_id}"
    printf 'data-urlencode = "client_secret=%s"\n' "${secret}"
  } >>"${config}"
  curl --config "${config}" | jq -er .access_token
}

deploy_acceptance_process() {
  local tenant="$1" process_id="$2" token="$3" bpmn response config
  bpmn="$(secure_temp_file)"
  cat >"${bpmn}" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="http://camunda.io/schema/1.0/bpmn">
  <process id="${process_id}" name="${process_id}" isExecutable="true">
    <startEvent id="start"/><sequenceFlow id="flow" sourceRef="start" targetRef="end"/><endEvent id="end"/>
  </process>
</definitions>
EOF
  config="$(curl_config "https://${ORCH_HOST}$(tenant_api_path "${tenant}")/deployments" "${token}")"
  response="$(curl --config "${config}" --fail \
    -F "resources=@${bpmn};type=application/vnd.bpmn+xml" \
    )"
  jq -er '.deployments[0].processDefinition.processDefinitionKey' <<<"${response}"
}

start_acceptance_process() {
  local tenant="$1" definition_key="$2" marker="$3" token="$4" config
  config="$(curl_config "https://${ORCH_HOST}$(tenant_api_path "${tenant}")/process-instances" "${token}")"
  jq -n --arg key "${definition_key}" --arg marker "${marker}" \
    '{processDefinitionKey: $key, variables: {physicalTenantAcceptanceMarker: $marker}}' \
    | curl --config "${config}" --fail \
      -H 'Content-Type: application/json' \
      --data-binary @- >/dev/null
}

optimize_definitions() {
  local context_path="$1" token="$2" config
  config="$(curl_config "https://${HUB_HOST}${context_path}/api/definition/process/keys" "${token}")"
  curl --config "${config}" -w '\n%{http_code}'
}

wait_for_optimize_import() {
  local own="$1" context_path="$2" own_process="$3" token="$4"
  local response body status attempt
  for attempt in {1..60}; do
    response="$(optimize_definitions "${context_path}" "${token}")"
    status="${response##*$'\n'}"
    body="${response%$'\n'*}"
    if [[ "${status}" == "200" ]] && definitions_contain "${body}" "${own_process}"; then
      return 0
    fi
    sleep 5
  done
  fail "Optimize for '${own}' did not import '${own_process}' within five minutes (last HTTP ${status:-none})."
}

assert_optimize_excludes_siblings() {
  local own="$1" context_path="$2" token="$3"
  shift 3
  local response body status sibling attempt
  for attempt in {1..6}; do
    response="$(optimize_definitions "${context_path}" "${token}")"
    status="${response##*$'\n'}"
    body="${response%$'\n'*}"
    [[ "${status}" == "200" ]] || fail "Optimize for '${own}' returned HTTP ${status} during isolation check."
    for sibling in "$@"; do
      definitions_contain "${body}" "${sibling}" && fail "Optimize for '${own}' imported sibling process '${sibling}'."
    done
    sleep 5
  done
}

assert_cross_token_rejected() {
  local source="$1" target="$2" target_path="$3" token="$4" response status
  response="$(optimize_definitions "${target_path}" "${token}")"
  status="${response##*$'\n'}"
  if [[ "${status}" != "401" && "${status}" != "403" ]]; then
    fail "Optimize for '${target}' accepted '${source}' credentials (HTTP ${status})."
  fi
}

PORT_FORWARD_LOG="$(mktemp)"
kubectl "${CONTEXT_ARGS[@]}" -n "${NAMESPACE}" port-forward \
  "statefulset/${RELEASE}-zeebe" ":9600" >"${PORT_FORWARD_LOG}" 2>&1 &
PORT_FORWARD_PID=$!
trap 'kill "${PORT_FORWARD_PID}" 2>/dev/null || true; rm -f "${PORT_FORWARD_LOG}"; rm -rf "${TEMP_DIR}"' EXIT

# kubectl picks the local port when it is left empty; read it back from its first line.
LOCAL_PORT=""
for _ in {1..30}; do
  LOCAL_PORT="$(sed -nE 's/^Forwarding from 127\.0\.0\.1:([0-9]+).*/\1/p' "${PORT_FORWARD_LOG}" | head -1)"
  [[ -n "${LOCAL_PORT}" ]] && break
  sleep 1
done
if [[ -z "${LOCAL_PORT}" ]]; then
  echo "Port-forward to ${RELEASE}-zeebe management port never reported a local port." >&2
  cat "${PORT_FORWARD_LOG}" >&2
  exit 1
fi

# The management endpoints sit under the Orchestration Cluster context path when one is
# configured, so probe both rather than assuming the chart default.
ACTUATOR=""
for candidate in "/orchestration/actuator" "/actuator"; do
  if curl --silent --fail --max-time 10 "http://127.0.0.1:${LOCAL_PORT}${candidate}" >/dev/null 2>&1; then
    ACTUATOR="${candidate}"
    break
  fi
done
if [[ -z "${ACTUATOR}" ]]; then
  echo "Could not reach the broker actuator on the management port." >&2
  exit 1
fi

# /actuator/partitions is keyed by physical tenant id, so it is both the tenant inventory
# and the per-tenant exporting state. A tenant whose exporter config is missing while its
# partition state still enables the id runs a BlockingExporter: exporting pauses and the
# exported position stays at -1, which also stops log compaction for those entries.
partitions=""
for attempt in {1..60}; do
  # kubectl port-forward exits with its target pod; without this the remaining attempts curl
  # a dead local port and the diagnostics blame the tenant exporters.
  kill -0 "${PORT_FORWARD_PID}" 2>/dev/null || {
    echo "Port-forward to ${RELEASE}-zeebe died before every physical tenant reported exporting." >&2
    cat "${PORT_FORWARD_LOG}" >&2
    fail "Lost the management port while waiting for physical tenants to export."
  }

  partitions="$(curl --silent --show-error --max-time 10 \
    "http://127.0.0.1:${LOCAL_PORT}${ACTUATOR}/partitions" 2>/dev/null)" || true

  if partitions_are_healthy <<<"${partitions}"; then
    break
  fi

  if (( attempt == 60 )); then
    fail "Physical tenants did not all reach a healthy exporting state."
  fi
  echo "Waiting for every physical tenant to export (attempt ${attempt}/60)..."
  sleep 5
done

mapfile -t TENANTS < <(jq -r 'keys[]' <<<"${partitions}")
echo "Physical tenants reporting partitions: ${TENANTS[*]}"

broker_log="$(kubectl "${CONTEXT_ARGS[@]}" logs -n "${NAMESPACE}" \
  "statefulset/${RELEASE}-zeebe" --all-containers)"

if grep -q 'BlockingExporter' <<<"${broker_log}"; then
  fail "A physical tenant fell back to a BlockingExporter: an exporter id is enabled in the partition state but its configuration is missing."
fi

# Optimize reads the legacy exporter, not the Camunda exporter, so a per-tenant Optimize is
# only fed if the legacy exporter opens on that tenant's partitions.
for tenant in "${TENANTS[@]}"; do
  if ! grep -E 'broker\.exporter\.(elasticsearch|opensearch)' <<<"${broker_log}" \
      | grep -qE "physicalTenant=${tenant}[},]"; then
    fail "The legacy exporter never opened for physical tenant '${tenant}'."
  fi
done

echo "Legacy exporter opened for all ${#TENANTS[@]} physical tenants, with no blocked exporters."

# Secrets must never appear in bash -x output inherited from the lifecycle runner.
set +x
VENOM_SECRET="$(secret_value identity-admin-client-password)"
DEFAULT_SECRET="$(secret_value identity-optimize-default-client-token)"
TENANTA_SECRET="$(secret_value identity-optimize-tenanta-client-token)"
TENANTB_SECRET="$(secret_value identity-optimize-tenantb-client-token)"

VENOM_TOKEN="$(client_token venom "${VENOM_SECRET}")"
DEFAULT_TOKEN="$(client_token optimize-orcha-default "${DEFAULT_SECRET}")"
TENANTA_TOKEN="$(client_token optimize-orcha-tenanta "${TENANTA_SECRET}")"
TENANTB_TOKEN="$(client_token optimize-orcha-tenantb "${TENANTB_SECRET}")"

token_has_expected_audience "${DEFAULT_TOKEN}" optimize-orcha-default-api optimize-orcha-tenanta-api optimize-orcha-tenantb-api || fail "Default Optimize token has the wrong audience."
token_has_expected_audience "${TENANTA_TOKEN}" optimize-orcha-tenanta-api optimize-orcha-default-api optimize-orcha-tenantb-api || fail "Tenant A Optimize token has the wrong audience."
token_has_expected_audience "${TENANTB_TOKEN}" optimize-orcha-tenantb-api optimize-orcha-default-api optimize-orcha-tenanta-api || fail "Tenant B Optimize token has the wrong audience."

RUN_ID="$(date +%s)-$$"
DEFAULT_PROCESS="pt-accept-default-${RUN_ID}"
TENANTA_PROCESS="pt-accept-tenanta-${RUN_ID}"
TENANTB_PROCESS="pt-accept-tenantb-${RUN_ID}"

DEFAULT_KEY="$(deploy_acceptance_process default "${DEFAULT_PROCESS}" "${VENOM_TOKEN}")"
TENANTA_KEY="$(deploy_acceptance_process tenanta "${TENANTA_PROCESS}" "${VENOM_TOKEN}")"
TENANTB_KEY="$(deploy_acceptance_process tenantb "${TENANTB_PROCESS}" "${VENOM_TOKEN}")"
start_acceptance_process default "${DEFAULT_KEY}" "${DEFAULT_PROCESS}" "${VENOM_TOKEN}"
start_acceptance_process tenanta "${TENANTA_KEY}" "${TENANTA_PROCESS}" "${VENOM_TOKEN}"
start_acceptance_process tenantb "${TENANTB_KEY}" "${TENANTB_PROCESS}" "${VENOM_TOKEN}"

wait_for_optimize_import default "${OPTDEFAULT_OPTIMIZE_CONTEXT_PATH}" "${DEFAULT_PROCESS}" "${DEFAULT_TOKEN}"
wait_for_optimize_import tenanta "${OPTTA_OPTIMIZE_CONTEXT_PATH}" "${TENANTA_PROCESS}" "${TENANTA_TOKEN}"
wait_for_optimize_import tenantb "${OPTTB_OPTIMIZE_CONTEXT_PATH}" "${TENANTB_PROCESS}" "${TENANTB_TOKEN}"

assert_optimize_excludes_siblings default "${OPTDEFAULT_OPTIMIZE_CONTEXT_PATH}" "${DEFAULT_TOKEN}" "${TENANTA_PROCESS}" "${TENANTB_PROCESS}"
assert_optimize_excludes_siblings tenanta "${OPTTA_OPTIMIZE_CONTEXT_PATH}" "${TENANTA_TOKEN}" "${DEFAULT_PROCESS}" "${TENANTB_PROCESS}"
assert_optimize_excludes_siblings tenantb "${OPTTB_OPTIMIZE_CONTEXT_PATH}" "${TENANTB_TOKEN}" "${DEFAULT_PROCESS}" "${TENANTA_PROCESS}"

assert_cross_token_rejected default tenanta "${OPTTA_OPTIMIZE_CONTEXT_PATH}" "${DEFAULT_TOKEN}"
assert_cross_token_rejected default tenantb "${OPTTB_OPTIMIZE_CONTEXT_PATH}" "${DEFAULT_TOKEN}"
assert_cross_token_rejected tenanta default "${OPTDEFAULT_OPTIMIZE_CONTEXT_PATH}" "${TENANTA_TOKEN}"
assert_cross_token_rejected tenanta tenantb "${OPTTB_OPTIMIZE_CONTEXT_PATH}" "${TENANTA_TOKEN}"
assert_cross_token_rejected tenantb default "${OPTDEFAULT_OPTIMIZE_CONTEXT_PATH}" "${TENANTB_TOKEN}"
assert_cross_token_rejected tenantb tenanta "${OPTTA_OPTIMIZE_CONTEXT_PATH}" "${TENANTB_TOKEN}"

echo "Default, tenanta, and tenantb Optimize releases imported only their own process and rejected sibling credentials."
