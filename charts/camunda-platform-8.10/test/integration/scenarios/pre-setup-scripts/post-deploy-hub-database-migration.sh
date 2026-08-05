#!/usr/bin/env bash
# Verifies the real 8.9-to-8.10 Hub data migration before restoring normal
# serving replicas and traffic.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
NS="${NAMESPACE:-${TEST_NAMESPACE}}"
RELEASE="${RELEASE_NAME:-integration}"

(
    cd "${repo_root}/scripts/hub-migration-integration"
    go run . \
        --namespace "${NS}" \
        --release "${RELEASE}" \
        --chart-path "${repo_root}/charts/camunda-platform-8.10" \
        verify-and-activate
)
