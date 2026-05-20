---
id: github-actions-workflows
title: GitHub Actions Workflows
---

This repo has many [GitHub Actions workflows](https://github.com/camunda/camunda-platform-helm/tree/main/.github/workflows) for different aspects of the CI pipelines.

- [Camunda Helm chart deployment](#camunda-helm-chart-deployment)
  - [Getting started](#getting-started)
  - [Workflow inputs](#workflow-inputs)
  - [Workflow patterns](#workflow-patterns)
- [Check `values-latest.yaml`](#check-values-latestyaml)
- [Integration test workflows](#integration-test-workflows)

## Camunda Helm chart deployment

The Distro team provides a GitHub Actions workflow to deploy the Camunda Helm chart via GitHub Actions. This workflow is customizable and supports different patterns. For example: disabling integration tests, single namespace, multi-namespace, persistent setup with a defined TTL (not deleted after the workflow), and more.

### Getting started

The quickest way to use this workflow is the example template:

- Copy [`test-helm-chart.yaml`](https://github.com/camunda/camunda-platform-helm/blob/main/.github/workflows/examples/test-helm-chart.yaml) to your repo under `.github/workflows/test-helm-chart.yaml`.
- Replace `<USE_CASE>` in that file with a clear identifier for your use case (e.g. `my-team-dev-env`).
- **Done.** The Helm chart integration test will run with each PR in your repo.

You can see the workflow in action on the Camunda apps repo: [Camunda Helm Chart Integration Test](https://github.com/camunda/camunda/actions/workflows/camunda-helm-integration.yaml).

### Workflow inputs

These inputs allow you to customize Helm chart deployments.

In most cases, you just need to set `identifier`, `camunda-helm-dir`, and `extra-values`.

```yaml
jobs:
  # ...
  helm-deploy:
    name: Helm chart Integration Tests
    uses: camunda/camunda-platform-helm/.github/workflows/test-integration-template.yaml@main
    secrets: inherit
    with:
      # Unique identifier used in the deployment hostname
      # Required: true
      identifier: 'console-team-dev-env'

      # A reference for the Camunda Helm chart directory which allows testing unreleased changes from the Git repo.
      # The latest supported chart doesn't have a version in its directory name like `camunda-platform`.
      # The previous releases have the Camunda version in their directory name e.g. `camunda-platform-8.4`.
      # Required: true
      camunda-helm-dir: 'camunda-platform-8.8'

      # Optional: Deploy the packaged chart artifact from the OCI registry instead of packaging from the git checkout.
      # Example values: '13.4.0-rc', '13.4.0', '13.4.0-dev-abc1234'
      # Required: false
      helmChartVersion: ''

      # Pass extra values to the Helm chart during deployment
      # Default: ''
      # Required: false
      extra-values: |
        global:
          image:
            tag: 8.2.10
        console:
          image:
            tag: xyz

      # Git reference for the Camunda Helm chart repository
      # Default: 'main'
      # Required: false
      camunda-helm-git-ref: 'main'

      # Git reference of the caller's repository (branch, tag, or commit SHA) that initiated the workflow
      # Default: 'main'
      # Required: false
      caller-git-ref: 'main'

      # Define a TTL for the deployment after the workflow is completed
      # Note: All persistent deployments will be deleted frequently to save costs
      # Default: ''
      # Required: false
      deployment-ttl: ''

      # Specifies the cloud platform that is currently used
      # Default: 'gke'
      # Required: false
      platforms: 'gke'

      # Types of operations to perform with the Helm chart, like install, upgrade
      # Default: 'install'
      # Required: false
      flows: 'install'

      # Flag to enable or disable the execution of test scenarios after Helm chart deployment
      # Default: true
      # Required: false
      test-enabled: true

      # Define the infrastructure that will be used to run the deployment.
      # Default: 'preemptible'
      # Required: false
      infra-type: 'preemptible'
```

<details>
  <summary>Notes</summary>

**General**

- Adjust `identifier`, `caller-git-ref`, `flows`, `test-enabled`, and `extra-values` as needed for your specific testing scenario.
- The `identifier` is essential for distinguishing between different deployments, particularly useful in environments with multiple parallel deployments.
- For `extra-values`, ensure the YAML format is correct and that the values specified meet the requirements for your environment.
- For more details on how to use these inputs within the workflow or to modify them for specific testing needs, refer to the official [GitHub Actions documentation](https://docs.github.com/en/actions).

**Lifecycle**

- The default behavior in the integration tests workflow is to delete the test resources after the test is finished.
- To keep the deployment for at least one day, set `deployment-ttl: 1d`.
- You need to rerun the workflow when you need the deployment to be persistent with a defined `deployment-ttl`.
- Example `deployment-ttl` values:
  - `360s`: 360 seconds
  - `10m`: 10 minutes
  - `24h`: 24 hours
  - `7d`: 7 days
  - `2w`: 2 weeks

</details>

### Workflow patterns

The most important workflow is the [main integration workflow](https://github.com/camunda/camunda-platform-helm/blob/main/.github/workflows/test-integration-template.yaml), where each Camunda component can reuse that workflow in their CI pipelines to ensure each component works as expected within Camunda as a whole.

#### Single namespace

To embed the Camunda Helm chart integration tests in your GHA workflow, use the following:

```yaml
jobs:
  # ...
  helm-deploy:
    name: Helm chart Integration Tests
    uses: camunda/camunda-platform-helm/.github/workflows/test-integration-template.yaml@main
    secrets: inherit
    with:
      identifier: dev-console-sm
      extra-values: |
        global:
          image:
            tag: 8.2.10
        console:
          image:
            tag: xyz
```

Adding that will run Camunda Helm chart integration tests and add the deployment URL in your repo (the URL will show in the PR or the GH deployment section).

Check the example in the [Getting started](#getting-started) section for more details.

#### Multi-namespace

```yaml
jobs:
  # ...
  helm-deploy:
    strategy:
      matrix:
        deployment:
          - id: management
            extra-values: |
              zeebe:
                enabled: false
              zeebeGateway:
                enabled: false
              operate:
                enabled: false
              tasklist:
                enabled: false
              optimize:
                enabled: false
              connectors:
                enabled: false
              elasticsearch:
                enabled: false
          - id: team01
            extra-values: |
              global:
                identity:
                  service:
                    url: "http://integration-identity.camunda-platform-id-dev-console-sm-main.svc.cluster.local:80/identity"
                  keycloak:
                    url:
                      protocol: "http"
                      host: "integration-keycloak.camunda-platform-id-dev-console-sm-main.svc.cluster.local"
                      port: "80"
              identity:
                enabled: false
              webModeler:
                enabled: false
              postgresql:
                enabled: false
              console:
                enabled: false
          - id: team02
            extra-values: |
              global:
                identity:
                  service:
                    url: "http://integration-identity.camunda-platform-id-dev-console-sm-main.svc.cluster.local:80/identity"
                  keycloak:
                    url:
                      protocol: "http"
                      host: "integration-keycloak.camunda-platform-id-dev-console-sm-main.svc.cluster.local"
                      port: "80"
              identity:
                enabled: false
              webModeler:
                enabled: false
              postgresql:
                enabled: false
              console:
                enabled: false
    name: Helm integration tests - ${{ matrix.deployment.id }}
    uses: camunda/camunda-platform-helm/.github/workflows/test-integration-template.yaml@main
    secrets: inherit
    with:
      identifier: dev-console-sm-${{ matrix.deployment.id }}
      deployment-ttl: 1d
      extra-values: |
        ${{ matrix.deployment.extra-values }}
```

#### Persistent deployment

If you have long-running workflows with multiple jobs, you can set `deployment-ttl: 1d`, which keeps the deployment namespace for one day instead of deleting it after the workflow is done.

#### Non-ephemeral infrastructure

By default, all of our CI workloads run on preemptible nodes, meaning the workflows should be fault-tolerant and have a retry mechanism. If your team needs a stable, non-ephemeral infrastructure, please contact the Distribution team for instructions.

## Check `values-latest.yaml`

The [`check-values-latest.yaml`](https://github.com/camunda/camunda-platform-helm/blob/main/.github/workflows/check-values-latest.yaml) workflow validates that all `values-latest.yaml` files contain the latest Docker image versions for Camunda components.

### Purpose

We provide `values-latest.yaml` files as a method to install the latest app versions even before the next Helm chart is released. These files are automatically updated by Renovate, but this workflow validates that the auto-update process is working correctly.

### When it runs

- **Daily:** Scheduled to run at 6 AM UTC every day.
- **On pull requests:** When changes are made to:
  - `charts/camunda-platform-*/values-latest.yaml`
  - `scripts/check-values-latest.sh`
  - `.github/workflows/check-values-latest.yaml`
- **Manually:** Via `workflow_dispatch` with an optional chart version parameter.

### What it checks

The workflow validates that image tags in `values-latest.yaml` files match the latest available tags in Docker Hub for:

- Camunda components (console, connectors, operate, optimize, tasklist, zeebe, identity, webModeler).
- All supported chart versions (8.2, 8.3, 8.4, 8.5, 8.6, 8.7, 8.8, 8.9).

### Behavior

- **SNAPSHOT/alpha tags:** Automatically skipped, as they always represent the latest development versions.
- **Non-Camunda images:** Currently skipped (Bitnami images, etc.).
- **Validation failures:** The workflow fails and comments on PRs if any image versions are outdated.

### Manual usage

To check a specific chart version:

```yaml
# Via GitHub UI: Actions > Check values-latest.yaml > Run workflow
# Specify chart version (e.g. 8.7) or leave empty to check all versions
```

To run the validation script locally:

```bash
# Check all chart versions
./scripts/check-values-latest.sh

# Check specific version
./scripts/check-values-latest.sh 8.7
```

#### Image git commit information

The integration test workflows automatically display git commit information for each Docker image used in the deployment. This information appears in the GitHub Actions job summary after the Helm install or upgrade completes.

The feature:

- Extracts the `org.opencontainers.image.revision` label from each Docker image using `skopeo`.
- Uses remote image inspection without pulling full images (fast and efficient).
- Displays the git commit hash in a markdown table format.
- Makes it easy to trace a CI job to specific application commits.
- Eliminates the need to manually pull and inspect Docker images.

Example output:

| Component | Git Commit |
|---|---|
| zeebe | `f508f35d5b4` |
| operate | `f508f35d5b4` |
| tasklist | `f508f35d5b4` |

This information is displayed automatically for all workflows using the `test-integration` templates.

## Integration test workflows

Integration tests use a three-layer workflow chain. Each layer fans out via GitHub Actions matrix strategies to test across platforms (GKE, EKS, ROSA) and flows (install, upgrade) in parallel.

```
test-chart-version-template.yaml          (Layer 1: per-platform dispatch)
  └─▶ test-integration-template.yaml      (Layer 2: platform × flow matrix generation)
       └─▶ test-integration-runner.yaml   (Layer 3: deploy + test execution per shard)
```

### Why three layers?

GitHub Actions' `needs` keyword waits for _all_ matrix legs of an upstream job before _any_ downstream job starts. Without sharding into separate reusable workflow calls, GKE tests would block until the slowest platform (e.g. ROSA) finishes installing. Each reusable workflow call is an independent "shard" with its own `install → upgrade → test` pipeline, so platforms don't wait on each other.

### Layer 1: `test-chart-version-template`

> [`.github/workflows/test-chart-version-template.yaml`](https://github.com/camunda/camunda-platform-helm/blob/main/.github/workflows/test-chart-version-template.yaml)

Entry point called by PR and release workflows. Splits `platforms` (e.g. `"gke,eks"`) into a JSON array and calls Layer 2 once per platform. Sets `deployment-ttl` to `1w` when the PR has the `test-persistent` label.

### Layer 2: `test-integration-template`

> [`.github/workflows/test-integration-template.yaml`](https://github.com/camunda/camunda-platform-helm/blob/main/.github/workflows/test-integration-template.yaml)

Generates the full **platform × flow** matrix and dispatches each combination to Layer 3.

#### How matrix generation works

The `init` job reads [`.github/config/test-integration-matrix.yaml`](https://github.com/camunda/camunda-platform-helm/blob/main/.github/config/test-integration-matrix.yaml) — a YAML file with `$VARIABLE` placeholders in `if` fields. The job:

1. Exports boolean env vars (`INPUTS_PLATFORMS_GKE=true`, `INPUTS_FLOWS_INSTALL=false`, etc.) based on the workflow inputs.
2. Runs `envsubst` to replace the `$VARIABLE` references with `true`/`false`.
3. Relies on the matrix `exclude` block to remove entries where `if: false`.
4. Converts the result to JSON via `yq` for the runner's `strategy.matrix`.

The `matrix-data` input can override the file-based matrix entirely.

### Layer 3: `test-integration-runner`

> [`.github/workflows/test-integration-runner.yaml`](https://github.com/camunda/camunda-platform-helm/blob/main/.github/workflows/test-integration-runner.yaml)

Executes a single shard (one platform + one flow). Job dependency graph:

```
install ─────┬──→ upgrade ──────┬──→ playwright-ITs-after-upgrade ──┐
             │                  └──→ playwright-e2e-after-upgrade ───┤
             ├──→ playwright-ITs-after-install ─────────────────────┤
             └──→ playwright-e2e-after-install ─────────────────────┤
                                                                    ↓
                                                          merge-e2e-reports
                                                                    ↓
                                                                cleanup
```

- **`install` flow:** install → tests → cleanup.
- **`upgrade-*` flows:** install (old version) → upgrade (current version) → tests → cleanup.
- **`modular-upgrade-minor`:** install skipped → upgrade only → tests → cleanup.

Each job runs in a fresh container and re-authenticates to the cluster independently. The Kubernetes namespace is the shared state between jobs.

### Test flows

#### `install`

Fresh deployment of the current branch's chart. Validates the chart installs cleanly.

#### `upgrade-patch`

Upgrade within the same minor version (e.g. `8.8.0 → 8.8.1`). The install job deploys the **latest released chart version** (via `camunda-helm-upgrade-version`), then the upgrade job upgrades to the current branch. A [pre-upgrade script](https://github.com/camunda/camunda-platform-helm/blob/main/charts/camunda-platform-8.8/test/integration/scenarios/pre-setup-scripts/pre-upgrade-patch.sh) handles incompatible resource cleanup (StatefulSets, PVCs).

#### `upgrade-minor`

Upgrade across minor versions (e.g. `8.7 → 8.8`). The install job deploys the **previous minor version** (via `camunda-version-previous`), then the upgrade job upgrades to the current version. For chart version 13+ (Camunda 8.13+), the upgrade automatically enables the data migrator.

#### `modular-upgrade-minor`

Component-by-component upgrade from a previous minor version. This flow **skips the install job** — it expects the namespace and Helm release to already exist from a prior deployment. It uses the same namespace (no suffix) as the install flow and runs only the upgrade job.

> **Important:** This flow cannot be invoked via `generate-chart-matrix.sh`. It must be called directly on `test-integration-template.yaml`. When used alongside `install` in the same flows string (e.g. `"install,modular-upgrade-minor"`), the two run as independent shards with **no ordering guarantee**.

### Namespace management

Each flow creates (or reuses) a namespace with a flow-specific suffix:

| Flow | Suffix | Example |
|---|---|---|
| `install` | _(none)_ | `ci-1234-intg-8.9-gke-es` |
| `upgrade-patch` | `-upgp` | `ci-1234-intg-8.9-gke-es-upgp` |
| `upgrade-minor` | `-upgm` | `ci-1234-intg-8.9-gke-es-upgm` |
| `modular-upgrade-minor` | _(none, reuses install)_ | `ci-1234-intg-8.9-gke-es` |

Namespaces are annotated with `cleaner/ttl` and `janitor/ttl` for automatic deletion by cluster-level cleaners, even if the workflow is cancelled. See [`.github/actions/workflow-vars/action.yaml`](https://github.com/camunda/camunda-platform-helm/blob/main/.github/actions/workflow-vars/action.yaml) for naming logic.
