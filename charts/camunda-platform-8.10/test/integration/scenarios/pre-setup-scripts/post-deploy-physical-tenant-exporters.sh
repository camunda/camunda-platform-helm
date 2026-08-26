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

PORT_FORWARD_LOG="$(mktemp)"
kubectl "${CONTEXT_ARGS[@]}" -n "${NAMESPACE}" port-forward \
  "statefulset/${RELEASE}-zeebe" ":9600" >"${PORT_FORWARD_LOG}" 2>&1 &
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
