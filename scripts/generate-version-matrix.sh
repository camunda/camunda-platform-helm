#!/bin/bash
# TODO: Use gomplate when it supports JQ filter expressions.
# https://docs.gomplate.ca/functions/coll/

set -euo pipefail

CHART_VERSION="${CHART_VERSION:-latest}"
CHART_SOURCE="${CHART_SOURCE:-charts/camunda-platform}"


print_version_header () {
    echo "${OUTPUT_VERSION_HEADER:-"## Chart version $CHART_VERSION"}"
}

get_chart_images () {
    helm template --skip-tests camunda "${CHART_SOURCE}" --version "${CHART_VERSION}" \
      --set "webModeler.enabled=true,webModeler.restapi.mail.fromAddress=dummy" 2> /dev/null |
    tr -d "\"'" | awk '/image:/{gsub(/^(camunda|bitnami)/, "docker.io/&", $2); printf "- %s\n", $2}' |
    sort | uniq
}

print_version_matrix_single () {
  major=`echo $CHART_VERSION | cut -d. -f1`
  minor=`echo $CHART_VERSION | cut -d. -f2`
  patch=`echo $CHART_VERSION | cut -d. -f3`
  if (( $major == 8 && $minor == 2 && $patch >= 23 )); then
    OPTIMIZE_VERSION_NOTE="\n* The optimize image version is 8.2.x instead of 3.10.x because of a recent versioning change in optimize. In versions greater than 8.3.0, optimize's image will be identical to the other application components\n"
  elif (( $major == 8 && $minor == 3 && $patch >= 2)); then
    OPTIMIZE_VERSION_NOTE="\n* The optimize image version is uniform with the other components.\n"
  elif (( $major == 9 )); then
    OPTIMIZE_VERSION_NOTE="\n* The optimize image version is uniform with the other components.\n"
  else
    OPTIMIZE_VERSION_NOTE=""
  fi
cat << EOF
$(print_version_header)

Camunda images:

$(get_chart_images | grep "camunda")

Non-Camunda images:

$(get_chart_images | grep -v "camunda")
$( printf "$OPTIMIZE_VERSION_NOTE" )

EOF
}

print_version_matrix_all () {
    CHART_SOURCE="camunda/camunda-platform"

    echo '<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->'
    echo -e "# Camunda 8 Helm Chart Version Matrix\n"
    git tag -l camunda-platform-{8..100}* | sed 's/camunda-platform-//' | sort -Vr | while read CHART_VERSION; do
        print_version_matrix_single
    done
}

print_help () {
    echo "[ERROR] No option provided, exit."
    exit 1
}

# Parse script input args.
while test -n "${1}"; do
    case "${1}" in
        --single)
          print_version_matrix_single
          ;;
        --all)
          print_version_matrix_all
          ;;
        *)
          print_help
          ;;
    esac
    shift

    # Handling exit if no more script args to avoid "unbound variable" error.
    test -z "${1:-}" && exit 0
done
