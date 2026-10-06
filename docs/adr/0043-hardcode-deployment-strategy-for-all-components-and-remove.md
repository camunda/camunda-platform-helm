# Hardcode deployment strategy, with an opt-in for components with chart-managed persistence

- Status: accepted
- Date: 2024-07-30
- Decision-makers: Hamza Masood, Distribution team

## Context and Problem Statement

The Camunda Platform Helm charts exposed deployment strategy (e.g., `RollingUpdate`, `Recreate`) as a user-configurable value for all stateless components across chart versions 8.2–8.4. This configuration surface provided no practical benefit — the correct strategy is architecturally determined by each component's statefulness — while creating risk of user misconfiguration leading to downtime or split-brain scenarios during rollouts.

That premise does not hold for components with **chart-managed persistence**. When a component mounts a chart-managed `PersistentVolumeClaim` with the `ReadWriteOnce` (RWO) access mode, a `RollingUpdate` rollout creates the replacement pod before it terminates the old one. With single-node RWO storage the new pod cannot attach the volume (`Multi-Attach` error), so the rollout deadlocks during `helm upgrade` (SUPPORT-30069, Web Modeler restapi). For these components the correct strategy depends on whether the storage class supports RWO or `ReadWriteMany` (RWX), not on the component's architecture. Under [ADR 0091](0091-adopt-component-extraconfiguration-as-the-standard-application-configuration-mechanism.md)'s values classification this is a Tier 2 infrastructure value.

Four components have chart-managed persistence: either a shared RWO PVC or a per-pod generic ephemeral volume. A per-pod generic ephemeral volume gives each pod its own PVC, so a surge pod never contends with the outgoing pod and `RollingUpdate` is safe on that path.

| Component | Chart-managed volume (8.8) | Chart-managed volume (8.9) | Chart-managed volume (8.10) |
|---|---|---|---|
| Web Modeler restapi | Per-pod generic ephemeral volume | Per-pod generic ephemeral volume | Per-pod generic ephemeral volume |
| Connectors | `<release>-connectors-data` | `<release>-connectors-data` | Per-pod generic ephemeral volume |
| Identity | `<release>-identity-data` | `<release>-identity-data` | Per-pod generic ephemeral volume |
| Optimize | `<release>-optimize-data-tmp` and `<release>-optimize-data-camunda` | `<release>-optimize-data` | `<release>-optimize-data` |

Tasklist, Console, and Zeebe Gateway / orchestration mount no chart-managed PVC.

### Applicability by version

- **8.8, 8.9, 8.10** — the strategy is hardcoded, except for the opt-in value of the four components above.
- **8.7 and earlier** — these components mount no chart-managed PVC on a `Deployment`. Only the Zeebe `StatefulSet` uses `volumeClaimTemplates`, which has no `Deployment`-style `Multi-Attach` rollout deadlock. The strategy is hardcoded for all components.

## Decision Drivers

- **Operational safety:** Incorrect strategy choices (e.g., `Recreate` on a stateless service behind a load balancer) could cause unnecessary downtime that users wouldn't anticipate.
- **Operational correctness:** RWO-persistence users must be able to complete `helm upgrade` without a `Multi-Attach` deadlock; RWX users must retain zero-downtime rollouts.
- **Minimal API surface:** Reducing the values.yaml API surface decreases documentation burden, test matrix size, and support overhead. A strategy value exists only where it is genuinely meaningful.
- **Consistency:** All stateless components behave uniformly, and all components with chart-managed persistence converge on one pattern.
- **Opinionated-defaults philosophy:** The chart should encode operational best practices rather than defer every decision to the user, and expose configuration only where users have a legitimate, safe choice.
- **Backward compatibility of released charts:** A render failure for values that a released chart accepts is a breaking change (`docs/policies/breaking-changes.md`, Breaking Change Policy). The Deprecation Policy requires a deprecation in one minor release before a removal in the next major release. The 8.10 chart (15.0.0) is the next major release after the 8.9 chart (14.x).

## Considered Options

- **Keep strategy configurable but improve defaults** — Rejected because no legitimate use case existed for overriding on stateless components; keeping the option implied it was safe to change.
- **Hardcode only for specific components** — Rejected in favor of uniform treatment across all stateless components to avoid inconsistency and confusion about which components are tunable.
- **Hardcode `Recreate` for components with chart-managed persistence** — Rejected because it penalizes RWX users with unnecessary downtime on every rollout.
- **Re-expose a top-level `<component>.deploymentStrategy`** — Rejected because it decouples the strategy from any volume, which reopens the misconfiguration risk, and would apply to stateless components too.
- **Fail the render in 8.8 and 8.9 for an unsafe `Recreate`** — Rejected because 8.8 and 8.9 are released charts, and breaking changes are prohibited in released charts.
- **Hardcode the strategy, and expose it only under the `persistence` block of components with chart-managed persistence, gated and enum-validated (chosen).**

## Decision Outcome

The deployment strategy is hardcoded in the Helm templates for all components, and `values.yaml` has no top-level `strategy` field. Components with chart-managed persistence — Web Modeler restapi, Connectors, Identity, and Optimize — MAY expose an opt-in strategy value. The following constraints are normative:

1. **Location.** The value lives under the component's persistence block as `<component>.persistence.deploymentStrategy`. It MUST NOT be exposed as a top-level component value, so it cannot exist independently of a volume. In 8.10, Web Modeler resolves `camundaHub.persistence.deploymentStrategy` first, then `webModeler.persistence.deploymentStrategy`. The path `camundaHub.webModeler.persistence.deploymentStrategy` is retired and fails the render with a `camundaPlatform.keyRenamed` error.
2. **Allowed values.** `RollingUpdate` and `Recreate` only, enforced by a `values.schema.json` enum **and** a Helm template guard that fails with a clear message if schema validation is bypassed.
3. **Default.** `RollingUpdate` for Web Modeler, Connectors, and Identity, which keeps zero-downtime rollouts for RWX users. `Recreate` for Optimize, which keeps its existing behavior; RWX users opt into `RollingUpdate`.
4. **Scope.** Components without chart-managed persistence keep a hardcoded strategy. Adding the value to another component requires that component to first gain chart-managed persistence and a documented RWO deadlock case.
5. **`Recreate` on a per-pod ephemeral path.** Where the chart-managed path of a component is a per-pod generic ephemeral volume, `Recreate` requires `existingClaim`, not `persistence.enabled` alone.
   - In chart 8.10, the render MUST fail with a `[camunda][error]` when `Recreate` is set without both `persistence.enabled: true` and `existingClaim`. This applies to `webModeler.persistence.*` and to the `camundaHub.persistence.*` override.
   - In charts 8.8 and 8.9, when `Recreate` is set with `persistence.enabled: true` and no `existingClaim`, the render succeeds and the chart MUST emit a `[camunda][warning]`. When `Recreate` is set without `persistence.enabled: true`, the render MUST fail.
   - `RollingUpdate` renders MUST NOT change in any version.

Web Modeler restapi exposes the value in charts 8.8, 8.9, and 8.10. Connectors and Identity keep a hardcoded `RollingUpdate`, and Optimize keeps a hardcoded `Recreate`, until each adopts this pattern.

### Positive Consequences

- Eliminates a class of user misconfiguration that could cause production outages during deployments.
- RWO-persistence users can opt into `Recreate` and complete upgrades without a `Multi-Attach` deadlock, while RWX users keep zero-downtime `RollingUpdate` by default.
- No strategy value exists without chart-managed persistence, so it cannot be misapplied to a stateless service.

### Negative Consequences

- Users with custom deployment strategies on stateless components lose that capability without forking the chart.
- Each component that exposes the value adds schema, template-guard, test, and golden-file maintenance.

## Links

- Builds on [ADR 0091](0091-adopt-component-extraconfiguration-as-the-standard-application-configuration-mechanism.md) — classifies the strategy of a component with chart-managed persistence as a Tier 2 infrastructure value.

## Changelog

- 2026-06-01 — [#6271](https://github.com/camunda/camunda-platform-helm/pull/6271) — Allow an opt-in strategy for components with chart-managed RWO persistence (was ADR 0092).
- 2026-07-21 — [#6539](https://github.com/camunda/camunda-platform-helm/pull/6539) — Resolve the 8.10 Web Modeler value through `camundaHub.persistence.*` and retire `camundaHub.webModeler.persistence.deploymentStrategy`.
- 2026-10-06 — [#7459](https://github.com/camunda/camunda-platform-helm/pull/7459) — Require `existingClaim` for `Recreate` on a per-pod ephemeral path: fail in 8.10, warn in 8.8 and 8.9.
