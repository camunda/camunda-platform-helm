# Adopt per-component Kubernetes ServiceAccounts for Camunda platform Helm chart

- Status: accepted
- Date: 2024-05-08
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

All Camunda platform components (Connectors, Identity, Operate, Optimize, Tasklist, Web Modeler, Zeebe, Zeebe Gateway) shared a single global Kubernetes ServiceAccount when deployed via the Helm chart. This violated the principle of least privilege: any RBAC permission granted for one component's needs was implicitly available to every other component in the namespace. For production deployments requiring fine-grained access control, workload identity bindings (GCP Workload Identity, AWS IRSA), and auditability, this shared identity model was a security liability.

## Decision Drivers

- **Principle of least privilege:** Each component has distinct API permission requirements; a shared SA conflates these into a single overprivileged identity.
- **Cloud workload identity compatibility:** Per-component SAs are required to bind individual components to distinct cloud IAM roles via annotations (Workload Identity, IRSA).
- **Auditability and incident response:** Kubernetes audit logs attribute actions to ServiceAccounts; shared SAs make it impossible to distinguish which component performed a given API call.
- **Chart self-containment:** The chart should manage its own SA lifecycle without requiring users to pre-create external resources.

## Considered Options

- **Status quo (shared ServiceAccount):** Minimal configuration but unacceptable security posture for production. Rejected because it prevents per-component RBAC scoping.
- **Namespace-level isolation:** Running each component in its own namespace with default SAs. Rejected due to excessive operational complexity, cross-namespace networking overhead, and incompatibility with the single-namespace deployment model users expect.
- **External-only SA management:** Requiring operators to pre-create and configure SAs outside the chart. Rejected because it breaks the chart's self-contained deployment model and degrades developer experience for standard installations.

## Decision Outcome

Each Camunda component now creates and references its own dedicated ServiceAccount via component-specific helper templates. Deployments and StatefulSets explicitly set `serviceAccountName` to their component's SA, and the values.yaml schema exposes per-component SA configuration (name, annotations, automount). The global SA remains as a fallback default but no longer serves as the runtime identity for any component.

### Positive Consequences

- Enables fine-grained RBAC policies scoped to exactly the permissions each component requires, reducing blast radius of any compromised pod.
- Unlocks per-component cloud IAM binding via SA annotations, a prerequisite for secure secret-less authentication to cloud services.
- Improves audit trail clarity — Kubernetes API server logs now attribute actions to the specific component that performed them.

### Negative Consequences

- Increases Helm template surface area (35 files modified) and ongoing maintenance burden for helper functions, SA templates, and golden test files across every component.
- Existing deployments upgrading may require RBAC policy migration if custom ClusterRoleBindings or RoleBindings referenced the old shared SA name, creating a potential breaking change for operators with bespoke configurations.