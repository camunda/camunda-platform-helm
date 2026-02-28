# Build stage
FROM node:22-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY tsconfig.json ./
COPY src/ ./src/
RUN npm run build

# Production stage
FROM node:22-alpine
WORKDIR /app

# Install Helm CLI
RUN apk add --no-cache curl bash openssl && \
    curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash && \
    apk del curl bash openssl

# Copy built application
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
COPY package.json ./

# Create data directory
RUN mkdir -p /app/data

# Default to fetch mode (no CAMUNDA_HELM_CHART_PATH set)
ENTRYPOINT ["node", "dist/entrypoint.js"]
