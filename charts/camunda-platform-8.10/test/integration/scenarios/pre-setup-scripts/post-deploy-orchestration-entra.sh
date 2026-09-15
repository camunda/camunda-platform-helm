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

: "${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"
repo_root="$(git rev-parse --show-toplevel)"
exec go -C "${repo_root}/scripts/orchestration-entra-lifecycle" run . \
  --namespace "${TEST_NAMESPACE}" \
  --release "${RELEASE_NAME:-integration}" \
  --context "${KUBE_CONTEXT:-}" \
  --bpmn-file "${repo_root}/test/integration/testsuites/core/files/test-inbound-process.bpmn"
