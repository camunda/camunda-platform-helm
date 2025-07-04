#!/usr/bin/env bash

source "$(dirname "$0")/base_playwright_script.sh"

validate_args() {
  local chart_path="$1"
  local namespace="$2"

  if [[ -z "$chart_path" ]]; then
    echo "--absolute-chart-path is required"
    exit 1
  fi

  if [[ ! -f "$chart_path/Chart.yaml" ]]; then
    echo "Error: chart path '$chart_path' does not contain a Chart.yaml file" >&2
    exit 1
  fi

  if [[ -z "$namespace" ]]; then
    echo "--namespace is required"
    exit 1
  fi

  if ! kubectl get namespace "$namespace" >/dev/null 2>&1; then
    echo "Error: namespace '$namespace' not found in the current Kubernetes context" >&2
    exit 1
  fi
}

setup_env_file() {
  local env_file="$1"
  local test_suite_path="$2"
  local hostname="$3"
  local namespace="$4"
  local is_ci="$5"

  export TEST_INGRESS_HOST="$hostname"
  envsubst <"$test_suite_path"/.env.template >"$env_file"

  # during helm install, we create a secret with the credentials for the services
  # that are used to test the platform. This is grabbing those credentials and
  # adding them to the .env file so that we can run the tests from any environment
  # with an authorized kubectl context.

  {
    echo "PLAYWRIGHT_BASE_URL=https://$hostname"
    echo "CLUSTER_VERSION=8"
    echo "MINOR_VERSION=SM-8.7"
    echo "DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD=$(kubectl -n "$namespace" get secret integration-test-credentials -o jsonpath='{.data.identity-user-password}' | base64 -d)"
    echo "DISTRO_QA_E2E_TESTS_KEYCLOAK_PASSWORD=$(kubectl -n "$namespace" get secret integration-test-credentials -o jsonpath='{.data.identity-keycloak-admin-password}' | base64 -d)"
    echo "CI=${is_ci}"
    echo "CLUSTER_NAME=integration"
  } >>"$env_file"

  if $VERBOSE; then
    log "Contents of .env file:"
    cat "$env_file"
  fi
}

usage() {
  cat <<EOF
This script runs the integration tests for the Camunda Platform Helm chart.

Usage:
  $0 [options]

Options:
  --absolute-chart-path ABSOLUTE_CHART_PATH   The absolute path to the chart directory.
  --namespace NAMESPACE                       The namespace c8 is deployed into
  --show-html-report                          Show the HTML report after the tests have run.
  --shard-index SHARD_INDEX                   The shard index to run.
  --shard-total SHARD_TOTAL                   The total number of shards.
  --test-exclude TEST_EXCLUDE                  The tests to exclude
  --not-ci                                    Don't set the CI env var to true
  --run-smoke-tests                           Run the smoke tests
  -v | --verbose                              Show verbose output.
  -h | --help                                 Show this help message and exit.
EOF
}

# ------------------------------------------------------------------------------
# Main
# ------------------------------------------------------------------------------

ABSOLUTE_CHART_PATH=""
NAMESPACE=""
SHOW_HTML_REPORT=false
VERBOSE=false
SHARD_INDEX=1
SHARD_TOTAL=1
TEST_EXCLUDE=""
IS_CI=true
RUN_SMOKE_TESTS=false

check_required_cmds

while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --absolute-chart-path)
    ABSOLUTE_CHART_PATH="$2"
    shift 2
    ;;
  --namespace)
    NAMESPACE="$2"
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
  -v | --verbose)
    VERBOSE=true
    shift
    ;;
  -h | --help)
    usage
    exit 0
    ;;
  *)
    echo "Unknown option: $key"
    usage
    exit 1
    ;;
  esac
done

validate_args "$ABSOLUTE_CHART_PATH" "$NAMESPACE"

TEST_SUITE_PATH="${ABSOLUTE_CHART_PATH%/}/test/e2e"

hostname=$(get_ingress_hostname "$NAMESPACE")
setup_env_file "$TEST_SUITE_PATH/.env" "$TEST_SUITE_PATH" "$hostname" "$NAMESPACE" "$IS_CI"

log "$TEST_SUITE_PATH"
log "Running smoke tests: $RUN_SMOKE_TESTS"
run_playwright_tests "$TEST_SUITE_PATH" "$SHOW_HTML_REPORT" "$SHARD_INDEX" "$SHARD_TOTAL" "blob" "$TEST_EXCLUDE" "$RUN_SMOKE_TESTS"
