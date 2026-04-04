#!/usr/bin/env bash

# helm-schema-make-strict.sh
#
# Post-processes Helm values.schema.json to add "additionalProperties": false
# to all non-root objects with "properties". This enforces strict validation
# and prevents silent configuration errors from typos or invalid keys.
#
# Usage: ./helm-schema-make-strict.sh <schema-file>
#
# Example:
#   ./helm-schema-make-strict.sh charts/camunda-platform-8.10/values.schema.json
#
# The script modifies the file in place.

set -euo pipefail

if [ $# -ne 1 ]; then
    echo "Usage: $0 <schema-file>" >&2
    exit 1
fi

SCHEMA_FILE="$1"

if [ ! -f "${SCHEMA_FILE}" ]; then
    echo "Error: Schema file not found: ${SCHEMA_FILE}" >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JQ_FILTER="${SCRIPT_DIR}/helm-schema-make-strict.jq"

if [ ! -f "${JQ_FILTER}" ]; then
    echo "Error: jq filter not found: ${JQ_FILTER}" >&2
    exit 1
fi

# Create a temporary file
TEMP_FILE="${SCHEMA_FILE}.tmp"

# Apply the jq filter
jq --indent 4 -f "${JQ_FILTER}" "${SCHEMA_FILE}" > "${TEMP_FILE}"

# Replace the original file
mv "${TEMP_FILE}" "${SCHEMA_FILE}"

echo "Successfully made schema strict: ${SCHEMA_FILE}"
