# Playwright Runner Image for Camunda Platform Helm Integration Tests
# Extends the official Playwright image with additional CI tools
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

# Install system dependencies with retry logic for apt mirror sync issues
RUN for attempt in 1 2 3 4 5; do \
        echo "Attempt $attempt of 5..." && \
        apt-get update && apt-get install -y --no-install-recommends \
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
        && break; \
        echo "Failed, cleaning apt cache and retrying in $((attempt * 30)) seconds..." && \
        rm -rf /var/lib/apt/lists/* && \
        sleep $((attempt * 30)); \
        if [ $attempt -eq 3 ]; then \
            echo "Switching to alternative Ubuntu mirror..." && \
            sed -i 's|http://archive.ubuntu.com|http://mirrors.edge.kernel.org|g' /etc/apt/sources.list 2>/dev/null || true && \
            sed -i 's|http://security.ubuntu.com|http://mirrors.edge.kernel.org|g' /etc/apt/sources.list 2>/dev/null || true; \
        fi; \
        [ $attempt -eq 5 ] && exit 1; \
    done \
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

# Set up asdf environment variables (use /opt/asdf for all users)
ENV ASDF_DIR=/opt/asdf
ENV ASDF_DATA_DIR=/opt/asdf
ENV PATH="${ASDF_DIR}/shims:${ASDF_DIR}/bin:${PATH}"

# Install asdf to /opt/asdf (accessible by all users)
RUN git clone https://github.com/asdf-vm/asdf.git ${ASDF_DIR} --branch v0.15.0

# Create a minimal .tool-versions with only the tools needed for Playwright tests
COPY .tool-versions /tmp/.tool-versions-full

# Extract only the tools needed for Playwright runner
RUN grep -E "^(helm|kubectl|oc|task|yq|zbctl|jq) " /tmp/.tool-versions-full > /opt/asdf/.tool-versions \
    && cat /opt/asdf/.tool-versions

WORKDIR /opt/asdf

# Add required asdf plugins
RUN asdf plugin add helm || true \
    && asdf plugin add kubectl || true \
    && asdf plugin add oc || true \
    && asdf plugin add task https://github.com/particledecay/asdf-task.git || true \
    && asdf plugin add yq || true \
    && asdf plugin add zbctl || true \
    && asdf plugin add jq || true

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

# Copy .tool-versions to common locations
RUN cp /opt/asdf/.tool-versions /root/.tool-versions \
    && cp /opt/asdf/.tool-versions /.tool-versions \
    && cp /opt/asdf/.tool-versions /home/pwuser/.tool-versions 2>/dev/null || true

# Set up shell initialization for root
RUN echo 'export ASDF_DIR=/opt/asdf' >> /root/.bashrc \
    && echo 'export ASDF_DATA_DIR=/opt/asdf' >> /root/.bashrc \
    && echo 'export PATH="${ASDF_DIR}/shims:${ASDF_DIR}/bin:${PATH}"' >> /root/.bashrc \
    && echo '. "${ASDF_DIR}/asdf.sh"' >> /root/.bashrc

# Set up shell initialization for pwuser
RUN echo 'export ASDF_DIR=/opt/asdf' >> /home/pwuser/.bashrc \
    && echo 'export ASDF_DATA_DIR=/opt/asdf' >> /home/pwuser/.bashrc \
    && echo 'export PATH="${ASDF_DIR}/shims:${ASDF_DIR}/bin:${PATH}"' >> /home/pwuser/.bashrc \
    && echo '. "${ASDF_DIR}/asdf.sh"' >> /home/pwuser/.bashrc

# Set up for GitHub Actions runner user (uid 1001)
RUN mkdir -p /home/runner \
    && echo 'export ASDF_DIR=/opt/asdf' >> /home/runner/.bashrc \
    && echo 'export ASDF_DATA_DIR=/opt/asdf' >> /home/runner/.bashrc \
    && echo 'export PATH="${ASDF_DIR}/shims:${ASDF_DIR}/bin:${PATH}"' >> /home/runner/.bashrc \
    && echo '. "${ASDF_DIR}/asdf.sh"' >> /home/runner/.bashrc \
    && cp /opt/asdf/.tool-versions /home/runner/.tool-versions

# Also set up system-wide bash configuration
RUN echo 'export ASDF_DIR=/opt/asdf' >> /etc/bash.bashrc \
    && echo 'export ASDF_DATA_DIR=/opt/asdf' >> /etc/bash.bashrc \
    && echo 'export PATH="${ASDF_DIR}/shims:${ASDF_DIR}/bin:${PATH}"' >> /etc/bash.bashrc

# Create workspace directory
WORKDIR /workspace

# Verify installations work from the shims directly
RUN echo "Verifying tool installations..." \
    && /opt/asdf/shims/helm version --short \
    && /opt/asdf/shims/kubectl version --client \
    && /opt/asdf/shims/task --version \
    && /opt/asdf/shims/yq --version \
    && /opt/asdf/shims/jq --version \
    && echo "✅ All tools verified successfully"

# Keep root user as default for GitHub Actions (--user root in container options)
# pwuser can be used for local testing if needed
USER root

# Default working directory
WORKDIR /workspace

# Default shell
SHELL ["/bin/bash", "-c"]

ENTRYPOINT ["/bin/bash", "-c"]
CMD ["bash"]
