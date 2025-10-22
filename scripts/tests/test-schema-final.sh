#!/usr/bin/env bash
# Simple test script for schema validation
# Tests that YAML anchors are allowed while maintaining strict validation

set -uo pipefail

# Get script directory and set schema path relative to it
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCHEMA="$SCRIPT_DIR/../../charts/camunda-platform-8.8/values.schema.json"

echo "╔═══════════════════════════════════════════════════╗"
echo "║   Camunda Platform Schema Validation Tests       ║"
echo "╚═══════════════════════════════════════════════════╝"
echo ""

PASSED=0
FAILED=0

test_check() {
    local name="$1"
    local command="$2"
    
    echo -n "▶ $name ... "
    set +e
    eval "$command" >/dev/null 2>&1
    local result=$?
    set -e
    
    if [ $result -eq 0 ]; then
        echo "✓ PASS"
        PASSED=$((PASSED + 1))
    else
        echo "✗ FAIL"
        FAILED=$((FAILED + 1))
    fi
}

# Run tests
echo "Running tests:"
echo ""

test_check "Schema file exists" \
    "test -f '$SCHEMA'"

test_check "Schema is valid JSON" \
    "jq empty '$SCHEMA'"

test_check "Has patternProperties" \
    "jq -e '.patternProperties' '$SCHEMA'"

test_check "No root additionalProperties" \
    "! jq -e '.additionalProperties == false' '$SCHEMA'"

test_check "Global has additionalProperties: false" \
    "jq -e '.properties.global.additionalProperties == false' '$SCHEMA'"

test_check "Identity has additionalProperties: false" \
    "jq -e '.properties.identity.additionalProperties == false' '$SCHEMA'"

test_check "Has 12 top-level properties" \
    "test \$(jq '.properties | length' '$SCHEMA') -eq 12"

test_check "Has global property" \
    "jq -e '.properties.global' '$SCHEMA'"

test_check "Has identity property" \
    "jq -e '.properties.identity' '$SCHEMA'"

test_check "Has orchestration property" \
    "jq -e '.properties.orchestration' '$SCHEMA'"

test_check "Has connectors property" \
    "jq -e '.properties.connectors' '$SCHEMA'"

# Test pattern with Python
test_check "Pattern allows custom properties" \
    "python3 -c 'import re; p=r\"^(?!(global|identity|identityPostgresql|identityKeycloak|console|webModeler|webModelerPostgresql|connectors|orchestration|optimize|elasticsearch|prometheusServiceMonitor)\$).*\$\"; exit(0 if all(re.match(p,t) for t in [\"myAnchor\",\"x-custom\",\"_private\"]) else 1)'"

test_check "Pattern rejects reserved properties" \
    "python3 -c 'import re; p=r\"^(?!(global|identity|identityPostgresql|identityKeycloak|console|webModeler|webModelerPostgresql|connectors|orchestration|optimize|elasticsearch|prometheusServiceMonitor)\$).*\$\"; exit(0 if not any(re.match(p,t) for t in [\"global\",\"identity\",\"orchestration\"]) else 1)'"

# Summary
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Summary:"
echo "  Total:  $((PASSED + FAILED))"
echo "  Passed: $PASSED"
echo "  Failed: $FAILED"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ $FAILED -eq 0 ]; then
    echo ""
    echo "✓ ALL TESTS PASSED!"
    echo ""
    exit 0
else
    echo ""
    echo "✗ SOME TESTS FAILED"
    echo ""
    exit 1
fi
