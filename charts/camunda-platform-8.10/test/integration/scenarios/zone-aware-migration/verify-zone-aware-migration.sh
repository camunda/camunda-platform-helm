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
# Verifies the two runtime guarantees of the zone-aware migration, which template rendering
# cannot prove: entering the migration must not recreate the retained numbered brokers, and
# the two generations must be schedulable together.
#
# Both are checked against a live cluster. Works on kind or GKE; it only needs a namespace
# and one schedulable node. The single-node case is the point of the anti-affinity check:
# both generations carry the same component label, so a hostname anti-affinity that is not
# scoped per generation leaves the zoned pod Pending forever.
#
# Usage: CHART_DIR=charts/camunda-platform-8.10 ./verify-zone-aware-migration.sh [namespace]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHART_DIR="${CHART_DIR:-$(cd "${SCRIPT_DIR}/../../../.." && pwd)}"
NAMESPACE="${1:-zone-aware-migration}"
RELEASE="${RELEASE:-zam}"
TIMEOUT="${TIMEOUT:-5m}"

CREATED_NAMESPACE=false
CREATED_RELEASE=false

cleanup() {
    [ "${CREATED_RELEASE}" = true ] \
        && helm uninstall "${RELEASE}" --namespace "${NAMESPACE}" >/dev/null 2>&1
    [ "${CREATED_NAMESPACE}" = true ] \
        && kubectl delete namespace "${NAMESPACE}" --wait=false >/dev/null 2>&1
    return 0
}
trap cleanup EXIT

fail() {
    echo "FAIL: $*" >&2
    kubectl get pods --namespace "${NAMESPACE}" -o wide >&2 || true
    exit 1
}

pod_uid() {
    kubectl get pod "$1" --namespace "${NAMESPACE}" -o jsonpath='{.metadata.uid}' 2>/dev/null || true
}

kubectl get namespace "${NAMESPACE}" >/dev/null 2>&1 \
    && fail "namespace ${NAMESPACE} already exists; refusing to reuse and delete it"
helm status "${RELEASE}" --namespace "${NAMESPACE}" >/dev/null 2>&1 \
    && fail "release ${RELEASE} already exists in ${NAMESPACE}"

kubectl create namespace "${NAMESPACE}" >/dev/null
CREATED_NAMESPACE=true

echo "==> Installing the pre-migration numbered cluster"
CREATED_RELEASE=true
helm install "${RELEASE}" "${CHART_DIR}" \
    --namespace "${NAMESPACE}" \
    --values "${SCRIPT_DIR}/values-numbered.yaml" \
    --timeout "${TIMEOUT}" >/dev/null

kubectl rollout status "statefulset/${RELEASE}-zeebe" --namespace "${NAMESPACE}" --timeout "${TIMEOUT}"

NUMBERED_POD="${RELEASE}-zeebe-0"
UID_BEFORE="$(pod_uid "${NUMBERED_POD}")"
[ -n "${UID_BEFORE}" ] || fail "${NUMBERED_POD} did not come up before the migration"
REPLICAS_BEFORE="$(kubectl get "statefulset/${RELEASE}-zeebe" --namespace "${NAMESPACE}" -o jsonpath='{.spec.replicas}')"
echo "    ${NUMBERED_POD} uid=${UID_BEFORE} replicas=${REPLICAS_BEFORE}"

echo "==> Entering the migration (keepUnzonedBrokers=true)"
helm upgrade "${RELEASE}" "${CHART_DIR}" \
    --namespace "${NAMESPACE}" \
    --values "${SCRIPT_DIR}/values-numbered.yaml" \
    --values "${SCRIPT_DIR}/values-migration.yaml" \
    --timeout "${TIMEOUT}" >/dev/null

kubectl rollout status "statefulset/${RELEASE}-zeebe-zone-a" --namespace "${NAMESPACE}" --timeout "${TIMEOUT}"
# Settle the retained StatefulSet too: reading its pod UID before its controller has
# reconciled would pass even if this upgrade had changed the pod template.
kubectl rollout status "statefulset/${RELEASE}-zeebe" --namespace "${NAMESPACE}" --timeout "${TIMEOUT}"

# The retained brokers must survive the upgrade untouched. A different UID means the pod was
# recreated, which loses the Raft state the migration exists to preserve.
UID_AFTER="$(pod_uid "${NUMBERED_POD}")"
[ "${UID_AFTER}" = "${UID_BEFORE}" ] \
    || fail "retained ${NUMBERED_POD} was recreated: ${UID_BEFORE} -> ${UID_AFTER:-gone}"

RESTARTS="$(kubectl get pod "${NUMBERED_POD}" --namespace "${NAMESPACE}" -o jsonpath='{.status.containerStatuses[0].restartCount}')"
[ "${RESTARTS}" = "0" ] || fail "retained ${NUMBERED_POD} restarted ${RESTARTS} time(s)"

REPLICAS_AFTER="$(kubectl get "statefulset/${RELEASE}-zeebe" --namespace "${NAMESPACE}" -o jsonpath='{.spec.replicas}')"
[ "${REPLICAS_AFTER}" = "${REPLICAS_BEFORE}" ] \
    || fail "retained StatefulSet resized: ${REPLICAS_BEFORE} -> ${REPLICAS_AFTER}"

# Both generations have to be placeable. On a single-node cluster this only holds because the
# anti-affinity is scoped per generation.
ZONED_POD="${RELEASE}-zeebe-zone-a-0"
ZONED_PHASE="$(kubectl get pod "${ZONED_POD}" --namespace "${NAMESPACE}" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
[ "${ZONED_PHASE}" = "Running" ] || fail "${ZONED_POD} is ${ZONED_PHASE:-absent}, expected Running"

echo "==> Leaving the migration (keepUnzonedBrokers=false)"
ZONED_UID_BEFORE="$(pod_uid "${ZONED_POD}")"
helm upgrade "${RELEASE}" "${CHART_DIR}" \
    --namespace "${NAMESPACE}" \
    --values "${SCRIPT_DIR}/values-numbered.yaml" \
    --values "${SCRIPT_DIR}/values-migration.yaml" \
    --set orchestration.multiregion.keepUnzonedBrokers=false \
    --set orchestration.multiregion.regions=1 \
    --set-string orchestration.clusterSize=1 \
    --set-string orchestration.replicationFactor=1 \
    --timeout "${TIMEOUT}" >/dev/null

kubectl wait --for=delete "statefulset/${RELEASE}-zeebe" --namespace "${NAMESPACE}" --timeout "${TIMEOUT}" 2>/dev/null || true
kubectl rollout status "statefulset/${RELEASE}-zeebe-zone-a" --namespace "${NAMESPACE}" --timeout "${TIMEOUT}"
kubectl get "statefulset/${RELEASE}-zeebe" --namespace "${NAMESPACE}" >/dev/null 2>&1 \
    && fail "the numbered StatefulSet survived disabling keepUnzonedBrokers"

# Disabling retention must not disturb the zoned brokers, which by then hold the partitions.
ZONED_UID_AFTER="$(pod_uid "${ZONED_POD}")"
[ "${ZONED_UID_AFTER}" = "${ZONED_UID_BEFORE}" ] \
    || fail "zoned ${ZONED_POD} was recreated when retention was disabled"

# The PVCs of the removed generation are deliberately left behind for the operator.
kubectl get "pvc/data-${NUMBERED_POD}" --namespace "${NAMESPACE}" >/dev/null 2>&1 \
    || fail "the retained generation's PVC was deleted; it must survive for manual cleanup"

echo "PASS: retained brokers preserved, both generations co-scheduled, zoned brokers undisturbed on cleanup"
