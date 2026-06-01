# Allow opt-in deployment strategy for components with chart-managed RWO persistence

- Status: accepted
- Date: 2026-06-01
- Decision-makers: Distribution team
- Amends: [ADR 0043](0043-hardcode-deployment-strategy-for-all-components-and-remove.md)

## Context and Problem Statement

[ADR 0043](0043-hardcode-deployment-strategy-for-all-components-and-remove.md) hardcoded the
deployment strategy for all components (Identity, Optimize, Tasklist, Zeebe Gateway, Connectors,
Console, Web Modeler) and removed the `strategy` field from `values.yaml`, on the grounds that the
correct strategy is *architecturally determined* by each component's statefulness and that "no
legitimate use case existed for overriding."

That premise does not hold for components that mount a **chart-managed `PersistentVolumeClaim`
defaulting to `ReadWriteOnce` (RWO)**. When `<component>.persistence.enabled=true` with an RWO access
mode, a `RollingUpdate` rollout creates the replacement pod before terminating the old one. With
single-node RWO storage the new pod cannot attach the volume (`Multi-Attach` error), so the rollout
deadlocks during `helm upgrade`. This was reported in SUPPORT-30069 / issue #4767 for Web Modeler
restapi.

For these components the correct strategy is **infrastructure-determined** — it depends on whether the
backing storage class supports `ReadWriteOnce` or `ReadWriteMany` (RWX), not on the component's
architecture. Under [ADR 0091](0091-adopt-component-extraconfiguration-as-the-standard-application-configuration-mechanism.md)'s
values classification this is a legitimate Tier 2 infrastructure value, which ADR 0043's blanket
removal did not anticipate.

Four of the seven components ADR 0043 covered mount a chart-managed RWO PVC (PVC names below are for
charts 8.9/8.10; the 8.8 layout differs — see "Applicability by version"):

| Component | Chart-managed PVC (8.9/8.10) | Hardcoded strategy (pre-amendment) |
|---|---|---|
| Web Modeler restapi | `<release>-webmodeler-data` | `RollingUpdate` |
| Connectors | `<release>-connectors-data` | `RollingUpdate` |
| Identity | `<release>-identity-data` | `RollingUpdate` |
| Optimize | `<release>-optimize-data` | `Recreate` |

The remaining three (Tasklist, Console, Zeebe Gateway / orchestration) mount no chart-managed PVC and
are unaffected.

### Applicability by version

The `Multi-Attach` premise requires a chart-managed PVC on a `Deployment`, which only exists in charts
**8.8 and later**:

- **8.8, 8.9, 8.10** — all four components mount a chart-managed RWO PVC; this amendment applies. Note
  layout differences that prevent a copy-paste across versions:
  - **Optimize** uses a single PVC (`<release>-optimize-data`) in 8.9/8.10 but **two** PVCs
    (`<release>-optimize-data-tmp` and `<release>-optimize-data-camunda`) in 8.8.
  - **Web Modeler** restapi mounts `<release>-webmodeler-data` in all three; 8.10 additionally resolves
    its values through the `camundaHub.*` override layer (see constraint 1).
- **8.7 and earlier** — these components do **not** mount a chart-managed PVC (only the Zeebe
  `StatefulSet` uses `volumeClaimTemplates`, which is not subject to `Deployment`-style `Multi-Attach`
  rollout deadlock). The premise does not exist, so this amendment does **not** apply; ADR 0043 remains
  fully in force there.

## Decision Drivers

- **Operational correctness:** RWO-persistence users must be able to complete `helm upgrade` without a
  `Multi-Attach` deadlock; RWX users must retain zero-downtime rollouts.
- **Preserve ADR 0043's safety intent:** the original concern — a stray `Recreate` on a stateless,
  load-balancer-fronted service causing surprise downtime — must remain structurally impossible.
- **Minimal, bounded API surface:** reintroduce the knob only where it is genuinely meaningful, not
  globally.
- **Consistency:** all four PVC-bearing components should converge on one pattern rather than each
  inventing its own.

## Considered Options

- **Leave ADR 0043 fully in force; hardcode `Recreate` for affected components** — Rejected. This is how
  Optimize was handled, and it penalizes RWX users with unnecessary downtime on every rollout.
- **Re-expose a top-level `<component>.deploymentStrategy`** — Rejected. It re-opens the exact
  misconfiguration risk ADR 0043 eliminated (strategy decoupled from any volume), and would apply to
  stateless components too.
- **Re-expose strategy only under the `persistence` block, gated and enum-validated (chosen)** — The
  knob exists only where a single-attach volume makes it meaningful, with a safe default and validation.

## Decision Outcome

Deployment strategy MAY be re-exposed as an opt-in value, **scoped to the following components only**:
Web Modeler restapi, Connectors, Identity, and Optimize. The following constraints are normative:

1. **Location.** The value lives under the component's persistence block as
   `<component>.persistence.deploymentStrategy` (e.g. `webModeler.persistence.deploymentStrategy`). It
   MUST NOT be exposed as a top-level component value, so it cannot exist independently of a PVC. In
   8.10, where components are configurable through the `camundaHub.*` override layer, the value resolves
   as `(or .Values.camundaHub.<component>.persistence.deploymentStrategy
   .Values.<component>.persistence.deploymentStrategy)`, matching every other 8.10 override, with the
   same default and guard.
2. **Allowed values.** `RollingUpdate` and `Recreate` only, enforced by a `values.schema.json` enum
   **and** a Helm template guard that fails with a clear message if schema validation is bypassed.
3. **Default.** `RollingUpdate` for components whose pre-amendment hardcoded strategy was
   `RollingUpdate` (Web Modeler, Connectors, Identity), preserving the zero-downtime behavior ADR 0043
   established for RWX users. Optimize keeps `Recreate` as its default to avoid changing existing
   behavior; RWX users opt into `RollingUpdate`.
4. **Scope.** Components without a chart-managed PVC (Tasklist, Console, Zeebe Gateway / orchestration)
   remain hardcoded per ADR 0043. This amendment does not reopen strategy configuration for them, and
   adding the knob to a new component requires that component to first gain a chart-managed PVC and a
   documented RWO deadlock case.

ADR 0043's underlying principle — *expose configuration only where users have a legitimate, safe
choice* — is retained and, for these components, satisfied: the knob is meaningful (RWO vs RWX is a real
user-facing storage decision), constrained (two enum values), and structurally coupled to a volume so it
cannot be misapplied to a stateless service.

First applied in PR #6014 (Web Modeler restapi, charts 8.8, 8.9, and 8.10). Connectors, Identity, and
Optimize are tracked as follow-ups (#6268, #6269, #6270) and should adopt this pattern across 8.8–8.10,
accounting for the per-version layout differences noted above.

### Positive Consequences

- RWO-persistence users can opt into `Recreate` and complete upgrades without a `Multi-Attach` deadlock.
- RWX users keep zero-downtime `RollingUpdate` by default (and Optimize RWX users gain the option).
- The misconfiguration class ADR 0043 closed stays closed: no strategy knob exists without a PVC.

### Negative Consequences

- Partially reverses ADR 0043's mechanical outcome for four of its seven components, so the decision
  record now spans two ADRs that must be read together.
- Reintroduces a small, bounded `values.yaml` surface and its associated schema, template-guard, test,
  and golden-file maintenance for each affected component.
- A behavioral split persists until the follow-ups land: only Web Modeler exposes the value initially,
  while Connectors and Identity remain hardcoded `RollingUpdate` and Optimize hardcoded `Recreate`.
