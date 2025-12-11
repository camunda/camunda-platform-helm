#!/usr/bin/env bash

# ==============================================================================
# Camunda Platform – Integration/e2e-Test Runner
# ------------------------------------------------------------------------------
# Why does this script exist?
#   *  A single, developer-friendly entry-point for running the Playwright-based
#      integration test-suite that lives under <chart>/test/integration or /test/e2e.
#   *  Works both locally on a developer laptop **and** inside GitHub Actions
#      without modification.
#   *  Hardened: performs extensive sanity-checks, validates prerequisites and
#      cleans up after itself so CI troubleshooting is painless.
#
# What does it actually do?
#   1. Verifies required CLI tools are available (kubectl, jq, git, npm, …).
#   2. Validates the supplied Helm chart path and Kubernetes namespace.
#   3. Detects the ingress hostname for the Camunda Platform installation and
#      exports it for the tests as TEST_INGRESS_HOST.
#   4. Builds a temporary .env file populated with service client secrets and
#      Playwright variables, removing it automatically on exit.
#   5. Installs Node dependencies with `npm ci` and finally executes the
#      Playwright test runner.
#
# Expected environment / assumptions
#   • kubectl context points at a cluster where the Camunda Platform Helm chart
#     is already installed in the provided namespace.
#   • A secret named `integration-test-credentials` exists in that namespace
#
# Usage examples
#   # Local run against KIND cluster
#   ./scripts/run-integration-tests.sh \
#       --chart-path /home/runner/work/camunda-platform-helm/charts/camunda-platform-8.7 \
#       --namespace camunda
#
#   ./scripts/run-e2e-tests.sh \
#       --chart-path /home/runner/work/camunda-platform-helm/charts/camunda-platform-8.7 \
#       --namespace camunda
#
# Any failure will terminate the script with a non-zero exit code so that CI
# systems mark the job as failed.
# ============================================================================

# Color definitions
COLOR_RESET='\033[0m'
COLOR_RED='\033[0;31m'
COLOR_GREEN='\033[0;32m'
COLOR_YELLOW='\033[0;33m'
COLOR_BLUE='\033[0;34m'
COLOR_MAGENTA='\033[0;35m'
COLOR_CYAN='\033[0;36m'
COLOR_GRAY='\033[0;90m'

log() {
  if $VERBOSE; then
    local message="$*"
    local color="$COLOR_RESET"
    
    # Color based on message type
    if [[ "$message" == *"ERROR"* ]] || [[ "$message" == *"Error"* ]] || [[ "$message" == "❌"* ]]; then
      color="$COLOR_RED"
    elif [[ "$message" == "✅"* ]]; then
      color="$COLOR_GREEN"
    elif [[ "$message" == "DEBUG:"* ]]; then
      color="$COLOR_GRAY"
    elif [[ "$message" == *"WARNING"* ]] || [[ "$message" == *"Warning"* ]]; then
      color="$COLOR_YELLOW"
    fi
    
    echo -e "${color}[$(date +'%Y-%m-%dT%H:%M:%S%z')]: $message${COLOR_RESET}" >&2
  fi
}

get_ingress_hostname() {
  local namespace="$1"
  local hostname

  hostname=$(kubectl -n "$namespace" get ingress -o json | jq -r '
    .items[]
    | select(all(.spec.rules[].host; (contains("zeebe") or contains("grpc")) | not))
    | ([.spec.rules[].host] | join(","))')

  if [[ -z "$hostname" || "$hostname" == "null" ]]; then
    echo "Error: unable to determine ingress hostname in namespace '$namespace'" >&2
    exit 1
  fi

  echo "$hostname"
}

check_required_cmds() {
  required_cmds=(kubectl jq git envsubst npm npx)
  for cmd in "${required_cmds[@]}"; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      echo "Error: required command '$cmd' not found in PATH" >&2
      exit 127
    fi
  done
}

# ==============================================================================
# Playwright Helper Functions
# ==============================================================================

# Setup playwright environment: change directory, install dependencies, create test-results dir
# Args: test_suite_path, [silent=false]
_setup_playwright_environment() {
  local test_suite_path="$1"
  local silent="${2:-false}"

  log "Changing directory to $test_suite_path"
  cd "$test_suite_path" || exit

  local npm_flags="--no-audit --no-fund"
  if [[ "$silent" == "true" ]]; then
    npm_flags="$npm_flags --silent"
  fi

  # Force fresh install to always get the latest dependencies
  # shellcheck disable=SC2086
  rm -rf node_modules package-lock.json && npm i $npm_flags

  mkdir -p "$test_suite_path/test-results"
}

# Install Playwright browsers (with deps on Linux)
_install_playwright_browsers() {
  if [[ "$(uname -s)" == "Linux" ]]; then
    npx playwright install --with-deps || exit 1
  else
    npx playwright install || exit 1
  fi
}

# Handle playwright test result and exit appropriately
# Args: playwright_rc, test_description, [should_exit=true]
_handle_playwright_result() {
  local playwright_rc="$1"
  local test_description="$2"
  local should_exit="${3:-true}"

  if [[ $playwright_rc -eq 0 ]]; then
    log "✅  $test_description passed"
    if [[ "$should_exit" == "true" ]]; then
      exit 0
    fi
  else
    log "❌  $test_description failed with code $playwright_rc"
    exit $playwright_rc
  fi
}

# Determine reporter based on show_html_report flag
# Args: current_reporter, show_html_report
_get_reporter() {
  local reporter="$1"
  local show_html_report="$2"

  if [[ "$show_html_report" == "true" ]]; then
    echo "html"
  else
    echo "$reporter"
  fi
}

# ==============================================================================
# Main Playwright Test Functions
# ==============================================================================

run_playwright_tests() {
  local test_suite_path="$1"
  local show_html_report="$2"
  local shard_index="$3"
  local shard_total="$4"
  local reporter="$5"
  local test_exclude="$6"
  local run_smoke_tests="$7"
  local enable_debug="$8"

  log "Smoke tests: $run_smoke_tests"
  log "Reporter: $reporter"

  _setup_playwright_environment "$test_suite_path" "false"
  _install_playwright_browsers

  reporter=$(_get_reporter "$reporter" "$show_html_report")

  # Enable Playwright debug and traces if requested
  local trace_flag=""
  if [[ "$enable_debug" == "true" ]]; then
    export DEBUG="${DEBUG:-pw:api,pw:browser*}"
    trace_flag="--trace=retain-on-failure"
    log "Playwright DEBUG enabled: $DEBUG"
  fi

  local project="full-suite"
  if [[ "$run_smoke_tests" == "true" ]]; then
    project="smoke-tests"
    log "Running smoke tests"
  else
    log "Running full suite"
  fi

  # shellcheck disable=SC2086
  npx playwright test --project="$project" --shard="${shard_index}/${shard_total}" --reporter="$reporter" --grep-invert="$test_exclude" $trace_flag
  local playwright_rc=$?

  # Only show HTML report locally, never in CI (it blocks waiting for Ctrl+C)
  if [[ "$show_html_report" == "true" && "${CI:-false}" != "true" ]]; then
    npx playwright show-report
  fi

  _handle_playwright_result "$playwright_rc" "All Playwright tests" "true"
}

# Run playwright tests for hybrid auth - runs specific test files with a specific auth type
# This function does NOT exit on success so multiple phases can run sequentially
run_playwright_tests_hybrid() {
  local test_suite_path="$1"
  local show_html_report="$2"
  local auth_type="$3"
  local test_files="$4"
  local test_exclude="$5"

  log "Running hybrid tests: auth_type='$auth_type' test_files='$test_files'"

  _setup_playwright_environment "$test_suite_path" "true"

  local reporter
  reporter=$(_get_reporter "html" "$show_html_report")

  # Run specific test files with the auth type set as environment variable
  # This overrides any TEST_AUTH_TYPE in .env file
  # shellcheck disable=SC2086
  TEST_AUTH_TYPE="$auth_type" npx playwright test $test_files --project=full-suite --reporter="$reporter" --grep-invert="$test_exclude"
  local playwright_rc=$?

  _handle_playwright_result "$playwright_rc" "Hybrid Playwright tests ($auth_type)" "false"
}
