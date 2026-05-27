#!/bin/bash
#
# Post-deploy hook: Create zeebe-record aliases in Elasticsearch for Optimize.
#
# In Camunda 8.10, the Zeebe ES exporter creates time-based indices like
# zeebe-record_process_8.10.0_2026-05-27 but does NOT create the aliases
# (e.g., zeebe-record-process) that Optimize uses to discover and import
# Zeebe data. This script:
#   1. Waits for at least one zeebe-record index to appear
#   2. Creates aliases (zeebe-record-{type}) for all existing indices
#   3. Patches index templates to include alias definitions so future
#      time-based indices also get aliases automatically
#
# Environment:
#   TEST_NAMESPACE — target K8s namespace (set by lifecycle hook runner)
#

set -euo pipefail

: "${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"

ES_POD="elasticsearch-master-0"
ES_URL="http://localhost:9200"
MAX_WAIT=120  # seconds to wait for zeebe-record indices to appear
POLL_INTERVAL=5

echo "[post-deploy-zeebe-aliases] Waiting for zeebe-record indices in namespace ${TEST_NAMESPACE}..."

elapsed=0
while true; do
    indices=$(kubectl exec -n "${TEST_NAMESPACE}" "${ES_POD}" -c elasticsearch -- \
        curl -s "${ES_URL}/_cat/indices?h=index" 2>/dev/null | grep "^zeebe-record_" || true)
    if [[ -n "${indices}" ]]; then
        break
    fi
    if (( elapsed >= MAX_WAIT )); then
        echo "[post-deploy-zeebe-aliases] ERROR: No zeebe-record indices appeared after ${MAX_WAIT}s"
        exit 1
    fi
    sleep "${POLL_INTERVAL}"
    elapsed=$((elapsed + POLL_INTERVAL))
done

echo "[post-deploy-zeebe-aliases] Found zeebe-record indices after ${elapsed}s"

# Extract unique record types from index names.
# Index pattern: zeebe-record_{type}_{version}_{date}
# We need alias: zeebe-record-{type}
types=$(echo "${indices}" | sed -n 's/^zeebe-record_\([^_]*\)_.*/\1/p' | sort -u)

# Build bulk alias actions
actions=""
for type in ${types}; do
    pattern="zeebe-record_${type}_*"
    alias="zeebe-record-${type}"
    # Use wildcard index pattern to cover current and future indices
    if [[ -n "${actions}" ]]; then
        actions="${actions},"
    fi
    actions="${actions}{\"add\":{\"index\":\"${pattern}\",\"alias\":\"${alias}\"}}"
done

echo "[post-deploy-zeebe-aliases] Creating aliases for types: ${types}"

result=$(kubectl exec -n "${TEST_NAMESPACE}" "${ES_POD}" -c elasticsearch -- \
    curl -s -X POST "${ES_URL}/_aliases" \
    -H "Content-Type: application/json" \
    -d "{\"actions\":[${actions}]}" 2>&1)

if echo "${result}" | grep -q '"acknowledged":true'; then
    echo "[post-deploy-zeebe-aliases] Aliases created successfully"
else
    echo "[post-deploy-zeebe-aliases] WARNING: Alias creation response: ${result}"
    # Don't fail — some indices might not exist yet and that's OK
fi

# Also patch index templates so future time-based indices get aliases automatically.
templates=$(kubectl exec -n "${TEST_NAMESPACE}" "${ES_POD}" -c elasticsearch -- \
    curl -s "${ES_URL}/_index_template?pretty" 2>/dev/null | \
    grep '"name"' | grep 'zeebe-record_' | \
    sed 's/.*"name" *: *"\([^"]*\)".*/\1/' || true)

for template in ${templates}; do
    # Extract type from template name: zeebe-record_{type}_{version}
    type=$(echo "${template}" | sed -n 's/^zeebe-record_\([^_]*\)_.*/\1/p')
    if [[ -z "${type}" ]]; then
        continue
    fi
    alias="zeebe-record-${type}"

    # Get existing template, inject alias, and PUT it back
    existing=$(kubectl exec -n "${TEST_NAMESPACE}" "${ES_POD}" -c elasticsearch -- \
        curl -s "${ES_URL}/_index_template/${template}" 2>/dev/null)

    # Check if template already has aliases configured
    if echo "${existing}" | grep -q "\"aliases\".*\"${alias}\""; then
        continue
    fi

    # Use jq to patch the template with alias (if jq available in ES pod)
    patched=$(kubectl exec -n "${TEST_NAMESPACE}" "${ES_POD}" -c elasticsearch -- \
        sh -c "curl -s '${ES_URL}/_index_template/${template}' | \
        python3 -c \"
import sys, json
data = json.load(sys.stdin)
tmpl = data['index_templates'][0]['index_template']
if 'template' not in tmpl:
    tmpl['template'] = {}
tmpl['template']['aliases'] = {'${alias}': {}}
# Remove meta fields that can't be PUT back
for k in ['version', 'data_stream']:
    tmpl.pop(k, None)
print(json.dumps(tmpl))
\" 2>/dev/null" 2>/dev/null) || continue

    if [[ -n "${patched}" && "${patched}" != "null" ]]; then
        kubectl exec -n "${TEST_NAMESPACE}" "${ES_POD}" -c elasticsearch -- \
            curl -s -X PUT "${ES_URL}/_index_template/${template}" \
            -H "Content-Type: application/json" \
            -d "${patched}" >/dev/null 2>&1 || true
    fi
done

echo "[post-deploy-zeebe-aliases] Done"
