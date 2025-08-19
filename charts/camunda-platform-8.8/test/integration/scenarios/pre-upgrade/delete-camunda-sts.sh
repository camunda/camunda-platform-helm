# Delete the StatefulSet of Camunda Orchestration cluster as the "matchLabels" changed
# after the rename from "core" to "orchestration".
kubectl delete sts -n "${TEST_NAMESPACE}" -l app.kubernetes.io/component=core --ignore-not-found
# Adding "orchestration" also, as this will change again before the final release, to avoid the need to delete the StatefulSet.
# It will be again "zeebe-broker" as the 8.7 release.
kubectl delete sts -n "${TEST_NAMESPACE}" -l app.kubernetes.io/component=orchestration --ignore-not-found
