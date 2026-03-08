#!/bin/bash
#
# This script will run before the Camunda Helm chart upgrade step in the "upgrade-minor" flow.
# Any necessary tasks should be performed here and removed after the release.
#

set -x

# Remove StatefulSets that commonly hit immutable-spec diffs between 8.7 and 8.8.
# They are recreated by Helm with the new spec while keeping PVC data.
kubectl delete sts -n "${TEST_NAMESPACE}" -l app.kubernetes.io/name=postgresql --ignore-not-found
kubectl delete sts -n "${TEST_NAMESPACE}" -l app.kubernetes.io/name=postgresql-web-modeler --ignore-not-found
