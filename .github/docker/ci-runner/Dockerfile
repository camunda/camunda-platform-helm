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

# Install system dependencies including bash
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
    && rm -rf /var/lib/apt/lists/*

# Set bash as default shell
SHELL ["/bin/bash", "-c"]

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

# Set up asdf environment variables
ENV ASDF_DIR=/opt/asdf
ENV ASDF_DATA_DIR=/opt/asdf
ENV PATH="${ASDF_DIR}/shims:${ASDF_DIR}/bin:${PATH}"

# Install asdf to /opt/asdf (accessible by all users)
RUN git clone https://github.com/asdf-vm/asdf.git ${ASDF_DIR} --branch v0.15.0

# Copy .tool-versions file
COPY .tool-versions /opt/asdf/.tool-versions

WORKDIR /opt/asdf

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
                asdf global "$tool" "$version"; \
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
    done < /opt/asdf/.tool-versions

# Reshim to ensure all binaries are available
RUN asdf reshim

# Copy .tool-versions to common locations where GitHub Actions might look
RUN cp /opt/asdf/.tool-versions /root/.tool-versions \
    && cp /opt/asdf/.tool-versions /.tool-versions

# Set up Go environment
ENV GOPATH=/root/go
ENV PATH="${GOPATH}/bin:${PATH}"

# Set up shell initialization for interactive and non-interactive shells
# This ensures asdf works in GitHub Actions which uses non-login shells
RUN echo 'export ASDF_DIR=/opt/asdf' >> /etc/bash.bashrc \
    && echo 'export ASDF_DATA_DIR=/opt/asdf' >> /etc/bash.bashrc \
    && echo 'export PATH="${ASDF_DIR}/shims:${ASDF_DIR}/bin:${PATH}"' >> /etc/bash.bashrc \
    && echo '. "${ASDF_DIR}/asdf.sh"' >> /etc/bash.bashrc \
    && echo 'export ASDF_DIR=/opt/asdf' >> /root/.bashrc \
    && echo 'export ASDF_DATA_DIR=/opt/asdf' >> /root/.bashrc \
    && echo 'export PATH="${ASDF_DIR}/shims:${ASDF_DIR}/bin:${PATH}"' >> /root/.bashrc \
    && echo '. "${ASDF_DIR}/asdf.sh"' >> /root/.bashrc

# Also set up for GitHub Actions runner user (uid 1001)
RUN mkdir -p /home/runner \
    && echo 'export ASDF_DIR=/opt/asdf' >> /home/runner/.bashrc \
    && echo 'export ASDF_DATA_DIR=/opt/asdf' >> /home/runner/.bashrc \
    && echo 'export PATH="${ASDF_DIR}/shims:${ASDF_DIR}/bin:${PATH}"' >> /home/runner/.bashrc \
    && echo '. "${ASDF_DIR}/asdf.sh"' >> /home/runner/.bashrc \
    && cp /opt/asdf/.tool-versions /home/runner/.tool-versions

# Create workspace directory
WORKDIR /workspace

# Verify installations work from the shims directly (no shell sourcing needed)
RUN echo "Verifying tool installations..." \
    && /opt/asdf/shims/helm version --short \
    && /opt/asdf/shims/kubectl version --client \
    && /opt/asdf/shims/task --version \
    && /opt/asdf/shims/yq --version \
    && /opt/asdf/shims/jq --version \
    && /opt/asdf/shims/go version \
    && echo "✅ All tools verified successfully"

# Default shell for RUN commands
SHELL ["/bin/bash", "-c"]

# Entry point that ensures PATH is set correctly
ENTRYPOINT ["/bin/bash", "-c"]
CMD ["bash"]
