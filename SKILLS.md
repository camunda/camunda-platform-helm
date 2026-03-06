# Operational Skills

This file contains instructions for using the primary operational tools in this repository: the `deploy-camunda` CLI, `kubectl`, `helm`, and the `task` runner. It is organized around problem-solving -- use it to deploy, debug, and test Camunda on Kubernetes.

Throughout this document, `$NS` refers to a Kubernetes namespace and `$RELEASE` refers to a Helm release name. Substitute your actual values.

---

## Table of Contents

- [deploy-camunda CLI](#deploy-camunda-cli)
  - [Installation](#installation)
  - [Deploy a Single Scenario](#deploy-a-single-scenario)
  - [Deploy with Selection + Composition](#deploy-with-selection--composition)
  - [Configuration Profiles](#configuration-profiles)
  - [Render Without Deploying](#render-without-deploying)
  - [Matrix Operations](#matrix-operations)
  - [Prepare Values Standalone](#prepare-values-standalone)
  - [Debug JVM Components](#debug-jvm-components)
  - [Run Tests After Deployment](#run-tests-after-deployment)
  - [Entra ID (OIDC) Management](#entra-id-oidc-management)
  - [Common Recipes](#common-recipes)
  - [Flag Reference](#flag-reference)
  - [Environment Variables](#environment-variables)
- [kubectl -- Debugging Deployments](#kubectl----debugging-deployments)
  - [Check Deployment Health](#check-deployment-health)
  - [Debug Failing Pods](#debug-failing-pods)
  - [Access Services Locally (Port-Forward)](#access-services-locally-port-forward)
  - [Inspect Helm Releases](#inspect-helm-releases)
  - [Manage Secrets](#manage-secrets)
  - [Namespace Lifecycle](#namespace-lifecycle)
  - [Post-Uninstall Cleanup](#post-uninstall-cleanup)
  - [Verify Permissions](#verify-permissions)
- [Task Runner -- Integration Tests](#task-runner----integration-tests)
  - [Prerequisites](#prerequisites)
  - [Running the Full Test Flow](#running-the-full-test-flow)
  - [Individual Steps](#individual-steps)
  - [Environment Variables for Test Scenarios](#environment-variables-for-test-scenarios)
  - [Common Test Scenarios](#common-test-scenarios)
- [Troubleshooting Decision Tree](#troubleshooting-decision-tree)

---

## deploy-camunda CLI

The `deploy-camunda` CLI orchestrates Camunda Platform Helm deployments. It resolves layered values files, performs environment variable substitution, manages Keycloak realms and Elasticsearch index prefixes, and supports parallel multi-scenario deployments.

**Source:** `scripts/deploy-camunda/`

### Installation

```bash
# Option 1: Install all Go CLI tools (recommended)
make install.dx-tooling

# Option 2: Install just deploy-camunda
make install.deploy-camunda

# Option 3: Build a local binary
cd scripts/deploy-camunda && go build -o deploy-camunda .

# Option 4: Run directly without installing
go run scripts/deploy-camunda/main.go [flags]
```

After installing, enable shell completions:

```bash
# bash
deploy-camunda completion bash > /etc/bash_completion.d/deploy-camunda

# zsh
deploy-camunda completion zsh > "${fpath[1]}/_deploy-camunda"
```

### Deploy a Single Scenario

The minimum required flags are `--chart-path`, `--namespace`, `--release`, and `--scenario`:

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS \
  --release $RELEASE \
  --scenario chart-full-setup
```

Deploy multiple scenarios in parallel (comma-separated):

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS \
  --release $RELEASE \
  --scenario keycloak,elasticsearch
```

### Deploy with Selection + Composition

The preferred way to configure deployments is through selection flags that compose layered values. These replace the older monolithic scenario files:

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS \
  --release $RELEASE \
  --scenario chart-full-setup \
  --identity keycloak-external \
  --persistence opensearch \
  --features multitenancy,documentstore \
  --qa
```

Available selections:

| Flag              | Values                                                     | Description                                           |
| ----------------- | ---------------------------------------------------------- | ----------------------------------------------------- |
| `--identity`      | `keycloak`, `keycloak-external`, `oidc`, `basic`, `hybrid` | Identity provider                                     |
| `--persistence`   | `elasticsearch`, `opensearch`, `rdbms`, `rdbms-oracle`     | Data backend                                          |
| `--test-platform` | `gke`, `eks`, `openshift`                                  | Platform-specific values                              |
| `--features`      | `multitenancy`, `rba`, `documentstore`                     | Feature toggles (comma-separated)                     |
| `--qa`            | (boolean)                                                  | Enable QA configuration (test users, image overrides) |
| `--image-tags`    | (boolean)                                                  | Enable image tag overrides from env vars              |
| `--upgrade-flow`  | (boolean)                                                  | Enable upgrade flow configuration                     |

The underlying layer resolution order is (last wins):

```
base.yaml -> identity -> persistence -> platform -> features -> QA -> image-tags -> upgrade
```

Layered values files live in: `charts/<version>/test/integration/scenarios/chart-full-setup/values/`

### Configuration Profiles

Instead of passing flags every time, create a config file at `.camunda-deploy.yaml` (project root) or `~/.config/camunda/deploy.yaml` (user-level):

```yaml
current: dev
repoRoot: /path/to/camunda-platform-helm
platform: gke
logLevel: info

keycloak:
  host: keycloak-24-9-0.ci.distro.ultrawombat.com
  protocol: https

deployments:
  dev:
    chartPath: ./charts/camunda-platform-8.9
    namespace: dev-test
    release: camunda
    scenario: chart-full-setup
    skipDependencyUpdate: true
    autoGenerateSecrets: true
    interactive: true

  qa-opensearch:
    chartPath: ./charts/camunda-platform-8.9
    namespace: qa-os
    release: integration
    scenario: chart-full-setup
    valuesPreset: enterprise,digest
```

Manage profiles:

```bash
# Create a new deployment profile
deploy-camunda config create my-profile

# Set values on it
deploy-camunda config set my-profile.chartPath ./charts/camunda-platform-8.9
deploy-camunda config set my-profile.namespace my-ns
deploy-camunda config set my-profile.release camunda
deploy-camunda config set my-profile.scenario chart-full-setup

# Switch to it
deploy-camunda config use my-profile

# Show merged configuration
deploy-camunda config show

# Deploy using the active profile (no flags needed)
deploy-camunda
```

### Render Without Deploying

Inspect what would be deployed without touching the cluster. This is the best way to debug values merging issues:

```bash
# Render to ./rendered/$RELEASE/
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE --scenario chart-full-setup \
  --render-templates

# Render to a custom directory
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE --scenario chart-full-setup \
  --render-templates --render-output-dir ./debug-output
```

The rendered output contains the full Kubernetes manifests after all values have been merged.

### Matrix Operations

The matrix commands manage the CI test matrix -- all scenario/version/platform combinations defined in each chart's `ci-test-config.yaml`.

```bash
# List all scenarios across all chart versions
deploy-camunda matrix list --repo-root .

# List scenarios for specific versions
deploy-camunda matrix list --repo-root . --versions 8.8,8.9

# Filter by scenario name
deploy-camunda matrix list --repo-root . --scenario-filter opensearch

# Filter by shortname
deploy-camunda matrix list --repo-root . --shortname-filter eske,eshy

# Output as JSON (for scripting)
deploy-camunda matrix list --repo-root . --format json

# Dry-run: show what would be deployed without deploying
deploy-camunda matrix run --repo-root . --dry-run --versions 8.9

# Show layer-breakdown coverage report
deploy-camunda matrix run --repo-root . --coverage --versions 8.9

# Run the full matrix against a cluster
deploy-camunda matrix run \
  --repo-root . \
  --versions 8.9 \
  --kube-context-gke my-gke-context \
  --ingress-base-domain-gke ci.distro.ultrawombat.com \
  --max-parallel 4 \
  --test-it
```

### Prepare Values Standalone

The `prepare-values` subcommand resolves and merges layered values files without deploying. It outputs the path to a merged values file. This is useful for debugging values composition or piping into Helm manually:

```bash
deploy-camunda prepare-values \
  --chart-path ./charts/camunda-platform-8.9 \
  --identity keycloak-external \
  --persistence elasticsearch \
  --test-platform gke \
  --features multitenancy \
  --qa \
  --interactive=false

# Returns: /tmp/merged-values-XXXXX.yaml
```

You can then inspect the merged file or use it directly with Helm:

```bash
MERGED=$(deploy-camunda prepare-values --chart-path ./charts/camunda-platform-8.9 --identity keycloak --persistence elasticsearch --interactive=false)
helm template my-release ./charts/camunda-platform-8.9 --values "$MERGED"
```

### Debug JVM Components

Attach a remote debugger to any JVM component:

```bash
# Debug orchestration on port 5005
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE --scenario chart-full-setup \
  --debug orchestration:5005

# Debug multiple components
deploy-camunda ... \
  --debug orchestration:5005 \
  --debug connectors:5006

# Suspend JVM until debugger attaches
deploy-camunda ... \
  --debug orchestration:5005 --debug-suspend
```

Then port-forward the debug port:

```bash
kubectl port-forward -n $NS svc/$RELEASE-zeebe 5005:5005
```

### Run Tests After Deployment

```bash
# Run integration tests after deployment
deploy-camunda ... --test-it

# Run E2E (Playwright) tests
deploy-camunda ... --test-e2e

# Run both
deploy-camunda ... --test-all

# Exclude specific test suites (Playwright --grep-invert)
deploy-camunda ... --test-e2e --test-exclude "flaky-suite|slow-suite"

# Generate a .env file for running E2E tests manually
deploy-camunda ... --output-test-env --output-test-env-path .env.test
```

### Entra ID (OIDC) Management

For OIDC integration test scenarios that use Microsoft Entra ID:

```bash
# Provision a venom Entra app registration + K8s secret
deploy-camunda entra ensure-venom-app \
  --namespace $NS \
  --kube-context my-context \
  --env-file .env.oidc

# Update redirect URIs on the parent Entra app
deploy-camunda entra update-redirect-uris \
  --ingress-host my-deployment.ci.distro.ultrawombat.com \
  --env-file .env.oidc

# Cleanup venom Entra app after tests
deploy-camunda entra cleanup-venom-app \
  --namespace $NS \
  --env-file .env.oidc
```

Required env vars (in `.env` file or environment): `ENTRA_APP_DIRECTORY_ID`, `ENTRA_APP_CLIENT_ID`, `ENTRA_APP_CLIENT_SECRET`, `ENTRA_APP_OBJECT_ID`.

### Common Recipes

**Local kind cluster (community contributors):**

```bash
kind create cluster --name camunda-test

deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace camunda \
  --release integration \
  --scenario chart-full-setup \
  --identity keycloak \
  --persistence elasticsearch \
  --auto-generate-secrets \
  --external-secrets=false \
  --skip-dependency-update=false \
  --timeout 15
```

**Deploy with external Keycloak (Camunda employees):**

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE \
  --scenario chart-full-setup \
  --identity keycloak-external \
  --persistence elasticsearch \
  --keycloak-host keycloak-24-9-0.ci.distro.ultrawombat.com \
  --ingress-base-domain ci.distro.ultrawombat.com \
  --ingress-subdomain my-test
```

**Deploy with OpenSearch + multi-tenancy:**

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE \
  --scenario chart-full-setup \
  --identity keycloak-external \
  --persistence opensearch \
  --features multitenancy \
  --qa
```

**Delete namespace and redeploy cleanly:**

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE --scenario chart-full-setup \
  --delete-namespace
```

**Apply chart-root value overlays (enterprise images, digest pinning):**

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE --scenario chart-full-setup \
  --values-preset enterprise,digest
```

**Supply extra values files (applied last, highest precedence):**

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE --scenario chart-full-setup \
  --extra-values ./my-overrides.yaml,./another-override.yaml
```

### Flag Reference

#### Root Command (`deploy-camunda`)

**Chart flags:**

| Flag                       | Short | Type     | Default       | Description                                                                |
| -------------------------- | ----- | -------- | ------------- | -------------------------------------------------------------------------- |
| `--chart-path`             |       | string   |               | Path to chart directory                                                    |
| `--chart`                  | `-c`  | string   |               | Chart name (remote)                                                        |
| `--version`                | `-v`  | string   |               | Chart version (requires `--chart`)                                         |
| `--repo-root`              |       | string   | auto-detected | Repository root path                                                       |
| `--skip-dependency-update` |       | bool     | `true`        | Skip `helm dependency update`                                              |
| `--values-preset`          |       | []string |               | Overlay files: `enterprise`, `digest`, `latest`, `local`, `bitnami-legacy` |

**Deployment flags:**

| Flag                  | Short | Type     | Default   | Description                                        |
| --------------------- | ----- | -------- | --------- | -------------------------------------------------- |
| `--namespace`         | `-n`  | string   |           | Kubernetes namespace                               |
| `--namespace-prefix`  |       | string   |           | Prefix for namespace (e.g. `distribution` for EKS) |
| `--release`           | `-r`  | string   |           | Helm release name                                  |
| `--scenario`          | `-s`  | string   |           | Scenario name (comma-separated for parallel)       |
| `--scenario-path`     |       | string   |           | Custom path to scenario files                      |
| `--platform`          |       | string   | `gke`     | Target platform: `gke`, `rosa`, `eks`              |
| `--flow`              |       | string   | `install` | Flow type                                          |
| `--timeout`           |       | int      | `5`       | Helm timeout in minutes                            |
| `--delete-namespace`  |       | bool     | `false`   | Delete namespace first                             |
| `--render-templates`  |       | bool     | `false`   | Render manifests instead of installing             |
| `--render-output-dir` |       | string   |           | Output dir for rendered manifests                  |
| `--extra-values`      |       | []string |           | Additional values files (applied last)             |
| `--env-file`          |       | string   |           | Path to `.env` file                                |
| `--interactive`       |       | bool     | `true`    | Interactive prompts for missing variables          |
| `--log-level`         | `-l`  | string   | `info`    | Log level: `debug`, `info`, `warn`, `error`        |
| `--config`            | `-F`  | string   |           | Path to config file                                |

**Selection + composition flags:**

| Flag              | Type     | Values                                                     |
| ----------------- | -------- | ---------------------------------------------------------- |
| `--identity`      | string   | `keycloak`, `keycloak-external`, `oidc`, `basic`, `hybrid` |
| `--persistence`   | string   | `elasticsearch`, `opensearch`, `rdbms`, `rdbms-oracle`     |
| `--test-platform` | string   | `gke`, `eks`, `openshift`                                  |
| `--features`      | []string | `multitenancy`, `rba`, `documentstore`                     |
| `--qa`            | bool     | Enable QA configuration                                    |
| `--image-tags`    | bool     | Enable image tag overrides from env                        |
| `--upgrade-flow`  | bool     | Enable upgrade flow configuration                          |

**Auth / Keycloak flags:**

| Flag                  | Type   | Default                                     | Description       |
| --------------------- | ------ | ------------------------------------------- | ----------------- |
| `--auth`              | string | `keycloak`                                  | Auth scenario     |
| `--keycloak-host`     | string | `keycloak-24-9-0.ci.distro.ultrawombat.com` | Keycloak host     |
| `--keycloak-protocol` | string | `https`                                     | Keycloak protocol |
| `--keycloak-realm`    | string | auto-generated                              | Keycloak realm    |

**Secrets flags:**

| Flag                         | Type   | Default | Description                       |
| ---------------------------- | ------ | ------- | --------------------------------- |
| `--external-secrets`         | bool   | `true`  | Enable external secrets           |
| `--external-secrets-store`   | string |         | External secrets store type       |
| `--vault-secret-mapping`     | string |         | Vault secret mapping content      |
| `--auto-generate-secrets`    | bool   | `false` | Auto-generate secrets for testing |
| `--use-vault-backed-secrets` | bool   | `false` | Use vault-backed external secrets |

**Docker registry flags:**

| Flag                       | Type   | Default | Description                   |
| -------------------------- | ------ | ------- | ----------------------------- |
| `--docker-username`        | string |         | Harbor registry username      |
| `--docker-password`        | string |         | Harbor registry password      |
| `--ensure-docker-registry` | bool   | `false` | Create Harbor pull secret     |
| `--dockerhub-username`     | string |         | Docker Hub username           |
| `--dockerhub-password`     | string |         | Docker Hub password           |
| `--ensure-docker-hub`      | bool   | `false` | Create Docker Hub pull secret |

**Ingress flags:**

| Flag                    | Type   | Description                                    |
| ----------------------- | ------ | ---------------------------------------------- |
| `--ingress-subdomain`   | string | Subdomain (requires `--ingress-base-domain`)   |
| `--ingress-base-domain` | string | Base domain (e.g. `ci.distro.ultrawombat.com`) |
| `--ingress-hostname`    | string | Full hostname (overrides subdomain)            |

**Index prefix flags:**

| Flag                           | Type   | Description                               |
| ------------------------------ | ------ | ----------------------------------------- |
| `--optimize-index-prefix`      | string | Optimize ES index prefix (auto-generated) |
| `--orchestration-index-prefix` | string | Orchestration ES index prefix             |
| `--tasklist-index-prefix`      | string | Tasklist ES index prefix                  |
| `--operate-index-prefix`       | string | Operate ES index prefix                   |

**Debug flags:**

| Flag              | Type     | Default | Description                                 |
| ----------------- | -------- | ------- | ------------------------------------------- |
| `--debug`         | []string |         | Component debug (e.g. `orchestration:5005`) |
| `--debug-port`    | int      | `5005`  | Default debug port                          |
| `--debug-suspend` | bool     | `false` | Suspend JVM until debugger attaches         |

**Test flags:**

| Flag                     | Type   | Default     | Description                           |
| ------------------------ | ------ | ----------- | ------------------------------------- |
| `--test-it`              | bool   | `false`     | Run integration tests after deploy    |
| `--test-e2e`             | bool   | `false`     | Run E2E tests after deploy            |
| `--test-all`             | bool   | `false`     | Run both IT and E2E tests             |
| `--test-exclude`         | string |             | Pipe-separated regex to exclude tests |
| `--output-test-env`      | bool   | `false`     | Generate `.env` for E2E tests         |
| `--output-test-env-path` | string | `.env.test` | Path for test env output              |
| `--kube-context`         | string |             | Kubernetes context                    |

#### `matrix list`

| Flag                 | Type     | Default | Description                   |
| -------------------- | -------- | ------- | ----------------------------- |
| `--versions`         | []string | all     | Chart versions to include     |
| `--include-disabled` | bool     | `false` | Include disabled scenarios    |
| `--scenario-filter`  | string   |         | Filter by scenario substring  |
| `--shortname-filter` | string   |         | Filter by shortname substring |
| `--flow-filter`      | string   |         | Filter by exact flow name     |
| `--format`           | string   | `table` | Output: `table` or `json`     |
| `--platform`         | string   |         | Filter by platform            |
| `--repo-root`        | string   |         | Repository root               |

#### `matrix run`

Inherits all `matrix list` flags, plus:

| Flag                                      | Type   | Default  | Description                       |
| ----------------------------------------- | ------ | -------- | --------------------------------- |
| `--dry-run`                               | bool   | `false`  | Preview without deploying         |
| `--coverage`                              | bool   | `false`  | Show layer-breakdown report       |
| `--max-parallel`                          | int    | `1`      | Max concurrent entries            |
| `--namespace-prefix`                      | string | `matrix` | Prefix for generated namespaces   |
| `--stop-on-failure`                       | bool   | `false`  | Stop on first failure             |
| `--cleanup`                               | bool   | `false`  | Delete namespace after tests      |
| `--delete-namespace`                      | bool   | `false`  | Delete namespace before deploying |
| `--timeout`                               | int    | `10`     | Helm timeout in minutes           |
| `--test-it`                               | bool   | `false`  | Run IT tests after each deploy    |
| `--test-e2e`                              | bool   | `false`  | Run E2E tests after each deploy   |
| `--test-all`                              | bool   | `false`  | Run both IT and E2E               |
| `--kube-context`                          | string |          | Default kube context              |
| `--kube-context-gke`                      | string |          | GKE-specific context              |
| `--kube-context-eks`                      | string |          | EKS-specific context              |
| `--ingress-base-domain`                   | string |          | Fallback base domain              |
| `--ingress-base-domain-gke`               | string |          | GKE base domain                   |
| `--ingress-base-domain-eks`               | string |          | EKS base domain                   |
| `--env-file`                              | string |          | Default `.env` file               |
| `--env-file-8.6` through `--env-file-8.9` | string |          | Version-specific `.env` files     |
| `--yes`                                   | bool   | `false`  | Skip confirmation prompts         |

#### `prepare-values`

| Flag              | Type     | Default            | Description                                              |
| ----------------- | -------- | ------------------ | -------------------------------------------------------- |
| `--scenario-path` | string   |                    | Path to scenario directory                               |
| `--chart-path`    | string   |                    | Path to chart directory                                  |
| `--scenario`      | string   | `chart-full-setup` | Scenario name                                            |
| `--identity`      | string   |                    | Identity selection                                       |
| `--persistence`   | string   |                    | Persistence selection                                    |
| `--test-platform` | string   |                    | Test platform selection                                  |
| `--platform`      | string   | `gke`              | Deploy platform (fallback)                               |
| `--features`      | []string |                    | Feature selections                                       |
| `--qa`            | bool     | `false`            | QA configuration                                         |
| `--image-tags`    | bool     | `false`            | Image tag overrides                                      |
| `--upgrade-flow`  | bool     | `false`            | Upgrade flow config                                      |
| `--flow`          | string   | `install`          | Flow type                                                |
| `--chart-version` | string   |                    | Chart version (migrator detection)                       |
| `--infra-type`    | string   |                    | Infra pool: `preemptible`, `distroci`, `standard`, `arm` |
| `--values-config` | string   |                    | JSON config for env var overlay                          |
| `--env-file`      | string   |                    | Path to `.env` file                                      |
| `--output-dir`    | string   | temp dir           | Output directory                                         |
| `--interactive`   | bool     | `false`            | Interactive prompts                                      |

### Environment Variables

Configuration precedence: CLI flags > config file > environment variables > defaults.

**Root-level (`CAMUNDA_*`):**

| Variable                         | Description                |
| -------------------------------- | -------------------------- |
| `CAMUNDA_CURRENT`                | Active deployment profile  |
| `CAMUNDA_REPO_ROOT`              | Repository root path       |
| `CAMUNDA_PLATFORM`               | Target platform            |
| `CAMUNDA_LOG_LEVEL`              | Log level                  |
| `CAMUNDA_HOSTNAME`               | Ingress hostname           |
| `CAMUNDA_KEYCLOAK_HOST`          | Keycloak host              |
| `CAMUNDA_KEYCLOAK_PROTOCOL`      | Keycloak protocol          |
| `CAMUNDA_KEYCLOAK_REALM`         | Keycloak realm             |
| `CAMUNDA_KUBE_CONTEXT`           | Kubernetes context         |
| `CAMUNDA_EXTERNAL_SECRETS`       | Enable external secrets    |
| `CAMUNDA_SKIP_DEPENDENCY_UPDATE` | Skip helm dep update       |
| `CAMUNDA_SCENARIO_ROOT`          | Scenario root path         |
| `CAMUNDA_VALUES_PRESET`          | Chart-root overlay presets |

**Matrix-level (`CAMUNDA_MATRIX_*`):**

| Variable                             | Description          |
| ------------------------------------ | -------------------- |
| `CAMUNDA_MATRIX_PLATFORM`            | Platform filter      |
| `CAMUNDA_MATRIX_NAMESPACE_PREFIX`    | Namespace prefix     |
| `CAMUNDA_MATRIX_MAX_PARALLEL`        | Max concurrency      |
| `CAMUNDA_MATRIX_HELM_TIMEOUT`        | Helm timeout         |
| `CAMUNDA_MATRIX_KUBE_CONTEXT`        | Default kube context |
| `CAMUNDA_MATRIX_INGRESS_BASE_DOMAIN` | Base domain          |
| `CAMUNDA_MATRIX_ENV_FILE`            | Default env file     |
| `CAMUNDA_MATRIX_DRY_RUN`             | Dry run mode         |

---

## kubectl -- Debugging Deployments

These are the kubectl patterns you need for diagnosing and fixing problems with Camunda deployments in this repository. They are not an exhaustive kubectl reference -- they cover what you actually encounter working with these Helm charts.

### Check Deployment Health

```bash
# Overview of all pods and their status
kubectl get pods -n $NS -o wide

# Check if all pods are ready (returns non-zero if any aren't)
kubectl wait --for=condition=Ready pods --all -n $NS --timeout=300s

# Quick status of all workloads
kubectl get deployments,statefulsets -n $NS

# Check Helm release status
helm status $RELEASE -n $NS

# Get current values of a running release
helm get values $RELEASE -n $NS
helm get values $RELEASE -n $NS --all  # includes defaults
```

**Interpreting pod states:**

| Status                    | Meaning                                             | Next Step                                     |
| ------------------------- | --------------------------------------------------- | --------------------------------------------- |
| `Pending`                 | Unschedulable (resource limits, node selector, PVC) | `kubectl describe pod`                        |
| `CrashLoopBackOff`        | Container starts then crashes repeatedly            | `kubectl logs` (check previous: `--previous`) |
| `ImagePullBackOff`        | Cannot pull container image                         | Check image name, registry secret             |
| `Init:Error`              | Init container failed                               | `kubectl logs <pod> -c <init-container>`      |
| `Running` but not `Ready` | Readiness probe failing                             | `kubectl describe pod` (check probe config)   |
| `Terminating` stuck       | Finalizer or pre-stop hook stuck                    | `kubectl describe pod` (check finalizers)     |

### Debug Failing Pods

Follow this sequence to diagnose a failing pod:

```bash
# 1. Get pod details -- look for Events section at the bottom
kubectl describe pod <pod-name> -n $NS

# 2. Get logs from the main container
kubectl logs <pod-name> -n $NS

# 3. Get logs from ALL containers (useful for sidecars, init containers)
kubectl logs <pod-name> -n $NS --all-containers

# 4. Get logs from a crashed container's previous run
kubectl logs <pod-name> -n $NS --previous

# 5. Get logs from a specific container in a multi-container pod
kubectl logs <pod-name> -n $NS -c <container-name>

# 6. Stream logs in real-time
kubectl logs <pod-name> -n $NS --follow --tail=100

# 7. Check namespace events (sorted by time, most recent last)
kubectl get events -n $NS --sort-by=.lastTimestamp

# 8. Describe ALL pods at once (useful when you don't know which one is failing)
kubectl describe pods -n $NS
```

For Camunda-specific components, the pod names follow this pattern:

```
$RELEASE-zeebe-0, $RELEASE-zeebe-1, ...        # Orchestration StatefulSet (8.8+)
$RELEASE-connectors-<hash>                       # Connectors Deployment
$RELEASE-identity-<hash>                         # Identity Deployment
$RELEASE-operate-<hash>                          # Operate Deployment (8.6/8.7)
$RELEASE-tasklist-<hash>                         # Tasklist Deployment (8.6/8.7)
$RELEASE-optimize-<hash>                         # Optimize Deployment
$RELEASE-web-modeler-restapi-<hash>              # Web Modeler REST API
$RELEASE-web-modeler-websockets-<hash>           # Web Modeler WebSockets
$RELEASE-console-<hash>                          # Console Deployment
$RELEASE-keycloak-0                              # Keycloak StatefulSet
$RELEASE-postgresql-0                            # PostgreSQL StatefulSet
$RELEASE-elasticsearch-master-0                  # Elasticsearch StatefulSet
```

### Access Services Locally (Port-Forward)

Port-forward to access Camunda services from your local machine:

```bash
# Zeebe Gateway (gRPC) -- for zbctl or client apps
kubectl port-forward svc/$RELEASE-zeebe-gateway 26500:26500 -n $NS

# Operate
kubectl port-forward svc/$RELEASE-operate 8081:80 -n $NS

# Tasklist
kubectl port-forward svc/$RELEASE-tasklist 8082:80 -n $NS

# Identity
kubectl port-forward svc/$RELEASE-identity 8084:80 -n $NS

# Connectors
kubectl port-forward svc/$RELEASE-connectors 8085:8080 -n $NS

# Optimize
kubectl port-forward svc/$RELEASE-optimize 8083:80 -n $NS

# Console
kubectl port-forward svc/$RELEASE-console 8088:80 -n $NS

# Web Modeler REST API
kubectl port-forward svc/$RELEASE-web-modeler-restapi 8070:80 -n $NS

# Web Modeler WebSockets
kubectl port-forward svc/$RELEASE-web-modeler-websockets 8071:80 -n $NS

# Keycloak (admin console)
kubectl port-forward svc/$RELEASE-keycloak 18080:80 -n $NS

# Elasticsearch
kubectl port-forward svc/$RELEASE-elasticsearch 9200:9200 -n $NS
```

Check Zeebe cluster topology after port-forwarding:

```bash
kubectl exec svc/$RELEASE-zeebe-gateway -n $NS -- zbctl --insecure status
```

### Inspect Helm Releases

```bash
# Check release status and last deployment info
helm status $RELEASE -n $NS

# Get the values that were used in the last deployment
helm get values $RELEASE -n $NS

# Get ALL values including defaults (large output)
helm get values $RELEASE -n $NS --all

# List all releases in a namespace
helm list -n $NS

# Get release history (useful for rollback)
helm history $RELEASE -n $NS

# Template locally to diff against deployed manifests
helm template $RELEASE ./charts/camunda-platform-8.9 --values my-values.yaml | kubectl diff -f -
```

### Manage Secrets

```bash
# List all secrets in the namespace
kubectl get secrets -n $NS

# Extract a specific value from a secret (base64-decoded)
kubectl get secret <secret-name> -n $NS -o jsonpath="{.data.<key>}" | base64 -d

# Common: get Identity firstuser password
kubectl get secret $RELEASE-identity -n $NS -o jsonpath="{.data.identity-firstuser-password}" | base64 -d

# Common: get Keycloak admin password
kubectl get secret $RELEASE-keycloak -n $NS -o jsonpath="{.data.admin-password}" | base64 -d

# Create a Docker registry pull secret (idempotent)
kubectl create secret docker-registry registry-camunda-cloud \
  --namespace $NS \
  --docker-server=registry.camunda.cloud \
  --docker-username="$DOCKER_USER" \
  --docker-password="$DOCKER_PASS" \
  --dry-run=client -o yaml | kubectl apply -f -

# View the full contents of a secret (all keys, decoded)
kubectl get secret <secret-name> -n $NS -o json | jq '.data | map_values(@base64d)'
```

### Namespace Lifecycle

```bash
# Create namespace idempotently
kubectl create ns $NS --dry-run=client -o yaml | kubectl apply -f -

# Label a namespace (for CI tracking, filtering)
kubectl label ns $NS github-id=123 test-flow=install --overwrite

# Annotate with TTL (for automatic cleanup by CI janitor)
kubectl annotate ns $NS cleaner/ttl=1h --overwrite

# Delete a namespace (waits for all resources to terminate)
kubectl delete namespace $NS --wait=true

# Delete all namespaces by label (CI cleanup)
kubectl delete ns -l github-run-id="12345"
```

### Post-Uninstall Cleanup

After `helm uninstall`, PVCs are not automatically deleted. Clean them up:

```bash
# Uninstall the Helm release
helm uninstall $RELEASE -n $NS

# Delete PVCs created by the release
kubectl delete pvc -l app.kubernetes.io/instance=$RELEASE -n $NS

# Delete PVCs by component (more targeted)
kubectl delete pvc -l app.kubernetes.io/component=zeebe -n $NS
kubectl delete pvc -l app.kubernetes.io/component=elasticsearch -n $NS

# Nuclear option: delete the entire namespace
kubectl delete namespace $NS
```

### Verify Permissions

```bash
# Check if you can create deployments (basic auth check)
kubectl auth can-i create deployment

# Check kubectl version and cluster connectivity
kubectl version
kubectl cluster-info

# Check current context
kubectl config current-context

# Switch context
kubectl config use-context <context-name>
```

---

### Environment Variables for Test Scenarios

**Required:**

| Variable         | Description          | Example            |
| ---------------- | -------------------- | ------------------ |
| `TEST_NAMESPACE` | Kubernetes namespace | `camunda-platform` |

**Selection + composition (set the scenario configuration):**

| Variable           | Values                                                     | Default         |
| ------------------ | ---------------------------------------------------------- | --------------- |
| `TEST_IDENTITY`    | `keycloak`, `keycloak-external`, `oidc`, `basic`, `hybrid` | `keycloak`      |
| `TEST_PERSISTENCE` | `elasticsearch`, `opensearch`, `rdbms`, `rdbms-oracle`     | `elasticsearch` |
| `TEST_PLATFORM`    | `gke`, `eks`, `openshift`                                  | `gke`           |
| `TEST_FEATURES`    | `multitenancy`, `rba`, `documentstore` (comma-separated)   | (none)          |
| `TEST_QA`          | `true`/`false`                                             | `false`         |
| `TEST_IMAGE_TAGS`  | `true`/`false`                                             | `false`         |
| `TEST_UPGRADE`     | `true`/`false`                                             | `false`         |

**Optional:**

| Variable                          | Description                                                       |
| --------------------------------- | ----------------------------------------------------------------- |
| `TEST_INGRESS_HOST`               | Ingress hostname (required by `setup.venom.env`)                  |
| `E2E_TESTS_LICENSE_KEY`           | Camunda license key                                               |
| `TEST_CREATE_DOCKER_LOGIN_SECRET` | Set to enable Docker registry secret creation                     |
| `TEST_DOCKER_USERNAME`            | Docker registry username                                          |
| `TEST_DOCKER_PASSWORD`            | Docker registry password                                          |
| `TEST_CHART_FLOW`                 | `install`, `upgrade-patch`, or `upgrade-minor`                    |
| `TEST_CLUSTER_TYPE`               | `kubernetes` or `openshift`                                       |
| `INFRA_TYPE`                      | Infrastructure type: `preemptible`, `distroci`, `standard`, `arm` |

**Deprecated (still work, mapped automatically):**

| Legacy Variable        | Replacement        |
| ---------------------- | ------------------ |
| `TEST_VALUES_AUTH`     | `TEST_IDENTITY`    |
| `TEST_VALUES_BACKEND`  | `TEST_PERSISTENCE` |
| `TEST_VALUES_FEATURES` | `TEST_FEATURES`    |
| `TEST_VALUES_QA`       | `TEST_QA`          |
| `TEST_VALUES_INFRA`    | `TEST_PLATFORM`    |

### Common Test Scenarios

These are the standard configurations tested in CI. Use them as references for local testing:

| Identity            | Persistence     | Features        | Description                     |
| ------------------- | --------------- | --------------- | ------------------------------- |
| `keycloak`          | `elasticsearch` | (none)          | Default: embedded Keycloak + ES |
| `keycloak-external` | `elasticsearch` | (none)          | External shared Keycloak + ES   |
| `keycloak-external` | `opensearch`    | (none)          | External Keycloak + OpenSearch  |
| `keycloak-external` | `elasticsearch` | `multitenancy`  | Multi-tenancy with ES           |
| `keycloak-external` | `opensearch`    | `multitenancy`  | Multi-tenancy with OpenSearch   |
| `keycloak-external` | `elasticsearch` | `documentstore` | Document store feature          |
| `keycloak-external` | `rdbms`         | (none)          | RDBMS persistence               |
| `oidc`              | `elasticsearch` | (none)          | OIDC (Entra ID) authentication  |
| `basic`             | `elasticsearch` | (none)          | Basic auth (no Keycloak)        |

---

## Troubleshooting Decision Tree

### Deployment fails (Helm timeout or error)

```
1. Check pod status:
   kubectl get pods -n $NS -o wide

2. If pods are Pending:
   kubectl describe pod <pod> -n $NS
   → Look for: insufficient resources, unbound PVC, node selector mismatch

3. If pods are CrashLoopBackOff:
   kubectl logs <pod> -n $NS --previous
   → Look for: missing config, bad env vars, OOM, connection refused

4. If pods are ImagePullBackOff:
   kubectl describe pod <pod> -n $NS
   → Check: image name typo, missing registry secret, expired credentials

5. Check namespace events for scheduling/volume issues:
   kubectl get events -n $NS --sort-by=.lastTimestamp

6. Check Helm release status:
   helm status $RELEASE -n $NS
```

### Values don't look right

```
1. Render templates without deploying:
   deploy-camunda --render-templates --render-output-dir ./debug ...

2. Or use prepare-values to see the merged file:
   deploy-camunda prepare-values --chart-path ./charts/camunda-platform-8.9 \
     --identity keycloak --persistence elasticsearch --interactive=false
   # Then inspect the output file

3. Compare against running release:
   helm get values $RELEASE -n $NS > running-values.yaml
   diff running-values.yaml <merged-values-file>
```

### Integration tests timeout

```
1. Check if all pods are ready:
   kubectl get pods -n $NS

2. Check the test job status:
   kubectl get jobs -n $NS
   kubectl describe job <test-job> -n $NS

3. Stream test logs:
   kubectl logs -n $NS --follow job/<test-job-name>

4. Increase Helm timeout:
   deploy-camunda --timeout 15 ...
   # or in Taskfile: TEST_HELM_EXTRA_ARGS="--timeout 25m0s"
```

### Scenario not found

```
1. The CLI will list available scenarios in the error message.

2. Check the scenario directory for your chart version:
   ls charts/camunda-platform-8.9/test/integration/scenarios/chart-full-setup/

3. For layered values, check the values/ subdirectory:
   ls charts/camunda-platform-8.9/test/integration/scenarios/chart-full-setup/values/

4. Use matrix list to see all configured scenarios:
   deploy-camunda matrix list --repo-root . --versions 8.9
```

### Cannot connect to services after deployment

```
1. Verify pods are running:
   kubectl get pods -n $NS

2. Check service exists:
   kubectl get svc -n $NS

3. Port-forward (see "Access Services Locally" section above)

4. If using ingress, check ingress resource:
   kubectl get ingress -n $NS
   kubectl describe ingress -n $NS

5. If using Gateway API:
   kubectl get httproute -n $NS
   kubectl get gateway -n $NS
```

### Post-uninstall issues (PVCs, stuck namespace)

```
1. Delete PVCs left behind:
   kubectl delete pvc -l app.kubernetes.io/instance=$RELEASE -n $NS

2. If namespace stuck in Terminating:
   kubectl get ns $NS -o json | jq '.spec.finalizers = []' | kubectl replace --raw "/api/v1/namespaces/$NS/finalize" -f -

3. Check for leaked resources:
   kubectl api-resources --verbs=list --namespaced -o name | \
     xargs -n 1 kubectl get --show-kind --ignore-not-found -n $NS
```
