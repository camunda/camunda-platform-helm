#!/bin/bash
set -euo pipefail

# Validate that image tags referenced in values-enterprise.yaml exist in registry.camunda.cloud.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

OVERALL_STATUS=0

check_required_tools() {
  for tool in yq jq docker; do
    if ! command -v "${tool}" >/dev/null 2>&1; then
      echo -e "${RED}ERROR: Required tool '${tool}' is not installed${NC}"
      exit 1
    fi
  done
}

extract_images() {
  local values_file="$1"

  yq eval -r '.. | select(type == "!!map" and has("image")) | .image | select(type == "!!map" and has("registry") and has("repository") and has("tag")) | "\(.registry)/\(.repository):\(.tag)"' "${values_file}" |
    sed '/^null\//d;/^null$/d;/^$/d' |
    sort -u
}

manifest_exists() {
  local image="$1"

  for attempt in {1..3}; do
    if docker manifest inspect "${image}" >/dev/null 2>&1; then
      return 0
    fi

    if [[ ${attempt} -lt 3 ]]; then
      sleep 2
    fi
  done

  return 1
}

validate_chart() {
  local chart_dir="$1"
  local values_file="${REPO_ROOT}/charts/${chart_dir}/values-enterprise.yaml"

  if [[ ! -f "${values_file}" ]]; then
    echo -e "${YELLOW}Skipping ${chart_dir}: no values-enterprise.yaml${NC}"
    return 0
  fi

  echo ""
  echo -e "${YELLOW}Validating ${chart_dir}${NC}"

  local chart_status=0

  local images_output
  images_output="$(extract_images "${values_file}")"

  while IFS= read -r image; do
    [[ -z "${image}" ]] && continue

    if manifest_exists "${image}"; then
      echo -e "  ${GREEN}✓${NC} ${image}"
    else
      echo -e "  ${RED}✗${NC} ${image} (not resolvable)"
      chart_status=1
    fi
  done <<< "${images_output}"

  if [[ ${chart_status} -eq 0 ]]; then
    echo -e "${GREEN}✓ ${chart_dir} validation passed${NC}"
  else
    echo -e "${RED}✗ ${chart_dir} validation failed${NC}"
    OVERALL_STATUS=1
  fi
}

main() {
  echo "=========================================="
  echo "Validating values-enterprise image tags"
  echo "=========================================="

  check_required_tools

  local charts=()
  if [[ $# -gt 0 ]]; then
    for version in "$@"; do
      charts+=("camunda-platform-${version}")
    done
  else
    while IFS= read -r chart_dir; do
      charts+=("$(basename "${chart_dir}")")
    done < <(find "${REPO_ROOT}/charts" -maxdepth 1 -type d -name 'camunda-platform-8.*' | sort)
  fi

  for chart in "${charts[@]}"; do
    validate_chart "${chart}"
  done

  echo ""
  echo "=========================================="
  if [[ ${OVERALL_STATUS} -eq 0 ]]; then
    echo -e "${GREEN}✓ All validations passed${NC}"
  else
    echo -e "${RED}✗ Some validations failed${NC}"
  fi
  echo "=========================================="

  exit ${OVERALL_STATUS}
}

main "$@"
