#!/usr/bin/env bash
# Build the CI Runner Docker images locally
# Usage: ./scripts/build-ci-runner-local.sh [--push] [--login] [--ci-only] [--playwright-only]
#
# Options:
#   --push             Build and push the images to the registry
#   --login            Force re-authentication to the registry (opens browser)
#   --ci-only          Build only the CI runner image
#   --playwright-only  Build only the Playwright runner image

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Image configuration
REGISTRY="registry.camunda.cloud"
CI_IMAGE_NAME="team-distribution/ci-runner"
PLAYWRIGHT_IMAGE_NAME="team-distribution/playwright-runner"
CI_FULL_IMAGE="${REGISTRY}/${CI_IMAGE_NAME}"
PLAYWRIGHT_FULL_IMAGE="${REGISTRY}/${PLAYWRIGHT_IMAGE_NAME}"

# Parse arguments
DO_PUSH=false
FORCE_LOGIN=false
BUILD_CI=true
BUILD_PLAYWRIGHT=true

for arg in "$@"; do
    case "$arg" in
        --push) DO_PUSH=true ;;
        --login) FORCE_LOGIN=true ;;
        --ci-only) BUILD_PLAYWRIGHT=false ;;
        --playwright-only) BUILD_CI=false ;;
        --help|-h)
            echo "Usage: $0 [--push] [--login] [--ci-only] [--playwright-only]"
            echo ""
            echo "Options:"
            echo "  --push             Build and push the images to the registry"
            echo "  --login            Force re-authentication to the registry (opens browser)"
            echo "  --ci-only          Build only the CI runner image"
            echo "  --playwright-only  Build only the Playwright runner image"
            echo ""
            echo "Images:"
            echo "  CI Runner:         ${CI_FULL_IMAGE}"
            echo "  Playwright Runner: ${PLAYWRIGHT_FULL_IMAGE}"
            echo ""
            exit 0
            ;;
    esac
done

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

echo "=========================================="
echo "Building CI Runner Docker Images"
echo "=========================================="
echo "Repository root: $REPO_ROOT"
echo "Tools hash: sha-${TOOLS_HASH}"
echo "Date tag: ${DATE_TAG}"
echo ""
echo "Images to build:"
[[ "$BUILD_CI" == "true" ]] && echo "  - CI Runner: ${CI_FULL_IMAGE}"
[[ "$BUILD_PLAYWRIGHT" == "true" ]] && echo "  - Playwright Runner: ${PLAYWRIGHT_FULL_IMAGE}"
echo ""

# Login if pushing or forced
if [[ "$DO_PUSH" == "true" ]] || [[ "$FORCE_LOGIN" == "true" ]]; then
    docker_login "${REGISTRY}"
    echo ""
fi

# Build CI Runner image
if [[ "$BUILD_CI" == "true" ]]; then
    echo "=========================================="
    echo "Building CI Runner Image"
    echo "=========================================="

    # Copy .tool-versions to docker context
    echo "Copying .tool-versions to docker context..."
    cp "$REPO_ROOT/.tool-versions" "$REPO_ROOT/.github/docker/ci-runner/.tool-versions"

    # Build the image
    echo "Building image..."
    docker build \
        -t "${CI_FULL_IMAGE}:latest" \
        -t "${CI_FULL_IMAGE}:sha-${TOOLS_HASH}" \
        -t "${CI_FULL_IMAGE}:${DATE_TAG}" \
        -f "$REPO_ROOT/.github/docker/ci-runner/Dockerfile" \
        "$REPO_ROOT/.github/docker/ci-runner"

    # Cleanup
    rm -f "$REPO_ROOT/.github/docker/ci-runner/.tool-versions"

    echo ""
    echo "✅ CI Runner build complete!"
    echo "   Tags: latest, sha-${TOOLS_HASH}, ${DATE_TAG}"
    echo ""
fi

# Build Playwright Runner image
if [[ "$BUILD_PLAYWRIGHT" == "true" ]]; then
    echo "=========================================="
    echo "Building Playwright Runner Image"
    echo "=========================================="

    # Copy .tool-versions to docker context
    echo "Copying .tool-versions to docker context..."
    cp "$REPO_ROOT/.tool-versions" "$REPO_ROOT/.github/docker/playwright-runner/.tool-versions"

    # Build the image
    echo "Building image..."
    docker build \
        -t "${PLAYWRIGHT_FULL_IMAGE}:latest" \
        -t "${PLAYWRIGHT_FULL_IMAGE}:sha-${TOOLS_HASH}" \
        -t "${PLAYWRIGHT_FULL_IMAGE}:${DATE_TAG}" \
        -f "$REPO_ROOT/.github/docker/playwright-runner/Dockerfile" \
        "$REPO_ROOT/.github/docker/playwright-runner"

    # Cleanup
    rm -f "$REPO_ROOT/.github/docker/playwright-runner/.tool-versions"

    echo ""
    echo "✅ Playwright Runner build complete!"
    echo "   Tags: latest, sha-${TOOLS_HASH}, ${DATE_TAG}"
    echo ""
fi

# Summary
echo "=========================================="
echo "Build Summary"
echo "=========================================="
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

# Push if requested
if [[ "$DO_PUSH" == "true" ]]; then
    echo "Pushing images to registry..."

    if [[ "$BUILD_CI" == "true" ]]; then
        echo "Pushing CI Runner images..."
        docker push "${CI_FULL_IMAGE}:latest"
        docker push "${CI_FULL_IMAGE}:sha-${TOOLS_HASH}"
        docker push "${CI_FULL_IMAGE}:${DATE_TAG}"
    fi

    if [[ "$BUILD_PLAYWRIGHT" == "true" ]]; then
        echo "Pushing Playwright Runner images..."
        docker push "${PLAYWRIGHT_FULL_IMAGE}:latest"
        docker push "${PLAYWRIGHT_FULL_IMAGE}:sha-${TOOLS_HASH}"
        docker push "${PLAYWRIGHT_FULL_IMAGE}:${DATE_TAG}"
    fi

    echo ""
    echo "✅ Push complete!"
else
    echo "To push the images, re-run with --push:"
    echo "  $0 --push"
fi
fi

# Cleanup
rm -f "$REPO_ROOT/.github/docker/ci-runner/.tool-versions"

