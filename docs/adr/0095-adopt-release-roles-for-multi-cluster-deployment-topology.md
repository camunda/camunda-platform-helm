# Adopt release roles for multi-cluster deployment topology

- Status: proposed
- Date: 2026-07-29
- Decision-makers: Distribution team, Management Identity owners, Camunda Hub owners

## Context and Problem Statement

The Camunda Platform Helm chart is an umbrella chart that can deploy Hub plane components
(Management Identity and Camunda Hub) and workload-plane components (Orchestration Cluster,
Connectors, and Optimize) in one release. Operators also need to run one shared Hub plane with
multiple independently deployed Orchestration Clusters. Namespace placement alone does not describe
that relationship: the Hub release must register clients and permissions, publish Camunda Hub
inventory, and provide a reachable Management Identity service, while each orchestration release must
retain its own authentication, storage, scaling, and lifecycle configuration.

Existing values support one remote workload through component-specific `alwaysRegister` switches and
manual Camunda Hub cluster inventory. Extending those singular switches for every additional cluster
would duplicate configuration across unrelated value trees and make the Hub and workload
releases disagree about cluster identity, endpoints, clients, audiences, roles, and secrets.

The chart therefore needs a durable contract that expresses how each Helm release participates in the
overall deployment, without coupling the contract to a particular namespace layout or requiring
imperative cluster discovery.

### Applicability by version

This decision applies to the Camunda 8.10 Helm chart and later chart versions that retain the
Hub/workload topology. Earlier chart versions keep their existing behavior and values.

The initial implementation lands in [PR #6688](https://github.com/camunda/camunda-platform-helm/pull/6688).
Upgrade validation from 8.9 to an 8.10 CI topology is tracked separately in
[issue #6696](https://github.com/camunda/camunda-platform-helm/issues/6696); it does not change this
deployment contract.

## Decision Drivers

- **One authoritative inventory.** Management Identity registration and Camunda Hub inventory must be
  derived from the same cluster records so client, audience, role, and endpoint configuration cannot
  drift independently.
- **Independent workload releases.** Each Orchestration Cluster must remain deployable, scalable,
  upgradable, and removable without declaring sibling clusters or duplicating its component settings
  under a Hub-only abstraction.
- **Backward compatibility.** Existing combined releases and existing `alwaysRegister` and manual Hub
  inventory values must continue to render as before unless topology mode is explicitly selected.
- **Declarative GitOps operation.** Helm, Argo CD, and Flux must render the same desired resources from
  values alone. The contract must not depend on `.Release.IsUpgrade`, live-cluster `lookup`, or
  imperative discovery.
- **Security isolation.** Multiple clusters that share a Hub plane or secondary storage must be
  able to use distinct clients, audiences, roles, secrets, and storage prefixes.
- **Future packaging independence.** The relationship between Hub and workload releases must
  survive a future split of the umbrella chart into separately published charts.

## Considered Options

- **Keep manual component configuration only.** Continue using `alwaysRegister` switches, disabled
  component values in the Hub release, and manual `camundaHub.restapi.clusters`. Rejected
  because each additional cluster duplicates related state across multiple value trees and leaves no
  authoritative cluster record.
- **Discover releases from the Kubernetes API.** Have the Hub release or chart enumerate
  namespaces, labels, Services, or Helm releases. Rejected because rendering would depend on cluster
  access and timing, would differ between Helm and GitOps tools, and would require broad discovery
  permissions.
- **Place full component configuration in Hub topology records.** Make the Hub release
  own every workload's complete auth, storage, and runtime configuration. Rejected because workload
  releases would no longer be self-contained and every workload change would require coordinated
  Hub updates.
- **Adopt explicit release roles with Hub-owned cluster inventory (chosen).** Each release declares
  its role. The Hub release owns remote cluster records used for central registration and Hub
  inventory; orchestration releases retain their existing component values and point to the Hub
  service.

## Decision Outcome

Adopt an explicit release-role contract under `global.topology`, with one Hub release owning
cluster inventory and any number of independently configured orchestration releases.

The following constraints are normative:

1. **Release roles.** A release MUST use one of `combined`, `hub`, or `orchestration`.
   `combined` MUST remain the default and MUST preserve existing single-release behavior.
2. **Hub ownership.** A Hub release MUST run Management Identity and MAY run Camunda
   Hub. It MUST be the only release that declares `global.topology.clusters` for the deployment.
3. **Cluster records.** Each Hub cluster record MUST have a stable unique ID and MUST declare
   the enabled workload components, external identity, client and audience identifiers, role policy,
   endpoint inventory, and namespace/release information needed to derive defaults.
4. **Shared derivation.** Management Identity presets, permissions, roles, Keycloak client
   initialization, and generated Camunda Hub inventory MUST derive from the same cluster records.
   Explicit manual Hub inventory MAY override generated inventory for backward compatibility.
5. **Workload ownership.** An orchestration release MUST remain self-contained. Existing
   `orchestration`, `connectors`, `optimize`, authentication, persistence, and Kubernetes workload
   values remain authoritative. The release MUST NOT declare sibling orchestration clusters.
6. **Management Identity connection.** An orchestration release MUST disable its local Management
   Identity workload and MUST provide a reachable `global.identity.service.url`. Authentication
   values in the workload release MUST match the corresponding Hub cluster record.
7. **OIDC topology.** Hub topology connections represented in Camunda Hub MUST use OIDC bearer
   tokens. Client IDs, audiences, role names, and generated Hub cluster IDs MUST be unique across
   built-in, legacy, custom, and topology-managed resources.
8. **Namespace independence.** Generated service endpoints MAY use same-Kubernetes-cluster DNS
   defaults. Operators MUST be able to override every endpoint needed when those defaults are not
   routable. The topology contract describes release relationships, not a requirement to use specific
   namespace names.
9. **Storage isolation.** Orchestration releases that share Elasticsearch or OpenSearch MUST use
   distinct Orchestration Cluster, Legacy Zeebe Exporter, Optimize reader, and Optimize application
   index prefixes. The chart and documentation MUST not imply that authentication isolation also
   isolates storage.
10. **Declarative rendering.** Resource ownership and topology behavior MUST render deterministically
    from values. Templates MUST NOT use release-operation state or live-cluster discovery to decide
    which topology resources exist.
11. **Additive reconciliation.** Generated Identity and Keycloak initialization is additive. Removing
    a cluster record MUST NOT implicitly delete external clients, resource servers, permissions, or
    roles. Explicit cleanup remains an operator responsibility until managed deletion semantics are
    designed and approved separately.
12. **Compatibility.** Existing singular `alwaysRegister`, component authentication, and manual Hub
    inventory values remain supported. Any future removal MUST follow the chart deprecation policy and
    occur no earlier than the next major chart version after a documented migration period.
13. **Packaging.** The public contract MUST describe deployment roles and connectivity rather than
    umbrella-chart internals so a future Hub/workload chart split can implement the same
    semantics without replacing the operator-facing model.

The initial chart implementation is scoped to fresh 8.10 topology deployments. Converting an existing
combined production release into split releases requires separate data, storage, and rollback planning
and is not defined by this ADR.

### Positive Consequences

- Management Identity and Camunda Hub consume one authoritative cluster inventory.
- Operators can add independently configured Orchestration Clusters without duplicating sibling state.
- Existing combined releases retain their default behavior.
- GitOps tools and Helm render the same topology resources.
- Client, role, endpoint, and storage isolation requirements become explicit and testable.
- The model can survive future chart decomposition.

### Negative Consequences

- The Hub inventory introduces a durable public values API that must evolve compatibly.
- Additive Identity reconciliation leaves explicit cleanup work when clusters are removed or renamed.
- Cluster records duplicate a subset of workload authentication identifiers so Hub can register
  them; validation is required to prevent drift and collisions.
- Same-cluster service discovery is only a default. Cross-cluster routing, trust, and failure handling
  remain operator responsibilities unless separately standardized.
- Supporting more clusters increases Identity initialization state and Camunda Hub polling load;
  supported scale limits require operational validation.

## Links

- Builds on [ADR 0026 — Hoist Identity authentication secrets to the parent chart to enable multi-namespace deployment](0026-hoist-identity-authentication-secrets-to-the-parent-chart.md): established shared authentication values needed across release boundaries.
- Builds on [ADR 0030 — Support OIDC as an alternative identity provider to Keycloak](0030-support-oidc-as-an-alternative-identity-provider-to.md): established external OIDC support and provider ownership boundaries.
- Builds on [ADR 0091 — Standardize `<component>.extraConfiguration` as the Application Configuration Mechanism](0091-adopt-component-extraconfiguration-as-the-standard-application-configuration-mechanism.md): classifies topology and cross-component coordination as Tier 2 Helm configuration.
- Related implementation: [camunda-platform-helm#6688](https://github.com/camunda/camunda-platform-helm/pull/6688).
- Related E2E support: [c8-cross-component-e2e-tests#2884](https://github.com/camunda/c8-cross-component-e2e-tests/pull/2884).
- Related documentation: [camunda-docs#9480](https://github.com/camunda/camunda-docs/pull/9480).
