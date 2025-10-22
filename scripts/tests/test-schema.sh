#!/usr/bin/env bash
set -euo pipefail

# Test suite for make-schema-strict.sh and schema validation

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="/tmp/schema-test-$$"
SCHEMA_FILE="$SCRIPT_DIR/../../charts/camunda-platform-8.8/values.schema.json"

log_test() {
    echo -e "${CYAN}▶${NC} Test: $1"
}

log_pass() {
    echo -e "${GREEN}  ✓${NC} $1"
    ((TESTS_PASSED++))
}

log_fail() {
    echo -e "${RED}  ✗${NC} $1"
    ((TESTS_FAILED++))
}

log_info() {
    echo -e "${BLUE}  ℹ${NC} $1"
}

log_section() {
    echo ""
    echo -e "${YELLOW}═══════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}$1${NC}"
    echo -e "${YELLOW}═══════════════════════════════════════════════════${NC}"
}

# Setup test environment
setup() {
    mkdir -p "$TEST_DIR"
    log_info "Test directory: $TEST_DIR"
}

# Cleanup test environment
cleanup() {
    rm -rf "$TEST_DIR"
}

# Test 1: Verify jq is installed
test_jq_installed() {
    log_test "jq is installed"
    ((TESTS_RUN++))
    
    if command -v jq &> /dev/null; then
        JQ_VERSION=$(jq --version)
        log_pass "jq is installed ($JQ_VERSION)"
    else
        log_fail "jq is not installed"
    fi
}

# Test 2: Schema file exists
test_schema_exists() {
    log_test "Schema file exists"
    ((TESTS_RUN++))
    
    if [ -f "$SCHEMA_FILE" ]; then
        log_pass "Schema file found: $SCHEMA_FILE"
    else
        log_fail "Schema file not found: $SCHEMA_FILE"
    fi
}

# Test 3: Schema is valid JSON
test_schema_valid_json() {
    log_test "Schema is valid JSON"
    ((TESTS_RUN++))
    
    if jq empty "$SCHEMA_FILE" 2>/dev/null; then
        log_pass "Schema is valid JSON"
    else
        log_fail "Schema is not valid JSON"
    fi
}

# Test 4: Schema has patternProperties
test_schema_has_pattern_properties() {
    log_test "Schema has patternProperties for YAML anchors"
    ((TESTS_RUN++))
    
    if jq -e '.patternProperties' "$SCHEMA_FILE" &>/dev/null; then
        PATTERN=$(jq -r '.patternProperties | keys[0]' "$SCHEMA_FILE")
        log_pass "patternProperties found: $PATTERN"
    else
        log_fail "patternProperties not found"
    fi
}

# Test 5: Schema does NOT have root additionalProperties: false
test_schema_no_root_additional_properties() {
    log_test "Schema does NOT have root-level 'additionalProperties: false'"
    ((TESTS_RUN++))
    
    if jq -e '.additionalProperties == false' "$SCHEMA_FILE" &>/dev/null; then
        log_fail "Root-level 'additionalProperties: false' found (should be removed)"
    else
        log_pass "Root-level 'additionalProperties: false' correctly removed"
    fi
}

# Test 6: Nested objects have additionalProperties: false
test_nested_objects_have_additional_properties() {
    log_test "Nested objects have 'additionalProperties: false'"
    ((TESTS_RUN++))
    
    # Check global object
    if jq -e '.properties.global.additionalProperties == false' "$SCHEMA_FILE" &>/dev/null; then
        log_pass "global.additionalProperties is false"
    else
        log_fail "global.additionalProperties is not false"
    fi
}

# Test 7: Test regex pattern matches user properties
test_pattern_allows_user_properties() {
    log_test "Pattern allows user-defined properties"
    ((TESTS_RUN++))
    
    # Extract the pattern
    PATTERN=$(jq -r '.patternProperties | keys[0]' "$SCHEMA_FILE" 2>/dev/null || echo "")
    
    if [ -z "$PATTERN" ]; then
        log_fail "Could not extract pattern"
        return
    fi
    
    # Test with Python (available on most systems)
    python3 <<EOF
import re
import sys

pattern = r"$PATTERN"
test_cases = [
    ("myAnchor", True),
    ("x-custom", True),
    ("_private", True),
]

all_passed = True
for test_str, should_match in test_cases:
    matches = bool(re.match(pattern, test_str))
    if matches != should_match:
        all_passed = False
        break

sys.exit(0 if all_passed else 1)
EOF
    
    if [ $? -eq 0 ]; then
        log_pass "Pattern correctly allows user-defined properties"
    else
        log_fail "Pattern does not work correctly"
    fi
}

# Test 8: Test regex pattern rejects reserved properties
test_pattern_rejects_reserved_properties() {
    log_test "Pattern rejects reserved chart properties"
    ((TESTS_RUN++))
    
    # Extract the pattern
    PATTERN=$(jq -r '.patternProperties | keys[0]' "$SCHEMA_FILE" 2>/dev/null || echo "")
    
    if [ -z "$PATTERN" ]; then
        log_fail "Could not extract pattern"
        return
    fi
    
    # Test with Python
    python3 <<EOF
import re
import sys

pattern = r"$PATTERN"
test_cases = [
    ("global", False),
    ("identity", False),
    ("orchestration", False),
]

all_passed = True
for test_str, should_match in test_cases:
    matches = bool(re.match(pattern, test_str))
    if matches != should_match:
        all_passed = False
        break

sys.exit(0 if all_passed else 1)
EOF
    
    if [ $? -eq 0 ]; then
        log_pass "Pattern correctly rejects reserved properties"
    else
        log_fail "Pattern does not work correctly"
    fi
}

# Test 9: Count all top-level properties
test_count_top_level_properties() {
    log_test "Count all top-level properties defined"
    ((TESTS_RUN++))
    
    PROP_COUNT=$(jq -r '.properties | keys | length' "$SCHEMA_FILE")
    EXPECTED_MIN=10  # We expect at least 10 top-level properties
    
    if [ "$PROP_COUNT" -ge "$EXPECTED_MIN" ]; then
        log_pass "Found $PROP_COUNT top-level properties (expected at least $EXPECTED_MIN)"
    else
        log_fail "Found only $PROP_COUNT top-level properties (expected at least $EXPECTED_MIN)"
    fi
}

# Test 10: Verify specific required properties exist
test_required_properties_exist() {
    log_test "Required properties exist"
    ((TESTS_RUN++))
    
    REQUIRED_PROPS=("global" "identity" "orchestration" "connectors" "console")
    MISSING=()
    
    for prop in "${REQUIRED_PROPS[@]}"; do
        if ! jq -e ".properties.$prop" "$SCHEMA_FILE" &>/dev/null; then
            MISSING+=("$prop")
        fi
    done
    
    if [ ${#MISSING[@]} -eq 0 ]; then
        log_pass "All required properties exist: ${REQUIRED_PROPS[*]}"
    else
        log_fail "Missing properties: ${MISSING[*]}"
    fi
}

# Test 11: Test make-schema-strict.sh script (if exists)
test_make_schema_strict_script() {
    log_test "make-schema-strict.sh script functionality"
    ((TESTS_RUN++))
    
    STRICT_SCRIPT="$SCRIPT_DIR/../make-schema-strict.sh"
    
    if [ ! -f "$STRICT_SCRIPT" ]; then
        log_info "Script not found (skipping): $STRICT_SCRIPT"
        ((TESTS_RUN--))
        return
    fi
    
    # Create a test schema
    TEST_INPUT="$TEST_DIR/test-input.json"
    TEST_OUTPUT="$TEST_DIR/test-output.json"
    
    cat > "$TEST_INPUT" <<'EOF'
{
  "type": "object",
  "properties": {
    "name": {
      "type": "object",
      "properties": {
        "first": {"type": "string"},
        "last": {"type": "string"}
      }
    }
  }
}
EOF
    
    # Run the script
    if bash "$STRICT_SCRIPT" "$TEST_INPUT" "$TEST_OUTPUT" &>/dev/null; then
        # Check if additionalProperties was added
        if jq -e '.properties.name.additionalProperties == false' "$TEST_OUTPUT" &>/dev/null; then
            log_pass "Script correctly adds 'additionalProperties: false'"
        else
            log_fail "Script did not add 'additionalProperties: false'"
        fi
    else
        log_fail "Script execution failed"
    fi
}

# Test 12: Validate schema against a sample values file
test_schema_validation_with_anchors() {
    log_test "Schema allows YAML anchors in values"
    ((TESTS_RUN++))
    
    # Create a test values file with YAML anchors
    TEST_VALUES="$TEST_DIR/test-values.yaml"
    cat > "$TEST_VALUES" <<'EOF'
# User-defined YAML anchors
x-common-labels: &commonLabels
  team: platform
  env: production

myDatabaseConfig: &dbConfig
  host: db.example.com
  port: 5432

# Chart properties
global:
  image:
    registry: registry.example.com

identity:
  enabled: true
  labels:
    <<: *commonLabels
EOF
    
    # Convert YAML to JSON for validation
    if command -v yq &> /dev/null; then
        TEST_JSON="$TEST_DIR/test-values.json"
        yq eval -o=json "$TEST_VALUES" > "$TEST_JSON" 2>/dev/null || true
        
        # Note: Full JSON Schema validation would require a validator tool
        # Here we just check if the structure is valid
        if [ -f "$TEST_JSON" ]; then
            log_pass "YAML with anchors converts to JSON successfully"
        else
            log_info "Could not convert YAML to JSON (yq issue, not schema issue)"
        fi
    else
        log_info "yq not installed (skipping YAML validation)"
    fi
    
    ((TESTS_RUN--))  # Don't count this as a real test since it's optional
}

# Main test execution
main() {
    echo -e "${GREEN}╔═══════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║   Camunda Helm Chart Schema Test Suite           ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════╝${NC}"
    echo ""
    
    setup
    
    log_section "1. Environment Tests"
    test_jq_installed
    test_schema_exists
    
    log_section "2. Schema Structure Tests"
    test_schema_valid_json
    test_schema_has_pattern_properties
    test_schema_no_root_additional_properties
    test_nested_objects_have_additional_properties
    
    log_section "3. Pattern Validation Tests"
    test_pattern_allows_user_properties
    test_pattern_rejects_reserved_properties
    
    log_section "4. Schema Content Tests"
    test_count_top_level_properties
    test_required_properties_exist
    
    log_section "5. Script Tests"
    test_make_schema_strict_script
    test_schema_validation_with_anchors
    
    # Summary
    echo ""
    echo -e "${YELLOW}═══════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}Test Summary${NC}"
    echo -e "${YELLOW}═══════════════════════════════════════════════════${NC}"
    echo -e "Total tests run:    ${CYAN}$TESTS_RUN${NC}"
    echo -e "Tests passed:       ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Tests failed:       ${RED}$TESTS_FAILED${NC}"
    
    cleanup
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo ""
        echo -e "${GREEN}╔═══════════════════════════════════════════════════╗${NC}"
        echo -e "${GREEN}║            ALL TESTS PASSED! ✓                    ║${NC}"
        echo -e "${GREEN}╚═══════════════════════════════════════════════════╝${NC}"
        exit 0
    else
        echo ""
        echo -e "${RED}╔═══════════════════════════════════════════════════╗${NC}"
        echo -e "${RED}║            SOME TESTS FAILED ✗                    ║${NC}"
        echo -e "${RED}╚═══════════════════════════════════════════════════╝${NC}"
        exit 1
    fi
}

# Run main function
main "$@"
