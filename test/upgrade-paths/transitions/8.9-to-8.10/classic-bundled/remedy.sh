#!/usr/bin/env bash
# Copyright 2026 Camunda Services GmbH
# Licensed under the Apache License, Version 2.0.

set -euo pipefail

: "${NAMESPACE:?set NAMESPACE to the Camunda release namespace}"
: "${RDBMS_POSTGRESQL_USERNAME:?set RDBMS_POSTGRESQL_USERNAME}"
: "${RDBMS_POSTGRESQL_PASSWORD:?set RDBMS_POSTGRESQL_PASSWORD}"

export TEST_NAMESPACE="${NAMESPACE}"
export RELEASE_NAME="${RELEASE_NAME:-integration}"

repo_root="$(git rev-parse --show-toplevel)"
exec "${repo_root}/charts/camunda-platform-8.10/test/integration/scenarios/pre-setup-scripts/post-infra-bitnami-migration.sh"
