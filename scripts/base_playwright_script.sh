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
log() {
  if $VERBOSE; then
    echo "[$(date +'%Y-%m-%dT%H:%M:%S%z')]: $*"
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
  required_cmds=(kubectl jq git envsubst npm npx make)
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

  log "Changing directory to $test_suite_path"
  log "Smoke tests: $run_smoke_tests"
  log "Reporter: $reporter"

  cd "$test_suite_path" || exit

  npm i --no-audit --no-fund --silent
  #  sudo npx playwright install-deps
  npx playwright install

  if [[ $show_html_report == "true" ]]; then
    reporter="html"
  fi

  mkdir -p "$test_suite_path/test-results"
  if [[ $run_smoke_tests == true ]]; then
    log "Running smoke tests"
    npx playwright test --project=smoke-tests --shard="${shard_index}/${shard_total}" --reporter="$reporter" --grep-invert="$test_exclude"
  else
    log "Running full suite"
    npx playwright test --project=full-suite --shard="${shard_index}/${shard_total}" --reporter="$reporter" --grep-invert="$test_exclude"
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
