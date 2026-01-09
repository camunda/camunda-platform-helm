# CI Runner Image for Camunda Platform Helm Integration Tests
# This image contains all tools pre-installed to avoid runtime downloads
# Tools are installed directly (not via asdf) for reliable PATH access in GitHub Actions
#
# Build with versions from .tool-versions:
#   ./scripts/build-ci-runner-local.sh --test
#
# Or manually:
#   docker build --build-arg GOLANG_VERSION=1.25.5 --build-arg HELM_VERSION=3.19.4 ... -t ci-runner .

FROM ubuntu:24.04

LABEL org.opencontainers.image.source="https://github.com/camunda/camunda-platform-helm"
LABEL org.opencontainers.image.description="CI Runner for Camunda Platform Helm integration tests"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# =============================================================================
# Build arguments for tool versions (passed from build script)
# These enable proper layer caching - layers only rebuild when versions change
# =============================================================================
ARG GOLANG_VERSION=1.25.5
ARG HELM_VERSION=3.19.4
ARG KUBECTL_VERSION=1.27.16
ARG JQ_VERSION=1.8.1
ARG YQ_VERSION=4.50.1
ARG TASK_VERSION=3.30.1
ARG KUSTOMIZE_VERSION=5.8.0
ARG OC_VERSION=4.17.11
ARG KIND_VERSION=0.31.0
ARG ZBCTL_VERSION=v8.6.0
ARG ORAS_VERSION=1.3.0
ARG HELM_CT_VERSION=3.11.0
ARG HELM_CR_VERSION=1.7.0
ARG GOMPLATE_VERSION=v4.3.3
ARG BATS_VERSION=1.11.0
ARG YAMLLINT_VERSION=1.37.1

# Avoid interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Set bash as default shell
SHELL ["/bin/bash", "-c"]

# =============================================================================
# Layer 1: System packages (very stable - rarely changes)
# =============================================================================
RUN rm -rf /var/lib/apt/lists/* \
    && apt-get clean \
    && apt-get update \
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
    python3-venv \
    pipx \
    && rm -rf /var/lib/apt/lists/*

# =============================================================================
# Layer 2: Cloud CLIs (stable - rarely changes)
# =============================================================================
# Google Cloud CLI for GKE authentication
RUN echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" \
    | tee -a /etc/apt/sources.list.d/google-cloud-sdk.list \
    && curl https://packages.cloud.google.com/apt/doc/apt-key.gpg \
    | gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg \
    && apt-get update && apt-get install -y --no-install-recommends \
    google-cloud-cli \
    google-cloud-cli-gke-gcloud-auth-plugin \
    && rm -rf /var/lib/apt/lists/*

# AWS CLI for EKS authentication
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" \
    && unzip awscliv2.zip \
    && ./aws/install \
    && rm -rf awscliv2.zip aws

# =============================================================================
# Layer 3: Python-based tools via pipx (version in ARG for caching)
# =============================================================================
RUN pipx install yamllint==${YAMLLINT_VERSION} && pipx ensurepath
ENV PATH="/root/.local/bin:${PATH}"

# =============================================================================
# Layer 4: Install tools directly from releases
# Each tool uses ARG for version - only rebuilds when that version changes
# =============================================================================

# Go
RUN curl -fsSL "https://go.dev/dl/go${GOLANG_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xzf - \
    && ln -sf /usr/local/go/bin/go /usr/local/bin/go \
    && ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt

# Helm
RUN curl -fsSL "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz" | tar -xzf - \
    && mv linux-amd64/helm /usr/local/bin/helm \
    && rm -rf linux-amd64

# kubectl
RUN curl -fsSL -o /usr/local/bin/kubectl "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
    && chmod +x /usr/local/bin/kubectl

# jq
RUN curl -fsSL -o /usr/local/bin/jq "https://github.com/jqlang/jq/releases/download/jq-${JQ_VERSION}/jq-linux-amd64" \
    && chmod +x /usr/local/bin/jq

# yq
RUN curl -fsSL -o /usr/local/bin/yq "https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_amd64" \
    && chmod +x /usr/local/bin/yq

# task (taskfile)
RUN curl -fsSL "https://github.com/go-task/task/releases/download/v${TASK_VERSION}/task_linux_amd64.tar.gz" | tar -xzf - -C /usr/local/bin task

# kustomize
RUN curl -fsSL "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz" | tar -xzf - -C /usr/local/bin

# oc (OpenShift CLI)
RUN curl -fsSL "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/${OC_VERSION}/openshift-client-linux.tar.gz" | tar -xzf - -C /usr/local/bin oc kubectl.1 || \
    curl -fsSL "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/stable-4.14/openshift-client-linux.tar.gz" | tar -xzf - -C /usr/local/bin oc

# kind
RUN curl -fsSL -o /usr/local/bin/kind "https://kind.sigs.k8s.io/dl/v${KIND_VERSION}/kind-linux-amd64" \
    && chmod +x /usr/local/bin/kind

# zbctl (ZBCTL_VERSION already has 'v' prefix)
RUN curl -fsSL "https://github.com/camunda/camunda/releases/download/${ZBCTL_VERSION}/zbctl" -o /usr/local/bin/zbctl \
    && chmod +x /usr/local/bin/zbctl || true

# oras
RUN curl -fsSL "https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/oras_${ORAS_VERSION}_linux_amd64.tar.gz" | tar -xzf - -C /usr/local/bin oras

# helm-ct (chart-testing)
RUN curl -fsSL "https://github.com/helm/chart-testing/releases/download/v${HELM_CT_VERSION}/chart-testing_${HELM_CT_VERSION}_linux_amd64.tar.gz" | tar -xzf - -C /usr/local/bin ct

# helm-cr (chart-releaser)
RUN curl -fsSL "https://github.com/helm/chart-releaser/releases/download/v${HELM_CR_VERSION}/chart-releaser_${HELM_CR_VERSION}_linux_amd64.tar.gz" | tar -xzf - -C /usr/local/bin cr

# gomplate (GOMPLATE_VERSION already has 'v' prefix)
RUN curl -fsSL -o /usr/local/bin/gomplate "https://github.com/hairyhenderson/gomplate/releases/download/${GOMPLATE_VERSION}/gomplate_linux-amd64" \
    && chmod +x /usr/local/bin/gomplate

# bats
RUN git clone --depth 1 --branch v${BATS_VERSION} https://github.com/bats-core/bats-core.git /tmp/bats \
    && /tmp/bats/install.sh /usr/local \
    && rm -rf /tmp/bats

# =============================================================================
# Layer 5: Configuration
# =============================================================================
# Set up Go environment
ENV GOPATH=/root/go
ENV PATH="${GOPATH}/bin:/root/.local/bin:/usr/local/bin:/usr/bin:/bin:/usr/local/go/bin"

# Configure git to allow any directory (fixes "dubious ownership" error in containers)
RUN git config --global --add safe.directory '*'

# Copy .tool-versions to common locations (for reference only, not used during build)
COPY .tool-versions /opt/.tool-versions
RUN cp /opt/.tool-versions /root/.tool-versions \
    && cp /opt/.tool-versions /.tool-versions \
    && mkdir -p /home/runner \
    && cp /opt/.tool-versions /home/runner/.tool-versions

# =============================================================================
# Layer 6: Verification
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
    && echo "✅ All tools verified successfully"

ENTRYPOINT ["/bin/bash", "-c"]
CMD ["bash"]

