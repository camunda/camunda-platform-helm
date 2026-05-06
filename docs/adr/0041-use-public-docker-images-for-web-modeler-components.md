# Use Public Docker Images for Web Modeler Components

- Status: accepted
- Date: 2024-06-26
- Decision-makers: Wolfgang Amann

## Context and Problem Statement

The Web Modeler components (REST API, webapp, websockets) were previously referenced using private/internal Docker image registries in the Camunda Platform Helm chart. This created a barrier for self-managed users who needed additional registry credentials to pull these images. The chart needed to transition to publicly available Docker images to align with the self-managed distribution model.

## Decision Drivers

- **Accessibility for self-managed deployments**: Users should be able to deploy the full platform without requiring special registry credentials for Web Modeler
- **Consistency with other components**: Other Camunda 8 components already use public Docker images; Web Modeler was an outlier
- **Reduced operational friction**: Eliminating private registry authentication simplifies cluster setup and CI/CD pipelines

## Considered Options

- **Maintain private registry with documented credential setup** — Rejected because it adds operational burden for every deployer and complicates air-gapped/mirrored setups
- **Use public Docker images (chosen)** — Simplest path for broad adoption with no credential management overhead

## Decision Outcome

The Helm chart values were updated to reference publicly available Docker images for all three Web Modeler components (REST API, webapp, websockets). This is a configuration-level change in the chart's default values, making Web Modeler deployable out-of-the-box without private registry access.

### Positive Consequences

- Self-managed users can deploy the complete Camunda Platform without any private registry credentials
- Simplified onboarding and reduced support burden around image pull errors
- Consistent image sourcing strategy across all platform components

### Negative Consequences

- Public image availability may lag behind internal builds, potentially delaying access to pre-release fixes
- Loss of ability to gate access to Web Modeler images via registry credentials (if that was previously used as a distribution control mechanism)