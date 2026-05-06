# Support single domain setup for Camunda Platform Helm chart ingress routing

- Status: accepted
- Date: 2022-09-07
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform Helm chart previously required separate subdomains (or separate ingress resources) for each component — Identity, Operate, Optimize, and Tasklist. This forced operators to provision wildcard DNS or multiple DNS entries and TLS certificates, adding operational overhead and complicating setups where only a single domain is available (e.g., constrained enterprise environments or simple dev clusters).

## Decision Drivers

- **Operational simplicity** — reduce the DNS and TLS certificate burden for platform operators deploying all components together.
- **Deployment flexibility** — allow both multi-domain and single-domain ingress configurations without forking the chart.
- **Consistent component communication** — ensure Identity-aware components (Operate, Optimize, Tasklist) can resolve each other's URLs correctly under a shared domain with path-based routing.

## Considered Options

- **Keep per-component subdomains only** — rejected because it imposes infrastructure requirements that many teams cannot satisfy easily.
- **External reverse-proxy documentation only (no chart support)** — rejected because it pushes complexity onto users and leads to divergent, untested configurations.
- **Single consolidated ingress with path-based routing (chosen)** — provides a first-class single-domain option within the chart while retaining backward compatibility.

## Decision Outcome

The chart was extended to support a single shared ingress resource that routes traffic to individual components via path prefixes rather than subdomains. Context-path environment variables were propagated into component deployments (Identity, Operate, Optimize, Tasklist) so each service correctly handles requests under its sub-path. The `_helpers.tpl` was updated with reusable logic to compute URLs consistently across both single-domain and multi-domain modes.

### Positive Consequences

- Operators can now deploy the full platform behind a single domain and TLS certificate, significantly lowering the infrastructure barrier.
- Backward compatibility is preserved — existing multi-subdomain setups continue to work without changes.
- Centralising URL construction in `_helpers.tpl` reduces duplication and the risk of cross-component URL mismatches.

### Negative Consequences

- Added conditional logic in templates increases chart complexity and the surface area for subtle path-routing bugs.
- Component deployments now carry additional environment variables for context paths, tightening the coupling between ingress configuration and application runtime settings.