#!/usr/bin/env bash

# Inputs passed in from the calling GitHub Action
MANUAL_TRIGGER="none"
MANUAL_SCENARIO="none"
MANUAL_FLOW="none"
ACTIVE_VERSIONS=""
ALL_MODIFIED_FILES="${ALL_MODIFIED_FILES}"
TIER_FILTER=""

# Resolve repository root based on this script's location so paths work from anywhere
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CHARTS_DIR="${REPO_ROOT}/charts"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --manual-trigger)
      MANUAL_TRIGGER="$2"
      shift 2
      ;;
    --manual-scenario)
      MANUAL_SCENARIO="$2"
      shift 2
      ;;
    --manual-flow)
      MANUAL_FLOW="$2"
      shift 2
      ;;
    --active-versions)
      ACTIVE_VERSIONS="$2"
      shift 2
      ;;
    --all-modified-files)
      ALL_MODIFIED_FILES="$2"
      shift 2
      ;;
    --tier)
      TIER_FILTER="$2"
      shift 2
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

# Resolve the `deploy-camunda` binary: prefer PATH, fall back to `go run`
# against the source tree so local dev / CI without dx-tooling installed
# still works. Always pass an explicit --repo-root so cwd is irrelevant.
deploy_camunda() {
  if command -v deploy-camunda >/dev/null 2>&1; then
    deploy-camunda --repo-root "${REPO_ROOT}" "$@"
  else
    (cd "${REPO_ROOT}/scripts/deploy-camunda" && go run . --repo-root "${REPO_ROOT}" "$@")
  fi
}

write_matrix_entry() {
  local camunda_version="$1"
  local chart_dir="$2"
  echo "⭐ Generating matrix for $camunda_version and chart $chart_dir"

  local camunda_version_previous
  camunda_version_previous="$(echo "$camunda_version" | awk -F. '{printf "%d.%d", $1, $2-1}')"

  # Forward script flags to deploy-camunda matrix list. CLI applies tier and
  # permitted-flows filtering natively; remaining policy (special-case skips,
  # manual-flow override, exact-scenario match) is handled by the jq filter
  # below since CLI only supports substring scenario filter.
  local cli_flags=(matrix list --versions "$camunda_version" --format json)
  if [ -n "$TIER_FILTER" ] && [ "$TIER_FILTER" != "0" ]; then
    cli_flags+=(--tier "$TIER_FILTER")
  fi
  if [[ "${MANUAL_SCENARIO}" != "none" && "${MANUAL_SCENARIO}" != "all" && -n "${MANUAL_SCENARIO}" ]]; then
    cli_flags+=(--scenario-filter "${MANUAL_SCENARIO}")
  fi

  local permitted_flows_json='{}'
  local pf_file="${REPO_ROOT}/.github/config/permitted-flows.yaml"
  if [ -f "$pf_file" ]; then
    permitted_flows_json="$(yq -o=json '.' "$pf_file")"
  fi

  deploy_camunda "${cli_flags[@]}" \
    | jq -r \
        --arg version "$camunda_version" \
        --arg version_prev "$camunda_version_previous" \
        --arg manual_scenario "${MANUAL_SCENARIO}" \
        --arg manual_flow "${MANUAL_FLOW}" \
        --argjson permitted_flows "$permitted_flows_json" \
        -f "${SCRIPT_DIR}/generate-chart-matrix.jq" \
    >> matrix_versions.txt
  local pipe_status=("${PIPESTATUS[@]}")
  if [ "${pipe_status[0]}" -ne 0 ] || [ "${pipe_status[1]}" -ne 0 ]; then
    echo "❌ matrix generation failed for $camunda_version (deploy-camunda=${pipe_status[0]} jq=${pipe_status[1]})" >&2
    exit 1
  fi
}

echo "Checking for manual-trigger"
touch matrix_versions.txt
echo "matrix:" > matrix_versions.txt
if [[ "${MANUAL_TRIGGER}" == "all" ]]; then
  echo "Requested to build all"
  for camunda_version in ${ACTIVE_VERSIONS}; do
    chart_dir="${CHARTS_DIR}/camunda-platform-${camunda_version}"
    write_matrix_entry "$camunda_version" "$chart_dir"
  done
elif [[ "${MANUAL_TRIGGER}" != "none" && "${MANUAL_TRIGGER}" != "" ]]; then
  echo "Manual trigger detected: ${MANUAL_TRIGGER}"
  chart_dir="${CHARTS_DIR}/camunda-platform-${MANUAL_TRIGGER}"
  if [ -d "$chart_dir" ]; then
    camunda_version="${MANUAL_TRIGGER}"
    write_matrix_entry "$camunda_version" "$chart_dir"
  else
    echo "Chart directory $chart_dir does not exist. Aborting."
    exit 1
  fi
else
  echo "Setting matrix based on changed files"
  echo "Changed files:"
  printf "%s\n" ${ALL_MODIFIED_FILES}

  # Directories/patterns that trigger building all chart versions when changed
  # Format: "pattern;;exclude_pattern;;description"
  # Use empty string for exclude_pattern if no exclusion is needed
  # Note: We use ';;' as delimiter to avoid conflicts with '|' in regex patterns
  BUILD_ALL_TRIGGERS=(
    '\.github/(workflows|actions);;;;.github/workflows or .github/actions'
    '\.github/config;;\.github/config/release-please;;.github/config (excluding release-please)'
    # Anchor required: tj-actions/changed-files with dir_names:true emits the
    # bare token "scripts" so unanchored "scripts/" misses top-level helper changes.
    '(^|[[:space:]])scripts(/|$|[[:space:]]);;;;scripts/ (any helper script)'
  )

  build_all_triggered=false
  for trigger in "${BUILD_ALL_TRIGGERS[@]}"; do
    pattern="${trigger%%;;*}"
    rest="${trigger#*;;}"
    exclude_pattern="${rest%%;;*}"
    description="${rest#*;;}"
    if echo "${ALL_MODIFIED_FILES}" | grep -qE "$pattern"; then
      # Check exclusion pattern if specified
      if [ -n "$exclude_pattern" ] && echo "${ALL_MODIFIED_FILES}" | grep -qE "$exclude_pattern"; then
        # All matches are in the excluded path, skip this trigger
        # Check if there are matches outside the exclusion
        if ! echo "${ALL_MODIFIED_FILES}" | grep -E "$pattern" | grep -qvE "$exclude_pattern"; then
          continue
        fi
      fi
      echo "Changes in ${description} detected — building all chart versions"
      for camunda_version in ${ACTIVE_VERSIONS}; do
        chart_dir="${CHARTS_DIR}/camunda-platform-${camunda_version}"
        write_matrix_entry "$camunda_version" "$chart_dir"
      done
      build_all_triggered=true
      break
    fi
  done

  # If no global trigger matched, only rebuild the affected charts
  if [ "$build_all_triggered" = false ]; then
    for camunda_version in ${ACTIVE_VERSIONS}; do
      if [[ $(echo "${ALL_MODIFIED_FILES}" | grep "charts/camunda-platform-${camunda_version}") ]]; then
        chart_dir="${CHARTS_DIR}/camunda-platform-${camunda_version}"
        write_matrix_entry "$camunda_version" "$chart_dir"
      fi
    done
  fi
fi
cat matrix_versions.txt

if [ "$(cat matrix_versions.txt)" = "matrix:" ]; then
  echo "No matching chart changes detected; emitting empty matrix and skipping downstream jobs."
  echo 'matrix={"include":[]}' | tee -a "${GITHUB_OUTPUT:-/dev/null}"
  exit 0
fi

matrix="$(cat matrix_versions.txt | yq -o=json '.matrix' | jq -c '{ "include": . }' \
  | jq -c 'walk(if type == "number" or type == "boolean" then tostring else . end)')"
echo "matrix=${matrix}" | tee -a "${GITHUB_OUTPUT:-/dev/null}"
