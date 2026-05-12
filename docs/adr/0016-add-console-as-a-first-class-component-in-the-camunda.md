# Add Console as a first-class component in the Camunda Platform Helm chart

- Status: accepted
- Date: 2023-08-16
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

Camunda was developing a new self-managed Console component to provide a unified management interface for the platform. The Helm chart needed to integrate this component following the same patterns as existing services (Operate, Tasklist, Optimize) while acknowledging that Console was not yet production-ready. The architectural question was whether to introduce Console as a full chart component now — with its own templates, configuration, and ingress — or defer until the component stabilized.

## Decision Drivers

- **Consistency of deployment model**: All Camunda platform components should follow the same Helm template patterns (deployment, service, ingress, configmap, serviceaccount) to reduce cognitive load for operators.
- **Early integration feedback**: Introducing the component early — even as non-production — allows the chart structure to be validated against real usage before GA.
- **Incremental delivery**: The platform chart should evolve incrementally rather than requiring large disruptive additions when components reach production readiness.
- **Configuration surface area**: Console requires its own configmap and integration with the shared ingress and release configmap, making early structural decisions important.

## Considered Options

- **Defer integration until Console is production-ready** — rejected because late integration increases risk of large, disruptive chart changes and misses early feedback from users testing pre-release versions.
- **Add Console as a subchart dependency** — rejected in favor of in-tree templates to maintain consistent patterns with other components (Operate, Tasklist) and simplify cross-component configuration via shared helpers.

## Decision Outcome

Console was added as a full in-tree component with dedicated Helm templates (deployment, service, ingress, configmap, serviceaccount) and integrated into the shared helpers, NOTES.txt, and release configmap. The component follows the exact same structural patterns as existing services, making it immediately familiar to chart maintainers and operators. The commit explicitly marks Console as not production-ready, signaling that the chart structure is stable but the upstream component is not.

### Positive Consequences

- Operators can enable Console with the same `values.yaml` patterns used for all other components, reducing onboarding friction.
- Shared ingress and release configmap integration ensures Console participates in the platform's service discovery model from day one.
- Template structure is locked in early, allowing future Console features to be additive rather than structural.

### Negative Consequences

- Increases the chart's surface area with a component that is explicitly not production-ready, which may confuse users who enable it without reading the caveat.
- Any upstream Console API or configuration changes during its heavy development phase will require corresponding chart maintenance before the component stabilizes.