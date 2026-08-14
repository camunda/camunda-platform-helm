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
# Check if latest chart version matches the latest release.
#

helm repo add camunda https://helm.camunda.io
helm repo update

chart_main_dir=$(ls -d1 charts/camunda-platform-8* | tail -n1)
chart_main_version="$(yq '.version' ${chart_main_dir}/Chart.yaml)"
components_versions=$(helm template camunda/camunda-platform | awk -F'helm.sh/chart: ' '/helm.sh\/chart:/ {print $2}' | sort | uniq)

components_count=2

print_components_versions() {
    echo "Current versions from Camunda Helm repo:"
    printf -- "- %s\n" ${components_versions}
}

if [[ $(echo "${components_versions}" | grep -c "${chart_main_version}") -lt "${components_count}" ]]; then
    echo '[ERROR] Not all Camunda components are updated!'
    print_components_versions
    exit 1
fi

echo '[INFO] All Camunda components are updated.'
print_components_versions
exit 0
