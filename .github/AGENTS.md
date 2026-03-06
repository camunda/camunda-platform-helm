# Agent Instructions

This is the **Camunda 8 Self-Managed Helm Charts** repository. It contains Helm charts for deploying the Camunda Platform on Kubernetes, along with Go-based CLI tooling for deployment automation, integration testing, and CI/CD.

## State File

Use `STATE.md` (repo root) to persist session context across conversations. This file is gitignored.

**On session start:** Read `STATE.md` if it exists. Use it to understand the current goal, what has been done, what remains, and any discoveries or decisions made so far.

**During work:** Update `STATE.md` whenever you make meaningful progress — after completing a task, discovering something important, or making a decision. Do not wait until the end of the session.

**Format:**

```markdown
## Goal

One-line summary of what we are working on.

## Instructions

Constraints, preferences, or standing orders from the user that apply across sessions.

## Discoveries

Key findings, root causes, gotchas, and architectural decisions made during investigation.

## Accomplished

Numbered list of completed items with enough detail to not repeat work.

## Not Yet Done

Numbered list of remaining items, in priority order.

## Relevant Files

Files and directories that are central to the current task, with brief annotations.
```

Keep it concise. The file should be useful to a fresh agent session that has never seen prior conversation history.

## Repository Structure

```
charts/
  camunda-platform-8.9/     # Latest (alpha) - unified "orchestration" component
  camunda-platform-8.8/     # Unified "orchestration" component
  camunda-platform-8.7/     # Separate zeebe/, operate/, tasklist/ templates
  camunda-platform-8.6/     # Separate zeebe/, operate/, tasklist/ templates
  camunda-platform-8.5/     # Extended support
  camunda-platform-8.4/     # Extended support
  camunda-platform-8.3/     # Extended support
  camunda-platform-8.2/     # Extended support
  keycloak-24/               # Bitnami Keycloak sub-chart
  web-modeler-postgresql*/   # PostgreSQL sub-charts for Web Modeler

scripts/
  deploy-camunda/            # Primary deployment CLI (Go, Cobra)
  camunda-deployer/          # Deployment orchestrator
  prepare-helm-values/       # Values preparation/merging
  vault-secret-mapper/       # Vault-to-K8s secret mapping
  camunda-core/              # Shared Go packages (scenario resolution, kube client)
  values-injector/           # Values injection utility

test/
  integration/scenarios/     # Cross-version integration test scenarios
  integration/testsuites/    # Test suite definitions (Venom, Playwright)

version-matrix/              # Version compatibility matrices (8.2 through 8.9)
infra/                       # Infrastructure values for shared ES/Keycloak
docs/                        # Internal developer documentation
```

### Architecture: 8.6/8.7 vs 8.8/8.9

- **8.6 and 8.7:** Separate template directories for `zeebe/`, `zeebe-gateway/`, `operate/`, `tasklist/`.
- **8.8 and 8.9:** These merged into a single `orchestration/` component (Zeebe + Operate + Tasklist unified).

When making changes across chart versions, check which structure applies. Do not assume templates are the same across versions.

### Chart Components (8.8+)

| Component     | Templates                  | Key Resources                                              |
| ------------- | -------------------------- | ---------------------------------------------------------- |
| Orchestration | `templates/orchestration/` | StatefulSet, Services, ConfigMap, GRPCRoute, HTTPRoute     |
| Connectors    | `templates/connectors/`    | Deployment, Service, ConfigMap, PVC                        |
| Console       | `templates/console/`       | Deployment, Service, ConfigMap                             |
| Identity      | `templates/identity/`      | Deployment, Service, ConfigMap, PVC                        |
| Optimize      | `templates/optimize/`      | Deployment, Service, ConfigMap, PVC                        |
| Web Modeler   | `templates/web-modeler/`   | 2 Deployments (restapi + websockets), Services, ConfigMaps |
| Common/Shared | `templates/common/`        | Ingress, Gateway, ReferenceGrant, shared ConfigMaps        |

### Chart Dependencies

Each chart version depends on: Bitnami Keycloak (local sub-chart), Bitnami PostgreSQL (OCI), Bitnami Elasticsearch (OCI), Bitnami common helpers (OCI). Web Modeler has its own PostgreSQL sub-chart.

## Conventions

### Commits and PRs

Commit messages and PR titles use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/#summary) format:

```
<type>[optional scope]: <description>
```

Valid types: `feat`, `fix`, `refactor`, `revert`, `test`, `docs`, `style`, `build`, `ci`, `cd`, `chore`, `chore(deps)`, `chore(release)`, `deps`, `perf`.

Keep the header under 120 chars (prefer under 72). The description should be in present tense.

### Go Code

- Complex CI logic (>20 lines) must be implemented as Go scripts in `scripts/`, not bash.
- All Go scripts must have unit tests.
- Go code uses the golden file (snapshot) testing pattern. After changes that affect rendered output, run:

  ```bash
  make go.update-golden-only
  ```

### Branches

Branch naming: `issueId-description` (e.g., `123-adding-bpel-support`).

## Tool Versions

Pinned in `.tool-versions` (managed by `asdf`). Install all with: `asdf install`

| Tool      | Version | Notes                                |
| --------- | ------- | ------------------------------------ |
| Go        | 1.26    | Required for all `scripts/` tooling  |
| Helm      | 3.20    | Chart rendering, linting, deployment |
| kubectl   | 1.27.16 | Matches CI cluster version           |
| kind      | 0.31    | Local Kubernetes clusters            |
| yq        | 4.52.4  | YAML processing                      |
| jq        | 1.8.1   | JSON processing                      |
| kustomize | 5.8.1   | Test suite deployment                |
| bats      | 1.11.0  | Bash testing                         |

## Common Development Tasks

```bash
make install.dx-tooling          # Build and install all Go CLI tools to $GOPATH/bin
make go.test                     # Run unit tests (checks against golden files)
make go.update-golden-only       # Update golden files after template changes
make helm.lint                   # Lint all Helm charts
make helm.dependency-update      # Update chart dependencies
make precommit.chores            # Pre-commit chores (lint + readme + schema + golden files)
make helm.template chartPath=charts/camunda-platform-8.9   # Template a chart (inspect output)
make helm.dry-run chartPath=charts/camunda-platform-8.9    # Dry-run an install
```

Most `make` targets accept `chartPath` to target a specific version (e.g., `make go.test chartPath=charts/camunda-platform-8.9`).

### Values Files

Each chart has multiple values overlays:

| File                         | Purpose                                      |
| ---------------------------- | -------------------------------------------- |
| `values.yaml`                | Default values (primary, heavily documented) |
| `values-latest.yaml`         | Latest upstream image tags                   |
| `values-local.yaml`          | Local development overrides                  |
| `values-enterprise.yaml`     | Enterprise feature overrides                 |
| `values-bitnami-legacy.yaml` | Legacy Bitnami compatibility                 |
| `values-digest.yaml`         | Image digest pinning                         |
| `values.schema.json`         | JSON Schema for validation                   |

## Layered Values System (Integration Tests)

For chart versions 8.6+, integration test values use a layered composition model instead of monolithic scenario files. Layers are resolved in order (last wins):

```
base.yaml -> base-upgrade.yaml (if upgrade flow) -> identity -> persistence -> platform -> features -> QA -> image-tags
```

These live in `test/integration/scenarios/chart-full-setup/values/` per chart version. The `deploy-camunda` CLI handles resolution and merging automatically.

**Critical: Helm arrays replace, they do not merge.** If a later layer sets `orchestration.env`, it completely replaces the array from `base.yaml`. Any env vars from base.yaml that are still needed must be re-included in the later layer. This is a common source of bugs when adding values to upgrade or feature layers.

For detailed documentation on how scenario resolution works, see `docs/integration-test-scenario-resolution.md`.

## CI Test Matrix

Each chart version has a `test/ci-test-config.yaml` defining scenarios (e.g., `elasticsearch`, `opensearch`). Each scenario specifies identity, persistence, platforms, and allowed flows. The matrix is filtered by `.github/config/permitted-flows.yaml` which denies specific flows per version (e.g., 8.9 denies `upgrade-patch` but allows `upgrade-minor`).

Upgrade flows are two-step: install the previous version's chart from the Helm repo, then `helm upgrade` to the local chart. The `base-upgrade.yaml` layer is included only in Step 2.

## Operational Skills

See `SKILLS.md` for instructions on using:

- **`deploy-camunda` CLI** — Deploy Camunda to Kubernetes, manage configs, run test matrices
- **`kubectl`** — Debug deployments, check pod health, access services, manage secrets

## Testing

### Unit Tests

Located in each chart: `charts/camunda-platform-<version>/test/unit/`. Uses terratest + testify with golden file snapshots. Run with `make go.test`.

### Integration Tests

Deploy to real Kubernetes clusters using predefined scenarios. Managed by `deploy-camunda` CLI. See `SKILLS.md` for operational details.

### E2E Tests

Playwright-based, located in `charts/camunda-platform-<version>/test/e2e/`. Run via `deploy-camunda --test-e2e`.

## Credentials

Docker registry credentials are required for cluster deployments. Before running `deploy-camunda matrix run` or any deployment that needs image pulling, ask the user to ensure the following environment variables are set:

- **Harbor** (`registry.camunda.cloud`): `TEST_DOCKER_USERNAME_CAMUNDA_CLOUD` and `TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD`
- **Docker Hub**: `TEST_DOCKER_USERNAME` and `TEST_DOCKER_PASSWORD`

Both are needed when `ensureDockerHub` and `ensureDockerRegistry` are `true` in `.camunda-deploy.yaml`. Do not attempt to extract credentials automatically — ask the user to set them up.

## Development tips

- Complex logic for CI pipelines (>20 lines) should be implemented as golang scripts inside the scripts directory and then called with github actions. Do not implement this in bash.
- When writing any golang, the scripts must have unit tests
