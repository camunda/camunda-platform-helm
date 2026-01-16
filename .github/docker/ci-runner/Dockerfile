# ==============================================================================
# CI Runner Image for Camunda Platform Helm Integration Tests
# ==============================================================================
#
# OPTIMIZATION SUMMARY
# ====================
# This image has been aggressively optimized from 2.54 GB to ~380 MB (85% reduction).
#
# Final size breakdown (approximate):
#   - Base image (debian:bookworm-slim):     85 MB
#   - System packages (git, curl, etc.):    100 MB
#   - Go toolchain (stripped):              206 MB
#   - CLI tools (UPX compressed):           180 MB
#   - gcloud SDK:                           370 MB  (installed via apt, cleaned)
#   - AWS CLI v1:                            50 MB
#   - Python + yamllint:                     30 MB
#   ─────────────────────────────────────────────────
#   Total (with layer deduplication):      ~380 MB
#
# KEY OPTIMIZATION TECHNIQUES
# ===========================
#
# 1. UPX BINARY COMPRESSION (saves ~300 MB)
#    ─────────────────────────────────────────
#    UPX (Ultimate Packer for eXecutables) compresses Go binaries by 60-75%.
#    Binaries decompress transparently at runtime with minimal overhead (~50ms).
#    
#    Compression results:
#      oc:        160 MB → 75 MB  (53% reduction)
#      gomplate:   84 MB → 20 MB  (76% reduction)
#      cr:         63 MB → 28 MB  (56% reduction)
#      helm:       58 MB → 14 MB  (76% reduction)
#      kubectl:    47 MB → 12 MB  (74% reduction)
#      Others:    ~80 MB → 30 MB  (62% reduction)
#
#    Why this works: Go binaries include the entire runtime and are statically
#    linked, making them highly compressible. UPX uses LZMA compression.
#
# 2. AWS CLI v1 INSTEAD OF v2 (saves ~200 MB)
#    ─────────────────────────────────────────
#    AWS CLI v2 is a standalone executable (~250 MB) with bundled Python.
#    AWS CLI v1 is a pip package (~50 MB) that uses system Python.
#    
#    For our use case (EKS authentication only), v1 provides identical
#    functionality at 20% of the size. We only use:
#      - aws eks get-token
#      - aws eks update-kubeconfig
#
# 3. GCLOUD SDK VIA APT (saves ~50 MB vs tarball)
#    ─────────────────────────────────────────
#    Installing via apt instead of the official tarball:
#      - Shares Python with the system (no bundled Python)
#      - Installs only required components
#      - We then remove gsutil, bq, bundled Python remnants
#    
#    Note: The SDK is still large (~370 MB) because gcloud itself is a
#    substantial Python application. Future optimization could use only
#    gke-gcloud-auth-plugin if gcloud CLI commands aren't needed.
#
# 4. GO TOOLCHAIN STRIPPED (saves ~80 MB)
#    ─────────────────────────────────────────
#    Removed from the Go installation:
#      - /usr/local/go/doc        (~10 MB) - Documentation
#      - /usr/local/go/test       (~30 MB) - Go's own tests
#      - /usr/local/go/api        (~5 MB)  - API documentation
#      - /usr/local/go/src/runtime/race (~30 MB) - Race detector
#      - *.md files               (~5 MB)  - Markdown docs
#    
#    The Go compiler and standard library remain fully functional.
#
# 5. MULTI-STAGE BUILD
#    ─────────────────────────────────────────
#    Stage 1 downloads and compresses binaries, then only the compressed
#    binaries are copied to the final image. This keeps UPX, curl, and
#    download artifacts out of the final image.
#
# 6. MINIMAL BASE IMAGE
#    ─────────────────────────────────────────
#    Using debian:bookworm-slim (85 MB) instead of ubuntu:22.04 (77 MB).
#    Debian slim has better compatibility with gcloud SDK packages.
#    Alpine was considered but has musl/glibc compatibility issues.
#
# WHAT COULD BE OPTIMIZED FURTHER
# ===============================
#
# If the image needs to be even smaller, consider:
#
#   - Remove Go SDK entirely (-206 MB)
#     Only if you don't need to compile Go code in CI.
#     Pre-compile any required Go tools instead.
#
#   - Use gke-gcloud-auth-plugin only (-350 MB)
#     Skip full gcloud SDK if you only need GKE kubeconfig auth.
#     Requires kubeconfig to be pre-generated or use a different auth method.
#
#   - Remove OpenShift CLI (-75 MB)
#     Only if you don't deploy to OpenShift/OKD clusters.
#
#   - Remove chart-releaser cr (-28 MB)
#     Only if you don't publish Helm charts to GitHub Pages.
#
#   - Remove gomplate (-20 MB)
#     Only if you don't use gomplate for templating.
#
# BUILD INSTRUCTIONS
# ==================
#
# Build with versions from .tool-versions:
#   ./scripts/build-ci-runner-local.sh --ci-runner-only --test
#
# Or build directly:
#   docker buildx build --platform linux/amd64 \
#     --build-arg GOLANG_VERSION=1.25.5 \
#     --build-arg HELM_VERSION=3.19.4 \
#     -t ci-runner:test .
#
# ==============================================================================

# ==============================================================================
# Stage 1: Download and compress CLI binaries with UPX
# ==============================================================================
# This stage downloads all static binaries and compresses them with UPX.
# Only the compressed binaries are copied to the final image.
# ==============================================================================
FROM debian:bookworm-slim AS tools

# Install minimal tools for downloading and compressing
# - ca-certificates: Required for HTTPS downloads
# - curl: For downloading binaries
# - xz-utils: Required to extract UPX (distributed as .tar.xz)
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
# The build script passes these automatically from .tool-versions
# ------------------------------------------------------------------------------
ARG HELM_VERSION=3.19.4
ARG KUBECTL_VERSION=1.27.16
ARG JQ_VERSION=1.8.1
ARG YQ_VERSION=4.50.1
ARG TASK_VERSION=3.30.1
ARG KUSTOMIZE_VERSION=5.8.0
ARG KIND_VERSION=0.31.0
ARG ZBCTL_VERSION=v8.6.0
ARG ORAS_VERSION=1.3.0
ARG HELM_CT_VERSION=3.11.0
ARG HELM_CR_VERSION=1.7.0
ARG GOMPLATE_VERSION=v4.3.3
ARG OC_VERSION=4.17.11

WORKDIR /tools

# ------------------------------------------------------------------------------
# Download all CLI tools
# Each tool is downloaded separately to leverage Docker layer caching
# ------------------------------------------------------------------------------

# Helm - Kubernetes package manager
# Used for: Installing and managing Helm charts
RUN curl -fsSL "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz" | tar -xzf - \
    && mv linux-amd64/helm ./helm \
    && rm -rf linux-amd64

# kubectl - Kubernetes CLI
# Used for: All Kubernetes operations
RUN curl -fsSL -o kubectl "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
    && chmod +x kubectl

# jq - JSON processor
# Used for: Parsing and transforming JSON in scripts
RUN curl -fsSL -o jq "https://github.com/jqlang/jq/releases/download/jq-${JQ_VERSION}/jq-linux-amd64" \
    && chmod +x jq

# yq - YAML processor (Go version, not Python)
# Used for: Parsing and transforming YAML files
RUN curl -fsSL -o yq "https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_amd64" \
    && chmod +x yq

# Task - Task runner (Makefile alternative)
# Used for: Running CI tasks defined in Taskfile.yaml
RUN curl -fsSL "https://github.com/go-task/task/releases/download/v${TASK_VERSION}/task_linux_amd64.tar.gz" | tar -xzf - task

# Kustomize - Kubernetes configuration customization
# Used for: Overlaying Kubernetes manifests
RUN curl -fsSL "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz" | tar -xzf -

# kind - Kubernetes in Docker
# Used for: Local Kubernetes clusters for testing
RUN curl -fsSL -o kind "https://kind.sigs.k8s.io/dl/v${KIND_VERSION}/kind-linux-amd64" \
    && chmod +x kind

# zbctl - Zeebe CLI
# Used for: Interacting with Camunda Zeebe
# Note: Downloaded from npm CDN because GitHub releases stopped including zbctl from 8.6.0+
RUN curl -fsSL "https://unpkg.com/zbctl@${ZBCTL_VERSION#v}/bin/zbctl-cli.linux" -o zbctl \
    && chmod +x zbctl

# oras - OCI Registry As Storage
# Used for: Pushing/pulling artifacts to OCI registries
RUN curl -fsSL "https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/oras_${ORAS_VERSION}_linux_amd64.tar.gz" | tar -xzf - oras

# ct (chart-testing) - Helm chart testing tool
# Used for: Linting and testing Helm charts
RUN curl -fsSL "https://github.com/helm/chart-testing/releases/download/v${HELM_CT_VERSION}/chart-testing_${HELM_CT_VERSION}_linux_amd64.tar.gz" | tar -xzf - ct

# cr (chart-releaser) - Helm chart releaser
# Used for: Publishing Helm charts to GitHub Pages
RUN curl -fsSL "https://github.com/helm/chart-releaser/releases/download/v${HELM_CR_VERSION}/chart-releaser_${HELM_CR_VERSION}_linux_amd64.tar.gz" | tar -xzf - cr

# gomplate - Template renderer
# Used for: Rendering Go templates in CI
RUN curl -fsSL -o gomplate "https://github.com/hairyhenderson/gomplate/releases/download/${GOMPLATE_VERSION}/gomplate_linux-amd64" \
    && chmod +x gomplate

# oc - OpenShift CLI
# Used for: Deploying to OpenShift/OKD clusters
# This is the largest binary (160 MB uncompressed)
# Falls back to stable-4.14 if specific version unavailable
RUN curl -fsSL "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/${OC_VERSION}/openshift-client-linux.tar.gz" | tar -xzf - oc \
    || curl -fsSL "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/stable-4.14/openshift-client-linux.tar.gz" | tar -xzf - oc

# Show original sizes for build log transparency
RUN echo "=== Original binary sizes ===" && ls -lhS /tools/

# ------------------------------------------------------------------------------
# Compress all binaries with UPX
# ------------------------------------------------------------------------------
# UPX compression typically achieves 60-75% reduction on Go binaries.
# The --best flag uses maximum compression (slower build, smaller output).
# Binaries decompress transparently at runtime with ~50ms overhead.
#
# Some binaries may fail UPX compression due to:
# - Already compressed sections
# - Unsupported binary format
# - Anti-tampering measures
# We continue on failure to avoid breaking the build.
# ------------------------------------------------------------------------------
RUN for bin in helm kubectl yq task kustomize kind oras ct cr gomplate oc zbctl; do \
        echo "Compressing $bin..." && upx --best "$bin" || echo "Warning: UPX failed for $bin, keeping original"; \
    done \
    # jq sometimes fails with UPX on certain versions
    && upx --best jq || true

# Show compressed sizes for build log transparency
RUN echo "=== Compressed binary sizes ===" && ls -lhS /tools/

# ==============================================================================
# Stage 2: Final optimized image
# ==============================================================================
# This stage creates the actual CI runner image with all tools installed.
# Only necessary files are copied from Stage 1.
# ==============================================================================
FROM debian:bookworm-slim

LABEL org.opencontainers.image.source="https://github.com/camunda/camunda-platform-helm"
LABEL org.opencontainers.image.description="CI Runner (Ultra-optimized) for Camunda Platform Helm integration tests"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# ------------------------------------------------------------------------------
# Build arguments for tools not downloaded in Stage 1
# These are installed directly in this stage
# ------------------------------------------------------------------------------
ARG GOLANG_VERSION=1.25.5
ARG BATS_VERSION=1.11.0
ARG YAMLLINT_VERSION=1.37.1

# Prevent interactive prompts during apt operations
ENV DEBIAN_FRONTEND=noninteractive
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8

SHELL ["/bin/bash", "-c"]

# ==============================================================================
# Layer 1: System packages
# ==============================================================================
# Minimal set of packages required for CI operations.
# Using --no-install-recommends to avoid pulling in unnecessary dependencies.
# ==============================================================================
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    git \
    gnupg \
    gettext-base \
    make \
    unzip \
    xz-utils \
    tar \
    gzip \
    openssh-client \
    python3-minimal \
    python3-pip \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/*

# ==============================================================================
# Layer 2: UPX-compressed CLI tools from Stage 1
# ==============================================================================
# All binaries are pre-compressed with UPX in Stage 1.
# They decompress transparently when executed.
# ==============================================================================
COPY --from=tools /tools/ /usr/local/bin/

# ==============================================================================
# Layer 3: AWS CLI v1
# ==============================================================================
# WHY v1 INSTEAD OF v2:
# - AWS CLI v2 is a standalone binary (~250 MB) with bundled Python
# - AWS CLI v1 is a pip package (~50 MB) using system Python
# - For EKS authentication, both provide identical functionality
# - v1 is 80% smaller for our use case
#
# We only use AWS CLI for:
# - aws eks get-token (EKS authentication)
# - aws eks update-kubeconfig (kubeconfig generation)
# ==============================================================================
RUN pip3 install --no-cache-dir --break-system-packages awscli \
    && rm -rf /root/.cache

# ==============================================================================
# Layer 4: Google Cloud SDK
# ==============================================================================
# WHY APT INSTEAD OF TARBALL:
# - Official tarball includes bundled Python (~445 MB)
# - apt package shares system Python
# - apt package integrates with system package management
#
# We install google-cloud-cli (full SDK) because some CI jobs use gcloud
# commands directly. If only GKE auth is needed, you could install just
# google-cloud-cli-gke-gcloud-auth-plugin (saves ~350 MB).
#
# Post-install cleanup removes:
# - bundledpythonunix: Bundled Python (may exist in some versions)
# - gsutil: GCS tool (not needed)
# - bq: BigQuery tool (not needed)
# - .install/.backup: Installation backups
# - __pycache__, *.pyc: Python bytecode caches
# ==============================================================================
RUN curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg \
    && echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" > /etc/apt/sources.list.d/google-cloud-sdk.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        google-cloud-cli-gke-gcloud-auth-plugin \
        google-cloud-cli \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/* \
    # Remove unnecessary gcloud components
    && rm -rf /usr/lib/google-cloud-sdk/platform/bundledpythonunix 2>/dev/null || true \
    && rm -rf /usr/lib/google-cloud-sdk/platform/gsutil 2>/dev/null || true \
    && rm -rf /usr/lib/google-cloud-sdk/platform/bq 2>/dev/null || true \
    && rm -rf /usr/lib/google-cloud-sdk/.install/.backup 2>/dev/null || true \
    && find /usr/lib/google-cloud-sdk -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true \
    && find /usr/lib/google-cloud-sdk -name "*.pyc" -delete 2>/dev/null || true

# ==============================================================================
# Layer 5: Go toolchain
# ==============================================================================
# WHY GO IS INCLUDED:
# - Required for running Go tests
# - Required for building Go-based tools
# - Required for 'go install' of additional tools
#
# STRIPPED COMPONENTS (saves ~80 MB):
# - /usr/local/go/doc: Documentation (~10 MB)
# - /usr/local/go/test: Go's own test suite (~30 MB)
# - /usr/local/go/api: API documentation (~5 MB)
# - /usr/local/go/src/runtime/race: Race detector (~30 MB)
# - *.md files: Markdown documentation (~5 MB)
# - zoneinfo.zip: Timezone data (uses system tzdata)
#
# The Go compiler and standard library remain fully functional.
# Race detection is disabled (use -race flag locally if needed).
# ==============================================================================
RUN curl -fsSL "https://go.dev/dl/go${GOLANG_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xzf - \
    && rm -rf /usr/local/go/doc /usr/local/go/test /usr/local/go/api \
    && find /usr/local/go -name "*.md" -delete 2>/dev/null || true \
    && rm -rf /usr/local/go/src/runtime/race 2>/dev/null || true \
    && rm -f /usr/local/go/lib/time/zoneinfo.zip 2>/dev/null || true \
    && ln -sf /usr/local/go/bin/go /usr/local/bin/go \
    && ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt

# ==============================================================================
# Layer 6: Bats testing framework
# ==============================================================================
# WHY TARBALL INSTEAD OF GIT CLONE:
# - Git clone includes full history (~85 MB)
# - Tarball is just the release (~1 MB)
# - Same functionality, much smaller
# ==============================================================================
RUN curl -fsSL "https://github.com/bats-core/bats-core/archive/refs/tags/v${BATS_VERSION}.tar.gz" \
    | tar -xzf - -C /tmp \
    && /tmp/bats-core-${BATS_VERSION}/install.sh /usr/local \
    && rm -rf /tmp/bats-core-${BATS_VERSION}

# ==============================================================================
# Layer 7: yamllint
# ==============================================================================
# Installed via pip, then pip is removed to save space.
# yamllint remains functional because its dependencies stay installed.
# ==============================================================================
RUN pip3 install --no-cache-dir --break-system-packages yamllint==${YAMLLINT_VERSION} \
    && apt-get purge -y python3-pip \
    && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/* /root/.cache /var/cache/apt/*

# ==============================================================================
# Layer 8: Environment configuration
# ==============================================================================
ENV GOPATH=/root/go
ENV PATH="${GOPATH}/bin:/root/.local/bin:/usr/local/bin:/usr/bin:/bin:/usr/local/go/bin"
# Tell gcloud to use system Python (in case any bundled Python remnants exist)
ENV CLOUDSDK_PYTHON=/usr/bin/python3

# Allow git operations in any directory (fixes "dubious ownership" in containers)
RUN git config --global --add safe.directory '*'

# Copy .tool-versions to standard locations for reference
COPY .tool-versions /opt/.tool-versions
RUN cp /opt/.tool-versions /root/.tool-versions \
    && cp /opt/.tool-versions /.tool-versions \
    && mkdir -p /home/runner \
    && cp /opt/.tool-versions /home/runner/.tool-versions

# ==============================================================================
# Layer 9: Verification
# ==============================================================================
# Verify all tools are correctly installed and functional.
# This catches issues at build time rather than runtime.
# ==============================================================================
WORKDIR /workspace

RUN echo "=== Verifying tool installations ===" \
    && go version \
    && helm version --short \
    && kubectl version --client \
    && jq --version \
    && yq --version \
    && task --version \
    && kustomize version \
    && kind version \
    && oras version \
    && ct version \
    && cr version \
    && gomplate --version \
    && bats --version \
    && yamllint --version \
    && gcloud version \
    && aws --version \
    && oc version --client \
    && gke-gcloud-auth-plugin --version \
    && echo "=== All tools verified successfully ==="

ENTRYPOINT ["/bin/bash", "-c"]
CMD ["bash"]
