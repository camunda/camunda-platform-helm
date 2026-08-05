#!/usr/bin/env bash
# Seeds representative 8.9 Hub data, then includes the Web Modeler database in
# the existing Bitnami-to-companion infrastructure migration.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
NS="${NAMESPACE:-${TEST_NAMESPACE}}"
RELEASE="${RELEASE_NAME:-integration}"

(
    cd "${repo_root}/scripts/hub-migration-integration"
    go run . --namespace "${NS}" --release "${RELEASE}" seed
)

export MIGRATE_WEBMODELER=true
export WEBMODELER_SOURCE_DB_NAME=web-modeler
export WEBMODELER_SOURCE_DB_USER=web-modeler
export WEBMODELER_DB_NAME=webmodeler
export WEBMODELER_DB_USER="${RDBMS_POSTGRESQL_USERNAME}"
export EXTERNAL_PG_WEBMODELER_HOST=postgresql
export EXTERNAL_PG_WEBMODELER_SECRET=external-pg-webmodeler

# shellcheck disable=SC1091
source "${repo_root}/charts/camunda-platform-8.10/test/integration/scenarios/pre-setup-scripts/post-infra-bitnami-migration.sh"
