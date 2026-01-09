# CI Runner Image for Camunda Platform Helm Integration Tests
# This image contains all tools pre-installed to avoid runtime downloads
# Tools are installed directly (not via asdf) for reliable PATH access in GitHub Actions
#
# Build: docker build -t registry.camunda.cloud/team-distribution/ci-runner:latest .
# Push:  docker push registry.camunda.cloud/team-distribution/ci-runner:latest

FROM ubuntu:24.04

LABEL org.opencontainers.image.source="https://github.com/camunda/camunda-platform-helm"
LABEL org.opencontainers.image.description="CI Runner for Camunda Platform Helm integration tests"
LABEL org.opencontainers.image.licenses="Apache-2.0"

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
# Layer 3: Python-based tools via pipx
# =============================================================================
RUN pipx install yamllint==1.37.1 && pipx ensurepath
ENV PATH="/root/.local/bin:${PATH}"

# =============================================================================
# Layer 4: Copy .tool-versions for version reference
# =============================================================================
COPY .tool-versions /opt/.tool-versions

# =============================================================================
# Layer 5: Install tools directly from releases (not via asdf)
# Each tool in its own layer for better caching
# =============================================================================

# Go
RUN GOLANG_VERSION=$(grep "^golang " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL "https://go.dev/dl/go${GOLANG_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xzf - \
    && ln -sf /usr/local/go/bin/go /usr/local/bin/go \
    && ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt

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

# kustomize
RUN KUSTOMIZE_VERSION=$(grep "^kustomize " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz" | tar -xzf - -C /usr/local/bin

# oc (OpenShift CLI)
RUN OC_VERSION=$(grep "^oc " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/${OC_VERSION}/openshift-client-linux.tar.gz" | tar -xzf - -C /usr/local/bin oc kubectl.1 || \
    curl -fsSL "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/stable-4.14/openshift-client-linux.tar.gz" | tar -xzf - -C /usr/local/bin oc

# kind
RUN KIND_VERSION=$(grep "^kind " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL -o /usr/local/bin/kind "https://kind.sigs.k8s.io/dl/v${KIND_VERSION}/kind-linux-amd64" \
    && chmod +x /usr/local/bin/kind

# zbctl (version in .tool-versions already has 'v' prefix)
RUN ZBCTL_VERSION=$(grep "^zbctl " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL -o /usr/local/bin/zbctl "https://github.com/camunda/zeebe/releases/download/${ZBCTL_VERSION}/zbctl.tar.gz" \
    && tar -xzf /usr/local/bin/zbctl -C /tmp \
    && mv /tmp/zbctl /usr/local/bin/zbctl \
    && chmod +x /usr/local/bin/zbctl || \
    curl -fsSL -o /usr/local/bin/zbctl "https://github.com/camunda/camunda/releases/download/${ZBCTL_VERSION}/zbctl" \
    && chmod +x /usr/local/bin/zbctl || true

# oras
RUN ORAS_VERSION=$(grep "^oras " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL "https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/oras_${ORAS_VERSION}_linux_amd64.tar.gz" | tar -xzf - -C /usr/local/bin oras

# helm-ct (chart-testing)
RUN CT_VERSION=$(grep "^helm-ct " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL "https://github.com/helm/chart-testing/releases/download/v${CT_VERSION}/chart-testing_${CT_VERSION}_linux_amd64.tar.gz" | tar -xzf - -C /usr/local/bin ct

# helm-cr (chart-releaser)
RUN CR_VERSION=$(grep "^helm-cr " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL "https://github.com/helm/chart-releaser/releases/download/v${CR_VERSION}/chart-releaser_${CR_VERSION}_linux_amd64.tar.gz" | tar -xzf - -C /usr/local/bin cr

# gomplate (version in .tool-versions already has 'v' prefix)
RUN GOMPLATE_VERSION=$(grep "^gomplate " /opt/.tool-versions | awk '{print $2}') \
    && curl -fsSL -o /usr/local/bin/gomplate "https://github.com/hairyhenderson/gomplate/releases/download/${GOMPLATE_VERSION}/gomplate_linux-amd64" \
    && chmod +x /usr/local/bin/gomplate

# bats
RUN BATS_VERSION=$(grep "^bats " /opt/.tool-versions | awk '{print $2}') \
    && git clone --depth 1 --branch v${BATS_VERSION} https://github.com/bats-core/bats-core.git /tmp/bats \
    && /tmp/bats/install.sh /usr/local \
    && rm -rf /tmp/bats

# =============================================================================
# Layer 6: Configuration
# =============================================================================
# Set up Go environment
ENV GOPATH=/root/go
ENV PATH="${GOPATH}/bin:/root/.local/bin:/usr/local/bin:/usr/bin:/bin:/usr/local/go/bin"

# Copy .tool-versions to common locations
RUN cp /opt/.tool-versions /root/.tool-versions \
    && cp /opt/.tool-versions /.tool-versions \
    && mkdir -p /home/runner \
    && cp /opt/.tool-versions /home/runner/.tool-versions

# =============================================================================
# Layer 7: Verification
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

