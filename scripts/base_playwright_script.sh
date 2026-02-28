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
#   5. Installs Node dependencies with `npm install` and finally executes the
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
  local kube_context="${2:-}"
  local hostname
  local kubectl_cmd="kubectl"

  if [[ -n "$TEST_INGRESS_HOST" ]]; then
    echo "$TEST_INGRESS_HOST"
    return 0
  fi
  
  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi

  hostname=$($kubectl_cmd -n "$namespace" get ingress -o json | jq -r '
    .items[]
    | select(all(.spec.rules[].host; (contains("zeebe") or contains("grpc")) | not))
    | ([.spec.rules[].host] | join(","))')
  if [[ -z "$hostname" ]]; then
    # might be using the Gateway api
    hostname=$($kubectl_cmd -n "$namespace" get gateway -o json | jq -r '.items[].spec.listeners[].hostname')
  fi

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

  # Check if we should skip npm install (node_modules exists and package-lock.json hasn't changed)
  if [[ -d "node_modules" ]] && [[ -f "package-lock.json" ]]; then
    # In CI with pre-configured containers, prefer using existing node_modules if available
    if [[ "${CI:-false}" == "true" ]] && [[ -f "node_modules/.package-lock.json" ]]; then
      log "node_modules exists in CI environment, checking if up to date..."
      # Compare package-lock.json hash with cached version
      local current_hash cached_hash
      current_hash=$(md5sum package-lock.json 2>/dev/null | cut -d' ' -f1 || echo "none")
      cached_hash=$(cat node_modules/.package-lock-hash 2>/dev/null || echo "")
      if [[ "$current_hash" == "$cached_hash" ]]; then
        log "node_modules is up to date, skipping npm install"
        mkdir -p "$test_suite_path/test-results"
        return 0
      fi
    fi
  fi

  # Use npm install in CI to handle package.json/lock file mismatches gracefully
  if [[ "${CI:-false}" == "true" ]]; then
    log "Running npm install (CI mode)..."
    # shellcheck disable=SC2086
    npm install $npm_flags
    # Store hash for future comparison
    md5sum package-lock.json 2>/dev/null | cut -d' ' -f1 > node_modules/.package-lock-hash || true
  else
    # Force fresh install locally to always get the latest dependencies
    log "Running npm install (local mode)..."
    # shellcheck disable=SC2086
    rm -rf node_modules package-lock.json && npm i $npm_flags
  fi

  mkdir -p "$test_suite_path/test-results"
}

# Install Playwright browsers (with deps on Linux)
# Skips installation if browsers are already present (e.g., in pre-built container image)
_install_playwright_browsers() {
  # Check if we're running in a container with pre-installed browsers
  # The official Playwright Docker image sets PLAYWRIGHT_BROWSERS_PATH
  if [[ -n "${PLAYWRIGHT_BROWSERS_PATH:-}" ]] && [[ -d "${PLAYWRIGHT_BROWSERS_PATH}" ]]; then
    local browser_count
    browser_count=$(find "${PLAYWRIGHT_BROWSERS_PATH}" -maxdepth 1 -type d | wc -l)
    if [[ "$browser_count" -gt 1 ]]; then
      log "Playwright browsers already installed at ${PLAYWRIGHT_BROWSERS_PATH}, skipping installation"
      return 0
    fi
  fi

  # Also check common Playwright browser locations
  local ms_playwright_path="/ms-playwright"
  if [[ -d "$ms_playwright_path" ]]; then
    local browser_count
    browser_count=$(find "$ms_playwright_path" -maxdepth 1 -type d | wc -l)
    if [[ "$browser_count" -gt 1 ]]; then
      log "Playwright browsers already installed at ${ms_playwright_path}, skipping installation"
      return 0
    fi
  fi

  log "Installing Playwright browsers..."
  if [[ "$(uname -s)" == "Linux" ]]; then
    npx playwright install --with-deps || exit 1
  else
    npx playwright install || exit 1
  fi
}

# Handle playwright test result and exit appropriately
# Args: playwright_rc, test_description, rerun_command, [should_exit=true]
_handle_playwright_result() {
  local playwright_rc="$1"
  local test_description="$2"
  local rerun_command="$3"
  local should_exit="${4:-true}"

  if [[ $playwright_rc -eq 0 ]]; then
    log "✅  $test_description passed"
    if [[ "$should_exit" == "true" ]]; then
      exit 0
    fi
  else
    log "❌  $test_description failed with code $playwright_rc"
    echo ""
    echo "========================================"
    echo "To rerun this test locally, run:"
    echo "========================================"
    echo ""
    echo "  $rerun_command"
    echo ""
    echo "========================================"
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
# Pod Health Check Functions (for spot instance resilience)
# ==============================================================================

# Check if all pods in namespace are Ready
# Returns 0 if all pods ready, 1 otherwise
# For pods to be considered ready:
#   - Completed/Succeeded pods (Jobs) are always considered ready
#   - Running pods must have all containers ready (e.g., 1/1, 2/2)
# Args: namespace
# Args: namespace, [kube_context]
_check_all_pods_ready() {
  local namespace="$1"
  local kube_context="${2:-}"
  local kubectl_cmd="kubectl"
  
  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi
  
  if [[ -z "$namespace" ]]; then
    log "WARNING: No namespace provided for pod check, skipping"
    return 0
  fi
  
  # Get pods that are NOT completed jobs AND are not fully ready
  # kubectl get pods output: NAME READY STATUS RESTARTS AGE
  # READY column (field 2) shows "X/Y" - we need X==Y for ready
  # STATUS column (field 3) shows Running, Completed, Succeeded, etc.
  local not_ready_pods
  not_ready_pods=$(kubectl get pods -n "$namespace" --no-headers 2>/dev/null | awk '
    # Skip completed jobs - they are always considered ready
    $3 == "Completed" || $3 == "Succeeded" { next }
    # For other pods, check if READY column shows all containers ready AND status is Running
    {
      split($2, ready, "/")
      if (ready[1] != ready[2] || $3 != "Running") {
        print $0
      }
    }
  ')
  local not_ready
  not_ready=$($kubectl_cmd get pods -n "$namespace" --no-headers 2>/dev/null | grep -cvE "Running|Completed" || true)
  
  if [[ -z "$not_ready_pods" ]]; then
    return 0
  else
    local count
    count=$(echo "$not_ready_pods" | wc -l | tr -d ' ')
    log "WARNING: $count pod(s) not ready in namespace $namespace:"
    while IFS= read -r line; do
      log "  $line"
    done <<< "$not_ready_pods"
    return 1
  fi
}

# Wait for all pods in namespace to be Ready
# Excludes completed Job pods (they use Succeeded status, not Ready condition)
# Args: namespace, [timeout_seconds=300], [kube_context]
_wait_for_pods_ready() {
  local namespace="$1"
  local timeout="${2:-300}"
  local kube_context="${3:-}"
  local kubectl_cmd="kubectl"
  
  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi
  
  if [[ -z "$namespace" ]]; then
    log "WARNING: No namespace provided for pod wait, skipping"
    return 0
  fi
  
  log "Waiting up to ${timeout}s for all pods in namespace $namespace to be Ready..."
  
  # Exclude completed Jobs (status.phase=Succeeded) - they don't have Ready condition
  if $kubectl_cmd wait --for=condition=Ready pods --all \
       --field-selector=status.phase!=Succeeded \
       -n "$namespace" \
       --timeout="${timeout}s" 2>/dev/null; then
    log "All pods in namespace $namespace are Ready"
    return 0
  else
    # kubectl wait failed - this can happen if:
    # 1. Actual timeout (pods not ready)
    # 2. A pod was deleted during the wait (e.g., Error pod removed by controller)
    # 
    # Re-check if all current pods are now ready. If they are, the failed pod
    # was likely deleted and we can proceed.
    log "kubectl wait failed, verifying current pod state..."
    if _check_all_pods_ready "$namespace" "$kube_context"; then
      log "All current pods in namespace $namespace are Ready (previous failure may have been due to pod deletion)"
      return 0
    fi
    
    log "ERROR: Timeout waiting for pods to be Ready in namespace $namespace"
    _dump_pod_status "$namespace"
    return 1
  fi
}

# Helper to dump pod status for diagnostics
_dump_pod_status() {
  local namespace="$1"
  log "Current pod status in namespace $namespace:"
  kubectl get pods -n "$namespace" -o wide 2>/dev/null | while IFS= read -r line; do
    log "  $line"
  done
}

# Configuration for pod failure retry logic
_POD_RETRY_MAX_ATTEMPTS=2
_POD_RETRY_TIMEOUT=420  # 7 minutes

# Run a playwright command with retry logic for pod failures (spot instance preemption)
# This function will retry the test if:
#   1. Pods are detected as not ready after test failure
#   2. Connection errors (ECONNREFUSED, etc.) are detected in test output
#      (handles race condition where pods recovered quickly after disruption)
# Args: namespace, kube_context, playwright_command...
# Returns: playwright exit code (0 = success, non-zero = failure)
_run_playwright_with_retry() {
  local namespace="$1"
  local kube_context="$2"
  shift 2
  local playwright_cmd=("$@")
  
  local attempt=0
  local playwright_rc=0
  local output_file=""
  
  # Cleanup temp file on function exit
  trap 'rm -f "$output_file"' RETURN
  
  while [[ $attempt -le $_POD_RETRY_MAX_ATTEMPTS ]]; do
    attempt=$((attempt + 1))
    
    # Check pods are ready before running
    if [[ -n "$namespace" ]]; then
      if ! _check_all_pods_ready "$namespace" "$kube_context"; then
        log "WARNING: Pods not ready before test attempt $attempt, waiting for recovery..."
        if ! _wait_for_pods_ready "$namespace" "$_POD_RETRY_TIMEOUT" "$kube_context"; then
          log "ERROR: Pods did not recover before test attempt $attempt"
          if [[ $attempt -ge $_POD_RETRY_MAX_ATTEMPTS ]]; then
            log "ERROR: Max retry attempts reached, pods never recovered"
            return 1
          fi
          log "Will continue to next attempt..."
          continue
        fi
      fi
    fi
    
    if [[ $attempt -gt 1 ]]; then
      log "Retry attempt $attempt/$_POD_RETRY_MAX_ATTEMPTS after pod recovery..."
    fi
    
    # Create temp file to capture output for connection error analysis
    # Clean up any previous iteration's file first
    rm -f "$output_file"
    output_file=$(mktemp)
    
    # Run the playwright command, tee output to file for analysis
    # Use pipefail to capture the actual playwright exit code
    set -o pipefail
    "${playwright_cmd[@]}" 2>&1 | tee "$output_file"
    playwright_rc=${PIPESTATUS[0]}
    set +o pipefail
    
    # If tests passed, we're done
    if [[ $playwright_rc -eq 0 ]]; then
      return 0
    fi
    
    # Tests failed - analyze why
    log "Tests failed with exit code $playwright_rc"
    
    # Check for connection-related errors in output (infrastructure issues)
    local has_connection_error=false
    if grep -qiE "ECONNREFUSED|ECONNRESET|ETIMEDOUT|connection refused|connection reset|transport error|dial tcp.*connect:|Unavailable desc = connection error" "$output_file" 2>/dev/null; then
      has_connection_error=true
      log "WARNING: Detected connection error in test output (likely infrastructure issue)"
    fi
    
    if [[ -n "$namespace" ]]; then
      log "Checking pod health after test failure..."
      
      if ! _check_all_pods_ready "$namespace" "$kube_context"; then
        # Pods are not ready - this was likely a spot instance preemption
        log "WARNING: Test failed and pods are not ready (possible spot instance preemption)"
        
        if [[ $attempt -lt $_POD_RETRY_MAX_ATTEMPTS ]]; then
          log "Waiting for pods to recover before retry..."
          if _wait_for_pods_ready "$namespace" "$_POD_RETRY_TIMEOUT" "$kube_context"; then
            log "Pods recovered, will retry tests..."
            continue
          else
            log "ERROR: Pods did not recover within timeout"
            return $playwright_rc
          fi
        else
          log "ERROR: Max retry attempts reached, pods still not ready"
          return $playwright_rc
        fi
      elif [[ "$has_connection_error" == "true" ]]; then
        # Pods are ready NOW, but we saw connection errors - likely recovered mid-test
        log "WARNING: Pods are ready now, but connection errors occurred during test"
        log "This suggests infrastructure disruption during test execution (pods may have recovered after failure)"
        
        if [[ $attempt -lt $_POD_RETRY_MAX_ATTEMPTS ]]; then
          log "Will retry due to connection errors..."
          # Brief pause to ensure stability after recovery
          log "Pausing 10s to ensure pod stability..."
          sleep 10
          continue
        else
          log "ERROR: Max retry attempts reached"
          _dump_pod_status "$namespace"
          return $playwright_rc
        fi
      else
        # Pods are ready and no connection errors - legitimate test failure
        log "Pods are healthy and no connection errors detected - this appears to be a legitimate test failure"
        _dump_pod_status "$namespace"
        return $playwright_rc
      fi
    else
      # No namespace provided, can't check pods - return the failure
      return $playwright_rc
    fi
  done
  
  return $playwright_rc
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
  local namespace="${9:-}"  # Optional: namespace for pod health checks
  local kube_context="${10:-}"  # Optional: kubernetes context
  local rerun_cmd="${11:-}"  # Optional: command to rerun tests locally

  log "Smoke tests: $run_smoke_tests"
  log "Reporter: $reporter"
  [[ -n "$namespace" ]] && log "Namespace for pod checks: $namespace"
  [[ -n "$kube_context" ]] && log "Kube context: $kube_context"

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

  # Build the playwright command arguments
  local -a playwright_args=(
  npx playwright test
  --project="$project"
  --shard="${shard_index}/${shard_total}"
  --reporter="$reporter,json"
  --output=test-results
  --reporter-options outputFile=test-results/playwright-results.json
)
  [[ -n "$test_exclude" ]] && playwright_args+=(--grep-invert="$test_exclude")
  [[ -n "$trace_flag" ]] && playwright_args+=($trace_flag)

  # Run with retry logic for pod failures (spot instance preemption)
  _run_playwright_with_retry "$namespace" "$kube_context" "${playwright_args[@]}"
  local playwright_rc=$?

  # Only show HTML report locally, never in CI (it blocks waiting for Ctrl+C)
  if [[ "$show_html_report" == "true" && "${CI:-false}" != "true" ]]; then
    npx playwright show-report
  fi

  _handle_playwright_result "$playwright_rc" "All Playwright tests" "$rerun_cmd" "true"
}

# Run playwright tests for hybrid auth - runs specific test files with a specific auth type
# This function does NOT exit on success so multiple phases can run sequentially
run_playwright_tests_hybrid() {
  local test_suite_path="$1"
  local show_html_report="$2"
  local auth_type="$3"
  local test_files="$4"
  local test_exclude="$5"
  local namespace="${6:-}"  # Optional: namespace for pod health checks
  local kube_context="${7:-}"  # Optional: kubernetes context
  local rerun_cmd="${8:-}"  # Optional: command to rerun tests locally

  log "Running hybrid tests: auth_type='$auth_type' test_files='$test_files'"
  [[ -n "$namespace" ]] && log "Namespace for pod checks: $namespace"
  [[ -n "$kube_context" ]] && log "Kube context: $kube_context"

  _setup_playwright_environment "$test_suite_path" "true"

  local reporter
  reporter=$(_get_reporter "html" "$show_html_report")

  # Build the playwright command arguments
  # shellcheck disable=SC2206
  local -a playwright_args=(npx playwright test $test_files --project=full-suite --reporter="$reporter")
  [[ -n "$test_exclude" ]] && playwright_args+=(--grep-invert="$test_exclude")

  # Run specific test files with the auth type set as environment variable
  # This overrides any TEST_AUTH_TYPE in .env file
  # Run with retry logic for pod failures (spot instance preemption)
  TEST_AUTH_TYPE="$auth_type" _run_playwright_with_retry "$namespace" "$kube_context" "${playwright_args[@]}"
  local playwright_rc=$?

  _handle_playwright_result "$playwright_rc" "Hybrid Playwright tests ($auth_type)" "$rerun_cmd" "false"
}
