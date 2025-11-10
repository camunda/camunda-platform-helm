#!/usr/bin/env bash
set -euo pipefail

# Defaults
CHART_PATH=""
SCENARIO=""
VALUES_CONFIG="${VALUES_CONFIG:-}" # allow env fallback
LICENSE_KEY="${E2E_TESTS_LICENSE_KEY:-}"
OUTPUT_FILE=""
VERBOSE=0
USE_COLOR=1

bold=""; red=""; yellow=""; green=""; blue=""; reset=""
setup_colors() {
  if [[ "${USE_COLOR}" -eq 1 ]]; then
    # Prefer tput if available
    if command -v tput >/dev/null 2>&1; then
      bold="$(tput bold || true)"
      red="$(tput setaf 1 || true)"
      green="$(tput setaf 2 || true)"
      yellow="$(tput setaf 3 || true)"
      blue="$(tput setaf 4 || true)"
      reset="$(tput sgr0 || true)"
    else
      bold=$'\e[1m'
      red=$'\e[31m'
      green=$'\e[32m'
      yellow=$'\e[33m'
      blue=$'\e[34m'
      reset=$'\e[0m'
    fi
  fi
}

log_info()  { printf "%b[INFO]%b %s\n"  "${blue}${bold}" "${reset}" "$*"; }
log_ok()    { printf "%b[ OK ]%b %s\n"  "${green}${bold}" "${reset}" "$*"; }
log_warn()  { printf "%b[WARN]%b %s\n"  "${yellow}${bold}" "${reset}" "$*"; }
log_error() { printf "%b[ERR ]%b %s\n"  "${red}${bold}" "${reset}" "$*"; }
log_dbg()   { if [[ "${VERBOSE}" -eq 1 ]]; then printf "%b[DBG ]%b %s\n" "${bold}" "${reset}" "$*"; fi; }

usage() {
  cat <<'USAGE'
Usage: prepare-helm-values.sh [OPTIONS]

Options:
  -c, --chart-path <path>     Required. Root chart path used to resolve scenarios dir
  -s, --scenario <name>       Required. Scenario name (used in values filename)
  -j, --values-config <json>  Optional. JSON config string for env injection; "{}" or empty = skip
  -l, --license-key <key>     Optional. License key to inject; defaults to $E2E_TESTS_LICENSE_KEY
  -o, --output <file>         Optional. Output file path (defaults to scenario values file in-place)
  -v, --verbose               Enable verbose logging
      --no-color              Disable colored output
  -h, --help                  Show this help and exit
USAGE
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log_error "Required command not found: $1"
    exit 127
  fi
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -c|--chart-path) CHART_PATH="${2-}"; shift 2 ;;
      -s|--scenario) SCENARIO="${2-}"; shift 2 ;;
      -j|--values-config) VALUES_CONFIG="${2-}"; shift 2 ;;
      -l|--license-key) LICENSE_KEY="${2-}"; shift 2 ;;
      -o|--output) OUTPUT_FILE="${2-}"; shift 2 ;;
      -v|--verbose) VERBOSE=1; shift ;;
      --no-color) USE_COLOR=0; shift ;;
      -h|--help) usage; exit 0 ;;
      *) log_error "Unknown argument: $1"; usage; exit 2 ;;
    esac
  done
}

main() {
  parse_args "$@"
  setup_colors

  [[ -n "${CHART_PATH}" ]] || { log_error "--chart-path is required"; usage; exit 2; }
  [[ -n "${SCENARIO}"   ]] || { log_error "--scenario is required"; usage; exit 2; }

  require_cmd envsubst
  require_cmd jq
  require_cmd yq

  local scenarios_dir="${CHART_PATH}/test/integration/scenarios/chart-full-setup"
  local default_values_file="${scenarios_dir}/values-integration-test-ingress-${SCENARIO}.yaml"
  local values_file
  if [[ -n "${OUTPUT_FILE}" ]]; then
    values_file="${OUTPUT_FILE}"
  else
    values_file="${default_values_file}"
  fi

  log_info "Using chart path: ${CHART_PATH}"
  log_info "Scenario: ${SCENARIO}"
  log_dbg "Scenarios dir: ${scenarios_dir}"
  log_dbg "Values file: ${values_file}"

  if [[ ! -f "${values_file}" ]]; then
    log_error "Values file not found: ${values_file}"
    exit 1
  fi

  # Prepare environment variables from JSON config if provided and not "{}"
  if [[ -n "${VALUES_CONFIG}" && "${VALUES_CONFIG}" != "{}" ]]; then
    log_info "Applying key/value env from provided JSON config"
    local cfg tmp_export
    cfg="$(mktemp)"
    printf '%s' "${VALUES_CONFIG}" > "${cfg}"
    # Export each key as string
    while IFS='=' read -r k v; do
      # shellcheck disable=SC2163
      export "${k}"="${v}"
      log_dbg "Exported ${k}=${v}"
    done < <(jq -r 'to_entries[] | "\(.key)=\(.value|tostring)"' "${cfg}")
    rm -f "${cfg}"
  else
    log_warn "No values-config provided (or {}), will only perform envsubst"
  fi

  # Substitute templates
  # Determine placeholders to substitute and ensure they exist in the environment
  log_info "Scanning for placeholders in: ${values_file}"
  local vars_text=""
  local vars=()
  # Use a portable grep-based scan for placeholders
  vars_text="$(grep -oE '\$\{[A-Za-z_][A-Za-z0-9_]*\}|\$[A-Za-z_][A-Za-z0-9_]*' "${values_file}" \
    | sed -E 's/^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?$/\1/' \
    | sort -u || true)"
  if [[ -n "${vars_text}" ]]; then
    while IFS= read -r v; do
      # Only include valid POSIX env var names; ignore others to avoid shell expansion errors
      if [[ -n "${v}" && "${v}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
        vars+=("${v}")
      elif [[ -n "${v}" ]]; then
        log_warn "Ignoring non-POSIX placeholder: ${v}"
      fi
    done <<< "${vars_text}"
  fi
  log_dbg "Found placeholders: ${vars[*]:-<none>}"

  # Validate that every placeholder has a value in the environment (allow empty values)
  local missing=()
  if ((${#vars[@]} > 0)); then
    for v in "${vars[@]}"; do
      # Use printenv to test existence without indirect expansion pitfalls
      if ! printenv "${v}" >/dev/null 2>&1; then
        missing+=("${v}")
      fi
    done
  fi
  if ((${#missing[@]} > 0)); then
    log_error "Missing required environment variables for substitution:"
    for v in "${missing[@]}"; do
      printf "%b[ERR ]%b   - %s\n" "${red}${bold}" "${reset}" "${v}"
    done
    exit 3
  fi

  log_info "Running envsubst replacement into: ${values_file}"
  local tmp
  tmp="$(mktemp)"
  if ((${#vars[@]} > 0)); then
    # Restrict substitution to detected placeholders
    local varlist=""
    for v in "${vars[@]}"; do
      varlist+=" \$${v}"
    done
    if [[ -n "${varlist// }" ]]; then
      envsubst "${varlist}" < "${values_file}" > "${tmp}"
    else
      # Safety: if the list somehow became empty whitespace, fall back to no-arg envsubst
      envsubst < "${values_file}" > "${tmp}"
    fi
  else
    # No placeholders detected; still run envsubst to be consistent
    envsubst < "${values_file}" > "${tmp}"
  fi
  mv "${tmp}" "${values_file}"
  log_ok "Template substitution complete"

  # Optional license key injection
  if [[ -n "${LICENSE_KEY}" ]]; then
    log_info "Injecting license key into values"
    # Make it available to yq command via env
    export E2E_TESTS_LICENSE_KEY="${LICENSE_KEY}"
    # Mask in CI logs if running on GitHub Actions
    echo "::add-mask::${E2E_TESTS_LICENSE_KEY}" || true
    yq -i '.global.license.key = env(E2E_TESTS_LICENSE_KEY)' "${values_file}"
    log_ok "License key injected"
  else
    log_warn "No license key provided; skipping license injection"
  fi

  log_ok "Prepared values file: ${values_file}"
  # Print the final contents (preserve original behavior)
  cat "${values_file}"
}

main "$@"


