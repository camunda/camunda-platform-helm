# Hoist Identity authentication secrets to the parent chart to enable multi-namespace deployment

- Status: accepted
- Date: 2023-11-24
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

Camunda 8 Self-Managed deploys multiple components (Operate, Tasklist, Zeebe, Connectors, etc.) that authenticate against a central Identity service using shared secrets. These secrets were previously owned by the Identity subchart, scoping them to Identity's release namespace. To support multi-namespace deployment — where each component runs in its own Kubernetes namespace — secrets must be accessible across namespace boundaries, which subchart-scoped resources cannot provide without external tooling.

## Decision Drivers

- **Multi-namespace deployment capability:** Components must authenticate against Identity regardless of which namespace they are deployed into.
- **Operational simplicity:** Avoid introducing external secret-syncing infrastructure (e.g., Kubernetes Reflector, External Secrets Operator) as a hard dependency.
- **Centralized auth authority:** Identity remains the single source of truth for component credentials; secrets must be generated and managed from a single control point.
- **Incremental delivery:** Enable the architectural foundation to ship before the full feature is production-ready, reducing blast radius per release.

## Considered Options

- **Keep secrets in the Identity subchart and use external secret-syncing mechanisms** (e.g., Kubernetes External Secrets, Reflector) — rejected because it adds operational complexity and an external dependency for a core platform capability.
- **Have each component generate its own credentials independently** — rejected because Identity is the central authentication authority; distributed credential generation would fragment the trust model and complicate rotation.
- **Hoist secrets to the parent chart** — selected as the approach that maintains a single point of secret ownership while enabling cross-namespace distribution through Helm's native release scoping.

## Decision Outcome

Identity authentication secrets for all components (Connectors, Console, Operate, Optimize, Tasklist, Zeebe) were relocated from the Identity subchart to a new `camunda/` directory in the parent chart's templates. This establishes the parent chart as the owner of platform-wide shared resources, creating an architectural seam between component-specific templates and cross-cutting platform concerns. The change ships as pre-alpha, acknowledging that a corresponding Identity code change is required for full automation.

### Positive Consequences

- **Enables multi-namespace topology:** Secrets are now scoped to the parent release and can be referenced or distributed to any component namespace without external tooling.
- **Establishes a platform-resource layer:** The new `templates/camunda/` directory creates a clear boundary for shared resources (secrets, configmaps, ingress), improving template organization as the chart grows.
- **Preserves centralized auth model:** Identity remains the authority for credentials; the parent chart simply takes ownership of the Kubernetes resource lifecycle.

### Negative Consequences

- **Increased parent chart complexity and coupling:** The top-level chart now encodes knowledge of Identity's authentication model, tightening coupling between the parent and Identity's internal contract.
- **Partial implementation requiring follow-up:** The feature is explicitly incomplete — a pending Identity code change is needed for full automation, meaning this ships as pre-alpha with instability expectations and carries coordination cost across future PRs.