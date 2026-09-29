#!/usr/bin/env bats
# Copyright 2026 Camunda Services GmbH
# SPDX-License-Identifier: Apache-2.0

@test "base chart dependencies use the current checkout toolchain" {
  root="$(cd "$(dirname "$BATS_TEST_FILENAME")/../.." && pwd)"
  mkdir -p "$BATS_TEST_TMPDIR/.migration-base/charts/camunda-platform-8.10" "$BATS_TEST_TMPDIR/bin"
  cp "$root/Makefile" "$BATS_TEST_TMPDIR/Makefile"
  cp "$root/Makefile" "$BATS_TEST_TMPDIR/.migration-base/Makefile"
  touch "$BATS_TEST_TMPDIR/.migration-base/charts/camunda-platform-8.10/Chart.yaml"
  export EXPECTED_CWD="$BATS_TEST_TMPDIR"
  export BASE_CHART_DIR="$BATS_TEST_TMPDIR/.migration-base/charts/camunda-platform-8.10"
  export HELM_LOG="$BATS_TEST_TMPDIR/helm.log"
  cat > "$BATS_TEST_TMPDIR/bin/helm" <<'SH'
#!/bin/bash
[[ "$PWD" == "$EXPECTED_CWD" ]] || { echo "base checkout selected an uninstalled Helm version"; exit 126; }
[[ "$1 $2" == "dependency update" && "$3" == "$BASE_CHART_DIR" ]] || exit 1
echo "current Helm prepared base chart" > "$HELM_LOG"
SH
  chmod +x "$BATS_TEST_TMPDIR/bin/helm"
  export PATH="$BATS_TEST_TMPDIR/bin:$PATH"
  command="$(yq -r '.jobs.zone-aware-migration.steps[] | select(.name == "Verify the migration sequence") | .run | split("\n") | .[0]' "$root/.github/workflows/test-zone-aware-migration.yaml")"
  cd "$BATS_TEST_TMPDIR"
  run bash -e -c "$command"
  [ "$status" -eq 0 ]
  [ -s "$HELM_LOG" ]
}
