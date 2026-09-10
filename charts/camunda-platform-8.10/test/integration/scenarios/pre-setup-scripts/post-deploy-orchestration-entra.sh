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

release="${RELEASE_NAME:-integration}"
context_args=()
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  context_args=(--context "${KUBE_CONTEXT}")
fi
kubectl_args=("${context_args[@]}" -n "${TEST_NAMESPACE}")
curl_args=(--connect-timeout 5 --max-time 20 --fail --silent --show-error)

for attempt in {1..24}; do
  ingresses="$(kubectl "${kubectl_args[@]}" get ingress -l "app.kubernetes.io/instance=${release}" -o json)"
  camunda_hostname="$(jq -r 'first(.items[].spec.rules[].host | select(startswith("grpc-") | not)) // empty' <<<"${ingresses}")"
  ingress_ip="$(jq -r --arg host "${camunda_hostname}" 'first(.items[] | select(any(.spec.rules[]?; .host == $host)) | .status.loadBalancer.ingress[0].ip) // empty' <<<"${ingresses}")"
  if [[ -n "${camunda_hostname}" && -n "${ingress_ip}" ]]; then
    break
  fi
  if (( attempt == 24 )); then
    echo "The Camunda ingress did not receive a load balancer address." >&2
    exit 1
  fi
  sleep 5
done
curl_resolve=(--resolve "${camunda_hostname}:443:${ingress_ip}")

kubectl "${kubectl_args[@]}" rollout status "statefulset/${release}-zeebe" --timeout=2m
kubectl "${kubectl_args[@]}" rollout status deployment -l app.kubernetes.io/component=connectors --timeout=2m
kubectl "${kubectl_args[@]}" rollout status deployment -l app.kubernetes.io/component=optimize --timeout=2m

identity_resources="$(kubectl "${kubectl_args[@]}" get deployment,service -l app.kubernetes.io/component=identity -o json)"
if jq -e '.items | length > 0' <<<"${identity_resources}" >/dev/null; then
  echo "Management Identity resources must not exist." >&2
  exit 1
fi

shared_identity_configmap="${release}-camunda-platform-identity-env-vars"
for configmap in "${shared_identity_configmap}" "${release}-camunda-platform-optimize-identity-env-vars"; do
  configmap_json="$(kubectl "${kubectl_args[@]}" get configmap "${configmap}" -o json)"
  if jq -e '.data | has("CAMUNDA_IDENTITY_BASEURL")' <<<"${configmap_json}" >/dev/null; then
    echo "Management Identity URL must not be configured in ${configmap}." >&2
    exit 1
  fi
done

token_url="$(kubectl "${kubectl_args[@]}" get configmap "${shared_identity_configmap}" -o jsonpath='{.metadata.annotations.keycloak-token-url}')"
test -n "${token_url}"
set +x
client_id="$(kubectl "${kubectl_args[@]}" get secret venom-entra-credentials -o jsonpath='{.data.client-id}' | base64 -d)"
client_secret="$(kubectl "${kubectl_args[@]}" get secret venom-entra-credentials -o jsonpath='{.data.client-secret}' | base64 -d)"
audience="$(kubectl "${kubectl_args[@]}" get secret venom-entra-credentials -o jsonpath='{.data.audience}' | base64 -d)"
token="$(curl "${curl_args[@]}" -X POST "${token_url}" \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode "client_id=${client_id}" \
  --data-urlencode "client_secret=${client_secret}" \
  --data-urlencode "scope=${audience}/.default" \
  --data-urlencode 'grant_type=client_credentials' | jq -er '.access_token | strings | select(length > 0)')"

test -n "${token}"
orchestration_url="https://${camunda_hostname}/orchestration/v2"
optimize_url="https://${camunda_hostname}/optimize/api"
bpmn_file="test/integration/testsuites/core/files/test-inbound-process.bpmn"

topology="$(curl "${curl_resolve[@]}" "${curl_args[@]}" -H "Authorization: Bearer ${token}" \
  --retry 24 --retry-all-errors --retry-delay 5 \
  "${orchestration_url}/topology")"
jq -e '.brokers | length > 0' <<<"${topology}" >/dev/null
curl "${curl_resolve[@]}" "${curl_args[@]}" -H "Authorization: Bearer ${token}" \
  --retry 24 --retry-all-errors --retry-delay 5 \
  "${optimize_url}/dashboard/management" >/dev/null

for attempt in {1..24}; do
  if curl "${curl_resolve[@]}" "${curl_args[@]}" -X POST \
    -H "Authorization: Bearer ${token}" \
    -H 'Accept: application/json' \
    -F "resources=@${bpmn_file}" \
    "${orchestration_url}/deployments" >/dev/null; then
    break
  fi
  if (( attempt == 24 )); then
    echo "The Entra test client did not receive BPMN deployment permission." >&2
    exit 1
  fi
  sleep 5
done

for attempt in {1..24}; do
  if curl "${curl_resolve[@]}" "${curl_args[@]}" -X POST \
    -H "Authorization: Bearer ${token}" \
    -H 'Content-Type: application/json' \
    --data '{"webhookDataKey":"webhookDataValue"}' \
    "https://${camunda_hostname}/connectors/inbound/test-mywebhook" >/dev/null; then
    break
  fi
  if (( attempt == 24 )); then
    echo "Connectors did not activate the inbound webhook." >&2
    exit 1
  fi
  sleep 5
done

for attempt in {1..24}; do
  if process_instances="$(curl "${curl_resolve[@]}" "${curl_args[@]}" -X POST \
      -H "Authorization: Bearer ${token}" \
      -H 'Content-Type: application/json' \
      --data '{"filter":{"processDefinitionId":"test-inbound-process","state":"COMPLETED"}}' \
      "${orchestration_url}/process-instances/search")" && \
      jq -e '.items | length > 0' <<<"${process_instances}" >/dev/null; then
    break
  fi
  if (( attempt == 24 )); then
    echo "The inbound connector did not complete a process instance." >&2
    exit 1
  fi
  sleep 5
done

for attempt in {1..60}; do
  if definitions="$(curl "${curl_resolve[@]}" "${curl_args[@]}" \
      -H "Authorization: Bearer ${token}" \
      "${optimize_url}/definition/process")" && \
      definition="$(jq -ce '[.[] | select(.key == "test-inbound-process")] | max_by(.version | tonumber)' <<<"${definitions}")"; then
    break
  fi
  if (( attempt == 60 )); then
    echo "Optimize did not import the connector process definition." >&2
    exit 1
  fi
  sleep 10
done

report_definition="$(jq -cn \
  --arg key "$(jq -r '.key' <<<"${definition}")" \
  --arg version "$(jq -r '.version' <<<"${definition}")" \
  --arg tenant "$(jq -r '.tenantId' <<<"${definition}")" \
  '{data:{definitions:[{key:$key,versions:[$version],tenantIds:[$tenant]}],view:{entity:"processInstance",properties:["frequency"]},groupBy:{type:"none",value:null},distributedBy:{type:"none",value:null},visualization:"number"}}')"
report_id="$(curl "${curl_resolve[@]}" "${curl_args[@]}" -X POST \
  -H "Authorization: Bearer ${token}" \
  -H 'Content-Type: application/json' \
  --data "${report_definition}" \
  "${optimize_url}/report/process/single" | jq -er '.id | strings | select(length > 0)')"
test -n "${report_id}"

for attempt in {1..60}; do
  if report="$(curl "${curl_resolve[@]}" "${curl_args[@]}" \
      -H "Authorization: Bearer ${token}" \
      "${optimize_url}/public/export/report/${report_id}/result/json")" && \
      jq -e '.data > 0' <<<"${report}" >/dev/null; then
    break
  fi
  if (( attempt == 60 )); then
    echo "The Optimize report did not include the connector process instance." >&2
    exit 1
  fi
  sleep 10
done

echo "Orchestration, Connectors, and an Optimize report worked with Entra and no Management Identity."
