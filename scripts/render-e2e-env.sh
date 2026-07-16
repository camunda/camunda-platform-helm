#!/usr/bin/env bash

# ==============================================================================
# Camunda Platform – E2E Environment File Renderer
# ------------------------------------------------------------------------------
# This script generates the .env file required for running E2E tests.
# It can be used standalone or sourced by other scripts.
#
# Usage:
#   Standalone:
#     ./scripts/render-e2e-env.sh \
#       --absolute-chart-path /path/to/chart \
#       --namespace my-namespace \
#       --output /custom/path/.env
#
#   Sourced:
#     source "$(dirname "$0")/render-e2e-env.sh"
#     render_env_file "$output" "$test_suite_path" "$hostname" "$namespace" ...
# ==============================================================================

# Source base script for shared utilities (log, get_ingress_hostname)
# Only source if not already sourced (check for log function)
if ! declare -f log > /dev/null 2>&1; then
  source "$(dirname "$0")/base_playwright_script.sh"
fi

# ------------------------------------------------------------------------------
# Helper Functions
# ------------------------------------------------------------------------------

# Mask a secret in GitHub Actions logs, but only if it's non-empty
mask_secret() {
  [[ -n "$1" ]] && echo "::add-mask::$1"
}

check_env_required_cmds() {
  local required_cmds=(kubectl jq envsubst)
  for cmd in "${required_cmds[@]}"; do
    if ! command -v "$cmd" > /dev/null 2>&1; then
      echo "Error: required command '$cmd' not found in PATH" >&2
      exit 127
    fi
  done
}

resolve_minor_version_from_identity() {
  local namespace="$1"
  local kube_context="${2:-}"
  local version_label=""
  local kubectl_cmd="kubectl"

  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi

  log "DEBUG: Resolving minor version for namespace $namespace"

  # Prefer Zeebe component, then Orchestration component
  version_label="$($kubectl_cmd -n "$namespace" get sts -l app.kubernetes.io/component=zeebe-broker -o jsonpath='{.items[0].metadata.labels.app\.kubernetes\.io/version}' 2> /dev/null || true)"
  if [[ -z "$version_label" ]]; then
    version_label="$($kubectl_cmd -n "$namespace" get sts -l app.kubernetes.io/component=orchestration -o jsonpath='{.items[0].metadata.labels.app\.kubernetes\.io/version}' 2> /dev/null || true)"
  fi

  if [[ -n "$version_label" ]]; then
    # Handle SNAPSHOT version format
    if [[ "$version_label" == "SNAPSHOT" ]]; then
      log "DEBUG: Resolved minor version: SM-8.10 (from SNAPSHOT)"
      printf "SM-8.10"
      return 0
    fi

    local major minor
    IFS='.' read -r major minor _ <<< "$version_label"
    # Strip any pre-release suffix (e.g. "7-SNAPSHOT" → "7") that leaks
    # through when the version has no patch number (e.g. "8.7-SNAPSHOT").
    minor="${minor%%-*}"
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
  local kube_context="${3:-}"
  local password=""
  local kubectl_cmd="kubectl"

  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi

  log "DEBUG: Resolving $env_var_name"

  # Try direct value
  password="$($kubectl_cmd -n "$namespace" get deployment -l app.kubernetes.io/component=identity -o jsonpath="{.items[0].spec.template.spec.containers[0].env[?(@.name==\"${env_var_name}\")].value}" 2> /dev/null || true)"
  if [[ -n "$password" ]]; then
    log "DEBUG: Found $env_var_name from direct env value"
    printf "%s" "$password"
    return 0
  fi

  # Try valueFrom secretKeyRef
  log "DEBUG: Trying secret reference for $env_var_name"
  local secret_name secret_key
  secret_name="$($kubectl_cmd -n "$namespace" get deployment -l app.kubernetes.io/component=identity -o jsonpath="{.items[0].spec.template.spec.containers[0].env[?(@.name==\"${env_var_name}\")].valueFrom.secretKeyRef.name}" 2> /dev/null || true)"
  secret_key="$($kubectl_cmd -n "$namespace" get deployment -l app.kubernetes.io/component=identity -o jsonpath="{.items[0].spec.template.spec.containers[0].env[?(@.name==\"${env_var_name}\")].valueFrom.secretKeyRef.key}" 2> /dev/null || true)"

  if [[ -n "$secret_name" && -n "$secret_key" ]]; then
    log "DEBUG: Retrieving $env_var_name from secret $secret_name/$secret_key"
    password="$($kubectl_cmd -n "$namespace" get secret "$secret_name" -o jsonpath="{.data['$secret_key']}" 2> /dev/null | base64 -d || true)"
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
  local kube_context="${2:-}"
  local password=""
  local kubectl_cmd="kubectl"

  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi

  log "DEBUG: Resolving Keycloak setup password"

  # Try KEYCLOAK_SETUP_PASSWORD
  password="$(resolve_env_password "$namespace" "KEYCLOAK_SETUP_PASSWORD" "$kube_context")"
  if [[ -n "$password" ]]; then
    printf "%s" "$password"
    return 0
  fi

  # Try VALUES_KEYCLOAK_SETUP_PASSWORD
  password="$(resolve_env_password "$namespace" "VALUES_KEYCLOAK_SETUP_PASSWORD" "$kube_context")"
  if [[ -n "$password" ]]; then
    printf "%s" "$password"
    return 0
  fi

  # Fallback to legacy secret
  log "DEBUG: Trying legacy vault-mapped-secrets for Keycloak password"
  password="$($kubectl_cmd -n "$namespace" get secret vault-mapped-secrets -o jsonpath='{.data.DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET}' 2> /dev/null | base64 -d || true)"
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
  local kube_context="${2:-}"
  local kubectl_cmd="kubectl"

  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi

  log "DEBUG: Resolving identity user passwords"

  # Try VALUES_* env vars from identity deployment first
  log "DEBUG: Trying identity deployment env vars"
  DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD="$(resolve_env_password "$namespace" "VALUES_IDENTITY_FIRSTUSER_PASSWORD" "$kube_context")"
  DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD="$(resolve_env_password "$namespace" "VALUES_IDENTITY_SECONDUSER_PASSWORD" "$kube_context")"
  DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD="$(resolve_env_password "$namespace" "VALUES_IDENTITY_THIRDUSER_PASSWORD" "$kube_context")"
  DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET="$(resolve_env_password "$namespace" "VALUES_TEST_CLIENT_SECRET" "$kube_context")"

  # Fallback to DISTRO_QA_E2E_TESTS_* keys in vault-mapped-secrets if VALUES_* resolved blank
  if [[ -z "$DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD" ]]; then
    log "DEBUG: Falling back to vault-mapped-secrets for DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD"
    DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD=$($kubectl_cmd -n "$namespace" get secret vault-mapped-secrets -o jsonpath='{.data.DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD}' 2> /dev/null | base64 -d || true)
  fi
  if [[ -z "$DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD" ]]; then
    log "DEBUG: Falling back to vault-mapped-secrets for DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD"
    DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD=$($kubectl_cmd -n "$namespace" get secret vault-mapped-secrets -o jsonpath='{.data.DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD}' 2> /dev/null | base64 -d || true)
  fi
  if [[ -z "$DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD" ]]; then
    log "DEBUG: Falling back to vault-mapped-secrets for DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD"
    DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD=$($kubectl_cmd -n "$namespace" get secret vault-mapped-secrets -o jsonpath='{.data.DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD}' 2> /dev/null | base64 -d || true)
  fi
  if [[ -z "$DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET" ]]; then
    log "DEBUG: Falling back to vault-mapped-secrets for DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET"
    DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET=$($kubectl_cmd -n "$namespace" get secret vault-mapped-secrets -o jsonpath='{.data.DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET}' 2> /dev/null | base64 -d || true)
  fi

  # Mask sensitive values in CI logs (these should go to stdout for GitHub Actions)
  mask_secret "$DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD"
  mask_secret "$DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD"
  mask_secret "$DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD"
  mask_secret "$DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET"
}

# _auth0_get_secret_key reads a single key from an Opaque secret and base64
# decodes it. Returns the empty string if the key is absent (so callers can
# distinguish "not set" from "set to an empty value" via the env-var override
# downstream). Doesn't `exit 1` on missing key — auth0-info-* keys are written
# best-effort and individual ones may legitimately be absent.
_auth0_get_secret_key() {
  local kubectl_cmd="$1"
  local namespace="$2"
  local secret_name="$3"
  local key="$4"
  local b64
  # The . inside the jsonpath needs to be escaped because it's part of the key
  # name, not a path separator. kubectl uses {.data['key-with.dots']} syntax;
  # for our keys (no dots) we can use the simpler form.
  b64=$($kubectl_cmd -n "$namespace" get secret "$secret_name" \
    -o "jsonpath={.data['${key}']}" 2>/dev/null || true)
  if [[ -z "$b64" ]]; then
    return 0
  fi
  printf "%s" "$b64" | base64 -d 2>/dev/null || true
}

render_env_file() {
  local env_file="$1"
  local test_suite_path="$2"
  local hostname="$3"
  local namespace="$4"
  local is_ci="$5"
  local is_opensearch="$6"
  local is_rba="$7"
  local is_mt="$8"
  local run_smoke_tests="$9"
  local kube_context="${10:-}"
  local is_auth0="${11:-false}"
  local kubectl_cmd="kubectl"

  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi

  log "DEBUG: Setting up env file: $env_file"

  # Auto-detect Optimize deployment — if no Optimize pods exist, set IS_OPTIMIZE=false
  # so E2E tests skip Optimize assertions (e.g. OpenSearch-only scenarios don't deploy Optimize).
  local is_optimize="true"
  local optimize_pods
  optimize_pods=$($kubectl_cmd -n "$namespace" get pods -l "app.kubernetes.io/component=optimize" --no-headers 2>/dev/null | wc -l)
  if [[ "$optimize_pods" -eq 0 ]]; then
    is_optimize="false"
    log "DEBUG: No Optimize pods found in namespace — setting IS_OPTIMIZE=false"
  fi

  # Generate base .env from template
  export TEST_INGRESS_HOST="$hostname"
  envsubst < "$test_suite_path"/.env.template > "$env_file"

  # Auth0 scenario short-circuit. The auth0-smoke Playwright project only
  # needs the Auth0 issuer + per-component client_ids; the matrix runner
  # publishes those into client-secret-for-components under auth0-info-*
  # keys at install time. Skip resolve_keycloak_setup_password /
  # resolve_identity_passwords because they `exit 1` when no Keycloak is
  # deployed.
  if [[ "$is_auth0" == "true" ]]; then
    log "DEBUG: Auth0 scenario detected — resolving auth0-info-* keys from client-secret-for-components"

    local auth0_secret="${AUTH0_SECRET_NAME:-client-secret-for-components}"
    if ! $kubectl_cmd -n "$namespace" get secret "$auth0_secret" > /dev/null 2>&1; then
      echo "Error: secret '$auth0_secret' not found in namespace '$namespace' — auth0 ensure-clients must run before the test job" >&2
      exit 13
    fi

    # Each row maps a kubernetes secret key onto an env var name.
    # Process-env values (set by the matrix runner during install) win when
    # both are present, so local invocations work without round-tripping
    # through the cluster.
    #                          secret-key                         env-var
    local mappings=(
      "auth0-info-issuer-url                      AUTH0_ISSUER_URL"
      "auth0-info-audience                        AUTH0_AUDIENCE"
      "auth0-info-identity-client-id              AUTH0_IDENTITY_CLIENT_ID"
      "auth0-info-orchestration-client-id         AUTH0_ORCHESTRATION_CLIENT_ID"
      "auth0-info-optimize-client-id              AUTH0_OPTIMIZE_CLIENT_ID"
      "auth0-info-connectors-client-id            AUTH0_CONNECTORS_CLIENT_ID"
      "auth0-info-web-modeler-client-id           AUTH0_WEB_MODELER_CLIENT_ID"
      "auth0-info-console-client-id               AUTH0_CONSOLE_CLIENT_ID"
    )

    {
      echo "PLAYWRIGHT_BASE_URL=https://$hostname"
      echo "CI=${is_ci}"
      echo "CLUSTER_NAME=integration"
      echo "IS_AUTH0=true"
      echo "IS_SMOKE=true"
      local row key envvar resolved
      for row in "${mappings[@]}"; do
        read -r key envvar <<< "$row"
        resolved="${!envvar:-$(_auth0_get_secret_key "$kubectl_cmd" "$namespace" "$auth0_secret" "$key")}"
        [[ -n "$resolved" ]] && echo "${envvar}=${resolved}"
      done
      [[ -n "${AUTH0_INITIAL_ADMIN_EMAIL:-}" ]] && echo "AUTH0_INITIAL_ADMIN_EMAIL=${AUTH0_INITIAL_ADMIN_EMAIL}"
    } >> "$env_file"
    log "DEBUG: Auth0 env file setup complete (Keycloak resolution skipped)"
    if [[ "$VERBOSE" == "true" ]]; then
      log "DEBUG: Contents of .env file:"
      cat "$env_file"
    fi
    return 0
  fi

  # Resolve credentials from cluster
  KEYCLOAK_SETUP_PASSWORD="$(resolve_keycloak_setup_password "$namespace" "$kube_context")" || exit 1
  mask_secret "$KEYCLOAK_SETUP_PASSWORD"

  resolve_identity_passwords "$namespace" "$kube_context"

  # Resolve minor version
  local minor_version_value
  if ! minor_version_value="$(resolve_minor_version_from_identity "$namespace" "$kube_context")"; then
    echo "Error: Could not determine minor version from Zeebe or Orchestration deployments." >&2
    exit 12
  fi
  keycloakUrl=$($kubectl_cmd -n "$namespace" get deployment -l app.kubernetes.io/component=identity -o jsonpath="{.items[0].metadata.annotations.keycloak-token-url}")
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

  # process the tokenUrl to get the host and protocol.
  # The URL may be a Keycloak token URL (.../auth/realms/<realm>/protocol/openid-connect/token)
  # or an external OIDC provider (e.g., Entra ID: .../oauth2/v2.0/token).
  keycloak_protocol=$(echo "$tokenUrl" | sed -n 's|^\([^:]*\)://.*|\1|p')
  keycloak_host=$(echo "$tokenUrl" | sed -n 's|^[^:]*://\([^/]*\).*|\1|p')
  keycloak_realm=$(echo "$tokenUrl" | sed -n 's|^[^:]*://[^/]*/auth/realms/\([^/]*\)/protocol/openid-connect/token$|\1|p')
  if [[ -z "$keycloak_realm" ]]; then
    keycloak_realm="camunda-platform"
  fi
  # Append runtime values to .env file
  {
    echo "KEYCLOAK_URL=$keycloak_protocol://$keycloak_host"
    echo "KEYCLOAK_REALM=${keycloak_realm}"
    echo "PLAYWRIGHT_BASE_URL=https://$hostname"
    echo "CAMUNDA_OPTIMIZE_BASE_URL=https://$hostname/optimize"
    echo "CLUSTER_ENDPOINT=http://integration-zeebe-gateway:26500"
    echo "CLUSTER_VERSION=8"
    echo "MINOR_VERSION=$minor_version_value"
    echo "DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD=$DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD"
    echo "DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD=$DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD"
    echo "DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD=$DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD"
    echo "DISTRO_QA_E2E_TESTS_KEYCLOAK_PASSWORD=$KEYCLOAK_SETUP_PASSWORD"
    echo "DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET=$DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET"
    echo "OAUTH_URL=$tokenUrl"
    echo "CI=${is_ci}"
    echo "CLUSTER_NAME=integration"
    echo "IS_OPENSEARCH=${is_opensearch}"
    echo "IS_RBA=${is_rba}"
    echo "IS_MT=${is_mt}"
    echo "IS_OPTIMIZE=${is_optimize}"
    echo "IS_SMOKE=${run_smoke_tests}"
  } >> "$env_file"

  log "DEBUG: Env file setup complete"
  if [[ "$VERBOSE" == "true" ]]; then
    log "DEBUG: Contents of .env file:"
    cat "$env_file"
  fi
}

# ------------------------------------------------------------------------------
# CLI Interface (only when run directly, not sourced)
# ------------------------------------------------------------------------------

render_env_usage() {
  cat << EOF
This script renders the .env file required for E2E tests.

Usage:
  $0 [options]

Options:
  --absolute-chart-path ABSOLUTE_CHART_PATH   The absolute path to the chart directory.
  --namespace NAMESPACE                       The namespace c8 is deployed into
  --output OUTPUT_PATH                        The path where to write the .env file
  --kube-context KUBE_CONTEXT                 The Kubernetes context to use (optional).
  --not-ci                                    Don't set the CI env var to true
  --run-smoke-tests                           Set IS_SMOKE to true
  --opensearch                                Set IS_OPENSEARCH to true
  --rba                                       Set IS_RBA to true
  --mt                                        Set IS_MT to true
  --auth0                                     Skip Keycloak resolution; forward AUTH0_* vars (Auth0 OIDC scenario)
  -v | --verbose                              Show verbose output.
  -h | --help                                 Show this help message and exit.
EOF
}

render_env_validate_args() {
  local chart_path="$1"
  local namespace="$2"
  local output="$3"
  local kube_context="${4:-}"
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

  if [[ -z "$output" ]]; then
    echo "Error: --output is required" >&2
    exit 1
  fi

  log "DEBUG: Arguments validated successfully"
}

# Only run main logic if script is executed directly (not sourced)
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  # Default values
  ABSOLUTE_CHART_PATH=""
  NAMESPACE=""
  OUTPUT_PATH=""
  KUBE_CONTEXT=""
  VERBOSE=false
  IS_CI=true
  RUN_SMOKE_TESTS=false
  IS_OPENSEARCH=false
  IS_RBA=false
  IS_MT=false
  IS_AUTH0=false

  check_env_required_cmds

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
      --output)
        OUTPUT_PATH="$2"
        shift 2
        ;;
      --kube-context)
        KUBE_CONTEXT="$2"
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
      -v | --verbose)
        VERBOSE=true
        shift
        ;;
      -h | --help)
        render_env_usage
        exit 0
        ;;
      *)
        echo "Unknown option: $1"
        render_env_usage
        exit 1
        ;;
    esac
  done

  log "DEBUG: Starting render-e2e-env.sh"
  log "DEBUG: Chart: $ABSOLUTE_CHART_PATH, Namespace: $NAMESPACE, Output: $OUTPUT_PATH, KubeContext: $KUBE_CONTEXT"

  render_env_validate_args "$ABSOLUTE_CHART_PATH" "$NAMESPACE" "$OUTPUT_PATH" "$KUBE_CONTEXT"

  TEST_SUITE_PATH="${ABSOLUTE_CHART_PATH%/}/test/e2e"
  hostname=$(get_ingress_hostname "$NAMESPACE" "$KUBE_CONTEXT")

  log "DEBUG: Hostname: $hostname"
  log "DEBUG: Test suite path: $TEST_SUITE_PATH"
  [[ "$IS_OPENSEARCH" == "true" ]] && log "IS_OPENSEARCH is set to true"
  [[ "$IS_RBA" == "true" ]] && log "IS_RBA is set to true"
  [[ "$IS_MT" == "true" ]] && log "IS_MT is set to true"
  [[ "$IS_AUTH0" == "true" ]] && log "IS_AUTH0 is set to true (Auth0 OIDC scenario)"

  render_env_file "$OUTPUT_PATH" "$TEST_SUITE_PATH" "$hostname" "$NAMESPACE" "$IS_CI" "$IS_OPENSEARCH" "$IS_RBA" "$IS_MT" "$RUN_SMOKE_TESTS" "$KUBE_CONTEXT" "$IS_AUTH0"

  log "DEBUG: Env file rendered to $OUTPUT_PATH"
fi
