#!/usr/bin/env bash

source "$(dirname "$0")/base_playwright_script.sh"

validate_args() {
  local chart_path="$1"
  local namespace="$2"
  local platform="$3"
  local kube_context="$4"

  log "validate_args: chart_path='${chart_path}' namespace='${namespace}' kube_context='${kube_context}'"

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

  local kubectl_cmd="kubectl"
  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi

  if ! $kubectl_cmd get namespace "$namespace" > /dev/null 2>&1; then
    echo "Error: namespace '$namespace' not found in the current Kubernetes context" >&2
    exit 1
  fi

  if [[ -z "$platform" ]]; then
    echo "--platform is required (gke, eks, etc.)"
    exit 1
  fi
}

# ANSI colors for logging
C_RESET=$'\033[0m'
C_CYAN=$'\033[36m'
C_GREEN=$'\033[32m'
C_YELLOW=$'\033[33m'
C_RED=$'\033[31m'

log() {
  if $VERBOSE; then
    printf "%b\n" "${C_CYAN}[$(date +'%Y-%m-%dT%H:%M:%S%z')]${C_RESET} ${C_GREEN}$*${C_RESET}"
  fi
}

# Mask a secret in GitHub Actions logs, but only if it's non-empty
mask_secret() {
  [[ -n "$1" ]] && echo "::add-mask::$1"
}

setup_env_file() {
  local env_file="$1"
  local test_suite_path="$2"
  local hostname="$3"
  local repo_root="$4"
  local namespace="$5"
  local test_auth_type="$6"
  local is_ci="$7"
  local platform="$8"
  local kube_context="$9"

  log "setup_env_file: env_file='${env_file}' test_suite_path='${test_suite_path}' hostname='${hostname}' repo_root='${repo_root}' namespace='${namespace}' test_auth_type='${test_auth_type}' is_ci='${is_ci}' kube_context='${kube_context}'"

  local kubectl_cmd="kubectl"
  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi

  export TEST_INGRESS_HOST="$hostname"

  # Only export TEST_AUTH_TYPE for 8.8+ where the template uses it
  if [[ "$test_suite_path" == *"8.8"* ]]; then
    export TEST_AUTH_TYPE="$test_auth_type"
  fi

  log "Rendering env template: '$test_suite_path/vars/playwright/files/playwright-job-vars.env.template' -> '$env_file'"
  keycloakUrl=$($kubectl_cmd -n "$namespace" get deployment -l app.kubernetes.io/component=identity -o jsonpath="{.items[0].metadata.annotations.keycloak-token-url}")
  host=""
  echo "::group::Keycloak URL parsing"
  if [[ -n "$keycloakUrl" ]]; then
    # This parses out the host from the keycloakUrl
    tokenUrl="${keycloakUrl}"
    echo "Resolved tokenUrl: $tokenUrl"
  else
    # This parses out the host from the keycloakUrl
    tokenUrl="https://${hostname}/auth/realms/camunda-platform/protocol/openid-connect/token"
    echo "Resolved tokenUrl: $tokenUrl"
  fi
  echo "::endgroup::"

  export TEST_KEYCLOAK_TOKEN_URL="$tokenUrl"
  export TEST_KEYCLOAK_HOST="$hostname"
  echo "TEST_KEYCLOAK_HOST=${TEST_KEYCLOAK_HOST}"

  envsubst < "$test_suite_path"/vars/playwright/files/playwright-job-vars.env.template > "$env_file"

  # during helm install, we create a secret with the credentials for the services
  # that are used to test the platform. This is grabbing those credentials and
  # adding them to the .env file so that we can run the tests from any environment
  # with an authorized kubectl context.

  if [[ "$test_suite_path" == *"8.8"* || "$test_suite_path" == *"8.9"* ]]; then
    if [[ "${platform,,}" == "gke" ]]; then
      for svc in CONNECTORS TASKLIST OPTIMIZE OPERATE ZEEBE ORCHESTRATION; do
        log "Fetching secret for service '$svc' (gke identity token)"
        secret=$($kubectl_cmd -n "$namespace" \
          get secret integration-test-credentials \
          -o jsonpath="{.data.identity-${svc,,}-client-token}" | base64 -d)
        mask_secret "$secret"
        echo "PLAYWRIGHT_VAR_${svc}_CLIENT_SECRET=${secret}" >> "$env_file"
      done
    else
      for svc in CONNECTORS TASKLIST OPTIMIZE OPERATE ZEEBE ORCHESTRATION; do
        log "Fetching secret for service '$svc' (identity token)"
        secret=$($kubectl_cmd -n "$namespace" \
          get secret integration-test-credentials \
          -o jsonpath="{.data.identity-${svc,,}-client-token}" | base64 -d)
        mask_secret "$secret"
        echo "PLAYWRIGHT_VAR_${svc}_CLIENT_SECRET=${secret}" >> "$env_file"
      done
    fi
  fi

  if [[ "$test_suite_path" == *"8.7"* || "$test_suite_path" == *"8.6"* ]]; then
    for svc in CONNECTORS TASKLIST OPTIMIZE OPERATE ZEEBE ORCHESTRATION; do
      if [[ "$PLATFORM" == "gke" ]]; then
        log "Fetching secret for service '$svc' (gke identity password)"
        secret=$($kubectl_cmd -n "$namespace" \
          get secret integration-test-credentials \
          -o jsonpath="{.data.identity-${svc,,}-client-password}" | base64 -d)
      else
        log "Fetching secret for service '$svc' (legacy secret key)"
        secret=$($kubectl_cmd -n "$namespace" \
          get secret integration-test-credentials \
          -o jsonpath="{.data.${svc,,}-secret}" | base64 -d)
      fi
      mask_secret "$secret"
      echo "PLAYWRIGHT_VAR_${svc}_CLIENT_SECRET=${secret}" >> "$env_file"
    done
  fi

  log "Fetching admin client password"
  secret=$($kubectl_cmd -n "$namespace" \
    get secret integration-test-credentials \
    -o jsonpath="{.data.identity-admin-client-password}" | base64 -d)
  mask_secret "$secret"
  echo "PLAYWRIGHT_VAR_ADMIN_CLIENT_SECRET=${secret}" >> "$env_file"

  # fixtures are the *.bpmn files that are used to test the platform. This is likely to change
  # to be more flexible in what we are testing.
  log "Setting FIXTURES_DIR to ${repo_root%/}/test/integration/testsuites/playwright.core/files"
  {
    echo "TEST_AUTH_TYPE=${test_auth_type}"
    echo "FIXTURES_DIR=${repo_root%/}/test/integration/testsuites/playwright.core/files"
    echo "TEST_BASE_PATH=${repo_root%/}/test/integration/testsuites/playwright.core/files"
    echo "CI=${is_ci}"
    echo "VERBOSE=${VERBOSE}"
  } >> "$env_file"

  log ".env written to '$env_file' ($(wc -c < "$env_file") bytes)"
  log "Contents of .env file:"
  if $VERBOSE; then
    cat "$env_file"
  fi
}

usage() {
  cat << EOF
This script runs the integration tests for the Camunda Platform Helm chart.

Usage:
  $0 [options]

Options:
  --absolute-chart-path ABSOLUTE_CHART_PATH   The absolute path to the chart directory.
  --namespace NAMESPACE                       The namespace c8 is deployed into
  --platform PLATFORM                         The platform where c8 is deployed (e.g., gke, eks).
  --kube-context KUBE_CONTEXT                 The Kubernetes context to use (optional).
  --show-html-report                          Show the HTML report after the tests have run.
  --shard-index SHARD_INDEX                   The shard index to run.
  --shard-total SHARD_TOTAL                   The total number of shards.
  --test-exclude TEST_EXCLUDE                 The test suites to exclude.
  --not-ci                                    Don't set the CI env var to true
  -v | --verbose                              Show verbose output.
  -h | --help                                 Show this help message and exit.
EOF
}

# ------------------------------------------------------------------------------
# Main
# ------------------------------------------------------------------------------

ABSOLUTE_CHART_PATH=""
NAMESPACE=""
PLATFORM=""
KUBE_CONTEXT=""
SHOW_HTML_REPORT=false
VERBOSE=false
TEST_AUTH_TYPE="${TEST_AUTH_TYPE:-keycloak}"
TEST_EXCLUDE="${TEST_EXCLUDE:-}"
IS_CI=true

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
    --platform)
      PLATFORM="$2"
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
    --test-auth-type)
      TEST_AUTH_TYPE="$2"
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

log "Args parsed:"
log "  ABSOLUTE_CHART_PATH='${ABSOLUTE_CHART_PATH}'"
log "  NAMESPACE='${NAMESPACE}'"
log "  KUBE_CONTEXT='${KUBE_CONTEXT}'"
log "  SHOW_HTML_REPORT='${SHOW_HTML_REPORT}'"
log "  TEST_AUTH_TYPE='${TEST_AUTH_TYPE}'"
log "  TEST_EXCLUDE='${TEST_EXCLUDE}'"
log "  IS_CI='${IS_CI}'"
log "  VERBOSE='${VERBOSE}'"

validate_args "$ABSOLUTE_CHART_PATH" "$NAMESPACE" "$PLATFORM" "$KUBE_CONTEXT"

REPO_ROOT="$(git rev-parse --show-toplevel)"
TEST_SUITE_PATH="${ABSOLUTE_CHART_PATH%/}/test/integration/testsuites"

hostname=$(get_ingress_hostname "$NAMESPACE" "$KUBE_CONTEXT")

# ── Namespace-scoped .env to avoid collisions during parallel matrix runs ──
# When multiple matrix entries target the same chart version, they share the
# same TEST_SUITE_PATH.  Writing a single .env would cause a race condition.
# Instead we write .env.<namespace> and source it into the process environment
# so that Playwright inherits the values.  The dotenv() calls in test specs
# are harmless no-ops because dotenv never overrides existing env vars.
ENV_FILE="${TEST_SUITE_PATH%/}/.env.${NAMESPACE}"
trap 'rm -f "$ENV_FILE"' EXIT

setup_env_file "$ENV_FILE" "$TEST_SUITE_PATH" "$hostname" "$REPO_ROOT" "$NAMESPACE" "$TEST_AUTH_TYPE" "$IS_CI" "$PLATFORM" "$KUBE_CONTEXT"

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
export PLAYWRIGHT_JUNIT_OUTPUT_FILE="${TEST_SUITE_PATH}/test-results/${NAMESPACE}/results.xml"

log "Invoking Playwright tests with:"
log "  TEST_SUITE_PATH='${TEST_SUITE_PATH}' SHOW_HTML_REPORT='${SHOW_HTML_REPORT}' TEST_EXCLUDE='${TEST_EXCLUDE}'"
log "  ENV_FILE='${ENV_FILE}'"
log "  PLAYWRIGHT_HTML_REPORT='${PLAYWRIGHT_HTML_REPORT}'"

# Build the rerun command for display on failure
RERUN_CMD="./scripts/run-integration-tests.sh --absolute-chart-path ${ABSOLUTE_CHART_PATH} --namespace ${NAMESPACE} --platform ${PLATFORM}"
[[ -n "$KUBE_CONTEXT" ]] && RERUN_CMD+=" --kube-context ${KUBE_CONTEXT}"
[[ -n "$TEST_AUTH_TYPE" && "$TEST_AUTH_TYPE" != "keycloak" ]] && RERUN_CMD+=" --test-auth-type ${TEST_AUTH_TYPE}"
[[ -n "$TEST_EXCLUDE" ]] && RERUN_CMD+=" --test-exclude \"${TEST_EXCLUDE}\""

if [[ "$TEST_AUTH_TYPE" == "hybrid" ]]; then
  log "Running hybrid auth tests - splitting by component auth type"
  # Run OIDC-based tests (Identity, Console) with keycloak auth
  log "Phase 1: Running OIDC components (identity, console) with keycloak auth"
  run_playwright_tests_hybrid "$TEST_SUITE_PATH" "$SHOW_HTML_REPORT" "keycloak" "identity.spec.ts console.spec.ts" "$TEST_EXCLUDE" "$NAMESPACE" "$KUBE_CONTEXT" "$RERUN_CMD"
  # Run basic auth tests (Connectors, Orchestration REST/gRPC)
  log "Phase 2: Running basic auth components (connectors, core-rest, core-grpc) with basic auth"
  run_playwright_tests_hybrid "$TEST_SUITE_PATH" "$SHOW_HTML_REPORT" "basic" "connectors.spec.ts core-rest.spec.ts core-grpc.spec.ts" "$TEST_EXCLUDE" "$NAMESPACE" "$KUBE_CONTEXT" "$RERUN_CMD"
else
  run_playwright_tests "$TEST_SUITE_PATH" "$SHOW_HTML_REPORT" "1" "1" "html" "$TEST_EXCLUDE" false false "$NAMESPACE" "$KUBE_CONTEXT" "$RERUN_CMD"
fi
