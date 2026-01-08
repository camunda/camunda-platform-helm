FROM mcr.microsoft.com/playwright:v1.50.0-noble 

WORKDIR /e2e
COPY package*.json ./
COPY playwright.config.ts ./
RUN npm ci                                     

COPY tests/ ./tests/

# Install Playwright browsers with retry logic for apt mirror sync issues
RUN for attempt in 1 2 3 4 5; do \
      echo "Attempt $attempt of 5..." && \
      npx playwright install --with-deps && break; \
      echo "Failed, cleaning apt cache and retrying in $((attempt * 30)) seconds..." && \
      rm -rf /var/lib/apt/lists/* && \
      sleep $((attempt * 30)); \
      if [ $attempt -eq 3 ]; then \
        echo "Switching to alternative Ubuntu mirror..." && \
        sed -i 's|http://archive.ubuntu.com|http://mirrors.edge.kernel.org|g' /etc/apt/sources.list 2>/dev/null || true && \
        sed -i 's|http://security.ubuntu.com|http://mirrors.edge.kernel.org|g' /etc/apt/sources.list 2>/dev/null || true; \
      fi; \
      [ $attempt -eq 5 ] && exit 1; \
    done

ENTRYPOINT ["npx","playwright","test","--reporter=html,junit"]
