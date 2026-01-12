# CI Runner Image for Camunda Platform Helm Integration Tests
# Optimized multi-stage build for reduced image size (~35% smaller)
#
# This image contains all tools pre-installed to avoid runtime downloads.
# Tools are installed directly for reliable PATH access in GitHub Actions.
#
# Build with versions from .tool-versions:
#   ./scripts/build-ci-runner-local.sh --test
#
# Or manually:
#   docker build --build-arg GOLANG_VERSION=1.25.5 --build-arg HELM_VERSION=3.19.4 ... -t ci-runner .

# =============================================================================
# Stage 1: Download static binaries
# =============================================================================
FROM debian:bookworm-slim AS downloader

# Build arguments for tool versions
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

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Download all static binaries in a single layer for efficiency
RUN mkdir -p /tools \
    # Helm
    && curl -fsSL "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz" | tar -xzf - \
    && mv linux-amd64/helm /tools/ \
    && rm -rf linux-amd64 \
    # kubectl
    && curl -fsSL -o /tools/kubectl "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
    # jq
    && curl -fsSL -o /tools/jq "https://github.com/jqlang/jq/releases/download/jq-${JQ_VERSION}/jq-linux-amd64" \
    # yq
    && curl -fsSL -o /tools/yq "https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_amd64" \
    # task (taskfile)
    && curl -fsSL "https://github.com/go-task/task/releases/download/v${TASK_VERSION}/task_linux_amd64.tar.gz" | tar -xzf - -C /tools task \
    # kustomize
    && curl -fsSL "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz" | tar -xzf - -C /tools \
    # kind
    && curl -fsSL -o /tools/kind "https://kind.sigs.k8s.io/dl/v${KIND_VERSION}/kind-linux-amd64" \
    # zbctl - Download from npm CDN (GitHub releases stopped including zbctl binary from 8.6.0+)
    && curl -fsSL "https://unpkg.com/zbctl@${ZBCTL_VERSION#v}/bin/zbctl-cli.linux" -o /tools/zbctl \
    # oras
    && curl -fsSL "https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/oras_${ORAS_VERSION}_linux_amd64.tar.gz" | tar -xzf - -C /tools oras \
    # helm-ct (chart-testing)
    && curl -fsSL "https://github.com/helm/chart-testing/releases/download/v${HELM_CT_VERSION}/chart-testing_${HELM_CT_VERSION}_linux_amd64.tar.gz" | tar -xzf - -C /tools ct \
    # helm-cr (chart-releaser)
    && curl -fsSL "https://github.com/helm/chart-releaser/releases/download/v${HELM_CR_VERSION}/chart-releaser_${HELM_CR_VERSION}_linux_amd64.tar.gz" | tar -xzf - -C /tools cr \
    # gomplate (GOMPLATE_VERSION already has 'v' prefix)
    && curl -fsSL -o /tools/gomplate "https://github.com/hairyhenderson/gomplate/releases/download/${GOMPLATE_VERSION}/gomplate_linux-amd64" \
    # Make all binaries executable
    && chmod +x /tools/*

# =============================================================================
# Stage 2: Final image
# =============================================================================
FROM debian:bookworm-slim

LABEL org.opencontainers.image.source="https://github.com/camunda/camunda-platform-helm"
LABEL org.opencontainers.image.description="CI Runner for Camunda Platform Helm integration tests"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# =============================================================================
# Build arguments for tool versions (passed from build script)
# =============================================================================
ARG GOLANG_VERSION=1.25.5
ARG OC_VERSION=4.17.11
ARG BATS_VERSION=1.11.0
ARG YAMLLINT_VERSION=1.37.1

# Avoid interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Set bash as default shell
SHELL ["/bin/bash", "-c"]

# =============================================================================
# Layer 1: System packages (minimal set)
# =============================================================================
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    git \
    gnupg \
    gettext-base \
    make \
    unzip \
    wget \
    xz-utils \
    tar \
    gzip \
    openssh-client \
    python3 \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

# =============================================================================
# Layer 2: Copy pre-downloaded static binaries from downloader stage
# =============================================================================
COPY --from=downloader /tools/ /usr/local/bin/

# =============================================================================
# Layer 3: Google Cloud CLI (standalone tarball, stripped down)
# Remove bundled Python (~445 MB) and unused tools (gsutil, bq) to save ~509 MB
# =============================================================================
RUN curl -fsSL "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-linux-x86_64.tar.gz" \
    | tar -xzf - -C /opt \
    && /opt/google-cloud-sdk/install.sh --quiet --path-update=false --command-completion=false --usage-reporting=false \
    && /opt/google-cloud-sdk/bin/gcloud components install gke-gcloud-auth-plugin --quiet \
    # Remove bundled Python - use system Python3 instead (saves ~445 MB)
    && rm -rf /opt/google-cloud-sdk/platform/bundledpythonunix \
    # Remove unused tools - we only need gcloud for kubeconfig generation
    && rm -rf /opt/google-cloud-sdk/platform/gsutil \
    && rm -rf /opt/google-cloud-sdk/platform/bq \
    # Cleanup installation artifacts
    && rm -rf /opt/google-cloud-sdk/.install/.backup \
    && rm -rf /opt/google-cloud-sdk/.install/.download \
    && find /opt/google-cloud-sdk -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true \
    && find /opt/google-cloud-sdk -name "*.pyc" -delete 2>/dev/null || true \
    # Create symlinks
    && ln -s /opt/google-cloud-sdk/bin/gcloud /usr/local/bin/gcloud \
    && ln -s /opt/google-cloud-sdk/bin/gke-gcloud-auth-plugin /usr/local/bin/gke-gcloud-auth-plugin

# =============================================================================
# Layer 4: AWS CLI for EKS authentication
# =============================================================================
RUN curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" \
    && unzip -q awscliv2.zip \
    && ./aws/install \
    && rm -rf awscliv2.zip aws

# =============================================================================
# Layer 5: Go (stripped of docs/tests to save ~54 MB)
# =============================================================================
RUN curl -fsSL "https://go.dev/dl/go${GOLANG_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xzf - \
    && rm -rf /usr/local/go/doc /usr/local/go/test /usr/local/go/api \
    && find /usr/local/go -name "*.md" -delete 2>/dev/null || true \
    && ln -sf /usr/local/go/bin/go /usr/local/bin/go \
    && ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt

# =============================================================================
# Layer 6: OpenShift CLI (oc)
# =============================================================================
RUN curl -fsSL "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/${OC_VERSION}/openshift-client-linux.tar.gz" | tar -xzf - -C /usr/local/bin oc kubectl.1 || \
    curl -fsSL "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/stable-4.14/openshift-client-linux.tar.gz" | tar -xzf - -C /usr/local/bin oc

# =============================================================================
# Layer 7: Bats (from tarball instead of git clone - saves ~85 MB)
# =============================================================================
RUN curl -fsSL "https://github.com/bats-core/bats-core/archive/refs/tags/v${BATS_VERSION}.tar.gz" \
    | tar -xzf - -C /tmp \
    && /tmp/bats-core-${BATS_VERSION}/install.sh /usr/local \
    && rm -rf /tmp/bats-core-${BATS_VERSION}

# =============================================================================
# Layer 8: yamllint (minimal Python install, then purge pip - saves ~135 MB)
# =============================================================================
RUN pip3 install --no-cache-dir --break-system-packages yamllint==${YAMLLINT_VERSION} \
    && apt-get purge -y python3-pip \
    && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/* /root/.cache

# =============================================================================
# Layer 9: Configuration
# =============================================================================
# Set up Go environment
ENV GOPATH=/root/go
ENV PATH="${GOPATH}/bin:/root/.local/bin:/usr/local/bin:/usr/bin:/bin:/usr/local/go/bin"

# Configure gcloud to use system Python (since we removed bundled Python)
ENV CLOUDSDK_PYTHON=/usr/bin/python3

# Configure git to allow any directory (fixes "dubious ownership" error in containers)
RUN git config --global --add safe.directory '*'

# Copy .tool-versions to common locations (for reference only, not used during build)
COPY .tool-versions /opt/.tool-versions
RUN cp /opt/.tool-versions /root/.tool-versions \
    && cp /opt/.tool-versions /.tool-versions \
    && mkdir -p /home/runner \
    && cp /opt/.tool-versions /home/runner/.tool-versions

# =============================================================================
# Layer 10: Verification
# =============================================================================
WORKDIR /workspace

# Verify critical tools work
RUN echo "Verifying tool installations..." \
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
    && echo "All tools verified successfully"

ENTRYPOINT ["/bin/bash", "-c"]
CMD ["bash"]
