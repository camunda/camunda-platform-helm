#!/usr/bin/env bash
set -euo pipefail

ES_URL="${ES_URL:?ES_URL must be set (e.g. http://localhost:9200)}"
ES_USERNAME="${ES_USERNAME:-}"
ES_PASSWORD="${ES_PASSWORD:-}"
if [[ -n "$ES_USERNAME" && -n "$ES_PASSWORD" && -z "$ES_AUTH" ]]; then
  ES_AUTH="-u ${ES_USERNAME}:${ES_PASSWORD}"
fi
ES_AUTH="${ES_AUTH:-}"

# Active job IDs from Kubernetes namespaces labeled with github-run-id.
active_ids=$(kubectl get ns \
  -o custom-columns=:metadata.labels.github-job-id \
  -l 'github-run-id' --no-headers | sort -u)

# All ES indexes, excluding system indexes (those starting with '.').
all_indexes=$(curl ${ES_AUTH} -s "${ES_URL}/_cat/indices?h=index" | tr -d ' ' | grep -v '^\.')

# Print indexes whose job-id prefix does not match any active job ID.
while IFS= read -r index; do
  [[ -z "$index" ]] && continue
  prefix="${index%%-*}"
  if ! grep -qxF "$prefix" <<< "$active_ids"; then
    echo "$index"
  fi
done <<< "$all_indexes"
