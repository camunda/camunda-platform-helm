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


SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

source "$(dirname "$0")/base_playwright_script.sh"
source "$(dirname "$0")/render-e2e-env.sh"

# ------------------------------------------------------------------------------
# Helper Functions
# ------------------------------------------------------------------------------

validate_args() {
  local chart_path="$1"
  local namespace="$2"
  local kube_context="${3:-}"
  local kubectl_cmd="kubectl"
  
  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi
  
  log "DEBUG: Validating arguments"

  if [[ -z "$chart_path" ]]; then
    echo "Error: --absolute-chart-path is required" >&2
    exit 1
  fi

  if [[ ! -f "$chart_path/Chart.yaml" ]]; then
    echo "Error: chart path '$chart_path' does not contain a Chart.yaml file" >&2
    exit 1
  fi

  if [[ -z "$namespace" ]]; then
    echo "Error: --namespace is required" >&2
    exit 1
  fi

  if ! $kubectl_cmd get namespace "$namespace" > /dev/null 2>&1; then
    echo "Error: namespace '$namespace' not found in the current Kubernetes context" >&2
    exit 1
  fi
  
  log "DEBUG: Arguments validated successfully"
}

resolve_deploy_camunda() {
  # An asdf `reshim golang` leaves a ~/.asdf/shims/deploy-camunda that only resolves for the golang
  # version the binary was installed under. When .tool-versions pins a different version the shim
  # exits non-zero for every invocation while still shadowing a working $GOPATH/bin build, so probe
  # candidates by running one instead of trusting `command -v`.
  local candidate
  for candidate in "${DEPLOY_CAMUNDA:-}" "$(command -v deploy-camunda 2> /dev/null)" \
    "$(go env GOPATH 2> /dev/null)/bin/deploy-camunda" "$HOME/go/bin/deploy-camunda"; do
    if [[ -n "$candidate" && -x "$candidate" ]] && "$candidate" e2e-env --help > /dev/null 2>&1; then
      echo "$candidate"
      return 0
    fi
  done
  return 1
}

build_rerun_cmd() {
  # Reconstructs the invocation from the parsed arguments so a failing run prints a
  # command that reproduces it. Reads the same globals the run itself used; the
  # topology arguments come from the *_ARG snapshots taken before "$ENV_FILE" was
  # sourced, since that file replaces OPTIMIZE_CONTEXT_PATH with an absolute URL.
  local cmd="./scripts/run-e2e-tests.sh --absolute-chart-path ${ABSOLUTE_CHART_PATH} --namespace ${NAMESPACE}"
  [[ -n "$KUBE_CONTEXT" ]] && cmd+=" --kube-context ${KUBE_CONTEXT}"
  [[ -n "$TEST_EXCLUDE" ]] && cmd+=" --test-exclude \"${TEST_EXCLUDE}\""
  [[ "$RUN_SMOKE_TESTS" == "true" ]] && cmd+=" --run-smoke-tests"
  [[ "$IS_OPENSEARCH" == "true" ]] && cmd+=" --opensearch"
  [[ "$IS_RBA" == "true" ]] && cmd+=" --rba"
  [[ "$IS_MT" == "true" ]] && cmd+=" --mt"
  [[ "$IS_AUTH0" == "true" ]] && cmd+=" --auth0"
  [[ -n "$VIDEO_MODE" ]] && cmd+=" --video ${VIDEO_MODE}"
  [[ -n "$TRACE_MODE" ]] && cmd+=" --trace ${TRACE_MODE}"
  [[ -n "$RETRIES" ]] && cmd+=" --retries ${RETRIES}"
  [[ -n "$LOCAL_TEST_SUITE" ]] && cmd+=" --local-test-suite ${LOCAL_TEST_SUITE}"
  # Without these a failing topology leg prints a rerun command that targets an
  # orchestration-only environment: it cannot reproduce the failure, and it would skip
  # Optimize again rather than reporting it.
  [[ -n "$HUB_NAMESPACE_ARG" ]] && cmd+=" --hub-namespace ${HUB_NAMESPACE_ARG}"
  [[ -n "$OPTIMIZE_NAMESPACE_ARG" ]] && cmd+=" --optimize-namespace ${OPTIMIZE_NAMESPACE_ARG}"
  [[ -n "$OPTIMIZE_CONTEXT_PATH_ARG" ]] && cmd+=" --optimize-context-path ${OPTIMIZE_CONTEXT_PATH_ARG}"
  [[ -n "$MODELER_CLUSTER_NAME_ARG" ]] && cmd+=" --modeler-cluster-name ${MODELER_CLUSTER_NAME_ARG}"
  echo "$cmd"
}

usage() {
  cat << EOF
This script runs the integration tests for the Camunda Platform Helm chart.

Usage:
  $0 [options]

Options:
  --absolute-chart-path ABSOLUTE_CHART_PATH   The absolute path to the chart directory.
  --namespace NAMESPACE                       The namespace c8 is deployed into
  --kube-context KUBE_CONTEXT                 The Kubernetes context to use (optional).
  --show-html-report                          Show the HTML report after the tests have run.
  --shard-index SHARD_INDEX                   The shard index to run.
  --shard-total SHARD_TOTAL                   The total number of shards.
  --test-exclude TEST_EXCLUDE                 The tests to exclude
  --not-ci                                    Don't set the CI env var to true
  --run-smoke-tests                           Run the smoke tests
  --opensearch                                Run the opensearch tests
  --rba                                       Run the rba tests
  --mt                                        Run the mt tests
  --auth0                                     Run the auth0-smoke project (Auth0 OIDC scenario)
  --playwright-debug                          Enable Playwright API debug logs and traces
  --video MODE                                Record video: on, off, retain-on-failure, on-first-retry (default: off)
  --trace MODE                                Record trace: on, off, retain-on-failure, on-first-retry (default: off)
  --retries N                                 Number of test retries (overrides playwright.config value)
  --local-test-suite DIR                      Use a local checkout of c8-cross-component-e2e-tests instead of the npm package
  --hub-namespace NAMESPACE                   For a multi-namespace topology: the namespace running the central
                                               Identity/Keycloak. --namespace is then the orchestration namespace.
                                               Requires deploy-camunda on PATH (used to merge the .env).
  --optimize-namespace NAMESPACE              For a topology whose Optimize runs as its own release: the namespace
                                               running it. Without this, Optimize is derived from --namespace and
                                               every Optimize spec is skipped. Requires --hub-namespace.
  --optimize-context-path PATH                 Ingress path that Optimize release is served on, e.g. /optimize-orcha.
  --modeler-cluster-name NAME                  For a topology with several orchestration releases: the cluster this leg
                                               must deploy to in the Hub's Web Modeler.
  -v | --verbose                              Show verbose output.
  -h | --help                                 Show this help message and exit.
EOF
}

# ------------------------------------------------------------------------------
# Main
# ------------------------------------------------------------------------------

# Default values
ABSOLUTE_CHART_PATH=""
NAMESPACE=""
KUBE_CONTEXT=""
SHOW_HTML_REPORT=false
VERBOSE=false
SHARD_INDEX=1
SHARD_TOTAL=1
TEST_EXCLUDE=""
IS_CI="${CI:-false}"
RUN_SMOKE_TESTS=false
IS_OPENSEARCH=false
IS_RBA=false
IS_MT=false
IS_AUTH0=false
PLAYWRIGHT_DEBUG=false
VIDEO_MODE=""
TRACE_MODE=""
RETRIES=""
LOCAL_TEST_SUITE=""
HUB_NAMESPACE=""
OPTIMIZE_NAMESPACE=""
OPTIMIZE_CONTEXT_PATH=""
MODELER_CLUSTER_NAME_ARG=""

check_required_cmds

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    --absolute-chart-path)
      ABSOLUTE_CHART_PATH="$2"
      shift 2
      ;;
    --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    --kube-context)
      KUBE_CONTEXT="$2"
      shift 2
      ;;
    --show-html-report)
      SHOW_HTML_REPORT=true
      shift
      ;;
    --shard-index)
      SHARD_INDEX="$2"
      shift 2
      ;;
    --shard-total)
      SHARD_TOTAL="$2"
      shift 2
      ;;
    --test-exclude)
      TEST_EXCLUDE="$2"
      shift 2
      ;;
    --not-ci)
      IS_CI=false
      shift
      ;;
    --run-smoke-tests)
      RUN_SMOKE_TESTS=true
      shift
      ;;
    --opensearch)
      IS_OPENSEARCH=true
      shift
      ;;
    --rba)
      IS_RBA=true
      shift
      ;;
    --mt)
      IS_MT=true
      shift
      ;;
    --auth0)
      IS_AUTH0=true
      shift
      ;;
    --playwright-debug)
      PLAYWRIGHT_DEBUG=true
      shift
      ;;
    --video)
      VIDEO_MODE="$2"
      shift 2
      ;;
    --trace)
      TRACE_MODE="$2"
      shift 2
      ;;
    --retries)
      RETRIES="$2"
      shift 2
      ;;
    --local-test-suite)
      LOCAL_TEST_SUITE="$2"
      shift 2
      ;;
    --hub-namespace)
      HUB_NAMESPACE="$2"
      shift 2
      ;;
    --optimize-namespace)
      OPTIMIZE_NAMESPACE="$2"
      shift 2
      ;;
    --optimize-context-path)
      OPTIMIZE_CONTEXT_PATH="$2"
      shift 2
      ;;
    --modeler-cluster-name)
      MODELER_CLUSTER_NAME_ARG="$2"
      shift 2
      ;;
    -v | --verbose)
      VERBOSE=true
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      usage
      exit 1
      ;;
  esac
done

log "DEBUG: Starting run-e2e-tests.sh"
log "DEBUG: Chart: $ABSOLUTE_CHART_PATH, Namespace: $NAMESPACE, KubeContext: $KUBE_CONTEXT"

validate_args "$ABSOLUTE_CHART_PATH" "$NAMESPACE" "$KUBE_CONTEXT"

# The Optimize flags are only read on the topology path below, which is selected by
# --hub-namespace. Without this guard they are silently dropped, render-e2e-env.sh sets
# IS_OPTIMIZE=false, and the suite reports success having skipped every Optimize spec --
# precisely the silent gap the flags exist to close.
if [[ -z "$HUB_NAMESPACE" ]] && { [[ -n "$OPTIMIZE_NAMESPACE" ]] || [[ -n "$OPTIMIZE_CONTEXT_PATH" ]]; }; then
  echo "Error: --optimize-namespace/--optimize-context-path require --hub-namespace." >&2
  echo "       They are read only on the multi-namespace topology path; without it they would be" >&2
  echo "       ignored and the run would silently skip Optimize coverage while still passing." >&2
  exit 1
fi

# Snapshot the topology targeting args before "$ENV_FILE" is sourced below. That file sets
# OPTIMIZE_CONTEXT_PATH to an absolute URL, which would otherwise replace the ingress path the
# caller passed in the rerun command printed on failure.
HUB_NAMESPACE_ARG="$HUB_NAMESPACE"
OPTIMIZE_NAMESPACE_ARG="$OPTIMIZE_NAMESPACE"
OPTIMIZE_CONTEXT_PATH_ARG="$OPTIMIZE_CONTEXT_PATH"

TEST_SUITE_PATH="${ABSOLUTE_CHART_PATH%/}/test/e2e"
hostname=$(get_ingress_hostname "$NAMESPACE" "$KUBE_CONTEXT")

if [[ "$IS_CI" != "true" ]]; then
  _wait_for_dns_resolution "$hostname" || exit 1
fi

_wait_for_ingress_ready "$hostname" "$NAMESPACE" 300 "$KUBE_CONTEXT" || true

if [[ "$IS_CI" != "true" ]] && [[ "$_NEEDS_DNS_FALLBACK" == "true" ]]; then
  _enable_dns_fallback "$hostname" "$_RESOLVED_IP"
fi

log "DEBUG: Hostname: $hostname"
log "DEBUG: Test suite path: $TEST_SUITE_PATH"
[[ "$IS_OPENSEARCH" == "true" ]] && log "IS_OPENSEARCH is set to true"
[[ "$IS_RBA" == "true" ]] && log "IS_RBA is set to true"
[[ "$IS_MT" == "true" ]] && log "IS_MT is set to true"
[[ "$IS_AUTH0" == "true" ]] && log "IS_AUTH0 is set to true (auth0-smoke Playwright project)"

# ── Namespace-scoped .env to avoid collisions during parallel matrix runs ──
# When multiple matrix entries target the same chart version, they share the
# same TEST_SUITE_PATH.  Writing a single .env would cause a race condition.
# Instead we write .env.<namespace> and source it into the process environment
# so that Playwright inherits the values.  The dotenv() calls in test configs
# are harmless no-ops because dotenv never overrides existing env vars.
ENV_FILE="${TEST_SUITE_PATH%/}/.env.${NAMESPACE}"
trap 'rm -f "$ENV_FILE"' EXIT

# The suite resolves this itself as `process.env.OPTIMIZE_CONTEXT_PATH ?? '/optimize'`, and `??` does
# not fall back on an empty string. Assigning "" above keeps any inherited export flag, so clear the
# name outright rather than leaving it exported and empty; the topology path sets it in $ENV_FILE.
[[ -z "$OPTIMIZE_CONTEXT_PATH" ]] && unset OPTIMIZE_CONTEXT_PATH

if [[ -n "$HUB_NAMESPACE" ]]; then
  log "DEBUG: Multi-namespace topology - merging orchestration ($NAMESPACE) + Hub ($HUB_NAMESPACE) into $ENV_FILE"
  DEPLOY_CAMUNDA_BIN=$(resolve_deploy_camunda) || {
    echo "Error: no working deploy-camunda found; refusing to run tests against an unmerged env." >&2
    echo "       Tried \$DEPLOY_CAMUNDA, PATH ($(command -v deploy-camunda || echo 'not found')), and \$GOPATH/bin." >&2
    echo "       Build it with 'make install.deploy-camunda', or set DEPLOY_CAMUNDA to a working binary." >&2
    exit 1
  }
  log "DEBUG: Using deploy-camunda at $DEPLOY_CAMUNDA_BIN"
  "$DEPLOY_CAMUNDA_BIN" e2e-env merge \
    --orchestration-namespace "$NAMESPACE" \
    --hub-namespace "$HUB_NAMESPACE" \
    --absolute-chart-path "$ABSOLUTE_CHART_PATH" \
    --output "$ENV_FILE" \
    --ci="$IS_CI" \
    --run-smoke-tests="$RUN_SMOKE_TESTS" \
    --render-script "$SCRIPT_DIR/render-e2e-env.sh" \
    ${OPTIMIZE_NAMESPACE:+--optimize-namespace "$OPTIMIZE_NAMESPACE"} \
    ${OPTIMIZE_CONTEXT_PATH:+--optimize-context-path "$OPTIMIZE_CONTEXT_PATH"} \
    ${KUBE_CONTEXT:+--kube-context "$KUBE_CONTEXT"} || {
    # This script does not run under `set -e`, so without this guard a failed merge falls through and
    # Playwright runs against a stale or orchestration-only .env: Modeler and Identity point at the
    # wrong host and the setup project times out, which reads as a product failure rather than a
    # missing merge.
    echo "Error: '$DEPLOY_CAMUNDA_BIN e2e-env merge' failed; refusing to run tests against an unmerged env." >&2
    exit 1
  }
else
  render_env_file "$ENV_FILE" "$TEST_SUITE_PATH" "$hostname" "$NAMESPACE" "$IS_CI" "$IS_OPENSEARCH" "$IS_RBA" "$IS_MT" "$RUN_SMOKE_TESTS" "$KUBE_CONTEXT" "$IS_AUTH0"
fi

# Export every variable from the namespace-scoped .env into the shell so that
# the npx playwright subprocess inherits them without needing the .env file.
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

# ── Namespace-scoped Playwright output directories ──
# Playwright defaults test artifacts to <cwd>/test-results and HTML reports to
# <cwd>/playwright-report.  When parallel entries cd into the same test suite
# directory these collide.  The env vars below isolate each run.
export PLAYWRIGHT_TEST_OUTPUT="${TEST_SUITE_PATH}/test-results/${NAMESPACE}"
export PLAYWRIGHT_HTML_REPORT="${TEST_SUITE_PATH}/playwright-report/${NAMESPACE}"
[[ -n "$VIDEO_MODE" ]] && export PLAYWRIGHT_E2E_VIDEO="$VIDEO_MODE"
[[ -n "$TRACE_MODE" ]] && export PLAYWRIGHT_E2E_TRACE="$TRACE_MODE"
[[ -n "$RETRIES" ]] && export PLAYWRIGHT_E2E_RETRIES="$RETRIES"
[[ -n "$LOCAL_TEST_SUITE" ]] && export PLAYWRIGHT_E2E_LOCAL_TEST_SUITE="$LOCAL_TEST_SUITE"
[[ -n "$MODELER_CLUSTER_NAME_ARG" ]] && export MODELER_CLUSTER_NAME="$MODELER_CLUSTER_NAME_ARG"

log "$TEST_SUITE_PATH"
log "Running smoke tests: $RUN_SMOKE_TESTS"
log "DEBUG: Shard: $SHARD_INDEX/$SHARD_TOTAL, Exclude: $TEST_EXCLUDE, Debug: $PLAYWRIGHT_DEBUG"
log "DEBUG: ENV_FILE='${ENV_FILE}'"
log "DEBUG: PLAYWRIGHT_HTML_REPORT='${PLAYWRIGHT_HTML_REPORT}'"

# Build the rerun command for display on failure
RERUN_CMD="$(build_rerun_cmd)"

run_playwright_tests "$TEST_SUITE_PATH" "$SHOW_HTML_REPORT" "$SHARD_INDEX" "$SHARD_TOTAL" "blob" "$TEST_EXCLUDE" "$RUN_SMOKE_TESTS" "$PLAYWRIGHT_DEBUG" "$NAMESPACE" "$KUBE_CONTEXT" "$RERUN_CMD" "$IS_AUTH0"

log "DEBUG: E2E tests completed"
