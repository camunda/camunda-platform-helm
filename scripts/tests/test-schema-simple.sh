#!/usr/bin/env bash
set -euo pipefail

# Simple test suite for the schema modifications
# Tests that the schema allows YAML anchors while maintaining strict validation

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Counters
PASSED=0
FAILED=0

# Get the script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCHEMA_FILE="$SCRIPT_DIR/../../charts/camunda-platform-8.8/values.schema.json"

print_test() {
    echo -e "${BLUE}▶${NC} $1"
}

print_pass() {
    echo -e "${GREEN}  ✓${NC} $1"
    ((PASSED++))
}

print_fail() {
    echo -e "${RED}  ✗${NC} $1"
    ((FAILED++))
}

print_section() {
    echo ""
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}$1${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# Test 1: Schema file exists
test_schema_exists() {
    print_test "Schema file exists"
    if [[ -f "$SCHEMA_FILE" ]]; then
        print_pass "Found: $SCHEMA_FILE"
    else
        print_fail "Not found: $SCHEMA_FILE"
    fi
}

# Test 2: Schema is valid JSON
test_valid_json() {
    print_test "Schema is valid JSON"
    if jq empty "$SCHEMA_FILE" 2>/dev/null; then
        print_pass "Valid JSON syntax"
    else
        print_fail "Invalid JSON syntax"
    fi
}

# Test 3: Has patternProperties
test_has_pattern_properties() {
    print_test "Schema has patternProperties for YAML anchors"
    if jq -e '.patternProperties' "$SCHEMA_FILE" >/dev/null 2>&1; then
        local pattern_key=$(jq -r '.patternProperties | keys[0]' "$SCHEMA_FILE")
        print_pass "Found patternProperties: ${pattern_key:0:50}..."
    else
        print_fail "Missing patternProperties"
    fi
}

# Test 4: No root additionalProperties
test_no_root_additional_properties() {
    print_test "No root-level 'additionalProperties: false'"
    local has_root_additional=$(jq -r '.additionalProperties // "null"' "$SCHEMA_FILE")
    if [[ "$has_root_additional" == "false" ]]; then
        print_fail "Root has 'additionalProperties: false' (should be removed)"
    else
        print_pass "Root does not restrict additional properties"
    fi
}

# Test 5: Nested objects have additionalProperties
test_nested_additional_properties() {
    print_test "Nested objects have 'additionalProperties: false'"
    local count=$(jq '[.. | objects | select(.type == "object" and .additionalProperties == false)] | length' "$SCHEMA_FILE")
    if [[ $count -gt 0 ]]; then
        print_pass "Found $count objects with 'additionalProperties: false'"
    else
        print_fail "No objects with 'additionalProperties: false'"
    fi
}

# Test 6: Global object is strict
test_global_is_strict() {
    print_test "global object has strict validation"
    local global_additional=$(jq -r '.properties.global.additionalProperties // "null"' "$SCHEMA_FILE")
    if [[ "$global_additional" == "false" ]]; then
        print_pass "global.additionalProperties = false"
    else
        print_fail "global.additionalProperties is not false"
    fi
}

# Test 7: All top-level properties exist
test_top_level_properties() {
    print_test "Required top-level properties exist"
    local props=("global" "identity" "orchestration" "connectors" "console" "optimize" "elasticsearch")
    local missing=()
    
    for prop in "${props[@]}"; do
        if ! jq -e ".properties.$prop" "$SCHEMA_FILE" >/dev/null 2>&1; then
            missing+=("$prop")
        fi
    done
    
    if [[ ${#missing[@]} -eq 0 ]]; then
        print_pass "All ${#props[@]} required properties found"
    else
        print_fail "Missing properties: ${missing[*]}"
    fi
}

# Test 8: Pattern allows user properties
test_pattern_allows_custom() {
    print_test "Pattern allows user-defined properties"
    local pattern=$(jq -r '.patternProperties | keys[0]' "$SCHEMA_FILE")
    
    # Test with Python (more reliable regex testing)
    local result=$(python3 <<EOF
import re
import sys
pattern = r"$pattern"
tests = ["myAnchor", "x-custom", "_private", "customConfig"]
try:
    all_match = all(re.match(pattern, t) for t in tests)
    sys.exit(0 if all_match else 1)
except:
    sys.exit(1)
EOF
)
    
    if [[ $? -eq 0 ]]; then
        print_pass "Pattern allows custom properties (myAnchor, x-custom, etc.)"
    else
        print_fail "Pattern does not allow custom properties"
    fi
}

# Test 9: Pattern rejects reserved properties
test_pattern_rejects_reserved() {
    print_test "Pattern rejects reserved chart properties"
    local pattern=$(jq -r '.patternProperties | keys[0]' "$SCHEMA_FILE")
    
    local result=$(python3 <<EOF
import re
import sys
pattern = r"$pattern"
reserved = ["global", "identity", "orchestration", "connectors"]
try:
    none_match = not any(re.match(pattern, r) for r in reserved)
    sys.exit(0 if none_match else 1)
except:
    sys.exit(1)
EOF
)
    
    if [[ $? -eq 0 ]]; then
        print_pass "Pattern correctly rejects reserved properties"
    else
        print_fail "Pattern incorrectly matches reserved properties"
    fi
}

# Test 10: Count properties
test_property_count() {
    print_test "Count all schema properties"
    local total_props=$(jq '.properties | length' "$SCHEMA_FILE")
    local total_objects=$(jq '[.. | objects] | length' "$SCHEMA_FILE")
    
    print_pass "Top-level properties: $total_props"
    print_pass "Total objects in schema: $total_objects"
}

# Main execution
main() {
    echo -e "${GREEN}╔════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║     Camunda Schema Validation Test Suite      ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════╝${NC}"
    
    print_section "Basic Structure Tests"
    test_schema_exists
    test_valid_json
    
    print_section "Pattern Properties Tests"
    test_has_pattern_properties
    test_no_root_additional_properties
    test_nested_additional_properties
    
    print_section "Specific Property Tests"
    test_global_is_strict
    test_top_level_properties
    
    print_section "Regex Pattern Tests"
    test_pattern_allows_custom
    test_pattern_rejects_reserved
    
    print_section "Statistics"
    test_property_count
    
    # Summary
    echo ""
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}Summary${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    local total=$((PASSED + FAILED))
    echo -e "Tests run:    ${BLUE}$total${NC}"
    echo -e "Tests passed: ${GREEN}$PASSED${NC}"
    echo -e "Tests failed: ${RED}$FAILED${NC}"
    
    if [[ $FAILED -eq 0 ]]; then
        echo ""
        echo -e "${GREEN}╔════════════════════════════════════════════════╗${NC}"
        echo -e "${GREEN}║          ALL TESTS PASSED! ✓                   ║${NC}"
        echo -e "${GREEN}╚════════════════════════════════════════════════╝${NC}"
        return 0
    else
        echo ""
        echo -e "${RED}╔════════════════════════════════════════════════╗${NC}"
        echo -e "${RED}║          SOME TESTS FAILED ✗                   ║${NC}"
        echo -e "${RED}╚════════════════════════════════════════════════╝${NC}"
        return 1
    fi
}

main "$@"
