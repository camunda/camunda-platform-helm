# Playwright Runner Image for Camunda Platform Helm Integration Tests
# Extends the official Playwright image with additional CI tools
#
# Build: docker build -t registry.camunda.cloud/team-distribution/playwright-runner:latest .
# Push:  docker push registry.camunda.cloud/team-distribution/playwright-runner:latest
#
# Layer order (most stable first):
# 1. Base system packages (rarely change)
# 2. Cloud CLIs - gcloud, aws (rarely change)
# 3. asdf installation (rarely changes)
# 4. asdf plugins (occasionally change)
# 5. Tool installations from .tool-versions (change when versions update)
# 6. Shell configuration (rarely changes)

FROM mcr.microsoft.com/playwright:v1.57.0-noble

LABEL org.opencontainers.image.source="https://github.com/camunda/camunda-platform-helm"
LABEL org.opencontainers.image.description="Playwright Runner for Camunda Platform Helm integration tests"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# Switch to root for installations
USER root

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
# Playwright runner only needs a subset of tools
# =============================================================================
WORKDIR /opt/asdf

RUN asdf plugin add helm || true
RUN asdf plugin add kubectl || true
RUN asdf plugin add oc || true
RUN asdf plugin add task https://github.com/particledecay/asdf-task.git || true
RUN asdf plugin add yq || true
RUN asdf plugin add jq || true
RUN asdf plugin add zbctl || true

# =============================================================================
# Layer 5: Tool versions (changes when .tool-versions updates)
# Copy full .tool-versions but only install what we need
# =============================================================================
COPY .tool-versions /tmp/.tool-versions-full

# Extract only the tools we need for Playwright runner
RUN grep -E "^(helm|kubectl|oc|task|yq|jq|zbctl) " /tmp/.tool-versions-full > /opt/asdf/.tool-versions \
    && cat /opt/asdf/.tool-versions

# Install tools individually for better layer caching
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

RUN asdf install oc $(grep "^oc " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global oc $(grep "^oc " /opt/asdf/.tool-versions | awk '{print $2}')

RUN asdf install zbctl $(grep "^zbctl " /opt/asdf/.tool-versions | awk '{print $2}') \
    && asdf global zbctl $(grep "^zbctl " /opt/asdf/.tool-versions | awk '{print $2}')

# Reshim to ensure all binaries are available
RUN asdf reshim

# =============================================================================
# Layer 6: Symlink tools to /usr/local/bin for standard PATH access
# This ensures tools work without asdf environment variables being set
# =============================================================================
RUN ln -sf /opt/asdf/shims/helm /usr/local/bin/helm \
    && ln -sf /opt/asdf/shims/kubectl /usr/local/bin/kubectl \
    && ln -sf /opt/asdf/shims/task /usr/local/bin/task \
    && ln -sf /opt/asdf/shims/yq /usr/local/bin/yq \
    && ln -sf /opt/asdf/shims/jq /usr/local/bin/jq \
    && ln -sf /opt/asdf/shims/oc /usr/local/bin/oc \
    && ln -sf /opt/asdf/shims/zbctl /usr/local/bin/zbctl

# =============================================================================
# Layer 7: Configuration (stable - rarely changes)
# =============================================================================
# Copy .tool-versions to common locations
RUN cp /opt/asdf/.tool-versions /root/.tool-versions \
    && cp /opt/asdf/.tool-versions /.tool-versions \
    && cp /opt/asdf/.tool-versions /home/pwuser/.tool-versions 2>/dev/null || true

# Set up for GitHub Actions runner user (uid 1001)
RUN mkdir -p /home/runner \
    && cp /opt/asdf/.tool-versions /home/runner/.tool-versions

# =============================================================================
# Layer 8: Verification and final setup
# =============================================================================
WORKDIR /workspace

# Verify critical tools work via /usr/local/bin symlinks
RUN echo "Verifying tool installations..." \
    && /usr/local/bin/helm version --short \
    && /usr/local/bin/kubectl version --client \
    && /usr/local/bin/task --version \
    && /usr/local/bin/yq --version \
    && /usr/local/bin/jq --version \
    && echo "✅ All tools verified successfully"

# Keep root user as default for GitHub Actions (--user root in container options)
USER root

ENTRYPOINT ["/bin/bash", "-c"]
CMD ["bash"]
