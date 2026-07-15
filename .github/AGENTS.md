# Agent Instructions — CI/CD Architecture

This is the **Camunda 8 Self-Managed Helm Charts** repository. It contains Helm charts for deploying the Camunda Platform on Kubernetes, along with Go-based CLI tooling for deployment automation, integration testing, and CI/CD.

This file covers repository structure and CI architecture. Coding rules, commands, conventions, and the `STATE.md` session-continuity protocol live in the root `AGENTS.md`; operational runbooks live in `.claude/skills/` (index: `SKILLS.md`).

## Repository Structure

```
charts/
  camunda-platform-8.10/    # Latest - unified "orchestration" component
  camunda-platform-8.9/     # Unified "orchestration" component
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
  prepare-helm-values/       # Values preparation/merging
  vault-secret-mapper/       # Vault-to-K8s secret mapping
  camunda-core/              # Shared Go packages (scenario resolution, kube client)
  release-tools/             # Release-pipeline CLI (image set, Harbor tags, release notes, values injection)

test/
  integration/scenarios/     # Cross-version integration test scenarios
  integration/testsuites/    # Test suite definitions (Venom, Playwright)

version-matrix/              # Version compatibility matrices (8.2 through 8.10)
infra/                       # Infrastructure values for shared ES/Keycloak
docs/                        # Internal developer documentation
```

### Architecture: 8.6/8.7 vs 8.8+

- **8.6 and 8.7:** Separate template directories for `zeebe/`, `zeebe-gateway/`, `operate/`, `tasklist/`.
- **8.8, 8.9, and 8.10:** These merged into a single `orchestration/` component (Zeebe + Operate + Tasklist unified).

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

Commit/branch/PR conventions: root `AGENTS.md`. Path-scoped chart-coding conventions live in `.github/instructions/*.instructions.md` — their `applyTo:` globs are not auto-applied by Claude Code, so read the matching guide explicitly before editing files in that path. Tool versions: `.tool-versions` (kubectl is pinned to match the CI cluster version).

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

These live in `test/integration/scenarios/chart-full-setup/values/` per chart version. The `deploy-camunda` CLI handles resolution and merging automatically — array merging is name-keyed, unlike raw Helm (see root `AGENTS.md` → Subchart Values Gotchas).

**Image-tag activation:** The image-tags layer (`base-image-tags.yaml`) is enabled by setting `image-tags: true` in `ci-test-config.yaml`. All `qa-*` scenarios have this set because they always receive SNAPSHOT versions from nightly CI. When active, neither `values-digest.yaml` nor `values-latest.yaml` is applied — image versions come entirely from `base-image-tags.yaml` with placeholder substitution from the `.env` file (loaded by `buildScenarioEnv()`). In CI, the workflow converts the `VALUES_CONFIG` JSON to a `.env` file and passes it via `--env-file`.

For the full data flow and per-version resolution, see `docs/skills/integration-test-scenario-resolution.md`.

## CI Test Matrix

Each chart version has a `test/ci-test-config.yaml` defining scenarios (e.g., `elasticsearch`, `opensearch`). Each scenario specifies identity, persistence, platforms, and allowed flows. The matrix is filtered by `.github/config/permitted-flows.yaml` which denies specific flows per version (e.g., 8.9 denies `upgrade-patch` but allows `upgrade-minor`).

**Tier split:** `pull_request` runs **tier-1 only** — `test-chart-version.yaml` passes `tier: 1` on PR events. The **full matrix** (tier-2 plus untiered scenarios) runs on `merge_group` (merge queue). A PR that adds or changes a tier-2 scenario gets no CI signal until merge is clicked; validate tier-2 changes locally before merge — see the `rfr-validation` skill (`.claude/skills/rfr-validation/SKILL.md`).

Scenarios and tiers live in the composable registry `test/ci/registry/manifest.yaml` (8.6 predates the registry, keeps the legacy inline `test/ci-test-config.yaml`, and has no active CI — manual `workflow_dispatch` only). Use `deploy-camunda matrix list --tier 2 --versions <v>` to re-derive the current set regardless of source.

Upgrade flows are two-step: install the previous version's chart from the Helm repo, then `helm upgrade` to the local chart. The `base-upgrade.yaml` layer is included only in Step 2.

## Testing

- **Unit tests:** `charts/camunda-platform-<version>/test/unit/` — terratest + testify with golden file snapshots; `make go.test`.
- **Integration tests:** deploy to real Kubernetes clusters using predefined scenarios, managed by the `deploy-camunda` CLI (see the `deploy-camunda` skill).
- **E2E tests:** Playwright-based, `charts/camunda-platform-<version>/test/e2e/`; run via `deploy-camunda --test-e2e` (see the `e2e-testing` skill).

## Credentials

Docker registry credentials are required for cluster deployments — the exact env vars, expected values, and pre-flight checks are in the `gke-verification` skill (`.claude/skills/gke-verification/SKILL.md`). Do not attempt to extract credentials automatically — ask the user to set them up.

## Development tips

- When debugging CI deployment failures, **always check the `diagnostics/` folder at the repo root first**. It contains output from the last `deploy-camunda matrix run` with pod status, events, and logs from failing pods. The folder is gitignored, so `glob` may not find it — use `read` on the repo root directory to see it.
