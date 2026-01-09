# Playwright Runner Image for Camunda Platform Helm Integration Tests
# Extends the official Playwright image with additional CI tools
# Tools are installed directly (not via asdf) for reliable PATH access in GitHub Actions
#
# Build: docker build -t registry.camunda.cloud/team-distribution/playwright-runner:latest .
# Push:  docker push registry.camunda.cloud/team-distribution/playwright-runner:latest

FROM mcr.microsoft.com/playwright:v1.57.0-noble

LABEL org.opencontainers.image.source="https://github.com/camunda/camunda-platform-helm"
LABEL org.opencontainers.image.description="Playwright Runner for Camunda Platform Helm integration tests"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# Switch to root for installations
USER root

# Avoid interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Set bash as default shell
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
# Layer 3: Copy .tool-versions for version reference
# =============================================================================
COPY .tool-versions /opt/.tool-versions

# =============================================================================
# Layer 4: Install tools directly from releases (only tools needed for Playwright)
# =============================================================================

# Helm
RUN HELM_VERSION=$(grep "^helm " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz" | tar -xzf - \
    && mv linux-amd64/helm /usr/local/bin/helm \
    && rm -rf linux-amd64

# kubectl
RUN KUBECTL_VERSION=$(grep "^kubectl " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL -o /usr/local/bin/kubectl "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
    && chmod +x /usr/local/bin/kubectl

# jq
RUN JQ_VERSION=$(grep "^jq " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL -o /usr/local/bin/jq "https://github.com/jqlang/jq/releases/download/jq-${JQ_VERSION}/jq-linux-amd64" \
    && chmod +x /usr/local/bin/jq

# yq
RUN YQ_VERSION=$(grep "^yq " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL -o /usr/local/bin/yq "https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_amd64" \
    && chmod +x /usr/local/bin/yq

# task (taskfile)
RUN TASK_VERSION=$(grep "^task " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL "https://github.com/go-task/task/releases/download/v${TASK_VERSION}/task_linux_amd64.tar.gz" | tar -xzf - -C /usr/local/bin task

# oc (OpenShift CLI)
RUN OC_VERSION=$(grep "^oc " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/${OC_VERSION}/openshift-client-linux.tar.gz" | tar -xzf - -C /usr/local/bin oc kubectl.1 || \
    curl -fsSL "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/stable-4.14/openshift-client-linux.tar.gz" | tar -xzf - -C /usr/local/bin oc

# zbctl (version in .tool-versions already has 'v' prefix)
RUN ZBCTL_VERSION=$(grep "^zbctl " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL -o /usr/local/bin/zbctl "https://github.com/camunda/zeebe/releases/download/${ZBCTL_VERSION}/zbctl.tar.gz" \
    && tar -xzf /usr/local/bin/zbctl -C /tmp \
    && mv /tmp/zbctl /usr/local/bin/zbctl \
    && chmod +x /usr/local/bin/zbctl || \
    curl -fsSL -o /usr/local/bin/zbctl "https://github.com/camunda/camunda/releases/download/${ZBCTL_VERSION}/zbctl" \
    && chmod +x /usr/local/bin/zbctl || true

# =============================================================================
# Layer 5: Configuration
# =============================================================================
# Copy .tool-versions to common locations
RUN cp /opt/.tool-versions /root/.tool-versions \
    && cp /opt/.tool-versions /.tool-versions \
    && mkdir -p /home/runner \
    && cp /opt/.tool-versions /home/runner/.tool-versions \
    && cp /opt/.tool-versions /home/pwuser/.tool-versions 2>/dev/null || true

# =============================================================================
# Layer 6: Verification
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

