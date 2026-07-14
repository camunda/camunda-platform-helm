---
name: e2e-testing
description: Run Camunda e2e/smoke tests against a deployed cluster — generate .env credentials with render-e2e-env.sh or deploy-camunda, run Playwright suites via c8e2e (distributed on-cluster runner) or --test-e2e, test multiple environments in parallel, and reproduce CI e2e failures locally. Use when running or debugging e2e/smoke tests, generating test credentials for a namespace, or reproducing a failing CI e2e job.
---

# Running E2E Tests

Throughout: `$NS` = Kubernetes namespace, `$RELEASE` = Helm release name.

## Generate a per-environment .env file

After deploying a namespace, generate the `.env.test` file that the Playwright test suite needs. This file contains the ingress hostname, resolved credentials (from Kubernetes secrets), Keycloak URLs, and feature flags — all auto-resolved from the live cluster.

**Via `deploy-camunda`:**

```bash
# Generate .env.test alongside the deployment
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE \
  --scenario chart-full-setup --identity keycloak --persistence elasticsearch \
  --output-test-env

# Custom path (useful for multiple environments side by side)
deploy-camunda ... --output-test-env --output-test-env-path .env.test.eske
```

For multi-scenario parallel deployments, each scenario gets its own `.env.test.{scenario}` file automatically.

**Standalone against an existing namespace:**

```bash
./scripts/render-e2e-env.sh \
  --absolute-chart-path $PWD/charts/camunda-platform-8.9 \
  --namespace $NS \
  --output .env.test \
  --kube-context $CTX        # optional: target a specific cluster
  # --opensearch             # set IS_OPENSEARCH=true
  # --rba                    # set IS_RBA=true
  # --mt                     # set IS_MT=true
  # --run-smoke-tests        # set IS_SMOKE=true
  # -v                       # verbose: show resolved values
```

## Run the tests

**Via `deploy-camunda` (deploy + test in one step):**

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE \
  --scenario chart-full-setup --identity keycloak --persistence elasticsearch \
  --test-e2e          # e2e tests after deploy
  # --test-it         # integration tests instead
  # --test-all        # both
```

**Via `c8e2e` (distributed Playwright runner on Kubernetes):**

`c8e2e` (`@camunda/c8e2e`) launches sharded Playwright test pods directly on the cluster — faster and more reliable than running locally. Point it at a deployed environment by its ingress URL:

```bash
# Run against a deployed namespace
c8e2e test \
  --target SM-8.9 \
  --endpoint https://$NS.ci.distro.ultrawombat.com \
  --feature-flags smoke \
  --follow

# Full test suite with sharding
c8e2e test \
  --target SM-8.9 \
  --endpoint https://my-env.ci.distro.ultrawombat.com \
  --shards 4 \
  --follow

# Filter to specific tests
c8e2e test \
  --target SM-8.9 \
  --endpoint https://my-env.ci.distro.ultrawombat.com \
  --grep "Basic Navigation"

# OpenSearch environment with multitenancy
c8e2e test \
  --target SM-8.9 \
  --endpoint https://os-env.ci.distro.ultrawombat.com \
  --feature-flags opensearch,mt \
  --follow
```

Manage running tests:

```bash
c8e2e list                    # List active test runs
c8e2e status <run-id>         # Check status
c8e2e logs <run-id>           # Stream logs
c8e2e results <run-id>        # Download results
c8e2e cancel <run-id>         # Cancel a run
```

## Multiple environments in parallel

Deploy multiple namespaces with unique subdomains, then run `c8e2e` against each:

```bash
# Deploy two environments with unique hostnames
deploy-camunda --chart-path ./charts/camunda-platform-8.9 \
  --namespace test-es --release camunda --ingress-subdomain test-es \
  --identity keycloak --persistence elasticsearch

deploy-camunda --chart-path ./charts/camunda-platform-8.9 \
  --namespace test-os --release camunda --ingress-subdomain test-os \
  --identity keycloak --persistence opensearch-embedded

# Run e2e against each in parallel
c8e2e test --target SM-8.9 --endpoint https://test-es.ci.distro.ultrawombat.com --feature-flags smoke --follow &
c8e2e test --target SM-8.9 --endpoint https://test-os.ci.distro.ultrawombat.com --feature-flags smoke,opensearch --follow &
wait
```

## Reproducing a CI Test Failure Locally

See `docs/skills/reproducing-ci-e2e-failures.md` for the step-by-step guide to pulling logs, downloading artifacts, decoding CI scenario shortnames, and spinning up an identical local environment.

For the full deploy → credentials → reproduce-on-main → verify-on-branch loop, see the `gke-verification` skill.
