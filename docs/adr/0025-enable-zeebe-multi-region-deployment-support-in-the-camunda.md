# Enable Zeebe Multi-Region Deployment Support in the Camunda Platform Helm Chart

- Status: accepted
- Date: 2023-10-31
- Decision-makers: Jesse Simpson

## Context and Problem Statement

Camunda's Zeebe workflow engine was limited to single-region Kubernetes deployments, which created a single point of failure for the entire process orchestration layer. Organizations requiring geographic redundancy or disaster recovery for their workflow infrastructure had no supported path within the official Helm chart. The architecture needed to evolve to allow Zeebe brokers to form clusters spanning multiple Kubernetes clusters in different regions.

## Decision Drivers

- **High availability requirements**: Production workloads demand resilience against entire region failures, not just node or pod failures
- **Data sovereignty and latency**: Some deployments need brokers geographically distributed to serve regional traffic while maintaining a unified cluster
- **Operational consistency**: Multi-region should be configurable through the same Helm values interface teams already use, avoiding custom forks or post-render patches
- **Upstream Zeebe support**: Zeebe's partition replication protocol supports cross-region awareness, but the chart needed to expose the necessary configuration surface

## Considered Options

- **Separate independent clusters per region with application-level routing**: Rejected because it breaks Zeebe's partition replication guarantees and requires custom synchronization logic
- **External operators or CRDs for multi-cluster orchestration**: Rejected due to added operational complexity and dependency on non-standard tooling
- **Helm chart configuration with StatefulSet and ConfigMap changes**: Chosen as it maintains the existing deployment model while exposing multi-region topology through standard values

## Decision Outcome

The Helm chart was extended to support multi-region Zeebe deployments through configmap and statefulset template modifications, allowing users to declare region topology and broker-to-region mapping via `values.yaml`. This enables Zeebe's built-in replication awareness to distribute partition replicas across failure domains without requiring external orchestration tooling.

### Positive Consequences

- **Disaster recovery without custom tooling**: Teams can achieve cross-region redundancy using the standard Helm chart and familiar values-based configuration
- **Partition-aware replica placement**: Zeebe can intelligently place partition replicas across regions, ensuring no single region failure loses quorum
- **Incremental adoption**: Existing single-region deployments are unaffected; multi-region is opt-in through additional configuration

### Negative Consequences

- **Increased networking complexity**: Cross-region Zeebe broker communication requires reliable inter-cluster networking (e.g., VPN, mesh), which is outside the chart's control but now implicitly required
- **Operational burden for multi-cluster coordination**: Users must manage Helm releases across multiple clusters with coordinated values, increasing the risk of configuration drift between regions