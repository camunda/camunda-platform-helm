#!/bin/bash -eu
# Expected usage is as an Helm post renderer.
# Example usage:
#   $ helm install my-release camunda/camunda-platform --post-renderer ./patch.sh
#
# This script is a Helm chart post-renderer for users on Helm 3.2.0 and greater. It allows removing default
# values set in sub-charts/dependencies, something which should be possible but is currently not working.
# See this issue for more: https://github.com/helm/helm/issues/9136
#
# By default, this script will use the kustomize binary built into the Openshift client (e.g. `oc kustomize`)
# If you wish to supersede this, specify a `KUSTOMIZE` variable.
#
# The result of patching the rendered Helm templates is printed out to STDOUT. Any other logging from the
# script is thus sent to STDERR.
#
# Note to contributors: this post-renderer is used in the integration tests, so make sure that it can be used
# from any working directory.

set -o pipefail

# Overwrite this with your own kustomize install if you want
KUSTOMIZE=${KUSTOMIZE:-""}
WORKTREE=$(mktemp -d)
if [ ! -e "${WORKTREE}" ]; then
	>&2 echo "Failed to create temporary patching directory"
	exit 1
else
  >&2 echo "Working out of ${WORKTREE}"
fi

function cleanup() {
  if [ -d "${WORKTREE}" ]; then
	  >&2 echo "Cleaning up temporary patching at ${WORKTREE}"
	  rm -rf "${WORKTREE}"
  fi
}
trap cleanup INT TERM EXIT HUP

# Read rendered manifest
cat <&0 > "${WORKTREE}/manifest.yaml"

# Copy patch definitions to working directory
cp "${BASH_SOURCE%/*}/kustomization.yaml" "${WORKTREE}/kustomization.yaml"

# Apply patches -
if [ -z "${KUSTOMIZE}" ]; then
  oc kustomize "${WORKTREE}"
else
  "${KUSTOMIZE}" "${WORKTREE}"
fi

exit 0
