#!/usr/bin/env bash
set -euo pipefail

# Script to add 'additionalProperties: false' to all object types in a JSON schema.
# This makes the schema strict by rejecting any properties not explicitly defined.

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

usage() {
    echo "Usage: $0 <input-schema.json> [output-schema.json]"
    echo ""
    echo "Arguments:"
    echo "  input-schema.json   Path to the input JSON schema file"
    echo "  output-schema.json  Optional. Path to the output file (default: <input>-strict.json)"
    exit 1
}

log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_error() {
    echo -e "${RED}✗${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Check arguments
if [ $# -lt 1 ]; then
    log_error "Missing required argument"
    usage
fi

INPUT_FILE="$1"
OUTPUT_FILE="${2:-}"

# Check if input file exists
if [ ! -f "$INPUT_FILE" ]; then
    log_error "File not found: $INPUT_FILE"
    exit 1
fi

# Default output file: add "-strict" suffix
if [ -z "$OUTPUT_FILE" ]; then
    BASENAME=$(basename "$INPUT_FILE" .json)
    DIRNAME=$(dirname "$INPUT_FILE")
    OUTPUT_FILE="$DIRNAME/${BASENAME}-strict.json"
fi

log_info "Reading schema from: $INPUT_FILE"

# Check if jq is available
if ! command -v jq &> /dev/null; then
    log_error "jq is not installed. Please install jq to use this script."
    log_info "Install with: brew install jq (macOS) or apt-get install jq (Linux)"
    exit 1
fi

# Validate input JSON
if ! jq empty "$INPUT_FILE" 2>/dev/null; then
    log_error "Invalid JSON in $INPUT_FILE"
    exit 1
fi

log_info "Processing schema..."

# Use jq to recursively add additionalProperties: false to all objects
# Walk through the entire structure and add the property where it's missing
jq 'walk(
    if type == "object" and .type == "object" and has("additionalProperties") | not then
        . + {"additionalProperties": false}
    else
        .
    end
)' "$INPUT_FILE" > "$OUTPUT_FILE"

# Validate output JSON
if ! jq empty "$OUTPUT_FILE" 2>/dev/null; then
    log_error "Failed to generate valid JSON output"
    exit 1
fi

# Count how many additionalProperties were added
ADDED_COUNT=$(jq -r '
    [.. | objects | select(.type == "object" and .additionalProperties == false)] | length
' "$OUTPUT_FILE")

log_success "Done! Strict schema created: $OUTPUT_FILE"
log_info "Added 'additionalProperties: false' to $ADDED_COUNT object(s)"
echo ""
echo "The schema now rejects any properties not explicitly defined."
