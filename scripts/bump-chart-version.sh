#!/bin/bash
set -euo pipefail

print_help () {
cat << EOF
Usage:
    $0 [chart-name]

Details:
    A simple script to bump the chart version.
    Updating the Chart.yaml version will trigger Helm release with the new version.

Notes:
    Default value for 'chart-name' is 'camunda-platform'.
EOF
}

if [ "${1:-''}" == '-h' ]; then
  print_help
  exit 1
fi

# Chart name is hard-coded since we only have 1 main chart,
# but it could be customized in case we have more in the future.
chart_name="camunda-platform"

# Generate new version based on the old one.
chart_version_old=$(grep -Po "(?<=^version: ).+" charts/${chart_name}/Chart.yaml)
chart_version_new=$(echo "${chart_version_old}" | awk -F '.' -v OFS='.' '{$NF += 1; print}')

# Update parent chart version
sed -i "s/version: ${chart_version_old}/version: ${chart_version_new}/g" charts/${chart_name}/Chart.yaml

# Update subcharts version.
sed -i "s/^version: ${chart_version_old}/version: ${chart_version_new}/g" charts/${chart_name}/charts/*/Chart.yaml

# Print the changes.
echo "The chart '${chart_name}' version has been bumped from '${chart_version_old}' to '${chart_version_new}'."
