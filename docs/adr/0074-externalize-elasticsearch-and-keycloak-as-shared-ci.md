
# Externalize Elasticsearch and Keycloak as shared CI infrastructure services

- Status: accepted
- Date: 2025-11-13
- Decision-makers: Eamonn Moloney

## Context and Problem Statement

Each CI test scenario was deploying its own Elasticsearch and Keycloak instances as Helm sub-charts within the test namespace. This duplicated heavyweight stateful services across every parallel test run, consuming significant GKE resources, increasing scenario deploy times, and diverging from production topologies where these services are typically managed externally to the Camunda platform namespace.

## Decision Drivers

- **CI resource efficiency** — parallel test runs were consuming excessive GKE capacity by each spinning up dedicated ES and KC pods
- **Deploy time reduction** — waiting for ES and KC readiness dominated scenario setup time, creating a bottleneck in the feedback loop
- **Production fidelity** — most customers run managed Elasticsearch and externally-hosted Keycloak; testing against bundled sub-charts left the external connectivity path under-validated
- **Maintainability** — duplicating ES/KC configuration across every scenario's values files created drift and inconsistency

## Considered Options

- **Keep bundled sub-charts (status quo)** — simplest operationally but wasteful, slow, and does not validate external service integration paths
- **Per-run ephemeral external deploys** — still duplicates provisioning work across parallel scenarios, offering no resource savings
- **Shared namespace with chart-internal services** — complicates Helm release lifecycle management and creates tight coupling between test scenarios sharing a release

## Decision Outcome

A new shared infrastructure layer (`infra/elasticsearch/`, `infra/keycloak/`) was introduced with dedicated deploy/delete/cleanup workflows that manage long-lived ES and KC instances on the CI cluster. Test scenarios across chart versions 8.6–8.9 were reconfigured to connect to these external endpoints, and the CI matrix plumbing was extended to support the external-infra pattern as a first-class deployment mode.

### Positive Consequences

- Significantly reduced per-scenario deploy time and GKE resource consumption by eliminating redundant stateful service provisioning
- CI now validates the external service connectivity path that mirrors real customer deployments, catching integration issues earlier
- Centralized ES/KC configuration reduces drift across chart versions and simplifies version upgrades of these dependencies

### Negative Consequences

- Increased operational complexity: shared infrastructure requires lifecycle management workflows, cleanup automation, and secret plumbing that did not previously exist
- Reduced test isolation — a corrupted or unavailable shared ES/KC instance can cascade failures across all scenarios, making root-cause analysis harder during infrastructure incidents