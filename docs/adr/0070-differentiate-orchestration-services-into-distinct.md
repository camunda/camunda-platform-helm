# Differentiate orchestration services into distinct Kubernetes resources for independent routing and observability

- Status: accepted
- Date: 2025-09-18
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The unified orchestration component introduced in Camunda 8.8 bundles multiple logical services (Zeebe broker, gateway, importer) into a single StatefulSet. However, exposing all these capabilities through a single Kubernetes Service creates operational ambiguity: ingress routing cannot target specific protocols independently, health checks conflate broker and gateway readiness, and ServiceMonitors cannot distinguish metrics sources. A structural separation at the Service layer is needed without breaking the unified deployment model.

## Decision Drivers

- **Routing precision**: Ingress rules for gRPC (gateway) and HTTP (REST API) traffic require distinct Service backends to function correctly with cloud load balancers and ingress controllers.
- **Observability granularity**: ServiceMonitors need to scrape different port/path combinations for broker vs. gateway metrics; a single Service makes this configuration fragile.
- **Operational clarity**: On-call engineers and platform teams need unambiguous DNS names and health endpoints to reason about system state during incidents.
- **Backward compatibility**: The change must not break existing deployments during Helm upgrades from earlier 8.8 patch releases.

## Considered Options

- **Separate Deployments** — Split broker and gateway into distinct workloads. Rejected: higher resource cost, contradicts the unified architecture introduced in 8.8, and increases scheduling complexity.
- **Port-based differentiation only** — Single Service with multiple named ports. Rejected: insufficient for independent ingress routing (most ingress controllers bind one backend per rule) and prevents granular ServiceMonitor targeting.
- **Status quo (single generic service)** — Rejected: creates operational ambiguity in routing and monitoring that is unacceptable for production deployments at scale.

## Decision Outcome

The orchestration component's Service layer was split into three distinct resources: a primary service (`service.yaml`) for broker communication, a gateway service (`service-gateway.yaml`) for client-facing gRPC/REST traffic, and a headless service (`service-headless.yaml`) for StatefulSet peer discovery. Helper templates were refactored to generate differentiated names, labels, and selectors for each service type, while the StatefulSet itself remains unified.

### Positive Consequences

- Ingress rules can now independently target gateway traffic (gRPC and HTTP) without affecting broker-to-broker communication paths.
- ServiceMonitors can be configured per service type, enabling precise alerting on gateway latency separately from broker replication health.
- Internal service discovery (headless) is cleanly separated from external-facing endpoints, reducing the risk of accidental exposure.

### Negative Consequences

- Increased Kubernetes resource count per release and additional template helpers increase chart maintenance burden and review surface area.
- Backward-compatibility shims (via `z_compatibility_helpers.tpl`) add hidden complexity that must be carried until the next major version boundary allows a clean break.