# Testing Architecture: camunda-platform-helm & c8-cross-component-e2e-tests

This document explains how integration and E2E testing works across the two repositories:

- **`camunda-platform-helm`** — Helm charts + Go tooling that deploys Camunda 8 to Kubernetes
- **`c8-cross-component-e2e-tests`** — Playwright tests that verify the running system end-to-end

---

## Repository Roles

| Repo | Role |
|---|---|
| `camunda-platform-helm` | Defines **what** to deploy (chart values, scenario config) and **how** (`deploy-camunda` CLI) |
| `c8-cross-component-e2e-tests` | Defines **what correctness looks like** (user flows, UI state, authorization checks) |

The `deploy-camunda` matrix runner is the bridge: after a successful deployment it calls out to the Playwright test suite from `c8-cross-component-e2e-tests`.

---

## Test Layers

### 1. Go Unit Tests (`make go.test`)

Fast, in-process, no cluster required.

- Chart template rendering (golden file snapshots in `test/unit/`)
- `deploy-camunda` CLI logic: flag parsing, YAML merging, scenario resolution
- Matrix runner helpers: `parseHelmValuesForIndexPrefixes`, `mergeHelmSets`, `parseHelmSetPairs`

```bash
make go.test chartPath=charts/camunda-platform-8.9
```

### 2. Integration Tests (IT)

Go tests in `charts/<version>/test/integration/` asserting on a live cluster. Triggered by `--test-it` on the matrix runner.

### 3. E2E Tests (Playwright)

Playwright tests from `c8-cross-component-e2e-tests`. Triggered by `--test-e2e`. Cover user-visible flows: login, process start, variable editing, authorization checks.

---

## Scenario Configuration

Everything starts in `charts/<version>/test/ci-test-config.yaml`. Each scenario composes four independent layers:

```
identity  x  persistence  x  flow  x  features
```

Example — nightly OpenSearch upgrade scenario:

```yaml
- name: qa-opensearch-upg
  flow: modular-upgrade-minor
  shortname: qaosupg
  identity: keycloak
  persistence: opensearch
  qa: true
  platforms: [gke]
```

The `flow` field controls which execution path the runner takes in `matrix/runner.go`.

---

## Values File Layering

The matrix runner assembles a values stack before every `helm install/upgrade`:

```
values.yaml                         (chart defaults)
  + values/persistence/<name>.yaml  (e.g. opensearch.yaml)
  + values/identity/<name>.yaml     (e.g. keycloak.yaml)
  + values/features/<name>.yaml     (e.g. tasklist-v1.yaml)
  + base-qa.yaml                    (QA image tags, license key)
  + base-upgrade.yaml               (upgrade-specific settings)
  + --set <ExtraHelmSets>           (runtime overrides, highest precedence)
```

The `persistence/opensearch.yaml` uses variable interpolation resolved by `prepare-helm-values` at deploy time:

```yaml
orchestration:
  index:
    prefix: $ORCHESTRATION_INDEX_PREFIX-$FLOW
    # e.g. "orch-qa-opensearch-abc123-install"
```

`$ORCHESTRATION_INDEX_PREFIX` is a random suffix generated per-run.  
`$FLOW` is the deployment flow name (e.g. `install`, `modular-upgrade-minor`).  
Together they make every deployment's OpenSearch index unique per (run, flow).

---

## Execution Flows

### Flow A — `install` (fresh deploy, no upgrade)

Used for PR-gating on most scenarios. Single `helm install`, tests run, namespace cleaned up.

```
ci-test-config.yaml
  flow: install
        │
        ▼
matrix/runner.go – runEntry()
        │
        ▼
prepareFlags
  generate random ORCHESTRATION_INDEX_PREFIX
        │
        ▼
helm dependency update
        │
        ▼
deploy.Execute()
  helm upgrade --install integration
  values stack assembled, index = PREFIX-install
        │
        ├─── --test-it ──► Go integration tests
        │                       │
        └─── --test-e2e ─►      ▼
                         Playwright E2E
                         (c8-cross-component-e2e-tests)
                                │
                                ▼
                         cleanup namespace
```

---

### Flow B — `upgrade-minor` (two-step, **single** CI job)

Used for PR-gating on upgrade scenarios. Both steps run in the same job, so prefix state lives in memory.

```
ci-test-config.yaml
  flow: upgrade-minor
        │
        ▼
executeTwoStepUpgrade()
        │
        ▼
ResolveUpgradeFromVersion()
  e.g. resolve "from 8.8"
        │
        ▼
PinScenarioPrefixes()
  generate ONE random prefix, write into flags.Index.OrchestrationIndexPrefix
  (held in memory for both steps)
        │
        ├── Step 1 ─────────────────────────────────────────────────────────────
        │   helm upgrade --install integration
        │   chart: Helm repo (old version 8.8), FLOW=install
        │   OpenSearch index created: PREFIX-install   ◄── Zeebe writes here
        │
        └── Step 2 ─────────────────────────────────────────────────────────────
            helm upgrade --install integration
            chart: local disk (new version 8.9), FLOW=upgrade-minor
            --set orchestration.index.prefix=PREFIX-install  ◄── pinned, same index
                    │
                    ▼
             Go IT + Playwright E2E
                    │
                    ▼
             cleanup namespace
```

**Key:** `PinScenarioPrefixes()` generates the prefix once and both `deploy.Execute()` calls inherit it from the same `flags` struct in memory.

---

### Flow C — `modular-upgrade-minor` (two-step, **separate** CI jobs)

Used for nightly upgrade workflows. Install and upgrade run as separate GitHub Actions jobs on potentially different runners — **no shared in-memory state**.

```
GitHub Actions: playwright_sm_nightly_upgrade_minor_opensearch_8_9.yml
┌─────────────────────────────────────────────────────────────────────┐
│                                                                     │
│  Job 1: install (chart 8.8, shortname=qaosupg2)                    │
│  ─────────────────────────────────────────────                      │
│  runEntry() / flow=install                                          │
│    generate INDEX_PREFIX=abc123                                     │
│    helm upgrade --install integration                               │
│    OpenSearch index created: abc123-install   ◄── Zeebe writes here│
│    namespace stays alive on GKE cluster                             │
│                                                                     │
│  Job 2: upgrade (chart 8.9, shortname=qaosupg)                     │
│  ────────────────────────────────────────────                       │
│  executeUpgradeOnly() / flow=modular-upgrade-minor                  │
│    run pre-upgrade-minor.sh (if exists)                             │
│    ┌─────────────────────────────────────────────────┐              │
│    │ readIndexPrefixesFromHelm()                      │              │
│    │   helm get values integration -n NS -o yaml     │              │
│    │   --> orchestration.index.prefix = abc123-install│              │
│    │   parseHelmValuesForIndexPrefixes()             │              │
│    │   --> inject into ExtraHelmSets as --set        │              │
│    └─────────────────────────────────────────────────┘              │
│    helm upgrade --install integration                               │
│    --set orchestration.index.prefix=abc123-install                  │
│    values file template would say "abc123-modular-upgrade-minor"    │
│    but --set wins --> same index reused, auth records found         │
│                                                                     │
│  Job 3: e2e-tests                                                   │
│  ────────────────                                                   │
│  Playwright hits Operate/Tasklist via ingress                       │
│  users can access components (auth records in index)                │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

**Why `PinScenarioPrefixes()` cannot be used here:** it stores prefix state in Go memory. A separate CI job starts from scratch and cannot access what the install job generated. The only durable truth is the deployed Helm release itself.

---

## The Index Prefix Problem (Root Cause of `actions/runs/25536942222`)

Without the fix, the two jobs resolved to completely different OpenSearch indices:

```
Install job  (FLOW=install)               --> index: orch-qa-opensearch-abc123-install
                                               Zeebe exported auth records here
Upgrade job  (FLOW=modular-upgrade-minor) --> index: orch-qa-opensearch-xyz789-modular-upgrade-minor
                                               different random prefix + different flow suffix
                                               this index was EMPTY
```

Zeebe's RocksDB PVC persists across the helm upgrade. On restart, `IdentitySetupInitializer.onRecovered()` fires but all sub-commands are rejected with `ALREADY_EXISTS` (state already in RocksDB). No new records are exported to OpenSearch.

Authorization failure chain:

```
getAuthorizedComponents()
  └── resolveResourceAccess()
        └── OpenSearch query on empty index
              └── authorizedComponents = []
                    └── isForbidden() = true
                          └── "You don't have access to this component"
```

After the fix, `executeUpgradeOnly()` reads `abc123-install` from the live Helm release and passes `--set orchestration.index.prefix=abc123-install`, so the 8.9 cluster reads from the same populated index as 8.8.

---

## Data Flow Across Both Repos

```
camunda-platform-helm
┌────────────────────────────────────────────────────────┐
│                                                        │
│  ci-test-config.yaml ──────────────────┐              │
│  (scenario definitions)                │              │
│                                        ▼              │
│  values/persistence/opensearch.yaml    deploy-camunda  │
│  values/identity/keycloak.yaml     --> CLI             │
│  (template: PREFIX-FLOW)               │              │
│                                        │              │
│  parseHelmValuesForIndexPrefixes() <───┤              │
│  reads PREFIX from live Helm release   │              │
│  injects as --set override             │              │
│                                        │              │
└────────────────────────────────────────┼──────────────┘
                                         │ helm upgrade --install
                                         ▼
                              GKE Cluster / Namespace
                         ┌────────────────────────────┐
                         │  Helm Release: integration  │
                         │  OpenSearch index (PREFIX)  │
                         │  Zeebe RocksDB PVC          │
                         └────────────┬───────────────┘
                                      │ ingress URL
                                      ▼
                         c8-cross-component-e2e-tests
                    ┌──────────────────────────────────┐
                    │  Playwright test runner           │
                    │  pages/SM-8.9/OperateHomePage.ts │
                    │  assert: users can access Operate │
                    └──────────────────────────────────┘
```

---

## Key Files Reference

### `camunda-platform-helm`

| File | Purpose |
|---|---|
| `charts/camunda-platform-8.9/test/ci-test-config.yaml` | Scenario matrix: identity x persistence x flow x features |
| `charts/.../values/persistence/opensearch.yaml` | OpenSearch backend values; uses `$ORCHESTRATION_INDEX_PREFIX-$FLOW` template |
| `scripts/deploy-camunda/matrix/runner.go` | `executeTwoStepUpgrade`, `executeUpgradeOnly`, `readIndexPrefixesFromHelm`, `parseHelmValuesForIndexPrefixes` |
| `scripts/deploy-camunda/matrix/helm_sets_test.go` | Unit tests for the parsing and merge helpers |
| `scripts/deploy-camunda/deploy/` | Core `deploy.Execute()` and `PinScenarioPrefixes()` |
| `scripts/camunda-core/pkg/executil/exec.go` | `RunCommandCapture()` used by `readIndexPrefixesFromHelm` |
| `scripts/camunda-core/pkg/versionmatrix/` | `ResolveUpgradeFromVersion`, `HasPreInstallScript`, `HasPreUpgradeScript` |

### `c8-cross-component-e2e-tests`

| File | Purpose |
|---|---|
| `.github/workflows/playwright_sm_nightly_upgrade_minor_opensearch_8_9.yml` | Nightly GHA workflow: install job -> upgrade job -> e2e job |
| `pages/SM-8.9/OperateHomePage.ts` | Page object model for Operate UI |
| `tests/SM-8.9/` | Test specs covering the 8.8->8.9 upgrade scenario |

---

## CI Trigger Summary

| Workflow type | Trigger | Flow used | Runner function |
|---|---|---|---|
| PR check | Pull request | `install` | `runEntry` |
| PR check (upgrade) | Pull request | `upgrade-minor` | `executeTwoStepUpgrade` |
| Nightly | Scheduled cron | `install` + `modular-upgrade-minor` (separate jobs) | `runEntry` + `executeUpgradeOnly` |
| Manual | `workflow_dispatch` | any | depends on shortname filter |
