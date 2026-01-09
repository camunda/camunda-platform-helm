# Playwright Runner Image for Camunda Platform Helm Integration Tests
# Extends the official Playwright image with additional CI tools
# Tools are installed directly (not via asdf) for reliable PATH access in GitHub Actions
#
# Build with versions from .tool-versions:
#   ./scripts/build-ci-runner-local.sh --playwright-only --test

FROM mcr.microsoft.com/playwright:v1.57.0-noble

LABEL org.opencontainers.image.source="https://github.com/camunda/camunda-platform-helm"
LABEL org.opencontainers.image.description="Playwright Runner for Camunda Platform Helm integration tests"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# =============================================================================
# Build arguments for tool versions (passed from build script)
# These enable proper layer caching - layers only rebuild when versions change
# =============================================================================
ARG HELM_VERSION=3.19.4
ARG KUBECTL_VERSION=1.27.16
ARG JQ_VERSION=1.8.1
ARG YQ_VERSION=4.50.1
ARG TASK_VERSION=3.30.1
ARG OC_VERSION=4.17.11
ARG ZBCTL_VERSION=v8.6.0

# Switch to root for installations
USER root

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
# Layer 3: Install tools directly from releases (only tools needed for Playwright)
# Each tool uses ARG for version - only rebuilds when that version changes
# =============================================================================

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

# oc (OpenShift CLI)
RUN curl -fsSL "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/${OC_VERSION}/openshift-client-linux.tar.gz" | tar -xzf - -C /usr/local/bin oc kubectl.1 || \
    curl -fsSL "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/stable-4.14/openshift-client-linux.tar.gz" | tar -xzf - -C /usr/local/bin oc

# zbctl (ZBCTL_VERSION already has 'v' prefix)
RUN curl -fsSL "https://github.com/camunda/camunda/releases/download/${ZBCTL_VERSION}/zbctl" -o /usr/local/bin/zbctl \
    && chmod +x /usr/local/bin/zbctl || true

# =============================================================================
# Layer 4: Configuration
# =============================================================================
# Copy .tool-versions to common locations (for reference only, not used during build)
COPY .tool-versions /opt/.tool-versions
RUN cp /opt/.tool-versions /root/.tool-versions \
    && cp /opt/.tool-versions /.tool-versions \
    && mkdir -p /home/runner \
    && cp /opt/.tool-versions /home/runner/.tool-versions \
    && cp /opt/.tool-versions /home/pwuser/.tool-versions 2>/dev/null || true

# =============================================================================
# Layer 5: Verification
# =============================================================================
WORKDIR /workspace

# Verify critical tools work
RUN echo "Verifying tool installations..." \
    && helm version --short \
    && kubectl version --client \
    && jq --version \
    && yq --version \
    && task --version \
    && echo "✅ All tools verified successfully"

# Keep root user as default for GitHub Actions
USER root

ENTRYPOINT ["/bin/bash", "-c"]
CMD ["bash"]

