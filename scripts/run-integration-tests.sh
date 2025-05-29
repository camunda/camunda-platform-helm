#!/usr/bin/env bash

# ==============================================================================
# Camunda Platform – Integration-Test Runner
# ------------------------------------------------------------------------------
# Why does this script exist?
#   *  A single, developer-friendly entry-point for running the Playwright-based
#      integration test-suite that lives under <chart>/test/integration.
#   *  Works both locally on a developer laptop **and** inside GitHub Actions
#      without modification.
#   *  Hardened: performs extensive sanity-checks, validates prerequisites and
#      cleans up after itself so CI troubleshooting is painless.
#
# What does it actually do?
#   1. Verifies required CLI tools are available (kubectl, jq, git, npm, …).
#   2. Validates the supplied Helm chart path and Kubernetes namespace.
#   3. Ensures Helm repos/dependencies for the chart are up-to-date (via Make).
#   4. Detects the ingress hostname for the Camunda Platform installation and
#      exports it for the tests as TEST_INGRESS_HOST.
#   5. Builds a temporary .env file populated with service client secrets and
#      Playwright variables, removing it automatically on exit.
#   6. Installs Node dependencies with `npm ci` and finally executes the
#      Playwright test runner.
#
# Expected environment / assumptions
#   • kubectl context points at a cluster where the Camunda Platform Helm chart
#     is already installed in the provided namespace.
#   • A secret named `integration-test-credentials` exists in that namespace and
#     contains `identity-admin-client-password`.
#
# Usage examples
#   # Local run against KIND cluster
#   ./scripts/run-integration-tests.sh \
#       --chart-path /home/runner/work/camunda-platform-helm/charts/camunda-platform-8.7 \
#       --namespace camunda
#
#   # Inside GitHub Actions (see .github/workflows/test-integration-template.yml)
#   # The same invocation is used; the workflow sets the arguments.
#
# Any failure will terminate the script with a non-zero exit code so that CI
# systems mark the job as failed.
# ============================================================================

set -euo pipefail

cleanup() {
  [[ -n "${ENV_FILE:-}" && -f "$ENV_FILE" ]] && rm -f "$ENV_FILE"
}

validate_args() {
  local chart_path="$1"
  local namespace="$2"

  if [[ -z "$chart_path" ]]; then
    echo "--chart-path is required"
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

log() {
  if $VERBOSE; then
    echo "[$(date +'%Y-%m-%dT%H:%M:%S%z')]: $*"
  fi
}

install_helm_dependencies() {
  local chart_path="$1"
  local repo_root="$2"

  export chartPath="charts/$(basename "$chart_path")" # this is used implicitly in the Makefile
  log "Installing Helm repos and dependencies with chartPath=$chartPath"
  cd "$repo_root"
  make helm.repos-add
  make helm.dependency-update
}

get_ingress_hostname() {
  local namespace="$1"
  local hostname
  
  hostname=$(kubectl -n "$namespace" get ingress -o json | jq -r '
    .items[]
    | select(all(.spec.rules[].host; contains("zeebe") | not))
    | ([.spec.rules[].host] | join(","))')

  if [[ -z "$hostname" || "$hostname" == "null" ]]; then
    echo "Error: unable to determine ingress hostname in namespace '$namespace'" >&2
    exit 1
  fi

  echo "$hostname"
}

setup_env_file() {
  local env_file="$1"
  local test_suite_path="$2"
  local hostname="$3"
  local repo_root="$4"
  local namespace="$5"
  
  export TEST_INGRESS_HOST="$hostname"
  envsubst < "$test_suite_path"/vars/playwright/files/playwright-job-vars.env.template > "$env_file"

  # during helm install, we create a secret with the credentials for the services
  # that are used to test the platform. This is grabbing those credentials and
  # adding them to the .env file so that we can run the tests from any environment
  # with an authorized kubectl context.
  for svc in CONNECTORS TASKLIST OPTIMIZE OPERATE TEST; do
    secret=$(kubectl -n "$namespace" \
               get secret integration-test-credentials \
               -o jsonpath='{.data.identity-admin-client-password}' | base64 -d)
    echo "PLAYWRIGHT_VAR_${svc}_CLIENT_SECRET=${secret}" >> "$env_file"
  done

  # fixtures are the *.bpmn files that are used to test the platform. This is likely to change
  # to be more flexible in what we are testing.
  log "Setting FIXTURES_DIR to ${repo_root%/}/test/integration/testsuites/playwright.core/files"
  echo "FIXTURES_DIR=${repo_root%/}/test/integration/testsuites/playwright.core/files" >> "$env_file"

  log "Contents of .env file:"
  if $VERBOSE; then
    cat "$env_file"
  fi
}

run_playwright_tests() {
  local test_suite_path="$1"
  local show_html_report="$2"
  log "Changing directory to $test_suite_path"
  log "Running test suite"

  cd "$test_suite_path"
  npm ci --no-audit --no-fund

  playwright_status=0
  npx playwright test --reporter=line,json${show_html_report:+",html"} || playwright_status=$?

  if $show_html_report; then
    npx playwright show-report || playwright_status=$?
  fi

  exit $playwright_status
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

required_cmds=(kubectl jq git envsubst npm npx make)
for cmd in "${required_cmds[@]}"; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Error: required command '$cmd' not found in PATH" >&2
    exit 127
  fi
done

trap cleanup EXIT
trap 'echo "[ERROR] Script failed at line $LINENO"; exit 1' ERR

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

REPO_ROOT="$(git rev-parse --show-toplevel)"
TEST_SUITE_PATH="${ABSOLUTE_CHART_PATH%/}/test/integration/testsuites"

install_helm_dependencies "$ABSOLUTE_CHART_PATH" "$REPO_ROOT"
hostname=$(get_ingress_hostname "$NAMESPACE")
setup_env_file "$TEST_SUITE_PATH/.env" "$TEST_SUITE_PATH" "$hostname" "$REPO_ROOT" "$NAMESPACE"

run_playwright_tests "$TEST_SUITE_PATH" "$SHOW_HTML_REPORT"