#!/usr/bin/env bash
# Build the CI Runner Docker images locally
# Usage: ./scripts/build-ci-runner-local.sh [--push] [--test] [--login] [--ci-only] [--playwright-only]
#
# Options:
#   --push             Build and push the images to the registry
#   --test             Test the images after building (before pushing)
#   --login            Force re-authentication to the registry (opens browser)
#   --ci-only          Build only the CI runner image
#   --playwright-only  Build only the Playwright runner image
#
# Note: Images are built for linux/amd64 platform to match CI runners

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Image configuration
REGISTRY="registry.camunda.cloud"
CI_IMAGE_NAME="team-distribution/ci-runner"
PLAYWRIGHT_IMAGE_NAME="team-distribution/playwright-runner"
CI_FULL_IMAGE="${REGISTRY}/${CI_IMAGE_NAME}"
PLAYWRIGHT_FULL_IMAGE="${REGISTRY}/${PLAYWRIGHT_IMAGE_NAME}"

# Local test image names
CI_TEST_IMAGE="ci-runner-test"
PLAYWRIGHT_TEST_IMAGE="playwright-runner-test"

# Target platform - CI runners are linux/amd64
TARGET_PLATFORM="linux/amd64"

# Parse arguments
DO_PUSH=false
DO_TEST=false
FORCE_LOGIN=false
BUILD_CI=true
BUILD_PLAYWRIGHT=true
USE_BUILDX=true

for arg in "$@"; do
    case "$arg" in
        --push) DO_PUSH=true ;;
        --test) DO_TEST=true ;;
        --login) FORCE_LOGIN=true ;;
        --ci-only) BUILD_PLAYWRIGHT=false ;;
        --playwright-only) BUILD_CI=false ;;
        --no-buildx) USE_BUILDX=false ;;
        --help|-h)
            echo "Usage: $0 [--push] [--test] [--login] [--ci-only] [--playwright-only] [--no-buildx]"
            echo ""
            echo "Options:"
            echo "  --push             Build and push the images to the registry"
            echo "  --test             Test the images after building (before pushing)"
            echo "  --login            Force re-authentication to the registry (opens browser)"
            echo "  --ci-only          Build only the CI runner image"
            echo "  --playwright-only  Build only the Playwright runner image"
            echo "  --no-buildx        Use standard docker build instead of buildx (for local testing)"
            echo ""
            echo "Images (built for ${TARGET_PLATFORM}):"
            echo "  CI Runner:         ${CI_FULL_IMAGE}"
            echo "  Playwright Runner: ${PLAYWRIGHT_FULL_IMAGE}"
            echo ""
            echo "Examples:"
            echo "  $0 --test              # Build and test locally"
            echo "  $0 --test --push       # Build, test, then push if tests pass"
            echo "  $0 --ci-only --test    # Build and test only CI runner"
            echo "  $0 --test --no-buildx  # Build locally without buildx (avoids cache issues)"
            echo ""
            exit 0
            ;;
    esac
done

# Test function - runs the test script
run_tests() {
    echo ""
    echo "=========================================="
    echo "Running Tests"
    echo "=========================================="

    local test_args="--local"

    if [[ "$BUILD_CI" == "true" && "$BUILD_PLAYWRIGHT" == "true" ]]; then
        test_args="$test_args --both"
    elif [[ "$BUILD_CI" == "true" ]]; then
        test_args="$test_args --ci-runner"
    elif [[ "$BUILD_PLAYWRIGHT" == "true" ]]; then
        test_args="$test_args --playwright-runner"
    fi

    if "${SCRIPT_DIR}/test-ci-runner-local.sh" $test_args; then
        echo ""
        echo "✅ All tests passed!"
        return 0
    else
        echo ""
        echo "❌ Tests failed!"
        return 1
    fi
}

# Ensure docker buildx is available for cross-platform builds
setup_buildx() {
    echo "Setting up Docker Buildx for cross-platform builds..."

    # Check if buildx is available
    if ! docker buildx version &>/dev/null; then
        echo "❌ Docker Buildx is required for cross-platform builds"
        echo "   Please install Docker Desktop or enable buildx"
        exit 1
    fi

    # Create/use a builder that supports multi-platform
    local builder_name="ci-runner-builder"
    if ! docker buildx inspect "$builder_name" &>/dev/null; then
        echo "Creating buildx builder: $builder_name"
        docker buildx create --name "$builder_name" --driver docker-container --bootstrap
    fi
    docker buildx use "$builder_name"
    echo "✅ Buildx ready (builder: $builder_name)"
}

# Harbor/Registry login function
# Uses OIDC browser-based login if available, falls back to manual credentials
docker_login() {
    local registry="$1"

    echo "Checking authentication to ${registry}..."

    # Check if already logged in by trying to pull a manifest
    if docker manifest inspect "${CI_FULL_IMAGE}:latest" &>/dev/null; then
        echo "✅ Already authenticated to ${registry}"
        return 0
    fi

    echo ""
    echo "Authentication required for ${registry}"
    echo ""

    # Check if we have the Harbor CLI for OIDC login
    if command -v harbor &>/dev/null; then
        echo "Using Harbor CLI for OIDC login..."
        harbor login "${registry}" --oidc
        # Harbor CLI sets up docker credentials automatically
        return $?
    fi

    # Try browser-based OIDC login via docker credential helper if available
    # This works with registries that support OIDC (like Harbor with OIDC configured)
    if [[ -f ~/.docker/config.json ]] && grep -q "credsStore" ~/.docker/config.json; then
        echo "Attempting login via credential helper..."
        if docker login "${registry}" 2>/dev/null; then
            echo "✅ Login successful via credential helper"
            return 0
        fi
    fi

    # Check for environment variables (CI mode)
    if [[ -n "${DOCKER_USERNAME:-}" ]] && [[ -n "${DOCKER_PASSWORD:-}" ]]; then
        echo "Using credentials from environment variables..."
        echo "${DOCKER_PASSWORD}" | docker login "${registry}" -u "${DOCKER_USERNAME}" --password-stdin
        return $?
    fi

    # Interactive browser-based login for Harbor with OIDC
    echo "Opening browser for OIDC authentication..."
    echo ""
    echo "If the browser doesn't open automatically, please visit:"
    echo "  https://${registry}/c/oidc/login"
    echo ""
    echo "After logging in via the browser, copy the CLI secret from:"
    echo "  https://${registry}/harbor/users → User Profile → CLI Secret"
    echo ""

    # Try to open browser (works on macOS, Linux with xdg-open, WSL)
    local login_url="https://${registry}/c/oidc/login"
    if command -v open &>/dev/null; then
        open "${login_url}" 2>/dev/null || true
    elif command -v xdg-open &>/dev/null; then
        xdg-open "${login_url}" 2>/dev/null || true
    elif command -v wslview &>/dev/null; then
        wslview "${login_url}" 2>/dev/null || true
    fi

    # Prompt for credentials after browser login
    echo ""
    read -r -p "Enter your username (email): " username
    read -r -s -p "Enter your CLI secret (from Harbor profile): " password
    echo ""

    echo "${password}" | docker login "${registry}" -u "${username}" --password-stdin
}

# Generate tags
TOOLS_HASH=$(sha256sum "$REPO_ROOT/.tool-versions" | cut -c1-8)
DATE_TAG=$(date +%Y%m%d)

# Extract versions from .tool-versions for build arguments
# This enables proper layer caching - layers only rebuild when specific versions change
extract_version() {
    local tool="$1"
    grep "^${tool} " "$REPO_ROOT/.tool-versions" | awk '{print $2}'
}

GOLANG_VERSION=$(extract_version "golang")
HELM_VERSION=$(extract_version "helm")
KUBECTL_VERSION=$(extract_version "kubectl")
JQ_VERSION=$(extract_version "jq")
YQ_VERSION=$(extract_version "yq")
TASK_VERSION=$(extract_version "task")
KUSTOMIZE_VERSION=$(extract_version "kustomize")
OC_VERSION=$(extract_version "oc")
KIND_VERSION=$(extract_version "kind")
ZBCTL_VERSION=$(extract_version "zbctl")
ORAS_VERSION=$(extract_version "oras")
HELM_CT_VERSION=$(extract_version "helm-ct")
HELM_CR_VERSION=$(extract_version "helm-cr")
GOMPLATE_VERSION=$(extract_version "gomplate")
BATS_VERSION=$(extract_version "bats")
YAMLLINT_VERSION=$(extract_version "yamllint")

# Build arguments for CI runner
CI_BUILD_ARGS="--build-arg GOLANG_VERSION=${GOLANG_VERSION} \
    --build-arg HELM_VERSION=${HELM_VERSION} \
    --build-arg KUBECTL_VERSION=${KUBECTL_VERSION} \
    --build-arg JQ_VERSION=${JQ_VERSION} \
    --build-arg YQ_VERSION=${YQ_VERSION} \
    --build-arg TASK_VERSION=${TASK_VERSION} \
    --build-arg KUSTOMIZE_VERSION=${KUSTOMIZE_VERSION} \
    --build-arg OC_VERSION=${OC_VERSION} \
    --build-arg KIND_VERSION=${KIND_VERSION} \
    --build-arg ZBCTL_VERSION=${ZBCTL_VERSION} \
    --build-arg ORAS_VERSION=${ORAS_VERSION} \
    --build-arg HELM_CT_VERSION=${HELM_CT_VERSION} \
    --build-arg HELM_CR_VERSION=${HELM_CR_VERSION} \
    --build-arg GOMPLATE_VERSION=${GOMPLATE_VERSION} \
    --build-arg BATS_VERSION=${BATS_VERSION} \
    --build-arg YAMLLINT_VERSION=${YAMLLINT_VERSION}"

# Build arguments for Playwright runner (subset of tools)
PW_BUILD_ARGS="--build-arg HELM_VERSION=${HELM_VERSION} \
    --build-arg KUBECTL_VERSION=${KUBECTL_VERSION} \
    --build-arg JQ_VERSION=${JQ_VERSION} \
    --build-arg YQ_VERSION=${YQ_VERSION} \
    --build-arg TASK_VERSION=${TASK_VERSION} \
    --build-arg OC_VERSION=${OC_VERSION} \
    --build-arg ZBCTL_VERSION=${ZBCTL_VERSION}"

echo "=========================================="
echo "Building CI Runner Docker Images"
echo "=========================================="
echo "Repository root: $REPO_ROOT"
echo "Tools hash: sha-${TOOLS_HASH}"
echo "Date tag: ${DATE_TAG}"
echo "Target platform: ${TARGET_PLATFORM}"
echo ""
echo "Tool versions:"
echo "  golang:    ${GOLANG_VERSION}"
echo "  helm:      ${HELM_VERSION}"
echo "  kubectl:   ${KUBECTL_VERSION}"
echo "  jq:        ${JQ_VERSION}"
echo "  yq:        ${YQ_VERSION}"
echo "  task:      ${TASK_VERSION}"
echo ""
echo "Images to build:"
[[ "$BUILD_CI" == "true" ]] && echo "  - CI Runner: ${CI_FULL_IMAGE}"
[[ "$BUILD_PLAYWRIGHT" == "true" ]] && echo "  - Playwright Runner: ${PLAYWRIGHT_FULL_IMAGE}"
echo ""

# Setup buildx for cross-platform builds (only if using buildx)
if [[ "$USE_BUILDX" == "true" ]]; then
    setup_buildx
    echo ""
fi

# Login if pushing or forced
if [[ "$DO_PUSH" == "true" ]] || [[ "$FORCE_LOGIN" == "true" ]]; then
    docker_login "${REGISTRY}"
    echo ""
fi

# Determine build output strategy
# If testing, we build locally first with --load, test, then optionally push
# If just pushing (no test), we push directly
if [[ "$DO_TEST" == "true" ]]; then
    BUILD_OUTPUT="--load"
    echo "ℹ️  Images will be built locally for testing"
    if [[ "$DO_PUSH" == "true" ]]; then
        echo "ℹ️  After tests pass, images will be pushed to registry"
    fi
elif [[ "$DO_PUSH" == "true" ]]; then
    BUILD_OUTPUT="--push"
    echo "ℹ️  Images will be pushed directly to registry (required for cross-platform builds)"
else
    # For local builds, we need to use --load which only works for single platform
    # and the platform must match the host or use emulation
    BUILD_OUTPUT="--load"
    echo "ℹ️  Images will be loaded to local Docker daemon"
fi

if [[ "$USE_BUILDX" == "false" ]]; then
    echo "ℹ️  Using standard docker build (--no-buildx)"
fi

# Build CI Runner image
if [[ "$BUILD_CI" == "true" ]]; then
    echo ""
    echo "=========================================="
    echo "Building CI Runner Image"
    echo "=========================================="

    # Copy .tool-versions to docker context
    echo "Copying .tool-versions to docker context..."
    cp "$REPO_ROOT/.tool-versions" "$REPO_ROOT/.github/docker/ci-runner/.tool-versions"

    # Determine image tags based on whether we're testing
    if [[ "$DO_TEST" == "true" ]]; then
        CI_BUILD_TAGS="-t ${CI_TEST_IMAGE}:latest"
    else
        CI_BUILD_TAGS="-t ${CI_FULL_IMAGE}:latest -t ${CI_FULL_IMAGE}:sha-${TOOLS_HASH} -t ${CI_FULL_IMAGE}:${DATE_TAG}"
    fi

    # Build the image
    echo "Building image for ${TARGET_PLATFORM}..."
    if [[ "$USE_BUILDX" == "true" ]]; then
        docker buildx build \
            --platform "${TARGET_PLATFORM}" \
            ${CI_BUILD_ARGS} \
            ${CI_BUILD_TAGS} \
            -f "$REPO_ROOT/.github/docker/ci-runner/Dockerfile" \
            ${BUILD_OUTPUT} \
            "$REPO_ROOT/.github/docker/ci-runner"
    else
        # Standard docker build (for local testing without buildx)
        docker build \
            ${CI_BUILD_ARGS} \
            ${CI_BUILD_TAGS} \
            -f "$REPO_ROOT/.github/docker/ci-runner/Dockerfile" \
            "$REPO_ROOT/.github/docker/ci-runner"

        # If pushing without test, push now (with --test, push happens after tests pass)
        if [[ "$DO_PUSH" == "true" && "$DO_TEST" == "false" ]]; then
            echo "Pushing CI Runner images..."
            docker push "${CI_FULL_IMAGE}:latest"
            docker push "${CI_FULL_IMAGE}:sha-${TOOLS_HASH}"
            docker push "${CI_FULL_IMAGE}:${DATE_TAG}"
        fi
    fi

    # Cleanup
    rm -f "$REPO_ROOT/.github/docker/ci-runner/.tool-versions"

    echo ""
    echo "✅ CI Runner build complete!"
fi

# Build Playwright Runner image
if [[ "$BUILD_PLAYWRIGHT" == "true" ]]; then
    echo ""
    echo "=========================================="
    echo "Building Playwright Runner Image"
    echo "=========================================="

    # Copy .tool-versions to docker context
    echo "Copying .tool-versions to docker context..."
    cp "$REPO_ROOT/.tool-versions" "$REPO_ROOT/.github/docker/playwright-runner/.tool-versions"

    # Determine image tags based on whether we're testing
    if [[ "$DO_TEST" == "true" ]]; then
        PW_BUILD_TAGS="-t ${PLAYWRIGHT_TEST_IMAGE}:latest"
    else
        PW_BUILD_TAGS="-t ${PLAYWRIGHT_FULL_IMAGE}:latest -t ${PLAYWRIGHT_FULL_IMAGE}:sha-${TOOLS_HASH} -t ${PLAYWRIGHT_FULL_IMAGE}:${DATE_TAG}"
    fi

    # Build the image
    echo "Building image for ${TARGET_PLATFORM}..."
    if [[ "$USE_BUILDX" == "true" ]]; then
        docker buildx build \
            --platform "${TARGET_PLATFORM}" \
            ${PW_BUILD_ARGS} \
            ${PW_BUILD_TAGS} \
            -f "$REPO_ROOT/.github/docker/playwright-runner/Dockerfile" \
            ${BUILD_OUTPUT} \
            "$REPO_ROOT/.github/docker/playwright-runner"
    else
        # Standard docker build (for local testing without buildx)
        docker build \
            ${PW_BUILD_ARGS} \
            ${PW_BUILD_TAGS} \
            -f "$REPO_ROOT/.github/docker/playwright-runner/Dockerfile" \
            "$REPO_ROOT/.github/docker/playwright-runner"

        # If pushing without test, push now (with --test, push happens after tests pass)
        if [[ "$DO_PUSH" == "true" && "$DO_TEST" == "false" ]]; then
            echo "Pushing Playwright Runner images..."
            docker push "${PLAYWRIGHT_FULL_IMAGE}:latest"
            docker push "${PLAYWRIGHT_FULL_IMAGE}:sha-${TOOLS_HASH}"
            docker push "${PLAYWRIGHT_FULL_IMAGE}:${DATE_TAG}"
        fi
    fi

    # Cleanup
    rm -f "$REPO_ROOT/.github/docker/playwright-runner/.tool-versions"

    echo ""
    echo "✅ Playwright Runner build complete!"
fi

# Run tests if requested
if [[ "$DO_TEST" == "true" ]]; then
    if ! run_tests; then
        echo ""
        echo "❌ Tests failed! Not pushing images."
        exit 1
    fi

    # If tests passed and push requested, now push the images
    if [[ "$DO_PUSH" == "true" ]]; then
        echo ""
        echo "=========================================="
        echo "Tests passed! Pushing images to registry..."
        echo "=========================================="

        # Re-tag test images with final names and push
        if [[ "$BUILD_CI" == "true" ]]; then
            echo "Tagging and pushing CI Runner..."
            docker tag "${CI_TEST_IMAGE}:latest" "${CI_FULL_IMAGE}:latest"
            docker tag "${CI_TEST_IMAGE}:latest" "${CI_FULL_IMAGE}:sha-${TOOLS_HASH}"
            docker tag "${CI_TEST_IMAGE}:latest" "${CI_FULL_IMAGE}:${DATE_TAG}"
            docker push "${CI_FULL_IMAGE}:latest"
            docker push "${CI_FULL_IMAGE}:sha-${TOOLS_HASH}"
            docker push "${CI_FULL_IMAGE}:${DATE_TAG}"
        fi

        if [[ "$BUILD_PLAYWRIGHT" == "true" ]]; then
            echo "Tagging and pushing Playwright Runner..."
            docker tag "${PLAYWRIGHT_TEST_IMAGE}:latest" "${PLAYWRIGHT_FULL_IMAGE}:latest"
            docker tag "${PLAYWRIGHT_TEST_IMAGE}:latest" "${PLAYWRIGHT_FULL_IMAGE}:sha-${TOOLS_HASH}"
            docker tag "${PLAYWRIGHT_TEST_IMAGE}:latest" "${PLAYWRIGHT_FULL_IMAGE}:${DATE_TAG}"
            docker push "${PLAYWRIGHT_FULL_IMAGE}:latest"
            docker push "${PLAYWRIGHT_FULL_IMAGE}:sha-${TOOLS_HASH}"
            docker push "${PLAYWRIGHT_FULL_IMAGE}:${DATE_TAG}"
        fi
    fi
fi

# Summary
echo ""
echo "=========================================="
echo "Build Summary"
echo "=========================================="
echo "Platform: ${TARGET_PLATFORM}"
echo ""
if [[ "$BUILD_CI" == "true" ]]; then
    echo "CI Runner:"
    echo "  - ${CI_FULL_IMAGE}:latest"
    echo "  - ${CI_FULL_IMAGE}:sha-${TOOLS_HASH}"
    echo "  - ${CI_FULL_IMAGE}:${DATE_TAG}"
fi
if [[ "$BUILD_PLAYWRIGHT" == "true" ]]; then
    echo "Playwright Runner:"
    echo "  - ${PLAYWRIGHT_FULL_IMAGE}:latest"
    echo "  - ${PLAYWRIGHT_FULL_IMAGE}:sha-${TOOLS_HASH}"
    echo "  - ${PLAYWRIGHT_FULL_IMAGE}:${DATE_TAG}"
fi
echo ""

if [[ "$DO_PUSH" == "true" ]]; then
    echo "✅ Images pushed to registry!"
elif [[ "$DO_TEST" == "true" ]]; then
    echo "✅ Tests passed! Images built locally as:"
    [[ "$BUILD_CI" == "true" ]] && echo "  - ${CI_TEST_IMAGE}:latest"
    [[ "$BUILD_PLAYWRIGHT" == "true" ]] && echo "  - ${PLAYWRIGHT_TEST_IMAGE}:latest"
    echo ""
    echo "To push to registry, run:"
    echo "  $0 --test --push"
else
    echo "To test the images locally, run:"
    echo "  $0 --test"
    echo ""
    echo "To build, test, and push to registry, run:"
    echo "  $0 --test --push"
    echo ""
    echo "Note: Cross-platform images (linux/amd64 on Apple Silicon) must be"
    echo "      pushed directly to registry. Use --push to build and push."
fi
