# Consolidate Web Modeler into a single-container architecture by removing the standalone webapp component

- Status: accepted
- Date: 2026-03-04
- Decision-makers: Jan Friedenstab

## Context and Problem Statement

Web Modeler was deployed as a multi-container architecture consisting of three separate Kubernetes workloads: restapi, webapp (frontend UI), and websockets. This introduced operational complexity in networking, ingress routing, and inter-component connectivity — all for serving what is fundamentally a single application. The upstream application evolved to serve the frontend directly from the restapi process, making the standalone webapp container architecturally redundant.

## Decision Drivers

- **Operational simplicity:** Reducing the number of independently deployed components decreases the failure surface area and simplifies troubleshooting for platform operators.
- **Ingress and networking complexity:** Routing traffic to separate frontend and backend containers required additional ingress rules and service definitions that added configuration burden without meaningful benefit.
- **Alignment with upstream application architecture:** The restapi component now serves the UI directly, making the webapp container a vestigial deployment artifact rather than a necessary architectural boundary.
- **Chart maintainability:** Fewer templates, golden files, and unit tests reduce the ongoing maintenance cost of the Helm chart.

## Considered Options

- **Maintain the multi-container architecture:** Keep webapp as a separate Deployment and Service, preserving independent scaling and deployment of the frontend. Rejected because the upstream application no longer requires this separation, and the operational cost outweighed the theoretical scaling benefit for a component with modest load characteristics.
- **Merge webapp into a sidecar container within the restapi Pod:** Would retain process isolation while reducing Kubernetes resource count. Rejected as unnecessarily complex given the application already serves the UI from a single process.

## Decision Outcome

The webapp Deployment, Service, ConfigMap, ingress routes, and all associated test infrastructure were removed from the Camunda Platform 8.9 Helm chart. The restapi component now serves both the API and the frontend UI, consolidating Web Modeler from three containers to two (restapi + websockets). The JSON schema and values structure were updated to remove webapp-specific configuration surface area.

### Positive Consequences

- Eliminates an entire class of deployment failures related to webapp-restapi connectivity and version skew between frontend and backend containers.
- Simplifies ingress configuration by removing frontend-specific routing rules, reducing misconfiguration risk for operators.
- Reduces Kubernetes resource consumption and chart template complexity, lowering the barrier for contributors and operators alike.

### Negative Consequences

- Removes the ability to independently scale the frontend serving layer separately from the API — acceptable given Web Modeler's load profile, but a constraint if frontend traffic grows disproportionately.
- Breaking change for users with custom `webModeler.webapp.*` values in their Helm overrides; these will require manual cleanup during upgrade to 8.9.