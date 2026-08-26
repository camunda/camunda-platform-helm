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
# Regenerate a chart's values schema from values.yaml and merge the
# hand-maintained values.schema.extra.json into it.
#
# Shared by `make helm.schema-update` (writes the committed values.schema.json)
# and `make helm.schema-validate-values` (writes a throwaway schema for the
# verification gate). Keeping the readme-generator + jq-merge recipe in one
# place stops the two targets from drifting.
#
# Usage: regenerate-values-schema.sh <values.yaml> <values.schema.extra.json> <output.json>
# The extra file is optional; when it is missing the generated schema is used as-is.
#

values_file="$1"
extra_file="$2"
out_file="$3"

readme-generator --values "${values_file}" --schema "${out_file}"

if [ -f "${extra_file}" ]; then
	merged="${out_file}.merged"
	jq --indent 4 -s 'reduce .[] as $obj ({}; . * $obj)' \
		"${out_file}" "${extra_file}" >"${merged}"
	mv "${merged}" "${out_file}"
fi
