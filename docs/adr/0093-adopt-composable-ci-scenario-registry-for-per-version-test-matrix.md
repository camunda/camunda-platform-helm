# Adopt a composable CI scenario registry for the per-version integration test matrix

- Status: accepted
- Date: 2026-06-01
- Decision-makers: Distribution team

## Context and Problem Statement

The Camunda Helm chart's CI integration test matrix is defined by one
`ci-test-config.yaml` per chart version at
`charts/camunda-platform-{8.7,8.8,8.9,8.10}/test/ci-test-config.yaml`.
Each file is an ordered list under
`integration.case.pr.scenario`; each list item is a ~30-line inline
`CIScenario` blob mixing identity, persistence, platform/infra,
flow, tier, lifecycle hooks, dependencies, and scenario scalars.
The 8.10 file is ~1200 lines.

The 100-commit sample in
[#6264](https://github.com/camunda/camunda-platform-helm/issues/6264)
shows 11–16 edits per version file. Structurally independent
changes (new scenarios, hook tweaks, infra routing, skip policy)
collide as merge conflicts in the same ordered list — structural
conflicts, not stale-branch artifacts.

Two duplication pressures compound it:

- **Inlined hooks** — identical `pre-install` fixtures and descriptions
  copied across every CloudNativePG-using scenario.
- **Inlined companion dependencies** — same `internal-keycloak-26` and
  `elastic/elasticsearch 8.5.1` repo+values blocks copied across every
  consumer; a single bump touches many scenarios × four files.

### Applicability by version

Applies to all currently supported chart versions — **8.7, 8.8, 8.9,
8.10**.

## Decision Drivers

- **Conflict surface area.** Independent scenario or policy changes
  must touch independent files. Primary motivation from #6264.
- **Reviewability.** A scenario diff must show all relevant policy
  without scrolling a 1200-line file; a hook or dependency change must
  show its full blast radius via ID references.
- **Migration safety.** `deploy-camunda matrix list/run` MUST
  produce a matrix equivalent to the pre-migration one at the
  migration cut, enforced by the equivalence test (item 4).
- **Single source of truth.** Post-migration, only the registry is
  edited; the legacy file is a frozen snapshot with a header comment.
- **Internal scope.** Registry is a CI-internal mechanism. MUST NOT
  alter user-facing chart values, templates, golden files, or
  `values.schema.json`. MUST NOT change the `CITestConfig` struct
  consumed by `deploy-camunda matrix run`.

## Considered Options

- **Keep the monolithic file; rely on smaller PRs and rebase
  discipline.** Rejected — conflict pattern is structural; process
  cannot remove file-level adjacency.
- **Full composable registry; identity, persistence, flows,
  platforms, infra, features, and e2e each extracted into their own
  subdirectory alongside scenarios, hooks, and dependencies.**
  Rejected — scenario-specific, not cited in #6264 as duplication
  sources; extracting them imposes multi-file reads without removing
  conflict surface.
- **Cross-version composable registry; one repo-global tree shared
  by 8.7/8.8/8.9/8.10 with per-version field overrides.** Rejected —
  version divergence collapses shared defaults into per-version
  overrides, and the override layer becomes the new
  single-point-of-conflict.
- **Scenarios into their own files, plus hooks and dependencies
  extracted; everything else inline (chosen).** Targets exactly the
  duplication sources #6264 cites — the ordered scenario list and
  the inlined hooks/dependencies. Layout in item 1.

## Decision Outcome

A composable CI scenario registry at
`charts/camunda-platform-<X.Y>/test/ci/registry/` becomes the source
of truth for that version's integration test matrix. `deploy-camunda`
reads the registry directly once migrated; the legacy
`ci-test-config.yaml` is frozen as the pre-migration snapshot for
item 4 and is deleted once all four versions have migrated and held
green for one nightly cycle on `main`. Normative constraints:

1. **Layout.** Thin manifest plus `scenarios/`, `hooks/`,
   `dependencies/` subdirectories; files keyed by stable ID:

   ```text
   charts/camunda-platform-<X.Y>/test/ci/registry/
     manifest.yaml             # ordered list of scenario IDs with numeric tier and enabled
     scenarios/<id>.yaml       # one scenario per file; inlines all `CIScenario` fields (identity, persistence, platforms, infra-type, features, shortname, prefix-key, helmVersion, skip-e2e, skip-it, qa, image-tags, upgrade, enterprise) except `flow`, `pre-install`, `post-deploy`, and `dependencies`
     hooks/<id>.yaml           # named pre-install / post-deploy LifecycleHook blocks
     dependencies/<id>.yaml    # named ChartDependency entries (chart, version, repo, values-file)
   ```

   Registry lives under the version's chart subtree, matching the
   ADR 0063 precedent.

   Scenario files carry a plural `flows: [<flow-id>, …]`. The loader
   fans out one scenario × N flows into N `CIScenario` entries with
   distinct singular `Flow`, preserving the existing `CITestConfig`
   struct unchanged. Inline companion-chart blocks MUST NOT be
   permitted; identity sections reference `dependencies/<id>` by ID.

   Values-layer files (under `chart-full-setup/values/`), hook
   manifests (`common/resources/`), and hook scripts
   (`pre-setup-scripts/`) are named by basename and resolved against
   the same version's `test/integration/scenarios/` tree.

2. **Loader.** `scripts/deploy-camunda/matrix/config.go` gains
   `LoadRegistry(chartDir string) (*CITestConfig, error)`, which
   resolves the manifest's ID references against the colocated
   registry and returns the existing `CITestConfig` struct. Legacy
   `LoadCITestConfig` is retained until all four supported versions
   have migrated, then removed.

3. **Validation.** A new `RegistryValidator` enforces at load time:
   - Every referenced hook and dependency ID resolves to a file.
   - Scenario IDs and shortnames are unique.
   - Platform × flow tuples do not duplicate.
   - Referenced basenames (hook manifests, hook scripts, dependency
     values-files, scenario values-layer files, feature values-files)
     resolve under `test/integration/scenarios/`.
   - Platform × flow combinations are not denied by
     `.github/config/permitted-flows.yaml`.

4. **Equivalence test (one-shot, in the migration PR).** A Go test
   in `scripts/deploy-camunda/matrix/` loads both the legacy
   `ci-test-config.yaml` and the registry and asserts the two
   `*CITestConfig` values equal via `cmp.Diff`, with slice-order
   normalization for order-independent fields and the post-fan-out
   scenario list. Proves the migration cut; not a long-running
   invariant. Post-migration regressions caught by the nightly
   integration suite.

5. **Migration order and source switch.** 8.10 migrates first
   (highest churn); 8.9, 8.8, 8.7 follow after 8.10 holds green on
   `main` for one full nightly run. Each migration is a single PR:
   registry files added, equivalence test (item 4) green, legacy
   file frozen with header comment. At runtime the loader picks the
   registry when `<chartDir>/test/ci/registry/manifest.yaml` exists,
   else falls back to `LoadCITestConfig`. No runtime flag.

Follow-ups (not in the first-landing 8.10 PR): 8.9 / 8.8 / 8.7
migrations; removal of `LoadCITestConfig` and the legacy files;
`runner.go` split into concern-scoped packages; migration of
scenario-specific behavior in `.github/workflows/test-integration-runner.yaml`
into registry data.

### Positive Consequences

- Adding a scenario creates one `scenarios/<id>.yaml` and one
  manifest line. Two unrelated scenario PRs collide only if they
  touch the same scenario or the same shared file.
- Editing a hook or dependency touches one file; blast radius
  visible via ID references back to consuming scenarios.
- Scenario review shows a focused `scenarios/<id>.yaml` instead of
  a blob in a 1200-line file.
- `deploy-camunda matrix list/run` behavior is preserved; GHA
  workflows and external callers continue to work unchanged.

### Negative Consequences

- Frozen legacy file mitigated only by header comment, not a CI
  guard; regressions surface in the nightly integration run, not at
  PR time.
- Reading a scenario's effective configuration means reading its
  file plus referenced hook and dependency files; two-hop
  composition replaces long inline blobs as the cognitive cost.
- New loader, validator, and equivalence test add maintenance surface
  in `scripts/deploy-camunda/matrix/`.

## Links

- Issue [#6264 — Reduce CI scenario merge conflicts with a composable scenario registry](https://github.com/camunda/camunda-platform-helm/issues/6264) — source problem statement and 100-commit evidence.
- [ADR 0071](0071-implement-matrix-driven-ci-framework-for-testing-supported.md) — established the matrix-driven CI framework this registry restructures.
- [ADR 0075](0075-rewrite-ci-shell-scripts-as-structured-go-clis-with-shared.md) — established the Go-CLI pattern (`deploy-camunda`) the registry loader extends.
- [ADR 0063](0063-internalize-integration-test-values-within-the-helm-chart.md) — established the precedent for keeping CI test artifacts inside the chart repo; this ADR extends that direction to the matrix definition itself.
