#!/bin/bash
#
# This script will run before the Camunda Helm chart upgrade step in the "upgrade-minor" flow.
# Any necessary tasks should be performed here and removed after the release.
#

set -x

# Build kubectl context flag if KUBE_CONTEXT is set (passed by deploy-camunda).
CONTEXT_FLAG=""
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  CONTEXT_FLAG="--context ${KUBE_CONTEXT}"
fi

# Speed up the minor upgrade in CI by allowing all Zeebe broker pods to update simultaneously.
# By default StatefulSet RollingUpdate replaces one pod at a time; setting maxUnavailable to 100%
# makes all pods cycle at once. The StatefulSet and PVCs are untouched, so data migration
# is still fully tested.
kubectl ${CONTEXT_FLAG} get sts -n "${TEST_NAMESPACE}" \
  -l app.kubernetes.io/component=zeebe-broker -o name \
  | xargs -I{} kubectl ${CONTEXT_FLAG} patch {} -n "${TEST_NAMESPACE}" \
      --type=merge -p '{"spec":{"updateStrategy":{"rollingUpdate":{"maxUnavailable":"100%"}}}}'
