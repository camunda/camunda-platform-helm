# CI Runner Docker Images

This directory contains Dockerfiles for building custom CI runner images used in the integration test workflows.

## Images

### CI Runner (`ci-runner`)

The main CI runner image used for `install`, `upgrade`, and `cleanup` jobs in the integration test workflow.

**Registry:** `ghcr.io/camunda/team-distribution/ci-runner`

**Included tools:**
- All tools from `.tool-versions` (golang, helm, kubectl, oc, task, yq, zbctl, jq, bats, kustomize, etc.)
- Google Cloud CLI (gcloud) with GKE auth plugin
- AWS CLI v2
- System utilities (curl, git, make, gettext-base, etc.)

### Playwright Runner (`playwright-runner`)

Extended Playwright image with additional CI tools for running integration and E2E tests.

**Registry:** `ghcr.io/camunda/team-distribution/playwright-runner`

**Base image:** `mcr.microsoft.com/playwright:v1.57.0-noble`

**Additional tools:**
- Subset of tools from `.tool-versions` (helm, kubectl, oc, task, yq, zbctl, jq)
- Google Cloud CLI and AWS CLI for cluster authentication
- System utilities (gettext-base for envsubst)

### Keycloak CI (`keycloak-ci`)

Pre-built Keycloak image for CI integration tests. Runs `kc.sh build` at image build time with the exact runtime config used in CI, so the container can start with `--optimized` and skip the ~30-40s Quarkus augmentation on every boot.

**Registry:** `ghcr.io/camunda/team-distribution/keycloak-ci`

**Base image:** `camunda/keycloak:26.3.3`

**Pre-baked build-time options:**
- `--cache=local` (single replica, no Infinispan)
- `--db=postgres`
- `--http-relative-path=/auth/`
- `--health-enabled=true`
- `--metrics-enabled=false`
- `--features-disabled=ciba,client-policies,dpop,dynamic-scopes,kerberos,par,step-up-authentication,web-authn`
- `--transaction-xa-enabled=false`
- `--http-enabled=true`

**Usage:** Set `identityKeycloak.image.repository` to `ghcr.io/camunda/team-distribution/keycloak-ci`, `production: true`, and `extraStartupArgs: "--optimized"` in test values.

**Updating Keycloak version:** Change the `ARG KEYCLOAK_VERSION=` in the Dockerfile and push to `main`.

## Building Images

Images are automatically built and pushed when:
- `.tool-versions` file changes on `main` branch
- Files in `.github/docker/**` change on `main` branch
- Manual workflow dispatch via GitHub Actions

### Manual Build

```bash
# Build CI Runner
cd .github/docker/ci-runner
cp ../../../.tool-versions .
docker build -t ghcr.io/camunda/team-distribution/ci-runner:latest .

# Build Playwright Runner
cd .github/docker/playwright-runner
cp ../../../.tool-versions .
docker build -t ghcr.io/camunda/team-distribution/playwright-runner:latest .

# Build Keycloak CI (pre-built for fast startup)
cd .github/docker/keycloak-ci
docker build -t ghcr.io/camunda/team-distribution/keycloak-ci:latest .
```

## Image Tags

- `latest` - Most recent build from main branch
- `sha-XXXXXXXX` - Hash-based tag from `.tool-versions` content (first 8 chars of SHA256)
- `<keycloak-version>` - Keycloak CI image is also tagged with the upstream Keycloak version (e.g. `26.3.3`)

## Usage in Workflows

The images are used in `test-integration-runner.yaml` as job containers:

```yaml
jobs:
  install:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/camunda/team-distribution/ci-runner:latest
      credentials:
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}
```

## Script Optimizations

The test scripts in `scripts/` have been optimized to detect pre-installed dependencies:

### Playwright Browsers
The `base_playwright_script.sh` detects if Playwright browsers are already installed (in `/ms-playwright` or `PLAYWRIGHT_BROWSERS_PATH`) and skips the `npx playwright install --with-deps` step. This saves ~1-2 minutes per job.

### npm Dependencies
When running in CI (`CI=true`), the scripts use `npm ci` instead of `npm install` for faster, reproducible installs. If `node_modules` is already present with a matching `package-lock.json` hash, npm install is skipped entirely.

## Updating Tools

1. Update `.tool-versions` in the repository root
2. Push to `main` branch
3. The `build-ci-runner-image.yaml` workflow will automatically trigger
4. New images will be built and pushed with updated tool versions

