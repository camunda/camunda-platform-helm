# CI Runner Image for Camunda Platform Helm Integration Tests
# This image contains all tools from .tool-versions pre-installed to avoid runtime downloads
#
# Build: docker build -t registry.camunda.cloud/team-distribution/ci-runner:latest .
# Push:  docker push registry.camunda.cloud/team-distribution/ci-runner:latest

FROM ubuntu:24.04

LABEL org.opencontainers.image.source="https://github.com/camunda/camunda-platform-helm"
LABEL org.opencontainers.image.description="CI Runner for Camunda Platform Helm integration tests"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# Avoid interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    gnupg \
    gettext-base \
    jq \
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
    && rm -rf /var/lib/apt/lists/*

# Install Google Cloud CLI for GKE authentication
RUN echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" \
    | tee -a /etc/apt/sources.list.d/google-cloud-sdk.list \
    && curl https://packages.cloud.google.com/apt/doc/apt-key.gpg \
    | gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg \
    && apt-get update && apt-get install -y --no-install-recommends \
    google-cloud-cli \
    google-cloud-cli-gke-gcloud-auth-plugin \
    && rm -rf /var/lib/apt/lists/*

# Install AWS CLI for EKS authentication
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" \
    && unzip awscliv2.zip \
    && ./aws/install \
    && rm -rf awscliv2.zip aws

# Set up asdf environment
ENV ASDF_DIR=/root/.asdf
ENV ASDF_DATA_DIR=/root/.asdf
ENV PATH="${ASDF_DIR}/shims:${ASDF_DIR}/bin:${PATH}"

# Install asdf
RUN git clone https://github.com/asdf-vm/asdf.git ${ASDF_DIR} --branch v0.15.0

# Copy .tool-versions file
COPY .tool-versions /root/.tool-versions

WORKDIR /root

# Add all asdf plugins
RUN asdf plugin add git-cliff || true \
    && asdf plugin add golang || true \
    && asdf plugin add gomplate || true \
    && asdf plugin add helm || true \
    && asdf plugin add helm-cr || true \
    && asdf plugin add helm-ct || true \
    && asdf plugin add kind || true \
    && asdf plugin add kubectl || true \
    && asdf plugin add kustomize || true \
    && asdf plugin add oc || true \
    && asdf plugin add oras || true \
    && asdf plugin add task https://github.com/particledecay/asdf-task.git || true \
    && asdf plugin add yamllint || true \
    && asdf plugin add yq || true \
    && asdf plugin add zbctl || true \
    && asdf plugin add jq || true \
    && asdf plugin add bats || true

# Install all tools from .tool-versions with retry logic
RUN set -e; \
    MAX_RETRIES=3; \
    while IFS= read -r line; do \
        # Skip empty lines and comments \
        case "$line" in \
            ''|'#'*) continue ;; \
        esac; \
        tool=$(echo "$line" | awk '{print $1}'); \
        version=$(echo "$line" | awk '{print $2}'); \
        echo "Installing $tool $version..."; \
        for attempt in 1 2 3; do \
            if asdf install "$tool" "$version"; then \
                echo "✅ Successfully installed $tool $version"; \
                break; \
            else \
                echo "⚠️ Attempt $attempt failed for $tool $version"; \
                if [ $attempt -eq $MAX_RETRIES ]; then \
                    echo "❌ Failed to install $tool $version after $MAX_RETRIES attempts"; \
                    exit 1; \
                fi; \
                sleep 10; \
            fi; \
        done; \
    done < /root/.tool-versions

# Set global versions for all tools
RUN asdf reshim

# Set up Go environment
ENV GOPATH=/root/go
ENV PATH="${GOPATH}/bin:${PATH}"

# Create workspace directory
WORKDIR /workspace

# Verify installations
RUN echo "Verifying tool installations..." \
    && helm version --short \
    && kubectl version --client \
    && task --version \
    && yq --version \
    && jq --version \
    && go version \
    && echo "✅ All tools verified successfully"

# Default entrypoint
ENTRYPOINT ["/bin/bash", "-c"]
CMD ["bash"]

