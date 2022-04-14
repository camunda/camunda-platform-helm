#!/bin/bash

set -eo pipefail

if [ -z $1 ]
then
  version=$(cat Chart.yaml | grep -m1 "version")
  echo "Expected usage: ./release.sh majorX.minorY.patchZ, current version $version"
  exit 1
fi

# Updating the Chart.yaml version will trigger the chart release to create a new helm release
newVersion=$1

# Update subchart versions
sed -i -E "s/version: 8.0.[0-9]+/version: $newVersion/g" **/**/Chart.yaml
# Update parent chart version
sed -i -E "s/version: 8.0.[0-9]+/version: $newVersion/g" Chart.yaml
