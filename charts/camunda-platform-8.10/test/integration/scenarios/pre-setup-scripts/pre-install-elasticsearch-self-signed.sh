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
#
# Pre-install script for the "elasticsearch-self-signed" persistence layer.
# Called by deploy-camunda's PreInstallHook mechanism before helm install.
#
# Required env vars (set by the matrix runner):
#   TEST_NAMESPACE  - target Kubernetes namespace
#   KUBE_CONTEXT    - kubectl context (optional)
#   RELEASE_NAME    - helm release name (optional, defaults to "integration")

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec bash "${SCRIPT_DIR}/create-elasticsearch-tls-secrets.sh"
