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

# Install system dependencies with retry logic for apt mirror sync issues
RUN for attempt in 1 2 3 4 5; do \
        echo "Attempt $attempt of 5..." && \
        apt-get update && apt-get install -y --no-install-recommends \
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

# Set up asdf environment
ENV ASDF_DIR=/root/.asdf
ENV ASDF_DATA_DIR=/root/.asdf
ENV PATH="${ASDF_DIR}/shims:${ASDF_DIR}/bin:${PATH}"

# Install asdf
RUN git clone https://github.com/asdf-vm/asdf.git ${ASDF_DIR} --branch v0.15.0

# Create a minimal .tool-versions with only the tools needed for Playwright tests
# These versions should match the main .tool-versions file
COPY .tool-versions /tmp/.tool-versions-full

# Extract only the tools needed for Playwright runner
RUN grep -E "^(helm|kubectl|oc|task|yq|zbctl|jq) " /tmp/.tool-versions-full > /root/.tool-versions \
    && cat /root/.tool-versions

WORKDIR /root

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

# Create workspace directory
WORKDIR /workspace

# Verify installations
RUN echo "Verifying tool installations..." \
    && helm version --short \
    && kubectl version --client \
    && task --version \
    && yq --version \
    && jq --version \
    && echo "✅ All tools verified successfully"

# Switch back to pwuser for running tests (Playwright convention)
USER pwuser

# Default working directory for tests
WORKDIR /home/pwuser

ENTRYPOINT ["/bin/bash", "-c"]
CMD ["bash"]

