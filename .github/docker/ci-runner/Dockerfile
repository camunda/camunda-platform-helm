# CI Runner Image for Camunda Platform Helm Integration Tests
# This image contains all tools from .tool-versions pre-installed to avoid runtime downloads
#
# Build: docker build -t registry.camunda.cloud/team-distribution/ci-runner:latest .
# Push:  docker push registry.camunda.cloud/team-distribution/ci-runner:latest
#
# Layer order (most stable first):
# 1. Base system packages (rarely change)
# 2. Cloud CLIs - gcloud, aws (rarely change)
# 3. asdf installation (rarely changes)
# 4. asdf plugins (occasionally change)
# 5. Tool installations from .tool-versions (change when versions update)
# 6. Shell configuration (rarely changes)

FROM ubuntu:24.04

LABEL org.opencontainers.image.source="https://github.com/camunda/camunda-platform-helm"
LABEL org.opencontainers.image.description="CI Runner for Camunda Platform Helm integration tests"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# Avoid interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Set bash as default shell early
SHELL ["/bin/bash", "-c"]

# =============================================================================
# Layer 1: System packages (very stable - rarely changes)
# =============================================================================
RUN apt-get update && apt-get install -y --no-install-recommends \
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
# Layer 3: asdf installation (stable - rarely changes)
# =============================================================================
ENV ASDF_DIR=/opt/asdf
ENV ASDF_DATA_DIR=/opt/asdf
ENV PATH="${ASDF_DIR}/shims:${ASDF_DIR}/bin:${PATH}"

RUN git clone https://github.com/asdf-vm/asdf.git ${ASDF_DIR} --branch v0.15.0

# =============================================================================
# Layer 4: asdf plugins (moderately stable - occasionally changes)
# =============================================================================
WORKDIR /opt/asdf

RUN asdf plugin add golang || true
RUN asdf plugin add helm || true
RUN asdf plugin add kubectl || true
RUN asdf plugin add kustomize || true
RUN asdf plugin add oc || true
RUN asdf plugin add task https://github.com/particledecay/asdf-task.git || true
RUN asdf plugin add yq || true
RUN asdf plugin add jq || true
RUN asdf plugin add bats || true
RUN asdf plugin add zbctl || true
RUN asdf plugin add oras || true
RUN asdf plugin add kind || true
RUN asdf plugin add helm-ct || true
RUN asdf plugin add helm-cr || true
RUN asdf plugin add gomplate || true

# =============================================================================
# Layer 5: Python-based tools via pipx (separate layer for caching)
# =============================================================================
RUN pipx install yamllint==1.37.1 && pipx ensurepath
ENV PATH="/root/.local/bin:${PATH}"

# =============================================================================
# Layer 6: Tool versions (changes when .tool-versions updates)
# This is the layer that changes most frequently
# =============================================================================
COPY .tool-versions /opt/asdf/.tool-versions

# Install tools individually for better layer caching on failures
# Core tools (most critical)
RUN asdf install golang $(grep "^golang " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global golang $(grep "^golang " /opt/asdf/.tool-versions | awk '{print $2}')

RUN asdf install helm $(grep "^helm " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global helm $(grep "^helm " /opt/asdf/.tool-versions | awk '{print $2}')

RUN asdf install kubectl $(grep "^kubectl " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global kubectl $(grep "^kubectl " /opt/asdf/.tool-versions | awk '{print $2}')

RUN asdf install task $(grep "^task " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global task $(grep "^task " /opt/asdf/.tool-versions | awk '{print $2}')

RUN asdf install yq $(grep "^yq " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global yq $(grep "^yq " /opt/asdf/.tool-versions | awk '{print $2}')

RUN asdf install jq $(grep "^jq " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global jq $(grep "^jq " /opt/asdf/.tool-versions | awk '{print $2}')

# Secondary tools
RUN asdf install oc $(grep "^oc " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global oc $(grep "^oc " /opt/asdf/.tool-versions | awk '{print $2}')

RUN asdf install kustomize $(grep "^kustomize " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global kustomize $(grep "^kustomize " /opt/asdf/.tool-versions | awk '{print $2}')

RUN asdf install zbctl $(grep "^zbctl " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global zbctl $(grep "^zbctl " /opt/asdf/.tool-versions | awk '{print $2}')

RUN asdf install bats $(grep "^bats " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global bats $(grep "^bats " /opt/asdf/.tool-versions | awk '{print $2}')

RUN asdf install oras $(grep "^oras " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global oras $(grep "^oras " /opt/asdf/.tool-versions | awk '{print $2}')

RUN asdf install kind $(grep "^kind " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global kind $(grep "^kind " /opt/asdf/.tool-versions | awk '{print $2}')

RUN asdf install helm-ct $(grep "^helm-ct " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global helm-ct $(grep "^helm-ct " /opt/asdf/.tool-versions | awk '{print $2}')

RUN asdf install helm-cr $(grep "^helm-cr " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global helm-cr $(grep "^helm-cr " /opt/asdf/.tool-versions | awk '{print $2}')

RUN asdf install gomplate $(grep "^gomplate " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global gomplate $(grep "^gomplate " /opt/asdf/.tool-versions | awk '{print $2}')

# Reshim to ensure all binaries are available
RUN asdf reshim

# =============================================================================
# Layer 7: Symlink tools to /usr/local/bin for standard PATH access
# This ensures tools work without asdf environment variables being set
# =============================================================================
RUN ln -sf /opt/asdf/shims/go /usr/local/bin/go \
    && ln -sf /opt/asdf/shims/gofmt /usr/local/bin/gofmt \
    && ln -sf /opt/asdf/shims/helm /usr/local/bin/helm \
    && ln -sf /opt/asdf/shims/kubectl /usr/local/bin/kubectl \
    && ln -sf /opt/asdf/shims/task /usr/local/bin/task \
    && ln -sf /opt/asdf/shims/yq /usr/local/bin/yq \
    && ln -sf /opt/asdf/shims/jq /usr/local/bin/jq \
    && ln -sf /opt/asdf/shims/oc /usr/local/bin/oc \
    && ln -sf /opt/asdf/shims/kustomize /usr/local/bin/kustomize \
    && ln -sf /opt/asdf/shims/zbctl /usr/local/bin/zbctl \
    && ln -sf /opt/asdf/shims/bats /usr/local/bin/bats \
    && ln -sf /opt/asdf/shims/oras /usr/local/bin/oras \
    && ln -sf /opt/asdf/shims/kind /usr/local/bin/kind \
    && ln -sf /opt/asdf/shims/ct /usr/local/bin/ct \
    && ln -sf /opt/asdf/shims/cr /usr/local/bin/cr \
    && ln -sf /opt/asdf/shims/gomplate /usr/local/bin/gomplate

# =============================================================================
# Layer 8: Configuration (stable - rarely changes)
# =============================================================================
# Copy .tool-versions to common locations
RUN cp /opt/asdf/.tool-versions /root/.tool-versions \
    && cp /opt/asdf/.tool-versions /.tool-versions

# Set up Go environment
ENV GOPATH=/root/go
ENV PATH="${GOPATH}/bin:/root/.local/bin:/usr/local/bin:/usr/bin:/bin"

# Set up for GitHub Actions runner user (uid 1001)
RUN mkdir -p /home/runner \
    && cp /opt/asdf/.tool-versions /home/runner/.tool-versions

# =============================================================================
# Layer 9: Verification and final setup (changes rarely)
# =============================================================================
WORKDIR /workspace

# Verify critical tools work via /usr/local/bin symlinks
RUN echo "Verifying tool installations..." \
    && /usr/local/bin/helm version --short \
    && /usr/local/bin/kubectl version --client \
    && /usr/local/bin/task --version \
    && /usr/local/bin/yq --version \
    && /usr/local/bin/jq --version \
    && /usr/local/bin/go version \
    && yamllint --version \
    && echo "✅ All tools verified successfully"

ENTRYPOINT ["/bin/bash", "-c"]
CMD ["bash"]
