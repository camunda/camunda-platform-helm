# Introduce Web Modeler as an in-tree subchart with multi-component deployment architecture

- Status: accepted
- Date: 2023-01-16
- Decision-makers: Jan Friedenstab

## Context and Problem Statement

Web Modeler needed to be included in the Camunda Platform self-managed distribution to provide a unified deployment experience. The question was how to package a multi-component application (REST API, webapp, websockets) within the existing Helm chart ecosystem while managing the risk of introducing a beta-quality component into a production chart used by all self-managed customers.

## Decision Drivers

- **Unified deployment lifecycle** — customers expect a single `helm install` to provision the entire Camunda Platform, and Web Modeler should not require a separate installation step
- **Independent scalability** — Web Modeler's components have different resource profiles and scaling characteristics that must be addressable independently
- **Risk containment** — as a beta component, Web Modeler must not affect existing deployments or destabilize the chart for users who do not need it
- **Versioning control** — the subchart must evolve in lockstep with the parent chart to avoid compatibility drift

## Considered Options

- **External chart dependency** — rejected because it would decouple versioning from the parent chart, complicate release coordination, and force users to manage an additional Helm repository
- **Single deployment with multiple containers (sidecar pattern)** — rejected because it couples component lifecycles, prevents independent scaling, and makes failure isolation difficult
- **In-tree subchart with multi-deployment architecture** — selected for tight integration, independent component scaling, and consistency with how Identity and other components are structured

## Decision Outcome

Web Modeler was added as an in-tree subchart containing three separate Kubernetes Deployments (restapi, webapp, websockets), disabled by default behind a feature flag. Shared configuration is consolidated into a common ConfigMap and Secret, while component-specific secrets remain isolated. Integration with Identity is handled through helper templates and environment variable injection, following established patterns in the chart.

### Positive Consequences

- Customers get Web Modeler deployment for free through the existing chart upgrade path with no additional tooling
- Each component can be scaled, restarted, and resource-tuned independently, matching production operational needs
- Disabled-by-default approach ensures zero impact on existing deployments and allows the API surface to stabilize before GA

### Negative Consequences

- Adds 48 files and a significant long-term maintenance burden, including golden file tests that must be kept in sync with template changes
- Private Docker registry requirement for Web Modeler images breaks the open-registry assumption used by all other components, complicating air-gapped deployments and requiring additional `imagePullSecrets` documentation and support