# Backport the orchestration topology role to supported charts

- Status: proposed
- Date: 2026-09-02
- Decision-makers: Distribution team, Management Identity owners, Camunda Hub owners
- Amends: [ADR 0095](0095-adopt-release-roles-for-multi-cluster-deployment-topology.md)

## Context and Problem Statement

[ADR 0095](0095-adopt-release-roles-for-multi-cluster-deployment-topology.md) limits the release-role contract to the 8.10 chart and later. Camunda Hub 8.10 can manage supported Orchestration Cluster versions 8.7 through 8.10, but the earlier charts cannot declare a workload-only release without manually disabling Hub-plane components and their related resources.

The supported chart versions need one consistent way to deploy an Orchestration Cluster that connects to an 8.10 Hub release. The Hub inventory and registration contract remains owned by 8.10 because Camunda Hub does not exist in the earlier chart versions.

## Decision Drivers

- **Supported-version compatibility.** One 8.10 Hub release must manage supported Orchestration Cluster versions without requiring operators to reconstruct component ownership through individual enable flags.
- **No default behavior change.** Existing combined deployments must render as before unless an operator selects the orchestration role.
- **Version-specific workload models.** Charts 8.8 and later use the unified Orchestration Cluster, while 8.7 uses separate Zeebe, Operate, and Tasklist workloads and endpoints.
- **One Hub inventory owner.** Registration, clients, permissions, and cluster inventory must remain in the 8.10 Hub release.

## Considered Options

- **Keep topology support in 8.10 only.** Operators would continue to maintain manual component enable flags in older workload releases. Rejected because the chart would not provide a tested deployment contract for combinations Camunda Hub supports.
- **Backport every topology role.** Charts 8.7 through 8.9 would gain Hub mode and cluster inventory. Rejected because those chart versions do not contain Camunda Hub and cannot own its inventory.
- **Backport orchestration mode only (chosen).** Supported older charts gain the workload-side role while the 8.10 chart remains the Hub owner.

## Decision Outcome

Backport the orchestration topology role to the 8.7, 8.8, and 8.9 charts.

1. Charts 8.7 through 8.9 MUST support `global.topology.mode` values `combined` and `orchestration`.
2. `combined` MUST remain the default and MUST preserve existing rendering.
3. The older charts MUST NOT support Hub mode or `global.topology.clusters`.
4. An orchestration release MUST disable its local Management Identity and provide a reachable `global.identity.service.url` for the 8.10 Hub release.
5. An orchestration release MUST retain its version-specific workload components. Charts 8.8 and 8.9 retain the unified Orchestration Cluster. Chart 8.7 retains Zeebe, Zeebe Gateway, Operate, Tasklist, Connectors, and Optimize.
6. The 8.10 Hub release MUST own cluster registration, clients, permissions, and inventory for every connected release.
7. Hub inventory for an 8.7 release MUST use the legacy Operate, Tasklist, and Zeebe service endpoints and MUST NOT publish an Orchestration Admin component.
8. CI MUST test one 8.10 Hub release against 8.10, 8.9, 8.8, and 8.7 orchestration releases. Each release MUST deploy from its own chart and values layers.
9. The backport MUST remain additive and opt-in. Existing users who do not select orchestration mode MUST see no resource ownership change.

This decision applies to chart versions 8.7 through 8.10. Hub mode and Hub-owned cluster inventory remain available only in 8.10 and later.

### Positive Consequences

- Supported Orchestration Cluster versions have one tested workload-only deployment contract.
- Operators no longer need to maintain a manual list of Hub-plane component disable flags.
- CI verifies the compatibility combinations in one multi-release deployment.

### Negative Consequences

- The same role has version-specific implementation details because 8.7 predates the unified Orchestration Cluster.
- Changes to the 8.10 inventory contract must retain the legacy endpoint mapping while 8.7 remains supported.
- Backporting a public value increases the maintenance scope of supported patch lines.

## Links

- Amends [ADR 0095 - Adopt release roles for multi-cluster deployment topology](0095-adopt-release-roles-for-multi-cluster-deployment-topology.md).
- Builds on [ADR 0091 - Standardize `<component>.extraConfiguration` as the Application Configuration Mechanism](0091-adopt-component-extraconfiguration-as-the-standard-application-configuration-mechanism.md): release roles are Tier 2 cross-component coordination.
