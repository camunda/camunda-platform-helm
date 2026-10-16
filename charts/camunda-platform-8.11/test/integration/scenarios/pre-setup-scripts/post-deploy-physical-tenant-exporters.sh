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

args=(acceptance physical-tenant-exporters \
  --namespace "${TEST_NAMESPACE:?TEST_NAMESPACE must be set}" \
  --release "${RELEASE_NAME:-integration}" \
  --hub-namespace "${HUB_NAMESPACE:?HUB_NAMESPACE must be set}" \
  --hub-host "${HUB_HOST:?HUB_HOST must be set}" \
  --orchestration-host "${ORCH_HOST:?ORCH_HOST must be set}" \
  --default-optimize-path "${OPTDEFAULT_OPTIMIZE_CONTEXT_PATH:?OPTDEFAULT_OPTIMIZE_CONTEXT_PATH must be set}" \
  --tenanta-optimize-path "${OPTTA_OPTIMIZE_CONTEXT_PATH:?OPTTA_OPTIMIZE_CONTEXT_PATH must be set}" \
  --tenantb-optimize-path "${OPTTB_OPTIMIZE_CONTEXT_PATH:?OPTTB_OPTIMIZE_CONTEXT_PATH must be set}")
[[ -n "${KUBE_CONTEXT:-}" ]] && args+=(--kube-context "${KUBE_CONTEXT}")
deploy-camunda "${args[@]}"
