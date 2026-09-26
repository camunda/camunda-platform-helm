# Camunda Hub operator (experimental)

An experimental Kubernetes operator that installs, upgrades and manages the
**Camunda Hub** layer of the Camunda 8.10 platform: a `camunda-platform` release
running with `global.topology.mode: hub` (Management Identity + Camunda Hub).

Status: **experiment**. Not supported, not released, not for production.

## The design in one paragraph

The operator does not template anything itself. It drives the published
`camunda-platform` chart through the Helm v4 Go SDK and writes ordinary Helm
release secrets, so `helm list`, `helm get values`, `helm history` and
`helm rollback` keep working against an operator-managed release. A release
installed with the Helm CLI can be adopted in place, and a customer can stop
using the operator and continue with the CLI at any point. Customers who cannot
use operators lose nothing: the chart is unchanged and remains the single source
of truth.

## Parity is the contract

`test/parity` renders the hub role twice — once through the operator's embedded
SDK, once through the pinned `helm` CLI — and requires the results to match. That
holds only because `helm.sh/helm/v4` in `go.mod` is the same version as the `helm`
entry in the repository's `.tool-versions`; `TestSDKVersionMatchesToolVersions`
fails if the two ever drift apart. Bump them together.

Chart 15.x also requires Helm v4 at render time
(`templates/common/constraints.tpl`), which is why the `operator-sdk` Helm-based
operator is not usable here — it embeds the Helm v3 SDK.

## Layout

| Path | Contents |
|---|---|
| `api/v1alpha1/` | `CamundaHub` types. `zz_generated.deepcopy.go` is generated. |
| `cmd/` | Manager entrypoint. |
| `internal/controller/` | Reconciler, plus `Decide`, the pure action-decision function. |
| `internal/helm/` | Helm v4 SDK driver and chart resolution, behind `ReleaseInfo`. |
| `internal/values/` | Values composition using Helm's own `loader.MergeMaps`. |
| `config/crd/bases/`, `config/rbac/` | Generated CRD and RBAC. |
| `config/samples/` | Example `CamundaHub`. |
| `test/parity/` | SDK-vs-CLI render parity and version-pinning guards. |

## Commands

Run from the repository root:

```bash
make operator.build       # compile
make operator.test        # parity suite (updates chart dependencies first)
make operator.generate    # regenerate deepcopy functions
make operator.manifests   # regenerate the CRD and RBAC
```

`api/v1alpha1/zz_generated.deepcopy.go`, `config/crd/bases/` and `config/rbac/`
are generated — regenerate them, never hand-edit.

## Values are passed through, not mirrored

`spec.values` is raw chart values. They are not re-declared as typed CRD fields,
because that would create a second public API that drifts against the chart's own
values contract on every release and would block customers from using a chart
feature until the CRD caught up. Validation comes from the resolved chart's own
`values.schema.json`. The only typed fields are the ones the controller itself
branches on.

`global.topology.mode` is not settable: a `CamundaHub` *is* the hub role, and the
operator sets it.

## Chart versions

**Chart 15.x (Camunda 8.10) or later only.** This is enforced, not advisory.

Chart 14.x (8.9) has no `global.topology.mode`, and its values schema accepts
unknown keys under `global`, so the hub role is *silently ignored* rather than
rejected. Rendering 8.9 with it produces connectors, identity **and a Zeebe
StatefulSet** — a whole Orchestration Cluster. A `CamundaHub` that quietly
deployed Zeebe is not an acceptable outcome, so the operator detects the missing
capability and refuses with `ChartLacksHubRole` before writing anything.

8.9 is also structurally incompatible in two other ways: there is no `camundaHub`
component (Console and Web Modeler are still separate), and the phase contract the
migration sequence needs does not exist. Managing 8.9 would need a different
resource kind, not a flag on this one.

## Adoption: taking over a Helm-installed release

Set `spec.adoption.adoptExisting: true` on a `CamundaHub` whose release name and
namespace match a release you installed with the Helm CLI. Adoption **writes
nothing**: it records the release that is already there and marks it owned. Any
convergence happens on the following reconcile as an ordinary `helm upgrade`, so
adopting a release that already matches the spec performs no Helm operation and
restarts no pods. `TestAdoptionPerformsNoHelmWrite` asserts exactly that.

Without that flag, an existing release the operator did not create blocks the
reconcile with `AdoptionRequired` rather than being seized silently. A release
owned by a *different* `CamundaHub` is never touched at all.

## The Hub 8.9 to 8.10 database migration

The migration is not backward compatible, so chart PR
[#6788](https://github.com/camunda/camunda-platform-helm/pull/6788) exposes
`camundaHub.upgrade.phase` and the operator sequences it:

```
normal --> quiesce --> [backup gate] --> migrate --> normal
```

- **quiesce** scales both Hub Deployments to zero, so nothing can write to the
  database.
- The sequence then **stops** until `spec.upgrade.backupVerified` is set to true.
  The migration is irreversible; the operator will not start it on an unverified
  database.
- **migrate** runs a single REST API pod. The Hub Services pin their selector to
  `camunda.io/upgrade-phase: normal`, so that pod migrates without ever receiving
  traffic.
- **normal** restores replicas and traffic.

Each phase is its own Helm revision, and `status.phase` is written before each
transition, so a manager that dies mid-sequence resumes at the next step rather
than restarting the migration.

**Rollback is disabled from `migrate` onward** and this is not configurable.
Once the schema may have moved, rolling back would leave older code against a
migrated database, which is worse than stopping and asking for a human.

Enable with `spec.upgrade.phased: true`. Against a chart that does not declare
`camundaHub.upgrade.phase` the operator detects the absence, warns, and performs
an ordinary single-step upgrade — so this works on chart lines that predate the
contract.

## Safety properties worth knowing

- **One release has one owner.** Two `CamundaHub` objects naming the same release
  are a conflict: the older one keeps it and the newer is blocked, so the outcome
  does not depend on reconcile order. Helm operations are additionally serialised
  per release in-process, and leader election covers the cross-process case —
  concurrent Helm operations otherwise fail with "another operation is in
  progress" and can leave a release pending that only a human can clear.
- `spec.paused` stops every write, so you can take a release over with the Helm CLI
  without the controller fighting you.
- `spec.deletionPolicy` defaults to `Retain`: deleting the `CamundaHub` leaves the
  Helm release running. `Delete` uninstalls but keeps history, PVCs and Secrets.
- Neither policy removes Identity or Keycloak clients. ADR-0095 leaves that to a
  human, so the operator emits a `CleanupRequired` event instead.
- A release in a `pending-*` state is reported, never clobbered.

## Testing

Three layers, and the distinction matters:

| Suite | Proves | Needs |
|---|---|---|
| `internal/...` | The operator *decides* correctly, including the phase machine. Uses a fake Helm driver. | nothing |
| `test/parity` | SDK renders identically to the pinned Helm CLI. | the 8.10 chart |
| `test/kind` | Adoption keeps the same pods; the migration really runs; the real 8.10 chart installs; the CRD schema is enforced. | a Kubernetes cluster |

```bash
make operator.test                       # unit + parity, cluster-free

kind create cluster
make operator.oci-registry-up            # optional; without it the OCI test skips
make operator.test-kind
make operator.oci-registry-down
```

The kind suite is build-tagged (`//go:build kind`) so `go test ./...` never needs
a cluster. Most of it uses a fixture chart, so it measures the operator rather
than the chart and needs no Camunda images; `TestInstallsRealCamundaChart` points
the operator at `charts/camunda-platform-8.10` itself and asserts the three
hub-role workloads appear under the names the chart gives them (Web Modeler keeps
its 8.9 resource names, which is what makes the 8.9-to-8.10 transition a rolling
update). It sets `atomic: false` so Helm does not wait for readiness, since the
alpha images and their database and identity provider are not available in kind.

### Things only a real cluster caught

Four bugs survived the fake-driver suites and were found the first time this ran
against Kubernetes. They are recorded here because each is a Helm v4 behaviour
that is easy to reintroduce:

1. **Ownership label.** `namespace/name` is not a legal label value, so every
   install failed at the release-secret write. Ownership is now the object UID.
2. **`WaitStrategy` is mandatory** in Helm v4; unset fails with "wait strategy not
   set".
3. **A dry run permanently swaps `cfg.Releases` for an in-memory store**
   (`action/install.go`). Sharing one `action.Configuration` between drift
   rendering and installing meant every write silently went to memory and never
   reached the cluster — while reporting success. `Template` now uses its own
   configuration.
4. **Resource namespace is not `Install.Namespace`.** That only sets where the
   release record is stored; namespace-less manifests land wherever the kube
   client resolves, which was `default`. A getter wrapper now pins it.

A fifth was a design flaw rather than a bug: the ownership label is only written
by an install or upgrade, and adoption performs neither, so the operator re-adopted
forever. Status now records adoption in that window.

## Drift detection: what it does and does not cover

Two things are checked, both chosen because they cannot produce a false positive:
an object from the release that no longer exists, and a rendered manifest whose
digest differs from the one last applied (chart content can move under a mutable
tag without the version or values changing).

Field-level comparison of live objects is deliberately not attempted. Defaulted
fields, admission webhooks and other field managers all rewrite parts of an object
legitimately, so a naive diff reports drift that is not drift. **Detecting a
hand-edit to an individual field is therefore a known gap**, not a solved problem.

## Not implemented yet

Validation of `spec.values` against the chart's `values.schema.json`, Identity and
Keycloak orphan cleanup (still report-only, per ADR-0095), and the UBI9 image plus
OLM bundle.

Known gaps in coverage, stated rather than implied:

- **Nothing has run on OpenShift.** `restricted-v2` compatibility and arbitrary-UID
  behaviour are unverified; the repo's `rosa` CI path exists but is unused here.
- **Camunda is started but never healthy.** `TestRealChartWorkloadsStart` proves
  the operator's pod specs are kubelet-acceptable — images resolve and pull, and
  every referenced ConfigMap and Secret key exists — but a kind cluster has no
  database or identity provider, so nothing becomes Ready. `camunda/hub` is a
  private alpha image and is excluded. Proving the platform actually works is the
  `SM-8.10` Playwright suite's job in `c8-cross-component-e2e-tests`.

The phase sequence is exercised against a fixture chart that implements the
`camundaHub.upgrade.phase` contract, because chart PR #6788 is not merged yet. It
should be re-verified against the real chart once that lands.
