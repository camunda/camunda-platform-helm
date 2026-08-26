#!/usr/bin/env bats

# Tests for scripts/run-e2e-tests.sh.
#
# Two behaviours here exist to stop a topology leg reporting success while
# having skipped Optimize entirely, and to make a failing leg reproducible:
#
#   * the Optimize targeting flags are read only on the multi-namespace path
#     selected by --hub-namespace, so supplying them without it must fail
#     rather than fall through to the orchestration-only render;
#   * the printed rerun command must carry the topology targeting, or it reruns
#     against an orchestration-only environment that cannot reproduce anything.

setup() {
  here="$(cd "$(dirname "${BATS_TEST_FILENAME}")" && pwd)"
  if ROOT="$(git -C "$here" rev-parse --show-toplevel 2>/dev/null)"; then
    :
  else
    ROOT="$(cd "$here/../.." && pwd)"
  fi
  export ROOT
  export SCRIPT="$ROOT/scripts/run-e2e-tests.sh"
  export CHART_PATH="$ROOT/charts/camunda-platform-8.10"

  TMPDIR_TEST="$(mktemp -d)"
  export TMPDIR_TEST
  export PATH="$TMPDIR_TEST/bin:$PATH"
  mkdir -p "$TMPDIR_TEST/bin"

  # validate_args runs before the guard under test and needs the namespace to
  # exist; everything else the script would reach is past the exit points here.
  cat > "$TMPDIR_TEST/bin/kubectl" << 'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "$TMPDIR_TEST/bin/kubectl"
}

teardown() {
  rm -rf "$TMPDIR_TEST"
}

@test "--optimize-namespace without --hub-namespace is rejected" {
  run "$SCRIPT" --absolute-chart-path "$CHART_PATH" --namespace test-ns \
    --optimize-namespace opt-ns --optimize-context-path /optimize-orcha

  [ "$status" -eq 1 ]
  [[ "$output" == *"require --hub-namespace"* ]]
}

@test "--optimize-context-path without --hub-namespace is rejected" {
  run "$SCRIPT" --absolute-chart-path "$CHART_PATH" --namespace test-ns \
    --optimize-context-path /optimize-orcha

  [ "$status" -eq 1 ]
  [[ "$output" == *"require --hub-namespace"* ]]
}

@test "the rerun command carries the topology targeting arguments" {
  ABSOLUTE_CHART_PATH="$CHART_PATH"
  NAMESPACE="matrix-810-mns-orcha"
  KUBE_CONTEXT=""
  TEST_EXCLUDE=""
  RUN_SMOKE_TESTS="true"
  IS_OPENSEARCH="false"
  IS_RBA="false"
  IS_MT="false"
  IS_AUTH0="false"
  VIDEO_MODE=""
  TRACE_MODE=""
  RETRIES=""
  LOCAL_TEST_SUITE=""
  HUB_NAMESPACE_ARG="matrix-810-mns-hub"
  OPTIMIZE_NAMESPACE_ARG="matrix-810-mns-opt-orcha"
  OPTIMIZE_CONTEXT_PATH_ARG="/optimize-orcha"
  MODELER_CLUSTER_NAME_ARG="Orchestration A"

  # Only the function is wanted, so define it from the script's own source.
  eval "$(sed -n '/^build_rerun_cmd() {/,/^}/p' "$SCRIPT")"

  run build_rerun_cmd

  [ "$status" -eq 0 ]
  [[ "$output" == *"--namespace matrix-810-mns-orcha"* ]]
  [[ "$output" == *"--hub-namespace matrix-810-mns-hub"* ]]
  [[ "$output" == *"--optimize-namespace matrix-810-mns-opt-orcha"* ]]
  [[ "$output" == *"--optimize-context-path /optimize-orcha"* ]]
  [[ "$output" == *"--modeler-cluster-name Orchestration\\ A"* ]]
}

@test "the rerun command omits topology flags for an ordinary single-namespace run" {
  ABSOLUTE_CHART_PATH="$CHART_PATH"
  NAMESPACE="matrix-810-single"
  KUBE_CONTEXT=""
  TEST_EXCLUDE=""
  RUN_SMOKE_TESTS="true"
  IS_OPENSEARCH="false"
  IS_RBA="false"
  IS_MT="false"
  IS_AUTH0="false"
  VIDEO_MODE=""
  TRACE_MODE=""
  RETRIES=""
  LOCAL_TEST_SUITE=""
  HUB_NAMESPACE_ARG=""
  OPTIMIZE_NAMESPACE_ARG=""
  OPTIMIZE_CONTEXT_PATH_ARG=""
  MODELER_CLUSTER_NAME_ARG=""

  eval "$(sed -n '/^build_rerun_cmd() {/,/^}/p' "$SCRIPT")"

  run build_rerun_cmd

  # A false trailing conditional must not leak out as a failing exit status.
  [ "$status" -eq 0 ]
  [[ "$output" == *"--namespace matrix-810-single"* ]]
  [[ "$output" != *"--hub-namespace"* ]]
  [[ "$output" != *"--optimize-"* ]]
  [[ "$output" != *"--modeler-cluster-name"* ]]
}
