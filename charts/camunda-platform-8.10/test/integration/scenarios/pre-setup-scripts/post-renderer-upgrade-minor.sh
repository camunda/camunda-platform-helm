#!/bin/bash
#
# Helm post-renderer for the upgrade-minor flow.
# Injects maxUnavailable=100% into the Zeebe broker StatefulSet so all pods
# are replaced simultaneously during CI upgrades instead of one at a time.
# The StatefulSet and PVCs are untouched — data migration is still fully tested.
#

yq e 'if .kind == "StatefulSet" and .metadata.labels["app.kubernetes.io/component"] == "zeebe-broker" then .spec.updateStrategy.rollingUpdate.maxUnavailable = "100%" else . end' -
