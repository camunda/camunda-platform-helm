#!/bin/bash
# Copyright 2026 Camunda Services GmbH
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

#
# List chart images.
#

helm template --skip-tests camunda "${CHART_SOURCE}" --version "${CHART_VERSION}" \
    --values "${CHART_VALUES_DIR}test/integration/scenarios/chart-full-setup/values-integration-test-ingress-keycloak.yaml" 2> /dev/null |
        tr -d "\"'" | awk '/image:/{gsub(/^(camunda|bitnami)/, "docker.io/&", $2); printf "- %s\n", $2}'
