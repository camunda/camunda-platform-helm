#!/bin/bash

# NOTE:
# The whole script will be removed once we have RenovateBot setup in place.

# NOTE:
# The "yq" tool is not unsable because of this bug:
# https://github.com/mikefarah/yq/issues/515
# When the bug is fixed we can use:
# 	yq -i '.global.image.tag = env(IMAGE_TAG_GLOBAL)' charts/camunda-platform/values.yaml

declare -A components_tags
declare -A components_values

components_tags+=(
  [GLOBAL]=global.image.tag
  [OPTIMIZE]=optimize.image.tag
  [WEBMODELER]=webModeler.image.tag
  [CONNECTORS]=connectors.image.tag
)

components_values+=(
  [GLOBAL]=${IMAGE_TAG_GLOBAL}
  [OPTIMIZE]=${IMAGE_TAG_OPTIMIZE}
  [WEBMODELER]=${IMAGE_TAG_WEBMODELER}
  [CONNECTORS]=${IMAGE_TAG_CONNECTORS}
)

# Ensure that there is at least 1 env var exported to avoid silent errors.
if [[ $(echo "${components_values[@]}" | tr -d '[:space:]') == "" ]]; then
    echo '[ERROR] One of the following vars should be defined:'
    printf -- '- IMAGE_TAG_%s\n' "${!components_values[@]}"
    exit 1
fi

# Update values.yaml file with the exported vars.
for key in "${!components_tags[@]}"; do
    tag="${components_tags[$key]}"
    value="${components_values[$key]}"
    if [[ -n "${value}" ]]; then
      sed -ri "s/(\s+)tag:.+# (${tag})/\1tag: ${value}  # \2/g" \
          charts/camunda-platform/values.yaml
      echo "Updated ${tag}=${value}"
    fi
done
