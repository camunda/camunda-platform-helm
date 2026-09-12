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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHART_DIR="${CHART_DIR:-$(cd "${SCRIPT_DIR}/../../../.." && pwd)}"
CHART_DIR="$(cd "${CHART_DIR}" && pwd)"
REPO_ROOT="$(cd "${CHART_DIR}/../.." && pwd)"

export CHART_DIR
if [[ -n "${BASE_CHART_DIR:-}" ]]; then
  export BASE_CHART_DIR="$(cd "${BASE_CHART_DIR}" && pwd)"
fi
cd "${REPO_ROOT}/scripts/zone-aware-migration"
exec go run . "$@"
