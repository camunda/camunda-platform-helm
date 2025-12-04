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

run_playwright_tests() {
  local test_suite_path="$1"
  local show_html_report="$2"
  local shard_index="$3"
  local shard_total="$4"
  local reporter="$5"
  local test_exclude="$6"
  local run_smoke_tests="$7"
  local enable_debug="$8"

  log "Changing directory to $test_suite_path"
  log "Smoke tests: $run_smoke_tests"
  log "Reporter: $reporter"

  cd "$test_suite_path" || exit

  npm install --no-audit --no-fund --prefer-online --package-lock=false
  # Ensure Playwright browsers are available (fresh install or version update)
  if [[ "$(uname -s)" == "Linux" ]]; then
    npx playwright install --with-deps || exit 1
  else
    npx playwright install || exit 1
  fi

  if [[ $show_html_report == "true" ]]; then
    reporter="html"
  fi

  # Enable Playwright debug and traces if requested
  TRACE_FLAG=""
  if [[ "$enable_debug" == "true" ]]; then
    export DEBUG="${DEBUG:-pw:api,pw:browser*}"
    TRACE_FLAG="--trace=retain-on-failure"
    log "Playwright DEBUG enabled: $DEBUG"
  fi

  mkdir -p "$test_suite_path/test-results"
  if [[ $run_smoke_tests == true ]]; then
    log "Running smoke tests"
    npx playwright test --project=smoke-tests --shard="${shard_index}/${shard_total}" --reporter="$reporter" --grep-invert="$test_exclude" $TRACE_FLAG
  else
    log "Running full suite"
    npx playwright test --project=full-suite --shard="${shard_index}/${shard_total}" --reporter="$reporter" --grep-invert="$test_exclude" $TRACE_FLAG
  fi
  playwright_rc=$? # <-- capture the exit status BEFORE doing anything else

  if [[ $show_html_report == "true" ]]; then
    npx playwright show-report
  fi

  if [[ $playwright_rc -eq 0 ]]; then
    log "✅  All Playwright tests passed"
    exit 0 # success exit for the script itself
  else
    log "❌  Playwright tests failed with code $playwright_rc"
    exit $playwright_rc # propagate the failure to CI
  fi
}

# Run playwright tests for hybrid auth - runs specific test files with a specific auth type
# This function does NOT exit so multiple phases can run sequentially
run_playwright_tests_hybrid() {
  local test_suite_path="$1"
  local show_html_report="$2"
  local auth_type="$3"
  local test_files="$4"
  local test_exclude="$5"
  local reporter="html"

  log "Running hybrid tests: auth_type='$auth_type' test_files='$test_files'"

  cd "$test_suite_path" || exit

  npm i --no-audit --no-fund --silent

  if [[ $show_html_report == "true" ]]; then
    reporter="html"
  fi

  mkdir -p "$test_suite_path/test-results"

  # Run specific test files with the auth type set as environment variable
  # This overrides any TEST_AUTH_TYPE in .env file
  # shellcheck disable=SC2086
  TEST_AUTH_TYPE="$auth_type" npx playwright test $test_files --project=full-suite --reporter="$reporter" --grep-invert="$test_exclude"
  playwright_rc=$?

  if [[ $playwright_rc -ne 0 ]]; then
    log "❌  Hybrid Playwright tests ($auth_type) failed with code $playwright_rc"
    exit $playwright_rc
  fi

  log "✅  Hybrid Playwright tests ($auth_type) passed"
}
