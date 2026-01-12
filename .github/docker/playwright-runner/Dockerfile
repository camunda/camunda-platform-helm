# Playwright Runner Image for Camunda Platform Helm Integration Tests
# Optimized single-browser (Chromium) image - significantly smaller than official multi-browser image
#
# Size comparison:
#   - Official Playwright image (3 browsers): ~3.4 GB
#   - This optimized image (Chromium only):   ~1.5 GB (estimated)
#
# Build with versions from .tool-versions:
#   ./scripts/build-ci-runner-local.sh --playwright-only --test

FROM ubuntu:24.04

LABEL org.opencontainers.image.source="https://github.com/camunda/camunda-platform-helm"
LABEL org.opencontainers.image.description="Playwright Runner (Chromium-only) for Camunda Platform Helm integration tests"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# =============================================================================
# Build arguments for tool versions (passed from build script)
# =============================================================================
ARG HELM_VERSION=3.19.4
ARG KUBECTL_VERSION=1.27.16
ARG JQ_VERSION=1.8.1
ARG YQ_VERSION=4.50.1
ARG TASK_VERSION=3.30.1
ARG ZBCTL_VERSION=v8.6.0
ARG NODE_VERSION=22
ARG PLAYWRIGHT_VERSION=1.57.0

# Avoid interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8

# Set bash as default shell
SHELL ["/bin/bash", "-c"]

# =============================================================================
# Layer 1: System packages + Node.js (stable layer)
# Install only the minimum dependencies needed for Chromium
# =============================================================================
RUN apt-get update && apt-get install -y --no-install-recommends \
    # Essential tools
    ca-certificates \
    curl \
    git \
    gnupg \
    make \
    gettext-base \
    # Chromium dependencies (subset of full Playwright deps)
    # These are the critical libs needed for headless Chromium
    libasound2t64 \
    libatk-bridge2.0-0 \
    libatk1.0-0 \
    libatspi2.0-0 \
    libcairo2 \
    libcups2 \
    libdbus-1-3 \
    libdrm2 \
    libgbm1 \
    libglib2.0-0 \
    libnspr4 \
    libnss3 \
    libpango-1.0-0 \
    libpangocairo-1.0-0 \
    libx11-6 \
    libx11-xcb1 \
    libxcb1 \
    libxcomposite1 \
    libxdamage1 \
    libxext6 \
    libxfixes3 \
    libxkbcommon0 \
    libxrandr2 \
    # Additional fonts for proper rendering
    fonts-liberation \
    fonts-noto-color-emoji \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/*

# Install Node.js from NodeSource
RUN curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg \
    && echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_${NODE_VERSION}.x nodistro main" >> /etc/apt/sources.list.d/nodesource.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends nodejs \
    && npm install -g yarn \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/* \
    && npm cache clean --force

# =============================================================================
# Layer 2: Install Playwright with Chromium ONLY
# This is the key optimization - skip Firefox and WebKit
# =============================================================================
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
ENV PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1

# Install playwright and download only Chromium
RUN mkdir -p /ms-playwright \
    && npm install -g playwright@${PLAYWRIGHT_VERSION} \
    && npx playwright install chromium \
    && npx playwright install-deps chromium \
    && chmod -R 777 /ms-playwright \
    && npm cache clean --force \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/*

# =============================================================================
# Layer 3: Cloud Authentication (minimal - only auth plugins)
# =============================================================================

# GKE authentication
RUN curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg \
    && echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" > /etc/apt/sources.list.d/google-cloud-sdk.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends google-cloud-cli-gke-gcloud-auth-plugin \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/*

# EKS authentication
ARG AWS_IAM_AUTH_VERSION=0.6.26
RUN curl -fsSL -o /usr/local/bin/aws-iam-authenticator \
    "https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/download/v${AWS_IAM_AUTH_VERSION}/aws-iam-authenticator_${AWS_IAM_AUTH_VERSION}_linux_amd64" \
    && chmod +x /usr/local/bin/aws-iam-authenticator

# =============================================================================
# Layer 4: CLI tools (helm, kubectl, jq, yq, task, zbctl)
# =============================================================================
RUN curl -fsSL "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz" | tar -xzf - \
    && mv linux-amd64/helm /usr/local/bin/helm \
    && rm -rf linux-amd64 \
    && curl -fsSL -o /usr/local/bin/kubectl "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
    && chmod +x /usr/local/bin/kubectl \
    && curl -fsSL -o /usr/local/bin/jq "https://github.com/jqlang/jq/releases/download/jq-${JQ_VERSION}/jq-linux-amd64" \
    && chmod +x /usr/local/bin/jq \
    && curl -fsSL -o /usr/local/bin/yq "https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_amd64" \
    && chmod +x /usr/local/bin/yq \
    && curl -fsSL "https://github.com/go-task/task/releases/download/v${TASK_VERSION}/task_linux_amd64.tar.gz" | tar -xzf - -C /usr/local/bin task

# zbctl via npm
RUN npm install -g zbctl@${ZBCTL_VERSION#v} \
    && npm cache clean --force

# =============================================================================
# Layer 5: Configuration
# =============================================================================
RUN git config --global --add safe.directory '*' \
    && adduser --disabled-password --gecos "" pwuser || true

COPY .tool-versions /opt/.tool-versions
RUN cp /opt/.tool-versions /root/.tool-versions \
    && cp /opt/.tool-versions /.tool-versions \
    && mkdir -p /home/runner /home/pwuser \
    && cp /opt/.tool-versions /home/runner/.tool-versions \
    && cp /opt/.tool-versions /home/pwuser/.tool-versions 2>/dev/null || true

# =============================================================================
# Layer 6: Verification
# =============================================================================
WORKDIR /workspace

RUN echo "=== Verifying tool installations ===" \
    && echo "Node: $(node --version)" \
    && echo "npm: $(npm --version)" \
    && echo "Playwright: $(npx playwright --version)" \
    && helm version --short \
    && kubectl version --client \
    && jq --version \
    && yq --version \
    && task --version \
    && gke-gcloud-auth-plugin --version \
    && aws-iam-authenticator version \
    && (zbctl version || true) \
    && echo "=== Checking Chromium installation ===" \
    && ls -la /ms-playwright/ \
    && echo "=== All tools verified successfully ==="

USER root

ENTRYPOINT ["/bin/bash", "-c"]
CMD ["bash"]
