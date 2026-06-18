#!/bin/bash
#
# Runs before the Camunda Helm chart upgrade step in the "upgrade-patch" flow (8.7).
#
# 8.7 deploys a single-broker Zeebe StatefulSet for CI. A patch upgrade bumps the
# Zeebe image, which otherwise triggers an in-place StatefulSet rolling update: the
# lone broker pod is deleted and recreated by the StatefulSet controller while
# "helm upgrade --wait" is already counting down its timeout. If the replacement
# pod lands on a different node, the ReadWriteOnce GCE persistent disk must detach
# from the old node before it can attach to the new one. Under CI node contention
# that detach/termination stalls for minutes, leaving the only broker absent long
# enough that the standalone gateway and connectors crash-loop with no broker to
# reach and the helm --wait deadline is exceeded.
#
# Deleting the broker StatefulSet here — before helm runs — moves pod termination
# and volume detach out of helm's --wait window. Helm then recreates the
# StatefulSet fresh and only waits on the (fast) attach. The PVC is retained, so
# the broker recovers its data on restart. Every other chart version already uses
# an equivalent pre-upgrade hook; 8.7 was the only upgrade flow missing one.

set -x

CONTEXT_FLAG=""
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  CONTEXT_FLAG="--context ${KUBE_CONTEXT}"
fi

# Delete only the broker StatefulSet (its PVC is retained so data survives the
# upgrade). --wait lets pod termination + volume detach complete here rather than
# inside the subsequent helm --wait window.
kubectl ${CONTEXT_FLAG} delete sts -n "${TEST_NAMESPACE}" \
  -l app.kubernetes.io/component=zeebe-broker --ignore-not-found --wait
