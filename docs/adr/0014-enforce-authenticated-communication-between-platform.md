# Enforce authenticated communication between platform components and Zeebe Gateway

- Status: accepted
- Date: 2023-05-10
- Decision-makers: Sebastian Bathke

## Context and Problem Statement

The Camunda Platform Helm chart deployed Zeebe Gateway without requiring authentication from internal platform components (Operate, Tasklist, Connectors, Identity). This meant any service within the Kubernetes namespace could submit commands to the workflow engine without proving its identity, creating an implicit trust boundary that weakened the platform's security posture in multi-tenant and shared-cluster environments.

## Decision Drivers

- **Zero-trust internal communication**: Moving toward authenticated service-to-service calls even within the cluster, reducing blast radius of a compromised component
- **Identity as the central authority**: Leveraging the existing Identity service to issue and validate credentials rather than introducing a separate auth mechanism for Zeebe
- **Upgrade safety**: Ensuring existing deployments can migrate to authenticated mode without breaking running workflows or requiring manual secret rotation
- **Consistency across components**: All platform services should authenticate to Zeebe the same way, reducing operational surprise

## Considered Options

- **mTLS-only via service mesh (e.g., Istio)**: Rejected because it introduces an infrastructure dependency not all users have, and does not provide application-level identity claims
- **Static shared secret without Identity integration**: Rejected because it bypasses the existing identity provider and creates secret sprawl
- **Keep unauthenticated internal communication**: Rejected because it does not meet enterprise security requirements for production deployments

## Decision Outcome

Zeebe Gateway authentication is enabled by default, with Identity issuing credentials (stored as a Kubernetes Secret) that Operate, Tasklist, and Connectors present when connecting to the gateway. The Helm templates for each component's deployment now inject the necessary environment variables referencing the shared Zeebe secret managed by the Identity subchart.

### Positive Consequences

- Platform components now cryptographically prove their identity to Zeebe, enabling future per-component authorization policies
- Secret lifecycle is centralized in the Identity subchart, giving operators a single point of rotation
- Integration and upgrade test scenarios validate the authenticated path, preventing silent regression

### Negative Consequences

- Adds a hard dependency on Identity being healthy before other components can communicate with Zeebe, tightening the startup dependency graph
- Existing deployments upgrading to this chart version must handle the new secret provisioning, increasing upgrade complexity