---
name: ci-scenario-authoring
description: Add or modify CI integration-test scenarios — new persistence layers, scenario entries in ci-test-config.yaml, features, shortnames, and lifecycle hooks (pre-install fixtures/scripts, post-deploy, pre-upgrade) with the TestLifecycleFixtures contract. Use when adding a new test scenario or persistence backend, wiring pre-install prerequisites like TLS secrets or a CloudNativePG cluster, or adding upgrade-flow cleanup hooks.
---

# Adding New Persistence Layers and Scenarios

## New Persistence Layer

A persistence layer is a values file at `charts/<version>/test/integration/scenarios/chart-full-setup/values/persistence/<name>.yaml`. To add one:

1. Create the YAML file with the values needed for the data backend.

No code change required — the name is discovered automatically from the filesystem.

## New CI Test Scenario

Scenarios are defined in `charts/<version>/test/ci-test-config.yaml`. Each entry specifies identity, persistence, platforms, flows, and optional features. Example:

```yaml
- name: elasticsearch-self-signed-upgrade
  enabled: false                    # set true when ready for CI
  identity: keycloak
  persistence: elasticsearch-self-signed
  platforms: [gke]
  flows: [upgrade-minor]
  features: [migrator]              # includes values/features/migrator.yaml
  shortname: esss                   # 4-char, used in namespace generation
```

The `features` array maps to `values/features/<name>.yaml`. The `migrator` feature enables identity and data migration jobs during upgrades — use it for any `upgrade-minor` scenario. Note: the automatic `needsMigrator()` function in `scenarios.go` only activates when `ChartVersion` starts with "13", but the matrix runner does not set `ChartVersion`, so always use `features: [migrator]` explicitly.

## Pre-Install Hooks (Scenario-Specific)

When a scenario needs prerequisites in the namespace before `helm install` (e.g., a CloudNativePG cluster, TLS secrets), declare them on the scenario in `ci-test-config.yaml`:

```yaml
- name: rdbms
  enabled: true
  identity: keycloak
  persistence: rdbms
  pre-install:
    fixtures:
      - postgresql-cluster.yaml
    description: |
      Provisions a CloudNativePG `Cluster` plus auth Secret in the scenario
      namespace; required before helm install.
```

There are two modes — pick exactly one per hook:

- `fixtures: [...]` — names YAML manifests under `charts/<version>/test/integration/scenarios/common/resources/`. The matrix runner applies them via Go server-side apply (`kube.Client.ApplyManifest`, `Force: true`, idempotent), substituting `$NAMESPACE`, `$RELEASE_NAME`, plus the env-var passthrough listed in `lifecycleVarPassthrough` (`RDBMS_POSTGRESQL_USERNAME`, `RDBMS_POSTGRESQL_PASSWORD`, `GITHUB_WORKFLOW_JOB_ID`, `POSTGRESQL_JDBC_URL`). Prefer this mode for trivial kubectl-apply cases.
- `script: <filename>` — names a shell script under `charts/<version>/test/integration/scenarios/pre-setup-scripts/`. The matrix runner runs it via `bash -x` with `TEST_NAMESPACE`, `KUBE_CONTEXT`, and the same env-var passthrough. Use only when the work cannot be expressed as a manifest (cert generation, JKS keystores, conditional kubectl ops). Example: `pre-install-elasticsearch-self-signed.sh` runs openssl + keytool, packages JKS, and creates three Secrets.

`description` is required and must explain the effect — reviewers must understand from a `ci-test-config.yaml` diff alone what a fixture does.

`TestLifecycleFixtures` (matrix package) walks every chart version's config and asserts: every script reference resolves on disk, every fixture reference resolves under `common/resources/`, every description is non-empty, exactly one of fixtures/script is set per hook, and every script in `pre-setup-scripts/` is referenced (orphan check). Files in `preSetupScriptAllowlist` (`pre-install-upgrade.sh` sed-marker, `create-elasticsearch-tls-secrets.sh` helper) are exempt.

## Post-Deploy Hooks (Scenario-Specific)

For resources whose CRDs are only registered by the chart itself (e.g., the Gateway API `ProxySettingsPolicy` on `gateway-keycloak`), declare a `post-deploy:` block alongside `pre-install:`. Same `fixtures` / `script` / `description` shape; runs after `helm upgrade/install` returns successfully and before the deploy result is reported. Example:

```yaml
- name: gateway-keycloak
  post-deploy:
    fixtures: [gateway-proxy-settings.yaml]
    description: |
      Applies the NGINX ProxySettingsPolicy that bumps gateway buffer sizes.
      Runs after helm install because the Gateway API CRD is only registered
      by the chart itself.
```

## Pre-Upgrade Hooks (Flow-Specific)

For cleanup between Step 1 (old version) and Step 2 (new version) of an upgrade flow, declare on the target version's `integration.flows.<flow>.pre-upgrade` block in `ci-test-config.yaml`:

```yaml
integration:
  flows:
    upgrade-patch:
      pre-upgrade:
        script: pre-upgrade-patch.sh
        description: |
          Deletes orchestration StatefulSets and the postgresql-web-modeler
          StatefulSet + PVC before the patch upgrade (PSQL 15→14 rollback).
```

Same `fixtures` / `script` / `description` shape as scenario-level `pre-install:`. The hook runs after Step 1 completes and before Step 2's `helm upgrade`, scoped to the *target* version (the version being upgraded to).

## Verifying a new or changed scenario

Registry snapshot (`charts/<v>/test/ci/registry-snapshot.yaml`) is the compiled view of the composable registry — use it as a diff target when editing scenarios/hooks/dependencies; regenerate with `make go.update-registry-golden` (see `AGENTS.md` → Generated Artifacts). To run the scenario locally before merge, see the `rfr-validation` skill.
