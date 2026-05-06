# Replace Keycloak fullname string with structured URL configuration map

- Status: accepted
- Date: 2022-10-25
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform Helm chart previously used a single string value (`.global.identity.keycloak.fullname`) to derive Keycloak connection URLs across all sub-charts. This approach was rigid — it assumed Keycloak was always co-deployed within the same cluster namespace and reachable via a predictable internal service name. Teams deploying against external or custom Keycloak instances had no clean way to override individual URL components (protocol, host, port, context path).

## Decision Drivers

- **Deployment flexibility**: Support both in-cluster and external Keycloak instances without workarounds or template overrides.
- **Separation of concerns**: Decouple the logical Keycloak endpoint configuration from Kubernetes service naming conventions.
- **Forward compatibility**: Establish a configuration structure that can accommodate future auth provider changes (e.g., different IdPs, multi-realm setups) without further breaking changes.
- **Explicit over implicit**: Make the full URL composition visible in values rather than buried in template logic.

## Considered Options

- **Keep fullname string and add override flags** — Rejected because it would lead to combinatorial complexity in templates and unclear precedence rules between fullname and individual overrides.
- **Single URL string field** — Rejected because a monolithic URL string is harder to template conditionally (e.g., changing only the port or protocol for TLS termination scenarios).
- **Structured URL map (chosen)** — Allows each component (protocol, host, port, path) to be independently configurable while templates compose the final URL deterministically.

## Decision Outcome

The Keycloak connection configuration was restructured from a single derived string into a dictionary/map at `.global.identity.keycloak.url`, with the previous `.global.identity.keycloak.fullname` deprecated. All sub-chart templates (Identity, Operate, Optimize, Tasklist) and the ingress configuration were updated to consume the new structured format. Validation constraints were added to enforce correct configuration at deploy time.

### Positive Consequences

- External Keycloak deployments can now be configured without forking or patching templates.
- Each URL component can be independently overridden per environment (e.g., TLS in production, plain HTTP in dev).
- Template logic becomes more explicit about what endpoint it constructs, improving debuggability of connection issues.

### Negative Consequences

- Breaking change requires all existing users who set `.global.identity.keycloak.fullname` or the old string-typed `.global.identity.keycloak.url` to migrate their values files.
- The structured map adds marginally more verbosity to the default `values.yaml` compared to the previous single-string approach.