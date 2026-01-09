#!/usr/bin/env bash
# Test the CI Runner Docker images locally
# Usage: ./scripts/test-ci-runner-local.sh [--ci-runner | --playwright-runner | --both]
#
# This script tests the Docker images with the same setup as GitHub Actions

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Image configuration
REGISTRY="registry.camunda.cloud"
CI_RUNNER_IMAGE="${REGISTRY}/team-distribution/ci-runner:latest"
PLAYWRIGHT_RUNNER_IMAGE="${REGISTRY}/team-distribution/playwright-runner:latest"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Parse arguments
TEST_CI=false
TEST_PLAYWRIGHT=false
USE_LOCAL=false

for arg in "$@"; do
    case "$arg" in
        --ci-runner) TEST_CI=true ;;
        --playwright-runner) TEST_PLAYWRIGHT=true ;;
        --both) TEST_CI=true; TEST_PLAYWRIGHT=true ;;
        --local) USE_LOCAL=true ;;
        --help|-h)
            echo "Usage: $0 [--ci-runner | --playwright-runner | --both] [--local]"
            echo ""
            echo "Options:"
            echo "  --ci-runner         Test the CI runner image"
            echo "  --playwright-runner Test the Playwright runner image"
            echo "  --both              Test both images"
            echo "  --local             Use locally built images (ci-runner-test, playwright-runner-test)"
            echo ""
            echo "If no option specified, defaults to --ci-runner"
            exit 0
            ;;
    esac
done

# Default to CI runner if nothing specified
if [[ "$TEST_CI" == "false" && "$TEST_PLAYWRIGHT" == "false" ]]; then
    TEST_CI=true
fi

# Use local image names if --local specified
if [[ "$USE_LOCAL" == "true" ]]; then
    CI_RUNNER_IMAGE="ci-runner-test"
    PLAYWRIGHT_RUNNER_IMAGE="playwright-runner-test"
    echo -e "${YELLOW}Using locally built images${NC}"
fi

run_test() {
    local test_name="$1"
    local command="$2"

    echo -n "  Testing $test_name... "
    if eval "$command" &>/dev/null; then
        echo -e "${GREEN}✓${NC}"
        return 0
    else
        echo -e "${RED}✗${NC}"
        return 1
    fi
}

test_ci_runner() {
    local image="$1"
    echo ""
    echo "=========================================="
    echo "Testing CI Runner Image"
    echo "=========================================="
    echo "Image: $image"
    echo ""

    # Check if image exists
    if ! docker image inspect "$image" &>/dev/null; then
        echo -e "${YELLOW}Image not found locally, attempting to pull...${NC}"
        if ! docker pull "$image"; then
            echo -e "${RED}Failed to pull image. You may need to login:${NC}"
            echo "  docker login ${REGISTRY}"
            return 1
        fi
    fi

    echo "Running tool verification tests..."

    local failed=0

    run_test "jq" "docker run --rm --user root $image bash -c 'jq --version'" || ((failed++))
    run_test "yq" "docker run --rm --user root $image bash -c 'yq --version'" || ((failed++))
    run_test "helm" "docker run --rm --user root $image bash -c 'helm version --short'" || ((failed++))
    run_test "kubectl" "docker run --rm --user root $image bash -c 'kubectl version --client'" || ((failed++))
    run_test "task" "docker run --rm --user root $image bash -c 'task --version'" || ((failed++))
    run_test "go" "docker run --rm --user root $image bash -c 'go version'" || ((failed++))
    run_test "kustomize" "docker run --rm --user root $image bash -c 'kustomize version'" || ((failed++))
    run_test "kind" "docker run --rm --user root $image bash -c 'kind version'" || ((failed++))
    run_test "oras" "docker run --rm --user root $image bash -c 'oras version'" || ((failed++))
    run_test "ct" "docker run --rm --user root $image bash -c 'ct version'" || ((failed++))
    run_test "cr" "docker run --rm --user root $image bash -c 'cr version'" || ((failed++))
    run_test "gomplate" "docker run --rm --user root $image bash -c 'gomplate --version'" || ((failed++))
    run_test "bats" "docker run --rm --user root $image bash -c 'bats --version'" || ((failed++))
    run_test "yamllint" "docker run --rm --user root $image bash -c 'yamllint --version'" || ((failed++))
    run_test "gcloud" "docker run --rm --user root $image bash -c 'gcloud version'" || ((failed++))
    run_test "aws" "docker run --rm --user root $image bash -c 'aws --version'" || ((failed++))
    run_test "envsubst" "docker run --rm --user root $image bash -c 'envsubst --version'" || ((failed++))

    echo ""
    echo "Testing workflow-like command..."
    if docker run --rm --user root \
        -e GITHUB_CONTEXT='{"extra-values": "test-value", "flow": "install"}' \
        "$image" \
        bash -c 'echo "Workflow Inputs:" && echo "$GITHUB_CONTEXT" | jq .' ; then
        echo -e "${GREEN}✓ Workflow simulation passed${NC}"
    else
        echo -e "${RED}✗ Workflow simulation failed${NC}"
        ((failed++))
    fi

    echo ""
    if [[ $failed -eq 0 ]]; then
        echo -e "${GREEN}All CI Runner tests passed!${NC}"
    else
        echo -e "${RED}$failed test(s) failed${NC}"
        return 1
    fi
}

test_playwright_runner() {
    local image="$1"
    echo ""
    echo "=========================================="
    echo "Testing Playwright Runner Image"
    echo "=========================================="
    echo "Image: $image"
    echo ""

    # Check if image exists
    if ! docker image inspect "$image" &>/dev/null; then
        echo -e "${YELLOW}Image not found locally, attempting to pull...${NC}"
        if ! docker pull "$image"; then
            echo -e "${RED}Failed to pull image. You may need to login:${NC}"
            echo "  docker login ${REGISTRY}"
            return 1
        fi
    fi

    echo "Running tool verification tests..."

    local failed=0

    run_test "jq" "docker run --rm --user root $image bash -c 'jq --version'" || ((failed++))
    run_test "yq" "docker run --rm --user root $image bash -c 'yq --version'" || ((failed++))
    run_test "helm" "docker run --rm --user root $image bash -c 'helm version --short'" || ((failed++))
    run_test "kubectl" "docker run --rm --user root $image bash -c 'kubectl version --client'" || ((failed++))
    run_test "task" "docker run --rm --user root $image bash -c 'task --version'" || ((failed++))
    run_test "gcloud" "docker run --rm --user root $image bash -c 'gcloud version'" || ((failed++))
    run_test "aws" "docker run --rm --user root $image bash -c 'aws --version'" || ((failed++))
    run_test "envsubst" "docker run --rm --user root $image bash -c 'envsubst --version'" || ((failed++))
    run_test "node" "docker run --rm --user root $image bash -c 'node --version'" || ((failed++))
    run_test "npm" "docker run --rm --user root $image bash -c 'npm --version'" || ((failed++))
    run_test "npx" "docker run --rm --user root $image bash -c 'npx --version'" || ((failed++))

    echo ""
    if [[ $failed -eq 0 ]]; then
        echo -e "${GREEN}All Playwright Runner tests passed!${NC}"
    else
        echo -e "${RED}$failed test(s) failed${NC}"
        return 1
    fi
}

# Main
echo "=========================================="
echo "CI Runner Docker Image Test Suite"
echo "=========================================="

exit_code=0

if [[ "$TEST_CI" == "true" ]]; then
    test_ci_runner "$CI_RUNNER_IMAGE" || exit_code=1
fi

if [[ "$TEST_PLAYWRIGHT" == "true" ]]; then
    test_playwright_runner "$PLAYWRIGHT_RUNNER_IMAGE" || exit_code=1
fi

echo ""
echo "=========================================="
if [[ $exit_code -eq 0 ]]; then
    echo -e "${GREEN}All tests passed!${NC}"
else
    echo -e "${RED}Some tests failed!${NC}"
fi
echo "=========================================="

exit $exit_code

