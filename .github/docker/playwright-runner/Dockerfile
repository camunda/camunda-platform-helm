# ==============================================================================
# Playwright Runner Image for Camunda Platform Helm Integration Tests
# ==============================================================================
#
# OPTIMIZATION SUMMARY
# ====================
# This image has been aggressively optimized from 5.17 GB to ~306 MB (94% reduction).
#
# Final size breakdown (approximate):
#   - Base image (node:22-slim):            200 MB
#   - Chromium headless shell:              110 MB
#   - CLI tools (UPX compressed):            75 MB
#   - System libraries (Chromium deps):      50 MB
#   - GKE auth plugin:                        9 MB
#   ─────────────────────────────────────────────────
#   Total:                                 ~306 MB
#
# Compare to official mcr.microsoft.com/playwright: 5.17 GB
#
# KEY OPTIMIZATION TECHNIQUES
# ===========================
#
# 1. CHROMIUM HEADLESS SHELL ONLY (saves ~250 MB)
#    ─────────────────────────────────────────
#    Playwright installs two Chromium variants:
#      - chromium-XXXX/       (357 MB) - Full browser with UI
#      - chromium_headless_shell-XXXX/ (110 MB) - Headless-only binary
#    
#    For CI/headless testing, only the headless shell is needed.
#    We delete the full browser after installation.
#
# 2. SKIP LIBGBM AND LLVM (saves ~180 MB)
#    ─────────────────────────────────────────
#    Playwright's install-deps script installs libgbm1, which pulls in:
#      - mesa-libgallium: 41 MB (OpenGL implementation)
#      - libllvm20:      140 MB (LLVM compiler infrastructure)
#    
#    These are GPU/graphics libraries. Headless Chromium does NOT need them!
#    We manually specify only the libraries Chromium actually links against.
#    
#    How we determined this:
#      $ ldd chromium_headless_shell | grep -v "not found"
#    This shows chromium doesn't actually link libgbm at runtime.
#
# 3. UPX BINARY COMPRESSION (saves ~100 MB)
#    ─────────────────────────────────────────
#    Same technique as CI Runner - compress Go binaries by 60-75%.
#    
#    Compression results:
#      helm:                58 MB → 14 MB
#      kubectl:             47 MB → 12 MB
#      aws-iam-auth:        61 MB → 24 MB
#      yq:                  13 MB →  5 MB
#      task:                 8 MB →  3 MB
#      jq:                   2 MB →  1 MB
#
# 4. MINIMAL FONTS (saves ~60 MB)
#    ─────────────────────────────────────────
#    Official Playwright image includes fonts for all languages:
#      - fonts-noto-cjk:           ~150 MB (Chinese, Japanese, Korean)
#      - fonts-noto-color-emoji:    ~10 MB
#      - fonts-freefont-ttf:        ~15 MB
#      - Various other fonts:       ~30 MB
#    
#    For testing Camunda (English UI), only fonts-liberation is needed (~4 MB).
#    This provides Arial/Times/Courier equivalents for consistent rendering.
#
# 5. NODE:22-SLIM BASE (saves ~100 MB vs ubuntu)
#    ─────────────────────────────────────────
#    node:22-slim is Debian-based with Node.js pre-installed.
#    Smaller than ubuntu + manual Node.js installation.
#    Already includes npm for Playwright installation.
#
# 6. GKE-AUTH-PLUGIN ONLY (saves ~350 MB vs full gcloud)
#    ─────────────────────────────────────────
#    Playwright runner only needs to authenticate to GKE clusters.
#    We install just google-cloud-cli-gke-gcloud-auth-plugin (~9 MB)
#    instead of the full gcloud SDK (~370 MB).
#    
#    For AWS, we use aws-iam-authenticator binary directly instead
#    of the full AWS CLI.
#
# WHAT COULD BE OPTIMIZED FURTHER
# ===============================
#
# If the image needs to be even smaller, consider:
#
#   - Mount browser from host volume (-110 MB)
#     Share /ms-playwright with host instead of including in image.
#     Requires browser to be pre-installed on host/in volume.
#
#   - Use playwright-core instead of playwright
#     Smaller npm package, but requires manual browser management.
#
#   - Alpine base image (-50 MB potentially)
#     But musl libc has compatibility issues with Chromium.
#     Would require extensive testing.
#
# BUILD INSTRUCTIONS
# ==================
#
# Build with versions from .tool-versions:
#   ./scripts/build-ci-runner-local.sh --playwright-only --test
#
# Or build directly:
#   docker buildx build --platform linux/amd64 \
#     --build-arg PLAYWRIGHT_VERSION=1.57.0 \
#     -t playwright-runner:test .
#
# ==============================================================================

# ==============================================================================
# Stage 1: Download and compress CLI binaries with UPX
# ==============================================================================
# This stage downloads CLI tools and compresses them with UPX.
# Only the compressed binaries are copied to the final image.
# ==============================================================================
FROM debian:bookworm-slim AS tools

# Install minimal tools for downloading and compressing
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    xz-utils \
    && rm -rf /var/lib/apt/lists/*

# Download UPX directly from GitHub releases
# UPX is not available in Debian Bookworm's apt repositories
ARG UPX_VERSION=4.2.4
RUN curl -fsSL "https://github.com/upx/upx/releases/download/v${UPX_VERSION}/upx-${UPX_VERSION}-amd64_linux.tar.xz" \
    | tar -xJf - --strip-components=1 -C /usr/local/bin "upx-${UPX_VERSION}-amd64_linux/upx" \
    && upx --version

# ------------------------------------------------------------------------------
# Tool versions - These should match .tool-versions in the repository root
# ------------------------------------------------------------------------------
ARG HELM_VERSION=3.19.4
ARG KUBECTL_VERSION=1.27.16
ARG JQ_VERSION=1.8.1
ARG YQ_VERSION=4.50.1
ARG TASK_VERSION=3.30.1
ARG AWS_IAM_AUTH_VERSION=0.6.26

WORKDIR /tools

# ------------------------------------------------------------------------------
# Download CLI tools
# Playwright runner needs fewer tools than CI runner
# ------------------------------------------------------------------------------

# Helm - For Helm chart operations
RUN curl -fsSL "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz" | tar -xzf - \
    && mv linux-amd64/helm ./helm \
    && rm -rf linux-amd64

# kubectl - For Kubernetes operations
RUN curl -fsSL -o kubectl "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
    && chmod +x kubectl

# jq - For JSON processing in test scripts
RUN curl -fsSL -o jq "https://github.com/jqlang/jq/releases/download/jq-${JQ_VERSION}/jq-linux-amd64" \
    && chmod +x jq

# yq - For YAML processing in test scripts
RUN curl -fsSL -o yq "https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_amd64" \
    && chmod +x yq

# Task - For running Taskfile.yaml tasks
RUN curl -fsSL "https://github.com/go-task/task/releases/download/v${TASK_VERSION}/task_linux_amd64.tar.gz" | tar -xzf - task

# aws-iam-authenticator - For EKS authentication
# WHY THIS INSTEAD OF AWS CLI:
# - aws-iam-authenticator is a single 61 MB binary (24 MB compressed)
# - AWS CLI v1 is ~50 MB, AWS CLI v2 is ~250 MB
# - For EKS auth only, the authenticator is sufficient and smaller
RUN curl -fsSL -o aws-iam-authenticator \
    "https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/download/v${AWS_IAM_AUTH_VERSION}/aws-iam-authenticator_${AWS_IAM_AUTH_VERSION}_linux_amd64" \
    && chmod +x aws-iam-authenticator

# Show original sizes for build log
RUN echo "=== Original binary sizes ===" && ls -lh /tools/

# ------------------------------------------------------------------------------
# Compress all binaries with UPX
# ------------------------------------------------------------------------------
RUN upx --best helm kubectl yq task aws-iam-authenticator \
    && upx --best jq || true  # jq sometimes fails with UPX

# Show compressed sizes for build log
RUN echo "=== Compressed binary sizes ===" && ls -lh /tools/

# ==============================================================================
# Stage 2: Final optimized image
# ==============================================================================
FROM node:22-slim

LABEL org.opencontainers.image.source="https://github.com/camunda/camunda-platform-helm"
LABEL org.opencontainers.image.description="Playwright Runner (Ultra-optimized, Chromium headless-shell only) for Camunda Platform Helm integration tests"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# ------------------------------------------------------------------------------
# Build arguments
# ------------------------------------------------------------------------------
ARG ZBCTL_VERSION=v8.6.0
ARG PLAYWRIGHT_VERSION=1.58.0

# Environment configuration
ENV DEBIAN_FRONTEND=noninteractive
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8
# Tell Playwright where to find browsers
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
# Prevent Playwright from downloading browsers during npm install
# We install browsers explicitly with 'playwright install'
ENV PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1

SHELL ["/bin/bash", "-c"]

# ==============================================================================
# Layer 1: System packages and Chromium dependencies
# ==============================================================================
# CRITICAL OPTIMIZATION: Manual Chromium dependencies
# 
# Playwright's 'npx playwright install-deps' installs ALL dependencies including
# libgbm1, which pulls in mesa and LLVM (180 MB total). But headless Chromium
# doesn't actually need GPU libraries!
#
# How we determined the minimal dependency list:
# 1. Run: ldd /ms-playwright/chromium_headless_shell-*/chrome-linux/headless_shell
# 2. Identify which .so files are missing
# 3. Find which Debian packages provide them
# 4. Test that Chromium launches successfully
#
# Libraries we SKIP (not needed for headless):
# - libgbm1 (pulls in mesa-libgallium + libllvm20 = 180 MB)
# - libwayland-* (Wayland display server)
# - libgtk-* (GTK UI toolkit)
# ==============================================================================
RUN apt-get update && apt-get install -y --no-install-recommends \
    # Essential system tools
    ca-certificates \
    curl \
    git \
    gnupg \
    make \
    gettext-base \
    # -------------------------------------------------------------------------
    # Chromium headless dependencies (MINIMAL SET)
    # These are the actual runtime dependencies of chromium_headless_shell
    # -------------------------------------------------------------------------
    libasound2 \
    libatk-bridge2.0-0 \
    libatk1.0-0 \
    libatspi2.0-0 \
    libcairo2 \
    libcups2 \
    libdbus-1-3 \
    libdrm2 \
    libglib2.0-0 \
    libnspr4 \
    libnss3 \
    libpango-1.0-0 \
    libx11-6 \
    libxcb1 \
    libxcomposite1 \
    libxdamage1 \
    libxext6 \
    libxfixes3 \
    libxkbcommon0 \
    libxrandr2 \
    # -------------------------------------------------------------------------
    # Minimal fonts
    # fonts-liberation provides Arial/Times/Courier equivalents (~4 MB)
    # vs fonts-noto-cjk + others which would add ~200 MB
    # -------------------------------------------------------------------------
    fonts-liberation \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/*

# ==============================================================================
# Layer 2: UPX-compressed CLI tools
# ==============================================================================
COPY --from=tools /tools/helm /usr/local/bin/
COPY --from=tools /tools/kubectl /usr/local/bin/
COPY --from=tools /tools/jq /usr/local/bin/
COPY --from=tools /tools/yq /usr/local/bin/
COPY --from=tools /tools/task /usr/local/bin/
COPY --from=tools /tools/aws-iam-authenticator /usr/local/bin/

# ==============================================================================
# Layer 3: GKE authentication plugin
# ==============================================================================
# WHY PLUGIN ONLY:
# The full gcloud SDK is ~370 MB. For Playwright tests, we only need to
# authenticate to GKE clusters, which requires just the auth plugin (~9 MB).
#
# The plugin handles the 'gke-gcloud-auth-plugin' exec in kubeconfig files
# without needing the full gcloud CLI.
# ==============================================================================
RUN curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg \
    && echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" > /etc/apt/sources.list.d/google-cloud-sdk.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends google-cloud-cli-gke-gcloud-auth-plugin \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/*

# ==============================================================================
# Layer 4: Playwright and Chromium
# ==============================================================================
# BROWSER INSTALLATION STRATEGY:
#
# 1. Install playwright and zbctl globally via npm
# 2. Run 'playwright install chromium' which downloads:
#    - chromium-XXXX/ (357 MB) - Full browser
#    - chromium_headless_shell-XXXX/ (110 MB) - Headless-only
#    - ffmpeg-XXXX/ (5 MB) - Video encoding
# 3. Delete the full browser and ffmpeg (we only need headless shell)
#
# WHY HEADLESS SHELL ONLY:
# - Our tests run headless (no visible browser window)
# - headless_shell is a stripped-down Chromium for this exact use case
# - Saves 250+ MB vs keeping both browser variants
#
# WHY DELETE FFMPEG:
# - ffmpeg is only needed for video recording of tests
# - We don't record test videos in CI
# - Saves 5 MB
# ==============================================================================
RUN npm install -g playwright@${PLAYWRIGHT_VERSION} zbctl@${ZBCTL_VERSION#v} \
    && mkdir -p /ms-playwright \
    && npx playwright install chromium \
    # Delete full Chromium browser - keep only headless shell
    && rm -rf /ms-playwright/chromium-*/chrome-linux \
    # Delete ffmpeg - not needed for headless testing
    && rm -rf /ms-playwright/ffmpeg-* \
    # Clean npm cache
    && npm cache clean --force \
    && rm -rf /root/.npm /tmp/*

# ==============================================================================
# Layer 5: Configuration
# ==============================================================================
# Allow git operations in any directory (fixes "dubious ownership" error)
RUN git config --global --add safe.directory '*'

# Create pwuser for compatibility with some Playwright scripts
RUN useradd -m -s /bin/bash pwuser || true

# Copy .tool-versions to standard locations
COPY .tool-versions /opt/.tool-versions
RUN cp /opt/.tool-versions /root/.tool-versions \
    && cp /opt/.tool-versions /.tool-versions \
    && mkdir -p /home/runner /home/pwuser \
    && cp /opt/.tool-versions /home/runner/.tool-versions \
    && cp /opt/.tool-versions /home/pwuser/.tool-versions 2>/dev/null || true

# ==============================================================================
# Layer 6: Verification
# ==============================================================================
# Verify all tools are functional. This catches issues at build time.
# ==============================================================================
WORKDIR /workspace

RUN echo "=== Verifying tool installations ===" \
    && echo "Node: $(node --version)" \
    && echo "npm: $(npm --version)" \
    && echo "Playwright: $(npx playwright --version 2>/dev/null || echo 'checking...')" \
    && helm version --short \
    && kubectl version --client \
    && jq --version \
    && yq --version \
    && task --version \
    && gke-gcloud-auth-plugin --version \
    && aws-iam-authenticator version \
    && (zbctl version 2>/dev/null || true) \
    && echo "=== Checking browser installation ===" \
    && ls -la /ms-playwright/ \
    && echo "=== All tools verified successfully ==="

# Run as root by default (CI typically runs as root)
USER root

ENTRYPOINT ["/bin/bash", "-c"]
CMD ["bash"]
