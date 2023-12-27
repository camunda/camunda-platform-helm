#!/bin/bash
set -euo pipefail

INPUT_FILE="${INPUT_FILE:-charts/camunda-platform/VERSION-MATRIX.md}"
TMP_FILE="$(mktemp)"
OUTPUT_HEADER=${OUTPUT_HEADER:-true}

get_chart_version () {
    awk '/^version: / {print $2}' charts/camunda-platform/Chart.yaml
}

get_chart_images () {
    helm template camunda charts/camunda-platform/ 2> /dev/null | grep "image:" | tr -d "\"'" |
    awk -F ": " '{gsub("docker.io/", "", $2); printf "- %s\n", $2}' | sort | uniq
}

get_version_matrix_header () {
[[ ${OUTPUT_HEADER} == false ]] && exit
cat << EOF
<!-- _VERSION_MATRIX_PLACEHOLDER_ -->

## Chart version $(get_chart_version)
EOF
}

get_version_matrix () {
cat << EOF
$(get_version_matrix_header)

Camunda images:

$(get_chart_images | grep "camunda")

Non-Camunda images:

$(get_chart_images | grep -v "camunda")

EOF
}

awk -v version_matrix="$(get_version_matrix)" \
    '{gsub(/<!-- _VERSION_MATRIX_PLACEHOLDER_ -->/, version_matrix, $0)}1' \
    "${INPUT_FILE}" > "${TMP_FILE}"

cp -a "${TMP_FILE}" "${INPUT_FILE}"
