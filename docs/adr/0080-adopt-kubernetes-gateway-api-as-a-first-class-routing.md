# Adopt Kubernetes Gateway API as a first-class routing alternative alongside Ingress in the Helm chart

- Status: accepted
- Date: 2026-02-17
- Decision-makers: Jesse Simpson

## Context and Problem Statement

The Camunda 8 Helm chart historically relied exclusively on Kubernetes Ingress resources for external traffic routing. As the Kubernetes ecosystem converges on the Gateway API specification — which offers richer routing semantics, native gRPC support, and cross-namespace reference grants — users adopting Gateway API were forced to maintain custom post-render patches or Kustomize overlays outside the chart. This created reproducibility and maintainability problems for GitOps workflows and shifted unnecessary complexity to operators.

## Decision Drivers

- **Ecosystem alignment** — Gateway API is the designated successor to Ingress; supporting it natively positions the chart for long-term Kubernetes compatibility without future breaking migrations.
- **Routing expressiveness** — gRPC-native routes (required by Zeebe/Orchestration) and cross-namespace ReferenceGrants cannot be cleanly expressed through Ingress annotations alone.
- **Operator autonomy** — Users should be able to adopt Gateway API through standard `values.yaml` configuration rather than maintaining external overlays that break upgrade paths.
- **Backward compatibility** — Existing Ingress-based deployments must continue working without modification; the transition must be opt-in.

## Considered Options

- **Ingress-only with provider-specific annotations** — Rejected because it cannot natively express gRPC routing or cross-namespace grants; forces provider lock-in through annotation semantics that vary across controllers.
- **External CRD overlays managed by operators** — Rejected because it shifts complexity to users, breaks GitOps reproducibility, and creates a maintenance burden that scales with the number of components.
- **Dual-stack in-chart with values toggles (chosen)** — Supports both Ingress and Gateway API as parallel networking stacks, selectable via configuration, maximizing compatibility across cluster configurations.

## Decision Outcome

The chart now includes first-class Gateway API resource templates (HTTPRoute, GRPCRoute, Gateway, ReferenceGrant) gated behind dedicated `values.yaml` keys per component. Ingress remains the default; Gateway API is opt-in. A new integration test scenario (`gateway-keycloak`) validates the full Gateway API path in CI, ensuring parity with the Ingress path.

### Positive Consequences

- Users can adopt Gateway API through declarative values configuration without external tooling or post-render scripts.
- gRPC routing for Orchestration is expressed natively via GRPCRoute rather than through fragile annotation hacks.
- The chart's networking abstraction is now future-proof against eventual Ingress deprecation in upstream Kubernetes.

### Negative Consequences

- Chart surface area increases significantly (36 files), creating ongoing maintenance burden for two parallel networking stacks that must be kept in sync across chart versions.
- CI cost increases with the new `gateway-keycloak` integration scenario adding cluster time to every PR pipeline run.