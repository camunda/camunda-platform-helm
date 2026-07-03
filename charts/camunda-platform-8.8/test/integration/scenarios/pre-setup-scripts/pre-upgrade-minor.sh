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

# No StatefulSet pre-deletions are required for the 8.7 -> 8.8 upgrade.
# The previous postgresql / postgresql-web-modeler deletions existed to
# work around immutable-spec diffs surfaced when helm upgrade ran with
# the --force flag (which uses helper.Replace -> HTTP PUT, requiring
# byte-exact immutable-field equality after server defaulting). With
# --force removed from the chart-upgrade Taskfile, helm uses strategic-
# merge-patch, which only validates the diff'd fields and does not
# require this pre-cleanup.
