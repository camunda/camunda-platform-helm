#!/bin/bash

# Note:
# The "yq" tool is not unsable because of this bug:
# https://github.com/mikefarah/yq/issues/515
# When the bug is fixed we can use:
# 	yq -i '.global.image.tag = env(GLOBAL_IMAGE_TAG)' charts/camunda-platform/values.yaml
# 	yq -i '.optimize.image.tag = env(OPTIMIZE_IMAGE_TAG)' charts/camunda-platform/values.yaml

if [[ -z "${GLOBAL_IMAGE_TAG}" && -z "${OPTIMIZE_IMAGE_TAG}" ]]; then \
    echo '[ERROR] One or both of the following vars should be defined:'
		echo -e "  - GLOBAL_IMAGE_TAG\n  - OPTIMIZE_IMAGE_TAG"
    exit 1
fi

if [[ -n "${GLOBAL_IMAGE_TAG}" ]]; then
    sed -ri "s/(\s+)tag:.+# (global.image.tag)/\1tag: ${GLOBAL_IMAGE_TAG}  # \2/g" \
        charts/camunda-platform/values.yaml
    echo "Updated global.image.tag=${GLOBAL_IMAGE_TAG}"
fi

if [[ -n "${OPTIMIZE_IMAGE_TAG}" ]]; then
    sed -ri "s/(\s+)tag:.+# (optimize.image.tag)/\1tag: ${OPTIMIZE_IMAGE_TAG}  # \2/g" \
        charts/camunda-platform/values.yaml
    echo "Updated optimize.image.tag=${OPTIMIZE_IMAGE_TAG}"
fi
