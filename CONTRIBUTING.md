# Contributing to the Camunda Helm chart

We welcome new contributions. We take pride in maintaining and encouraging a friendly, welcoming, and collaborative community.

Anyone is welcome to contribute to Camunda! The best way to get started is to choose an existing [issue](#starting-on-an-issue).

For community-maintained Camunda projects, please visit the [Camunda Community Hub](https://github.com/camunda-community-hub). For connectors and process blueprints, please visit [Camunda Marketplace](https://marketplace.camunda.com/en-US/home) instead.

## Table of Contents

- [Prerequisites](#prerequisites)
  - [Code of Conduct](#code-of-conduct)
- [GitHub Issue Guidelines](#github-issue-guidelines)
  - [Severity and Likelihood (bugs)](#severity-and-likelihood-bugs)
  - [Determining the severity of an issue](#determining-the-severity-of-an-issue)
  - [Starting on an issue](#starting-on-an-issue)
- [Commit Message Guidelines](#commit-message-guidelines)
  - [Commit message header](#commit-message-header)
- [CI](#ci)
- [Integration Testing](#integration-testing)
  - [Overview](#overview)
  - [Scenarios](#scenarios)
  - [The deploy-camunda CLI (alpha)](#the-deploy-camunda-cli-alpha)
    - [Installation](#installation)
    - [Basic Usage](#basic-usage)
    - [Key Flags](#key-flags)
    - [Configuration File](#configuration-file)
    - [Environment Variables](#environment-variables)
  - [Requirements](#requirements)
    - [For Camunda Employees](#for-camunda-employees)
    - [For Community Contributors](#for-community-contributors)
  - [Running E2E Tests After Deployment](#running-e2e-tests-after-deployment)
  - [Troubleshooting](#troubleshooting)

## Prerequisites

<!-- TODO: uncomment this section when we have a CLA in place -->
<!-- ### Contributor License Agreement -->
<!---->
<!-- You will be asked to sign our [Contributor License Agreement](https://cla-assistant.io/camunda-community-hub/community) when you open a Pull Request. We are not asking you to assign copyright to us but to give us the right to distribute your code without restriction. We ask this of all contributors to assure our users of the origin and continuing existence of the code. -->
<!---->
<!-- > [!NOTE] -->
<!-- > In most cases, you will only need to sign the CLA once. -->

### Code of Conduct

This project adheres to the [Camunda Code of Conduct](https://camunda.com/events/code-conduct/). By participating, you are expected to uphold this code. Please [report](https://camunda.com/events/code-conduct/reporting-violations/) unacceptable behavior as soon as possible.

## GitHub issue guidelines

If you want to report a bug or request a new feature, feel free to open a new issue on [GitHub][issues].

If you report a bug, please help speed up problem diagnosis by providing as much information as possible.

> [!NOTE]
> If you have a general usage question, please ask on the [forum][forum].

Every issue should have a meaningful name and a description that either describes:

- A new feature with details about the use case the feature would solve or
  improve
- A problem, how we can reproduce it, and what the expected behavior would be
- A change and the intention of how this would improve the system

### Severity and Likelihood (bugs)

To help us prioritize, please also determine the severity and likelihood of the bug. To help you with this, here are the definitions for the options:

Severity:

- _Low:_ Having little to no noticeable impact on usage for the user (e.g. log noise)
- _Mid:_ Having a noticeable impact on production usage, which does not lead to data loss, or for which there is a known configuration workaround.
- _High:_ Having a noticeable impact on production usage, which does not lead to data loss, but for which there is no known workaround, or the workaround is very complex. Examples include issues which lead to regular crashes and break the availability SLA.
- _Critical:_ Stop-the-world issue with a high impact that can lead to data loss (e.g. corruption, deletion, inconsistency, etc.), unauthorized privileged actions (e.g. remote code execution, data exposure, etc.), and for which there is no existing configuration workaround.
- _Unknown:_ If it's not possible to determine the severity of a bug without in-depth investigation, you can select unknown. This should be treated as high until we have enough information to triage it properly.

Likelihood:

- _Low:_ rarely observed issue/ rather unlikely edge-case
- _Mid:_ occasionally observed
- _High:_ recurring issue

#### Determining the severity of an issue

Whenever possible, please try to determine the severity of an issue to the best of your knowledge.
Only select `Unknown` if it's really difficult to tell without spending a non-negligible amount of time (e.g. >1h) to
figure it out.

### Starting on an issue

The `main` branch contains the current in-development state of the project. To work on an issue, follow these steps:

1. Check that a [GitHub issue][issues] exists for the task you want to work on.
   If one does not, create one. Refer to the [issue guidelines](#github-issue-guidelines).
2. Check that no one is already working on the issue, and make sure the team would accept a pull request for this topic. Some topics are complex and may touch multiple of [Camunda's Components](https://docs.camunda.io/docs/components/), requiring internal coordination.
3. Checkout the `main` branch and pull the latest changes.

   ```
   git checkout main
   git pull
   ```

4. Create a new branch with the naming scheme `issueId-description`.

   ```
   git checkout -b 123-adding-bpel-support
   ```

5. Implement the required changes on your branch and regularly push your
   changes to the origin so that the CI can run. Code formatting, style, and
   license header are fixed automatically by running Maven. Checkstyle
   violations have to be fixed manually.

   ```
   git commit -am 'feat: add BPEL execution support'
   git push -u origin 123-adding-bpel-support
   ```

6. If you think you finished the issue, please prepare the branch for review. Please consider our [pull requests and code reviews](https://github.com/camunda/camunda/wiki/Pull-Requests-and-Code-Reviews) guide, before requesting a review. In general, the commits should be squashed into meaningful commits with a helpful message. This means cleanup/fix etc. commits should be squashed into the related commit. If you made refactorings it would be best if they are split up into another commit. Think about how a reviewer can best understand your changes. Please follow the [commit message guidelines](#commit-message-guidelines).

## Commit message guidelines

Commit messages use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/#summary) format.

```
<header>
<BLANK LINE> (optional - mandatory with body)
<body> (optional)
<BLANK LINE> (optional - mandatory with footer)
<footer> (optional)
```

Camunda uses a GitHub Actions workflow to check your commit messages when a pull request is submitted. Please make sure to address any hints from the bot, otherwise the PR cannot be merged.

**Exception:** In some situations it is not possible to avoid having commits that violate above guidelines, e.g. when merging another PR into the branch of your PR via merge commit or when merging back a release branch. Only in those cases you should explain the motivation and add the `ci:ignore-commitlint` label to your PR to disable the commit message checks.

### Commit message header

Examples:

- `docs: add guide for external Elasticsearch`
- `perf: increase memory limit of Orchestration Cluster`
- `feat: add sidecar for Optimize`

The commit header should match the following pattern:

```
%{type}: %{description}
```

The commit header should be kept short, preferably under 72 chars but we allow a max of 120 chars.

- `type` should be one of:
  - `build`: Changes that affect the build system (e.g. Maven, Docker, etc)
  - `ci`: Changes to our CI configuration files and scripts (e.g. GitHub Actions, etc)
  - `deps`: A change to the external dependencies (was already used by Dependabot)
  - `docs`: A change to the documentation
  - `feat`: A new feature (both internal or user-facing)
  - `fix`: A bug fix (both internal or user-facing)
  - `perf`: A code change that improves performance
  - `refactor`: A code change that does not change the behavior
  - `style`: A change to align the code with our style guide
  - `test`: Adding missing tests or correcting existing tests
- `description`: short description of the change in present tense

## CI

CI is performed via GitHub Actions [workflow](.github/workflows).

## Integration Testing

Integration tests verify that Helm charts can be deployed to Kubernetes and that services work correctly together. Unlike unit tests (which are expected for all contributions), **integration tests are primarily maintained by the Camunda team** and require access to Kubernetes infrastructure.

> [!NOTE]
>
> **For community contributors:** You are not expected to run integration tests. The CI pipeline handles this automatically. This section is provided for transparency and for those who want to understand or contribute to the testing infrastructure.

### Overview

Integration tests deploy the Camunda Platform to a real Kubernetes cluster using predefined **scenarios**. Each scenario is a set of Helm values that configure a specific deployment topology (e.g., with Keycloak authentication, Elasticsearch, OpenSearch, multi-tenancy, etc.).

The `deploy-camunda` CLI tool automates the deployment process, handling:
- Helm values file preparation and merging
- Unique identifier generation (Keycloak realms, Elasticsearch index prefixes)
- Secret management and credential injection
- Parallel deployment of multiple scenarios

### Scenarios

Scenarios are YAML values files located in each chart's test directory:

```
charts/<version>/test/integration/scenarios/chart-full-setup/
```

Files follow the naming convention: `values-integration-test-ingress-<scenario-name>.yaml`

**Available scenarios include:**

| Scenario | Description |
|----------|-------------|
| `keycloak-original` | Full deployment with Keycloak authentication and external elasticsearch |
| `elasticsearch` | Deployment using Elasticsearch for data storage |
| `opensearch` | Deployment using OpenSearch for data storage |
| `multitenancy` | Multi-tenant configuration |
| `qa-elasticsearch` | QA-specific configuration with custom image tags |
| `qa-opensearch` | QA-specific OpenSearch configuration |

Scenario files can reference environment variables (e.g., `$CAMUNDA_HOSTNAME`, `$E2E_TESTS_ZEEBE_IMAGE_TAG`) that are substituted at deployment time.

### The deploy-camunda CLI (alpha)

The `deploy-camunda` CLI is located at `scripts/deploy-camunda/` and provides a streamlined way to deploy Camunda Platform for integration testing.

#### Installation

From the repository root:

```bash
cd scripts/deploy-camunda
go build -o deploy-camunda .
```

Or with make:
```bash
make install.dx-tooling
```

Or run directly:

```bash
go run scripts/deploy-camunda/main.go [flags]
```

#### Basic Usage

```bash
# Deploy a single scenario
deploy-camunda --chart-path ./charts/camunda-platform-8.6 \
  --namespace my-test-ns \
  --release integration \
  --scenario keycloak

# Deploy multiple scenarios in parallel
deploy-camunda --chart-path ./charts/camunda-platform-8.6 \
  --namespace my-test-ns \
  --release integration \
  --scenario keycloak,elasticsearch
```

#### Key Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--chart-path` | | Path to the Camunda chart directory |
| `--chart` | `-c` | Chart name (for remote charts) |
| `--version` | `-v` | Chart version (requires `--chart`) |
| `--namespace` | `-n` | Kubernetes namespace |
| `--release` | `-r` | Helm release name |
| `--scenario` | `-s` | Scenario name(s), comma-separated for parallel deployment |
| `--scenario-path` | | Custom path to scenario files |
| `--auth` | | Auth scenario (default: `keycloak`) |
| `--platform` | | Target platform: `gke`, `rosa`, `eks` (default: `gke`) |
| `--timeout` | | Helm deployment timeout in minutes (default: 5) |
| `--delete-namespace` | | Delete namespace before deploying |
| `--auto-generate-secrets` | | Generate random test secrets |
| `--render-templates` | | Render manifests without installing |
| `--extra-values` | | Additional values files to apply |
| `--config` | `-F` | Path to config file |

#### Configuration File

Instead of passing flags every time, you can create a configuration file at `.camunda-deploy.yaml` (project-level) or `~/.config/camunda/deploy.yaml` (global).

**Example configuration:**

```yaml
# Global defaults
repoRoot: /path/to/camunda-platform-helm
platform: gke
logLevel: info

# Keycloak settings (for Camunda internal infrastructure)
keycloak:
  host: keycloak.example.com
  protocol: https

# Named deployment profiles
deployments:
  local-test:
    chartPath: ./charts/camunda-platform-8.6
    namespace: camunda-test
    release: integration
    scenario: keycloak
    
  qa-full:
    chartPath: ./charts/camunda-platform-8.6
    namespace: qa-integration
    release: integration
    scenario: qa-elasticsearch
    valuesPreset: enterprise

# Set the active deployment
current: local-test
```

**Using deployment profiles:**

```bash
# List configured deployments
deploy-camunda config list

# Switch active deployment
deploy-camunda config use qa-full

# Show merged configuration
deploy-camunda config show

# Run with active deployment settings
deploy-camunda
```

#### Environment Variables

The CLI supports environment variable overrides with the `CAMUNDA_` prefix:

| Variable | Description |
|----------|-------------|
| `CAMUNDA_CURRENT` | Active deployment profile |
| `CAMUNDA_REPO_ROOT` | Repository root path |
| `CAMUNDA_PLATFORM` | Target platform |
| `CAMUNDA_HOSTNAME` | Ingress hostname |
| `CAMUNDA_KEYCLOAK_HOST` | External Keycloak host |
| `CAMUNDA_KEYCLOAK_REALM` | Keycloak realm name |

You can also use a `.env` file (loaded automatically or via `--env-file`):

```bash
CAMUNDA_HOSTNAME=my-cluster.example.com
DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD=secretpassword
```

### Requirements

#### For Camunda Employees

Internal integration tests use shared infrastructure:

- **Kubernetes clusters**: GKE, EKS, or ROSA with appropriate permissions
- **External Keycloak**: Shared Keycloak instance for authentication
- **External Elasticsearch**: Shared Elasticsearch cluster
- **Vault secrets**: Credentials managed via HashiCorp Vault
- **Docker registry access**: Enterprise image pull secrets

Contact the Platform team for access to shared infrastructure and required credentials.

#### For Community Contributors

To run integration tests independently, you need:

1. **Kubernetes cluster**: Any Kubernetes cluster (minikube, kind, cloud provider)
2. **Helm 3.x**: Same version as specified in `.tool-versions`
3. **kubectl**: Configured to access your cluster
4. **Docker registry secrets**: For pulling Camunda images (if using enterprise features)

**Minimal local setup:**

```bash
# Create a kind cluster
kind create cluster --name camunda-test

# Deploy with auto-generated secrets (no external dependencies)
go run scripts/deploy-camunda/main.go \
  --chart-path ./charts/camunda-platform-8.6 \
  --namespace camunda \
  --release integration \
  --scenario elasticsearch \
  --auto-generate-secrets \
  --skip-dependency-update=false
```

### Running E2E Tests After Deployment

Once deployed, E2E tests can be run using the provided script:

```bash
./scripts/run-e2e-tests.sh \
  --absolute-chart-path /path/to/charts/camunda-platform-8.6 \
  --namespace my-test-ns
```

Additional flags:
- `--opensearch`: Run OpenSearch-specific tests
- `--mt`: Run multi-tenancy tests
- `--rba`: Run role-based access tests
- `--run-smoke-tests`: Run smoke tests only
- `--verbose`: Show detailed output

### Troubleshooting

**Scenario not found:**

The CLI provides helpful error messages listing available scenarios:

```
Scenario "invalid-name" not found

Searched in: charts/camunda-platform-8.6/test/integration/scenarios/chart-full-setup
Expected file: values-integration-test-ingress-invalid-name.yaml

Available scenarios (10 found):
  - keycloak
  - elasticsearch
  - opensearch
  ...
```

**Helm timeout:**

Increase the timeout for slower clusters:

```bash
deploy-camunda --timeout 15 ...
```

**Viewing rendered manifests:**

Use `--render-templates` to inspect what would be deployed:

```bash
deploy-camunda --render-templates --render-output-dir ./rendered ...
```