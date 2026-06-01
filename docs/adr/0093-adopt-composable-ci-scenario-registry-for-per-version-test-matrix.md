# Adopt a composable CI scenario registry for the per-version integration test matrix

- Status: accepted
- Date: 2026-06-01
- Decision-makers: Distribution team

## Context and Problem Statement

The Camunda Helm chart's CI integration test matrix is defined by one
`ci-test-config.yaml` per chart version at
`charts/camunda-platform-{8.7,8.8,8.9,8.10}/test/ci-test-config.yaml`.
Each file is an ordered list under `integration.case.pr` and
`integration.case.nightly`. Each list item is an inline `CIScenario`
blob mixing identity, persistence, platform/infra routing, flow
selection, lifecycle hooks, companion-chart dependencies, skip policy,
shortname, prefix key, and Helm version override. The 8.10 file is
~1200 lines, ~30 lines per scenario.

The 100-commit sample in
[#6264](https://github.com/camunda/camunda-platform-helm/issues/6264)
shows 11–16 edits per per-version file. Structurally independent
changes (new scenarios, hook tweaks, infra routing, skip policy)
collide as merge conflicts in the same ordered list — structural
conflicts, not stale-branch artifacts.

Three duplication pressures compound it:

- **Inlined hooks** — identical `pre-install` fixtures and descriptions
  copied across every CloudNativePG-using scenario.
- **Inlined companion dependencies** — same `internal-keycloak-26` and
  `elastic/elasticsearch 8.5.1` repo+values blocks copied across every
  consumer; a single bump touches many scenarios × four files.
- **`pr` vs `nightly`** — scheduling a scenario in both tiers requires
  copying the full blob and toggling `enabled`.

Precedent: `.github/config/permitted-flows.yaml` is loaded separately
by `matrix/config.go` (`LoadPermittedFlows`, `FilterFlows`) and applied
as a cross-cutting filter. This ADR generalizes that pattern.

### Applicability by version

Applies to all currently supported chart versions — **8.7, 8.8, 8.9,
8.10**. Versions 8.3–8.6 remain on the legacy file until removed; they
are not migrated.

## Decision Drivers

- **Conflict surface area.** Independent scenario or policy changes
  must touch independent files. Primary motivation from #6264.
- **Reviewability.** A scenario diff must show all relevant policy
  without scrolling a 1200-line file; a hook or dependency change must
  show its full blast radius via ID references.
- **Migration safety.** `deploy-camunda matrix list/run` MUST
  produce equivalent CI matrices during transition, enforced by the
  equivalence test (item 5).
- **Single source of truth.** While both layouts exist, exactly one is
  hand-edited; the other is generated and CI-guarded against drift.
- **Internal scope.** Registry is a CI-internal mechanism. MUST NOT
  alter user-facing chart values, templates, golden files, or
  `values.schema.json`. MUST NOT change the `CITestConfig` struct
  consumed by `deploy-camunda matrix run`.

## Considered Options

- **Keep the monolithic file; rely on smaller PRs and rebase
  discipline.** Rejected — conflict pattern is structural; process
  cannot remove file-level adjacency.
- **Sort each version's scenario list alphabetically by `name`.**
  Rejected — reduces but does not eliminate adjacency; leaves hooks
  and dependencies duplicated; breaks reviewer ordering.
- **Split only `dependencies` into a shared file.** Rejected — removes
  one duplication but leaves hooks, infra, identity, persistence, and
  platforms coupled in the same ordered list.
- **Composable registry; each concept its own file, referenced by ID
  from a thin per-version manifest (chosen).** Independent concepts →
  independent files. Composition becomes data, validated cross-file
  at load time. Generalizes the `permitted-flows.yaml` precedent.

## Decision Outcome

A composable CI scenario registry rooted at `test/ci/registry/`
becomes the source of truth for the integration test matrix. The
legacy per-version `charts/<version>/test/ci-test-config.yaml` files
remain as **generated compatibility artifacts** during migration and
are removed once all four supported versions have migrated and held
green for one full nightly cycle on `main`. The following constraints
are normative:

1. **Layout.** Single repo-global path with one subdirectory per
   concern; files keyed by stable ID matching the filename stem:

   ```text
   test/ci/registry/
     versions/<X.Y>.yaml       # per-version manifest: scenario refs grouped by schedule (pr | nightly), each with enabled, numeric tier, and optional field overrides
     scenarios/<id>.yaml       # composition: refs to identity, persistence, flow, platforms, infra, e2e, hooks, deps, features; plus scenario scalars (shortname, prefix-key, helm-version-override)
     identities/<id>.yaml      # keycloak | oidc | auth0 | basic | hybrid; references companion charts by `dependencies/<id>` ID, never inlines them
     persistence/<id>.yaml     # elasticsearch | opensearch-embedded | opensearch-self-signed | rdbms | rdbms-self-signed
     flows/<id>.yaml           # install | upgrade-patch | upgrade-minor | modular-upgrade-minor (owns pre-upgrade hook)
     platforms/<id>.yaml       # gke | eks | openshift
     infra/<id>.yaml           # distroci | preemptible | alwaysgreen — selects values-infra-<id>.yaml
     hooks/<id>.yaml           # named pre-install / post-deploy LifecycleHook blocks
     dependencies/<id>.yaml    # named ChartDependency entries (chart, version, repo, values-file)
     features/<id>.yaml        # named feature toggles mapping to values/features/<id>.yaml (e.g. postgresql-companion, hub-pinned-images, migrator)
     e2e/<id>.yaml             # smoke | full | skip-keycloak-dependent
   ```

   Registry lives at repo root, not under `charts/<version>/`, because
   its purpose is cross-version sharing. ADR 0063 internalized test
   artifacts within this repository; it did not constrain them to the
   chart subtree.

   A version manifest references scenario IDs only, grouped under
   `pr:` and `nightly:` schedule lists. A scenario references
   identity, persistence, flow, platforms, infra, e2e, hooks,
   dependencies, and features by ID and owns its own scalars
   (shortname, prefix key, Helm version override). Numeric `tier`
   and `enabled` live on the version-manifest reference, not the
   shared scenario file. Identities reference companion charts via
   `dependencies/<id>` IDs; inline companion blocks in identity or
   version files MUST NOT be permitted.

   Version-scoped paths (values-layer files under `chart-full-setup/values/`,
   hook manifests under `common/resources/`, hook scripts under
   `pre-setup-scripts/`) are named by basename in registry files and
   resolved by the loader against the version's
   `charts/<X.Y>/test/integration/scenarios/` tree.

2. **Loader.** `scripts/deploy-camunda/matrix/config.go` gains
   `LoadRegistry(repoRoot, version string) (*CITestConfig, error)`,
   which resolves a version manifest's ID references against the
   registry and returns the existing `CITestConfig` struct. Legacy
   `LoadCITestConfig(chartDir)` is retained until all four supported
   versions have migrated, then removed.

3. **Validation.** A new `RegistryValidator` enforces at load time:
   - Every referenced ID resolves to a file on disk.
   - IDs and shortnames are unique within scope.
   - Per-version × platform × flow tuples do not duplicate.
   - Version-scoped paths (hooks, dependency values-files, scenario
     values-layer files, `features/<id>` → `values/features/<id>.yaml`)
     resolve under the target version's tree.
   - Version × platform × flow combinations are not denied by
     `.github/config/permitted-flows.yaml`.

   Existing `LifecycleHook.Validate` plain-filename rule is reused.

4. **Per-version overrides.** Per-version scenario drift (e.g. 8.7
   lacks `modular-upgrade-minor`; per-version values-file paths
   differ) is expressed as field-level overrides on the scenario
   reference in `versions/<X.Y>.yaml`. Listed fields override the
   shared `scenarios/<id>.yaml`; absent fields inherit. Forking the
   shared scenario file per version MUST NOT be permitted.

   ```yaml
   # versions/8.7.yaml
   pr:
     - id: orchestration-upgrade
       tier: 1
       enabled: true
       flows: [upgrade-patch, upgrade-minor]   # drops modular-upgrade-minor for 8.7
   nightly: []
   ```

5. **Equivalence test.** A Go test in
   `scripts/deploy-camunda/matrix/` MUST, per active chart version,
   load both the legacy `ci-test-config.yaml` and the registry and
   assert the two `*CITestConfig` values are equal via `cmp.Diff`,
   with explicit slice-order normalization for order-independent
   fields (e.g. `platforms`, `flows`). Gates the per-version rollout
   and remains until that version's legacy file is removed.

6. **Migration order and source switch.** 8.10 migrates first
   (highest churn); 8.9, 8.8, 8.7 follow after 8.10's equivalence
   test holds green on `main` for one full nightly run. Each
   migration is a single PR: registry files added, legacy
   `ci-test-config.yaml` regenerated and committed for review-diff
   visibility, equivalence test enabled. The loader picks the
   registry when `test/ci/registry/versions/<X.Y>.yaml` exists and
   falls back to `LoadCITestConfig` otherwise. No runtime flag.

7. **Generated-file guard.** A new `make ci.registry-check`, invoked
   from `make helm.lint-all`, regenerates each migrated version's
   `ci-test-config.yaml` and fails if the working tree differs. The
   generated file carries a header marker
   (`# GENERATED FROM test/ci/registry — DO NOT EDIT`); the guard
   uses it to detect direct edits and point at the corresponding
   registry file. Unmigrated versions lack the marker and are skipped.

First-landing PR: 8.10 registry migration. Follow-ups: 8.9, 8.8, 8.7
migrations; removal of `LoadCITestConfig` and the legacy files;
refactor of `scripts/deploy-camunda/matrix/runner.go` into
concern-scoped packages; migration of scenario-specific behavior
currently in `.github/workflows/test-integration-runner.yaml` into
registry data.

### Positive Consequences

- Adding a scenario creates one `scenarios/<id>.yaml` and one
  version-manifest line. Two unrelated scenario PRs collide only if
  they touch the same scenario or the same shared policy file.
- Editing a hook, dependency, identity, persistence, or infra mapping
  touches one file; blast radius is visible via ID references.
- Scenario review shows a composed document instead of a 30-line
  inline blob in a 1200-line file.
- `deploy-camunda matrix list/run` behavior is preserved; GHA
  workflows and external callers continue to work unchanged.
- The `permitted-flows.yaml` extraction precedent is generalized
  rather than duplicated.

### Negative Consequences

- During migration each migrated version has two sources of truth
  (registry + generated legacy file). The guard mitigates drift but
  contributors editing the legacy file see CI failures until they
  re-apply against the registry.
- Reading a scenario's effective configuration means reading its
  scenario file plus referenced identity, persistence, flow, hooks,
  and dependency files. Multi-file composition replaces long inline
  blobs as the cognitive cost.
- New loader, validator, and equivalence test add maintenance surface
  in `scripts/deploy-camunda/matrix/`.
- Per-version field-level overrides (item 4) make drift between a
  shared default and a version override invisible without rendering
  the resolved scenario. The equivalence test catches it only during
  the migration window; post-migration, confirm an override's effect
  via `deploy-camunda matrix list`.

## Links

- Issue [#6264 — Reduce CI scenario merge conflicts with a composable scenario registry](https://github.com/camunda/camunda-platform-helm/issues/6264) — source problem statement and 100-commit evidence.
- [ADR 0071](0071-implement-matrix-driven-ci-framework-for-testing-supported.md) — established the matrix-driven CI framework this registry restructures.
- [ADR 0075](0075-rewrite-ci-shell-scripts-as-structured-go-clis-with-shared.md) — established the Go-CLI pattern (`deploy-camunda`) the registry loader extends.
- [ADR 0063](0063-internalize-integration-test-values-within-the-helm-chart.md) — established the precedent for keeping CI test artifacts inside the chart repo; this ADR extends that direction to the matrix definition itself.
