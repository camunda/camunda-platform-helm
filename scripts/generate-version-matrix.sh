#!/bin/bash
# TODO: Use gomplate when it supports JQ filter expressions.
# https://docs.gomplate.ca/functions/coll/

set -euo pipefail

CHART_NAME="${CHART_NAME:-camunda-platform}"
CHART_VERSION="${CHART_VERSION:-latest}"
CHART_SOURCE="${CHART_SOURCE:-charts/$CHART_NAME}"

# Those are deleted charts and shouldn't be shown in the matrix.
CHART_EXCLUDED_VERSIONS="10.0.0 10.0.1"

print_version_header () {
    echo "${OUTPUT_VERSION_HEADER:-"## Chart version $CHART_VERSION"}"
}

# Update Helm and Git repos to get latest versions.
init_updates () {
    helm repo update
    # TODO: Limit the tags to supported versions only.
    git fetch origin tag "${CHART_NAME}-*"
}

get_chart_images () {
    helm template --skip-tests camunda "${CHART_SOURCE}" --version "${CHART_VERSION}" \
      --set "webModeler.enabled=true,webModeler.restapi.mail.fromAddress=dummy" \
      --set "console.enabled=true" 2> /dev/null |
    tr -d "\"'" | awk '/image:/{gsub(/^(camunda|bitnami)/, "docker.io/&", $2); printf "- %s\n", $2}' |
    sort | uniq
}

get_camunda_app_version () {
    camunda_app_version="$(helm show chart ${CHART_SOURCE} --version ${CHART_VERSION} | grep -Po '(?<=appVersion: )\d+\.\d+')"
    echo "[${camunda_app_version}](https://github.com/camunda/camunda-platform/releases?q=tag%3A${camunda_app_version}&expanded=true)"
}

get_chart_tag_name () {
    if [ "${CHART_VERSION}" == "latest" ]; then
        echo "$(git branch --show-current)"
    else
        git fetch origin tag "${CHART_NAME}-${CHART_VERSION}" --no-tags -q
        echo "${CHART_NAME}-${CHART_VERSION}"
    fi
}

get_helm_cli_version () {
    helm_cli_version="$(git show $(get_chart_tag_name):.tool-versions 2> /dev/null | grep "helm" | cut -d " " -f2)"
    if [ $? -eq 0 ]; then
        echo "[${helm_cli_version}](https://github.com/helm/helm/releases/tag/v${helm_cli_version})"
    else
        # Before 8.1 we didn't use ".tool-versions" as a source of truth for Helm CLI version.
        echo "N/A"
    fi
}

get_helm_values_docs () {
    echo "[${CHART_VERSION}](https://artifacthub.io/packages/helm/camunda/camunda-platform/${CHART_VERSION}#parameters)"
}

print_version_matrix_single () {
cat << EOF
$(print_version_header)

Supported versions:

- Helm CLI: $(get_helm_cli_version)
- Helm values: $(get_helm_values_docs)
- Camunda applications: $(get_camunda_app_version)

Camunda images:

$(get_chart_images | grep "camunda")

Non-Camunda images:

$(get_chart_images | grep -v "camunda")

EOF
}

print_version_matrix_all () {
    CHART_SOURCE="camunda/${CHART_NAME}"

    echo '<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->'
    echo -e "# Camunda 8 Helm Chart Version Matrix\n"
    print_version_matrix_prefix "*"
}

print_version_matrix_prefix () {
    version_prefix="$1"
    # TODO: Limit the tags to supported versions only.
    git tag -l ${CHART_NAME}-${version_prefix}* | sed "s/${CHART_NAME}-//" | sort -Vr | while read CHART_VERSION; do
        $(echo "${CHART_EXCLUDED_VERSIONS}" | grep -q "${CHART_VERSION}") && continue
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
        --init)
          init_updates
          ;;
        --single)
          print_version_matrix_single
          ;;
        --prefix)
          print_version_matrix_prefix "$2"
          shift
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
