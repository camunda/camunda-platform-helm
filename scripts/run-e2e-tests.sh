#!/usr/bin/env bash

source "$(dirname "$0")/base_playwright_script.sh"

# ------------------------------------------------------------------------------
# Helper Functions
# ------------------------------------------------------------------------------

resolve_minor_version_from_identity() {
  local namespace="$1"
  local version_label=""
  
  log "DEBUG: Resolving minor version for namespace $namespace"
  
  # Prefer Zeebe component, then Orchestration component
  version_label="$(kubectl -n "$namespace" get sts -l app.kubernetes.io/component=zeebe-broker -o jsonpath='{.items[0].metadata.labels.app\.kubernetes\.io/version}' 2>/dev/null || true)"
  if [[ -z "$version_label" ]]; then
    version_label="$(kubectl -n "$namespace" get sts -l app.kubernetes.io/component=orchestration -o jsonpath='{.items[0].metadata.labels.app\.kubernetes\.io/version}' 2>/dev/null || true)"
  fi
  
  if [[ -n "$version_label" ]]; then
    local major minor
    IFS='.' read -r major minor _ <<< "$version_label"
    if [[ -n "$major" && -n "$minor" ]]; then
      log "DEBUG: Resolved minor version: SM-$major.$minor"
      printf "SM-%s.%s" "$major" "$minor"
      return 0
    fi
  fi

  log "DEBUG: Could not resolve minor version"
  return 12
}

resolve_env_password() {
  local namespace="$1"
  local env_var_name="$2"
  local password=""
  
  log "DEBUG: Resolving $env_var_name"
  
  # Try direct value
  password="$(kubectl -n "$namespace" get deployment -l app.kubernetes.io/component=identity -o jsonpath="{.items[0].spec.template.spec.containers[0].env[?(@.name==\"${env_var_name}\")].value}" 2>/dev/null || true)"
  if [[ -n "$password" ]]; then
    log "DEBUG: Found $env_var_name from direct env value"
    printf "%s" "$password"
    return 0
  fi

  # Try valueFrom secretKeyRef
  log "DEBUG: Trying secret reference for $env_var_name"
  local secret_name secret_key
  secret_name="$(kubectl -n "$namespace" get deployment -l app.kubernetes.io/component=identity -o jsonpath="{.items[0].spec.template.spec.containers[0].env[?(@.name==\"${env_var_name}\")].valueFrom.secretKeyRef.name}" 2>/dev/null || true)"
  secret_key="$(kubectl -n "$namespace" get deployment -l app.kubernetes.io/component=identity -o jsonpath="{.items[0].spec.template.spec.containers[0].env[?(@.name==\"${env_var_name}\")].valueFrom.secretKeyRef.key}" 2>/dev/null || true)"
  
  if [[ -n "$secret_name" && -n "$secret_key" ]]; then
    log "DEBUG: Retrieving $env_var_name from secret $secret_name/$secret_key"
    password="$(kubectl -n "$namespace" get secret "$secret_name" -o jsonpath="{.data['$secret_key']}" 2>/dev/null | base64 -d || true)"
    if [[ -n "$password" ]]; then
      log "DEBUG: Successfully retrieved $env_var_name from secret"
      printf "%s" "$password"
      return 0
    fi
  fi

  log "DEBUG: Could not resolve $env_var_name, leaving blank"
  printf ""
}

resolve_keycloak_setup_password() {
  local namespace="$1"
  local password=""
  
  log "DEBUG: Resolving Keycloak setup password"
  
  # Try KEYCLOAK_SETUP_PASSWORD
  password="$(resolve_env_password "$namespace" "KEYCLOAK_SETUP_PASSWORD")"
  if [[ -n "$password" ]]; then
    printf "%s" "$password"
    return 0
  fi

  # Try VALUES_KEYCLOAK_SETUP_PASSWORD
  password="$(resolve_env_password "$namespace" "VALUES_KEYCLOAK_SETUP_PASSWORD")"
  if [[ -n "$password" ]]; then
    printf "%s" "$password"
    return 0
  fi

  # Fallback to legacy secret
  log "DEBUG: Trying legacy vault-mapped-secrets for Keycloak password"
  password="$(kubectl -n "$namespace" get secret vault-mapped-secrets -o jsonpath='{.data.DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET}' 2>/dev/null | base64 -d || true)"
  if [[ -n "$password" ]]; then
    log "DEBUG: Found Keycloak password from legacy secret"
    printf "%s" "$password"
    return 0
  fi

  log "Error: Could not determine Keycloak setup password from Identity deployment or legacy secret."
  return 1
}

resolve_identity_passwords() {
  local namespace="$1"
  
  log "DEBUG: Resolving identity user passwords"
  
  # Check if vault-mapped-secrets has the identity password key
  local vault_firstuser_pw
  vault_firstuser_pw=$(kubectl -n "$namespace" get secret vault-mapped-secrets -o jsonpath='{.data.DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD}' 2>/dev/null | base64 -d || true)
  
  if [[ -n "$vault_firstuser_pw" ]]; then
    log "DEBUG: Using vault-mapped-secrets"
    DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD="$vault_firstuser_pw"
    DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD=$(kubectl -n "$namespace" get secret vault-mapped-secrets -o jsonpath='{.data.DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD}' 2>/dev/null | base64 -d || true)
    DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD=$(kubectl -n "$namespace" get secret vault-mapped-secrets -o jsonpath='{.data.DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD}' 2>/dev/null | base64 -d || true)
  else
    log "DEBUG: Using identity deployment env vars"
    DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD="$(resolve_env_password "$namespace" "VALUES_IDENTITY_FIRSTUSER_PASSWORD")"
    DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD="$(resolve_env_password "$namespace" "VALUES_IDENTITY_SECONDUSER_PASSWORD")"
    DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD="$(resolve_env_password "$namespace" "VALUES_IDENTITY_THIRDUSER_PASSWORD")"
  fi
  
  # Mask sensitive values in CI logs (these should go to stdout for GitHub Actions)
  echo "::add-mask::$DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD"
  echo "::add-mask::$DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD"
  echo "::add-mask::$DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD"
}

validate_args() {
  local chart_path="$1"
  local namespace="$2"
  
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

  if ! kubectl get namespace "$namespace" > /dev/null 2>&1; then
    echo "Error: namespace '$namespace' not found in the current Kubernetes context" >&2
    exit 1
  fi
  
  log "DEBUG: Arguments validated successfully"
}

setup_env_file() {
  local env_file="$1"
  local test_suite_path="$2"
  local hostname="$3"
  local namespace="$4"
  local is_ci="$5"
  local is_opensearch="$6"
  local is_rba="$7"
  local is_mt="$8"
  local run_smoke_tests="$9"
  
  log "DEBUG: Setting up env file: $env_file"

  # Generate base .env from template
  export TEST_INGRESS_HOST="$hostname"
  envsubst < "$test_suite_path"/.env.template > "$env_file"

  # Resolve credentials from cluster
  KEYCLOAK_SETUP_PASSWORD="$(resolve_keycloak_setup_password "$namespace")" || exit 1
  echo "::add-mask::$KEYCLOAK_SETUP_PASSWORD"
  
  resolve_identity_passwords "$namespace"

  # Resolve minor version
  local minor_version_value
  if ! minor_version_value="$(resolve_minor_version_from_identity "$namespace")"; then
    echo "Error: Could not determine minor version from Zeebe or Orchestration deployments." >&2
    exit 12
  fi
  keycloakUrl=$(kubectl -n "$namespace" get deployment -l app.kubernetes.io/component=identity -o jsonpath="{.items[0].metadata.annotations.keycloak-token-url}")
  host=""
  tokenUrl=""
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
  
  # process the tokenUrl to get the host and protocol
  keycloak_host=$(echo "$tokenUrl" | sed -n 's|^[^:]*://\([^/]*\)/auth/realms/.*/protocol/openid-connect/token$|\1|p')
  keycloak_protocol=$(echo "$tokenUrl" | sed -n 's|^\([^:]*\)://.*|\1|p')
  keycloak_realm=$(echo "$tokenUrl" | sed -n 's|^[^:]*://[^/]*/auth/realms/\([^/]*\)/protocol/openid-connect/token$|\1|p')
  if [[ -z "$keycloak_realm" ]]; then
    keycloak_realm="camunda-platform"
  fi
  # Append runtime values to .env file
  {
    echo "KEYCLOAK_URL=$keycloak_protocol://$keycloak_host"
    echo "KEYCLOAK_REALM=${keycloak_realm}"
    echo "PLAYWRIGHT_BASE_URL=https://$hostname"
    echo "CLUSTER_ENDPOINT=https://$hostname:26501"
    echo "CLUSTER_VERSION=8"
    echo "MINOR_VERSION=$minor_version_value"
    echo "DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD=$DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD"
    echo "DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD=$DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD"
    echo "DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD=$DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD"
    echo "DISTRO_QA_E2E_TESTS_KEYCLOAK_PASSWORD=$KEYCLOAK_SETUP_PASSWORD"
    echo "DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET=$KEYCLOAK_CLIENTS_PASSWORD"
    echo "OAUTH_URL=$tokenUrl"
    echo "CI=${is_ci}"
    echo "CLUSTER_NAME=integration"
    echo "IS_OPENSEARCH=${is_opensearch}"
    echo "IS_RBA=${is_rba}"
    echo "IS_MT=${is_mt}"
    echo "IS_SMOKE=${run_smoke_tests}"
  } >> "$env_file"

  log "DEBUG: Env file setup complete"
  if [[ "$VERBOSE" == "true" ]]; then
    log "DEBUG: Contents of .env file:"
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
  --show-html-report                          Show the HTML report after the tests have run.
  --shard-index SHARD_INDEX                   The shard index to run.
  --shard-total SHARD_TOTAL                   The total number of shards.
  --test-exclude TEST_EXCLUDE                 The tests to exclude
  --not-ci                                    Don't set the CI env var to true
  --run-smoke-tests                           Run the smoke tests
  --opensearch                                Run the opensearch tests
  --rba                                       Run the rba tests
  --mt                                        Run the mt tests
  --playwright-debug                          Enable Playwright API debug logs and traces
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
SHOW_HTML_REPORT=false
VERBOSE=false
SHARD_INDEX=1
SHARD_TOTAL=1
TEST_EXCLUDE=""
IS_CI=true
RUN_SMOKE_TESTS=false
IS_OPENSEARCH=false
IS_RBA=false
IS_MT=false
PLAYWRIGHT_DEBUG=false

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
    --playwright-debug)
      PLAYWRIGHT_DEBUG=true
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
      echo "Unknown option: $1"
      usage
      exit 1
      ;;
  esac
done

log "DEBUG: Starting run-e2e-tests.sh"
log "DEBUG: Chart: $ABSOLUTE_CHART_PATH, Namespace: $NAMESPACE"

validate_args "$ABSOLUTE_CHART_PATH" "$NAMESPACE"

TEST_SUITE_PATH="${ABSOLUTE_CHART_PATH%/}/test/e2e"
hostname=$(get_ingress_hostname "$NAMESPACE")

log "DEBUG: Hostname: $hostname"
log "DEBUG: Test suite path: $TEST_SUITE_PATH"
[[ "$IS_OPENSEARCH" == "true" ]] && log "IS_OPENSEARCH is set to true"
[[ "$IS_RBA" == "true" ]] && log "IS_RBA is set to true"
[[ "$IS_MT" == "true" ]] && log "IS_MT is set to true"

setup_env_file "$TEST_SUITE_PATH/.env" "$TEST_SUITE_PATH" "$hostname" "$NAMESPACE" "$IS_CI" "$IS_OPENSEARCH" "$IS_RBA" "$IS_MT" "$RUN_SMOKE_TESTS"

log "$TEST_SUITE_PATH"
log "Running smoke tests: $RUN_SMOKE_TESTS"
log "DEBUG: Shard: $SHARD_INDEX/$SHARD_TOTAL, Exclude: $TEST_EXCLUDE, Debug: $PLAYWRIGHT_DEBUG"

run_playwright_tests "$TEST_SUITE_PATH" "$SHOW_HTML_REPORT" "$SHARD_INDEX" "$SHARD_TOTAL" "blob" "$TEST_EXCLUDE" "$RUN_SMOKE_TESTS" "$PLAYWRIGHT_DEBUG"

log "DEBUG: E2E tests completed"
