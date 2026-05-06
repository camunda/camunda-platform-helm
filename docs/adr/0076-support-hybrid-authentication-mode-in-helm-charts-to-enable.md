# Support hybrid authentication mode in Helm charts to enable mixed Identity/direct-credential deployments

- Status: accepted
- Date: 2025-11-27
- Decision-makers: Balázs

## Context and Problem Statement

The Camunda Self-Managed Helm charts enforced a single authentication mode globally — either full Identity/Keycloak auth for all components, or none. This prevented customers from running hybrid topologies where webapp components authenticate via Keycloak while external Zeebe clients or Connectors use direct M2M/OIDC credentials. The chart needed to support per-component auth mode selection with appropriate validation to prevent invalid combinations.

## Decision Drivers

- **Customer deployment flexibility** — enterprises require mixed auth topologies where internal services use Keycloak SSO while external integrations use direct credentials
- **Safety through validation** — allowing per-component auth without guardrails risks silent misconfigurations; constraint templates must catch invalid combinations at install time
- **Cross-version consistency** — charts 8.8 and 8.9 must both support hybrid auth to avoid fragmenting the supported deployment matrix
- **CI coverage confidence** — a new deployment topology requires dedicated integration test scenarios to prevent regressions in nightly pipelines

## Considered Options

- **Separate chart installations per auth mode** — one namespace for Keycloak-authed components, another for direct-auth components. Rejected: breaks shared-namespace assumptions, doubles operational overhead, and complicates inter-component communication.
- **Per-component auth overrides without validation** — simpler implementation with no constraint logic. Rejected: too easy to deploy invalid combinations silently, leading to runtime failures that are difficult to diagnose.
- **Hybrid auth with constraint validation (chosen)** — components declare their auth mode individually, with template-level constraints that fail fast on invalid combinations.

## Decision Outcome

The charts now support a hybrid authentication model where individual components can be configured for either Identity/Keycloak auth or direct credential auth within the same deployment. Constraint templates validate that the selected combination is coherent (e.g., Identity must still be deployed if any component references it). The values schema was extended to expose hybrid configuration as a first-class API surface, and a dedicated `ingress-hybrid` integration test scenario validates the topology end-to-end.

### Positive Consequences

- **Unblocks hybrid deployment topologies** — customers can now mix external Zeebe clients with Keycloak-protected webapps in a single Helm release
- **Fail-fast validation** — constraint templates catch invalid auth combinations at `helm install` time rather than at runtime
- **CI-exercised path** — the new `ingress-hybrid` scenario ensures regressions are caught in nightly pipelines before reaching customers

### Negative Consequences

- **Increased template complexity** — conditional auth logic in constraints, secrets, and deployment templates makes future template changes more error-prone and harder to review
- **Cross-version maintenance burden** — mirroring changes across 8.8 and 8.9 (41 files) means any future fix in this area requires coordinated backports, increasing the cost of iteration