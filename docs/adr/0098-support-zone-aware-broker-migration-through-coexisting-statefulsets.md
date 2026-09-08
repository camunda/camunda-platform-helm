# Support zone-aware broker migration through coexisting StatefulSets

- Status: proposed
- Date: 2026-09-04
- Decision-makers: Distribution team, Distributed Systems team

## Context and Problem Statement

Camunda Orchestration brokers without zone awareness use numeric member IDs, while zoned brokers use composite member IDs such as `<zone>_<index>`. Broker identity is persisted in Raft and cluster metadata, so an existing broker cannot safely adopt a zoned identity in place.

Migrating a running cluster therefore requires replacement brokers to run alongside the existing brokers while cluster membership and partition replicas move to the new identities. The Helm chart must own both generations during this transition, keep clients connected to all local brokers through stable Service DNS, and remove generation-specific legacy resources without restarting the zoned brokers.

A logical Orchestration cluster may span multiple zones and Kubernetes clusters. Each Helm release owns the resources for one Kubernetes cluster and migrates only its local unzoned StatefulSet. The overall migration proceeds one zone at a time.

### Applicability by version

This decision applies to the Camunda 8.10 Helm chart. Earlier chart versions retain their existing Orchestration topology and values.

## Decision Drivers

- **Processing continuity.** Clients must remain connected while partitions and membership move between broker generations.
- **Stable broker identity.** Persisted numeric broker identities must not be changed in place.
- **Declarative ownership.** All temporary and final Kubernetes resources must remain owned by Helm.
- **No unintended restarts.** Starting or completing a migration must not restart brokers that currently hold partitions.
- **Multi-cluster operation.** Each release must manage its local zone without attempting to own resources in another Kubernetes cluster.
- **Deterministic rendering.** Migration resources must derive entirely from Helm values and not from live-cluster discovery or Helm operation state.

## Considered Options

- **Change existing broker identities in place.** Rejected because broker identity is persisted in Raft storage and cluster metadata. Changing only the rendered configuration would make the broker inconsistent with its persisted state.
- **Deploy zoned brokers through a second Helm release.** Rejected because release-scoped labels prevent the existing Services from selecting both broker generations, and transferring resource ownership between releases requires unsafe manual reconciliation.
- **Apply manually rendered zoned resources outside Helm.** Rejected because the temporary StatefulSet and its supporting resources would not participate in the release lifecycle.
- **Render every zone from one Helm release.** Rejected because a release can manage resources only in its Kubernetes cluster. Remote zones are part of the logical topology but are owned by releases in their respective clusters.
- **Render coexisting broker generations from the existing release (chosen).** The release temporarily owns its unchanged legacy resources and a new set of zone-specific resources.

## Decision Outcome

The Camunda 8.10 chart will support zone-aware migration by allowing each Orchestration release to render its local unzoned and zoned broker generations concurrently.

The following constraints are normative:

1. **Replacement migration.** An existing broker MUST NOT be converted from a numeric identity to a zoned identity in place. Zoned brokers MUST use new StatefulSet pods and new persistent volumes.

2. **One local zone per release.** Each release in zoned mode MUST select one local zone. The release MUST render Kubernetes workloads only for that zone and MUST NOT render workloads belonging to remote Kubernetes clusters.

3. **Complete topology.** Every participating release MUST receive the complete zone topology. The selected local zone controls resource rendering; the complete topology controls cluster size, replication, partition placement, and broker membership configuration.

4. **Temporary coexistence.** The chart MUST provide an explicit migration state that renders the existing unzoned StatefulSet and its supporting resources alongside the local zoned StatefulSet.

5. **Legacy manifest stability.** Entering the migration state MUST preserve the existing unzoned StatefulSet pod template, selector, name, governing Service, configuration, and broker identity. The Helm upgrade MUST NOT roll the existing brokers solely because zoned resources were added.

6. **Resource ownership.** Zoned StatefulSets and their governing headless Services MUST use zone-specific names and selectors. Their ConfigMaps and PodDisruptionBudgets MUST be independently addressable from the retained numbered resources. Both broker generations MUST use the release's existing ServiceAccount because it represents one workload identity and contains no generation-specific state. The unsuffixed headless Service is not the governing Service of the zoned StatefulSet and MUST keep its existing name.

7. **Service continuity.** Shared client-facing Services MUST remain zone-agnostic and select both unzoned and zoned brokers in the local Kubernetes cluster during migration and after it. This includes the gateway Service and the unsuffixed headless broker Service (`orchestration.fullname`), which is the stable in-cluster DNS name used by Connectors and other clients. Zone-specific labels MUST NOT be added to selectors that clients rely on to reach all local brokers. That unsuffixed headless Service MUST also be rendered for a greenfield zoned install so the stable DNS name exists without a prior numbered deployment.

8. **Cross-cluster discovery.** The chart MAY generate Kubernetes DNS contact points only for resources owned by the local release. Operators MUST provide externally resolvable contact points for brokers owned by other releases or Kubernetes clusters.

9. **Explicit cluster transition.** Helm MUST only create and remove Kubernetes resources. Moving partitions, changing broker membership, and adopting zoned identities MUST remain explicit Orchestration management operations performed between Helm upgrades.

10. **Sequential migration.** A multi-zone cluster MUST be migrated one zone at a time. The local zoned brokers MUST join and the local unzoned brokers MUST leave cluster membership before the next zone begins its migration.

11. **Stable zoned workload.** Leaving the migration state MUST remove the generation-specific retained resources (the numbered StatefulSet, its ConfigMap, and its PodDisruptionBudget) without changing the zoned StatefulSet pod template. The unsuffixed headless Service and the shared ServiceAccount MUST remain. Migration-only bootstrap contact points MUST NOT cause the zoned brokers to restart when legacy retention is disabled.

12. **Persistent-volume cleanup.** Removing the legacy StatefulSet MUST NOT implicitly delete its persistent volume claims. Cleanup remains an explicit operator action after verifying that the corresponding brokers have left the logical cluster.

13. **Declarative behavior.** Resource coexistence and removal MUST render deterministically from values. Templates MUST NOT use `.Release.IsUpgrade`, Kubernetes `lookup`, or live cluster state to infer the migration phase.

The initial implementation is scoped to `charts/camunda-platform-8.10` and the Orchestration component.

### Positive Consequences

- Existing clusters can adopt zone-aware broker identities without stopping processing.
- Helm owns both temporary and final migration resources.
- Existing brokers remain unchanged while replacement brokers join.
- Both broker generations retain the same Kubernetes workload identity.
- Shared Services preserve local client connectivity throughout the transition and after it, because the unsuffixed headless Service keeps selecting zoned brokers.
- Each Kubernetes cluster can be migrated independently as part of a coordinated logical-cluster migration.
- The final release state matches a normal zoned deployment: generation-specific numbered workloads are gone, while the unsuffixed headless Service remains as the stable broker DNS name.

### Negative Consequences

- Migration temporarily doubles the local broker pods and storage requirements.
- Operators must coordinate Helm upgrades with Orchestration management API operations.
- Cross-cluster networking and externally resolvable broker addresses remain operator responsibilities.
- Legacy persistent volume claims require explicit cleanup.
- A partially completed migration leaves two broker generations running until the operator safely resumes or rolls back the procedure.
- The chart must preserve migration-sensitive resource names, selectors, and checksums as compatibility contracts.
- Zoned mode always renders both a zone-suffixed governing headless Service and the unsuffixed broker Service.

## Links

- Builds on [ADR 0025 — Enable Zeebe Multi-Region Deployment Support in the Camunda Platform Helm Chart](0025-enable-zeebe-multi-region-deployment-support-in-the-camunda.md).
- Related to [ADR 0095 — Adopt release roles for multi-cluster deployment topology](0095-adopt-release-roles-for-multi-cluster-deployment-topology.md).
