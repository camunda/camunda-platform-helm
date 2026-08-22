#!/usr/bin/env bash
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

if ! command -v go > /dev/null 2>&1; then
  echo "[ERR ] Go toolchain not found on PATH; cannot run prepare-helm-values CLI" >&2
  exit 127
fi

# Execute the Go CLI within its module directory so Go picks up the nested go.mod
script_dir="$(cd -- "$(dirname "$0")" > /dev/null 2>&1 && pwd)"
cli_dir="${script_dir}/prepare-helm-values"

cd "$cli_dir"
# Ensure dependencies and go.sum are present
go mod tidy -v
exec go run . "$@"
