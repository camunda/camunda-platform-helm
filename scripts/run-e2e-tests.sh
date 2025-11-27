#!/usr/bin/env bash

source "$(dirname "$0")/base_playwright_script.sh"

resolve_minor_version_from_identity() {
  local namespace="$1"
  local version_label=""
  local major=""
  local minor=""

  # Prefer Zeebe component, then Orchestration component
  version_label="$(kubectl -n "$namespace" get sts -l app.kubernetes.io/component=zeebe-broker -o jsonpath='{.items[0].metadata.labels.app\.kubernetes\.io/version}' 2>/dev/null || true)"
  if [[ -z "$version_label" ]]; then
    version_label="$(kubectl -n "$namespace" get sts -l app.kubernetes.io/component=orchestration -o jsonpath='{.items[0].metadata.labels.app\.kubernetes\.io/version}' 2>/dev/null || true)"
  fi
  if [[ -n "$version_label" ]]; then
    IFS='.' read -r major minor _ <<< "$version_label"
    if [[ -n "$major" && -n "$minor" ]]; then
      printf "SM-%s.%s" "$major" "$minor"
      return 0
    fi
  fi

  return 12
}

resolve_keycloak_setup_password() {
  local namespace="$1"
  local password=""
  local secret_name=""
  local secret_key=""

  # Try KEYCLOAK_SETUP_PASSWORD (value)
  password="$(kubectl -n "$namespace" get deploy -l app.kubernetes.io/component=identity -o jsonpath='{.items[0].spec.template.spec.containers[*].env[?(@.name=="KEYCLOAK_SETUP_PASSWORD")].value}' 2>/dev/null || true)"
  if [[ -n "$password" ]]; then
    printf "%s" "$password"
    return 0
  fi

  # Try KEYCLOAK_SETUP_PASSWORD (valueFrom.secretKeyRef)
  secret_name="$(kubectl -n "$namespace" get deploy -l app.kubernetes.io/component=identity -o jsonpath='{.items[0].spec.template.spec.containers[*].env[?(@.name=="KEYCLOAK_SETUP_PASSWORD")].valueFrom.secretKeyRef.name}' 2>/dev/null || true)"
  secret_key="$(kubectl -n "$namespace" get deploy -l app.kubernetes.io/component=identity -o jsonpath='{.items[0].spec.template.spec.containers[*].env[?(@.name=="KEYCLOAK_SETUP_PASSWORD")].valueFrom.secretKeyRef.key}' 2>/dev/null || true)"
  if [[ -n "$secret_name" && -n "$secret_key" ]]; then
    password="$(kubectl -n "$namespace" get secret "$secret_name" -o jsonpath="{.data['$secret_key']}" 2>/dev/null | base64 -d || true)"
  fi
  if [[ -n "$password" ]]; then
    printf "%s" "$password"
    return 0
  fi

  # Try VALUES_KEYCLOAK_SETUP_PASSWORD (value)
  password="$(kubectl -n "$namespace" get deploy -l app.kubernetes.io/component=identity -o jsonpath='{.items[0].spec.template.spec.containers[*].env[?(@.name=="VALUES_KEYCLOAK_SETUP_PASSWORD")].value}' 2>/dev/null || true)"
  if [[ -n "$password" ]]; then
    printf "%s" "$password"
    return 0
  fi

  # Try VALUES_KEYCLOAK_SETUP_PASSWORD (valueFrom.secretKeyRef)
  secret_name="$(kubectl -n "$namespace" get deploy -l app.kubernetes.io/component=identity -o jsonpath='{.items[0].spec.template.spec.containers[*].env[?(@.name=="VALUES_KEYCLOAK_SETUP_PASSWORD")].valueFrom.secretKeyRef.name}' 2>/dev/null || true)"
  secret_key="$(kubectl -n "$namespace" get deploy -l app.kubernetes.io/component=identity -o jsonpath='{.items[0].spec.template.spec.containers[*].env[?(@.name=="VALUES_KEYCLOAK_SETUP_PASSWORD")].valueFrom.secretKeyRef.key}' 2>/dev/null || true)"
  if [[ -n "$secret_name" && -n "$secret_key" ]]; then
    password="$(kubectl -n "$namespace" get secret "$secret_name" -o jsonpath="{.data['$secret_key']}" 2>/dev/null | base64 -d || true)"
  fi
  if [[ -n "$password" ]]; then
    printf "%s" "$password"
    return 0
  fi

  # Fallback to legacy secret
  password="$(kubectl -n "$namespace" get secret vault-mapped-secrets -o jsonpath='{.data.DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET}' 2>/dev/null | base64 -d || true)"
  if [[ -n "$password" ]]; then
    printf "%s" "$password"
    return 0
  fi

  echo "Error: Could not determine Keycloak setup password from Identity deployment or legacy secret." >&2
  return 1
}

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

  if ! kubectl get namespace "$namespace" > /dev/null 2>&1; then
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
  local is_opensearch="$6"
  local is_rba="$7"
  local is_mt="$8"

  export TEST_INGRESS_HOST="$hostname"
  envsubst < "$test_suite_path"/.env.template > "$env_file"

  # during helm install, we create a secret with the credentials for the services
  # that are used to test the platform. This is grabbing those credentials and
  # adding them to the .env file so that we can run the tests from any environment
  # with an authorized kubectl context.
  # Keycloak password resolution (all-in-one)
  KEYCLOAK_SETUP_PASSWORD="$(resolve_keycloak_setup_password "$namespace")" || exit 1
  echo "::add-mask::$KEYCLOAK_SETUP_PASSWORD"
  DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD=$(kubectl -n "$namespace" get secret vault-mapped-secrets -o jsonpath='{.data.DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD}' | base64 -d)
  echo "::add-mask::$DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD"
  DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD=$(kubectl -n "$namespace" get secret vault-mapped-secrets -o jsonpath='{.data.DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD}' | base64 -d)
  echo "::add-mask::$DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD"
  DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD=$(kubectl -n "$namespace" get secret vault-mapped-secrets -o jsonpath='{.data.DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD}' | base64 -d)
  echo "::add-mask::$DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD"

  # Resolve minor version and fail loudly if unavailable
  local minor_version_value=""
  if ! minor_version_value="$(resolve_minor_version_from_identity "$namespace")"; then
    echo "Error: Could not determine minor version from Zeebe or Orchestration deployments." >&2
    exit 12
  fi

  {
    echo "PLAYWRIGHT_BASE_URL=https://$hostname"
    echo "CLUSTER_VERSION=8"
    echo "MINOR_VERSION=$minor_version_value"
    echo "DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD=$DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD"
    echo "DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD=$DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD"
    echo "DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD=$DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD"
    echo "DISTRO_QA_E2E_TESTS_KEYCLOAK_PASSWORD=$KEYCLOAK_SETUP_PASSWORD"
    echo "CI=${is_ci}"
    echo "CLUSTER_NAME=integration"
    echo "IS_OPENSEARCH=${is_opensearch}"
    echo "IS_RBA=${is_rba}"
    echo "IS_MT=${is_mt}"
  } >> "$env_file"

  if $VERBOSE; then
    log "Contents of .env file:"
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
  --test-exclude TEST_EXCLUDE                  The tests to exclude
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
      echo "Unknown option: $key"
      usage
      exit 1
      ;;
  esac
done

validate_args "$ABSOLUTE_CHART_PATH" "$NAMESPACE"

TEST_SUITE_PATH="${ABSOLUTE_CHART_PATH%/}/test/e2e"

hostname=$(get_ingress_hostname "$NAMESPACE")
if [ "$IS_OPENSEARCH" == "true" ]; then
  log "IS_OPENSEARCH is set to true"
fi
if [ "$IS_RBA" == "true" ]; then
  log "IS_RBA is set to true"
fi
if [ "$IS_MT" == "true" ]; then
  log "IS_MT is set to true"
fi
setup_env_file "$TEST_SUITE_PATH/.env" "$TEST_SUITE_PATH" "$hostname" "$NAMESPACE" "$IS_CI" "$IS_OPENSEARCH" "$IS_RBA" "$IS_MT"

log "$TEST_SUITE_PATH"
log "Running smoke tests: $RUN_SMOKE_TESTS"
run_playwright_tests "$TEST_SUITE_PATH" "$SHOW_HTML_REPORT" "$SHARD_INDEX" "$SHARD_TOTAL" "blob" "$TEST_EXCLUDE" "$RUN_SMOKE_TESTS" "$PLAYWRIGHT_DEBUG"
