# Consolidate ingress routing into a single global combined ingress

- Status: accepted
- Date: 2024-11-19
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform Helm chart historically supported two ingress patterns: per-component individual ingress resources and a global combined ingress that routes all traffic through a single entry point. Maintaining both patterns created significant template complexity, duplicated configuration surfaces, and increased the testing burden across multiple components (Connectors, Console, Identity, Optimize, Web Modeler). The decision was whether to continue supporting both approaches or converge on a single ingress strategy.

## Decision Drivers

- **Reduced configuration complexity** — two ingress modes meant users had to understand when to use which, leading to misconfiguration and support overhead
- **Maintainability** — per-component ingress templates duplicated routing logic across six+ components, making changes error-prone
- **Operational consistency** — a single ingress resource simplifies TLS management, DNS configuration, and network policy reasoning in production deployments
- **Chart evolution** — the alpha chart track is the right place to make breaking simplifications before they reach stable versions

## Considered Options

- **Keep both ingress modes with a deprecation warning** — rejected because it delays complexity removal and still requires maintaining both code paths during the deprecation window
- **Keep only per-component ingress resources** — rejected because the global combined ingress better matches the platform's converged deployment model and reduces the number of external IP addresses and certificates required
- **Remove separated ingress and keep only the global combined ingress** — selected

## Decision Outcome

All per-component ingress templates (Connectors, Console, Identity, Optimize, Web Modeler) were removed in favor of the single global combined HTTP ingress resource. Routing to individual components is now exclusively managed through path-based rules on the unified ingress, enforcing a single entry point architecture for the platform.

### Positive Consequences

- Single point of TLS termination and DNS management reduces operational overhead for platform operators
- Template codebase is significantly smaller with fewer conditional branches, making future ingress changes straightforward
- Clearer mental model for users — one ingress, one hostname, path-based routing to all components

### Negative Consequences

- Breaking change for users who relied on per-component ingress resources with separate hostnames, certificates, or annotations — requires migration
- Reduced flexibility for advanced networking scenarios where individual components need distinct ingress controllers or load balancer configurations