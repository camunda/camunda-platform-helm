# Migrate Elasticsearch dependency from 7.x to 8.x for Camunda Platform Helm chart

- Status: accepted
- Date: 2023-10-05
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform Helm chart depended on Elasticsearch 7.x, which was approaching end-of-life. Camunda 8.3 application components (Operate, Optimize, Tasklist, Zeebe) required Elasticsearch 8 features and APIs, creating an incompatibility between the chart's bundled search backend and the applications it deployed. The chart needed to align its Elasticsearch dependency with upstream application requirements to maintain a functional and supportable deployment.

## Decision Drivers

- **Upstream compatibility**: Camunda 8.3 services required ES8 APIs; continuing with ES7 would produce runtime failures
- **Supportability**: ES7 was approaching end-of-life, meaning no security patches or bug fixes from Elastic
- **Maintenance simplicity**: Supporting two major versions of Elasticsearch in a single chart would double the configuration surface and testing matrix
- **Atomic correctness**: All template, security context, and API changes needed to land together to avoid partial-migration states

## Considered Options

- **Maintain dual ES7/ES8 support via feature flags** — Rejected due to the significant maintenance burden of diverging configuration surfaces, doubled test matrices, and the complexity of conditional templating across init containers, security contexts, and API calls
- **Defer migration to a later chart version** — Rejected because Camunda 8.3 applications already required ES8, making the current chart non-functional with its own application versions
- **Migrate incrementally (ES8 compatibility mode first, then full migration)** — Rejected in favor of a clean cut, as ES8's breaking changes to security defaults and API surfaces made a compatibility shim impractical in Helm templating

## Decision Outcome

The Elasticsearch subchart dependency was upgraded from 7.x to 8.x in a single atomic commit, with all downstream consequences addressed: init containers updated for ES8's changed security defaults, curator configurations migrated to ES8 APIs, OpenShift tuning profiles adjusted for new syscall requirements, and the full test suite (unit golden files, integration fixtures, CI workflows) regenerated against ES8. This establishes ES8 as the sole supported search backend for this chart version.

### Positive Consequences

- **Clear compatibility contract**: The chart version now maps unambiguously to a single Elasticsearch major version, eliminating configuration matrix complexity
- **Reduced long-term maintenance**: No conditional logic or feature flags needed to handle two divergent ES APIs across templates
- **Forward alignment**: The chart is positioned for ES8-only features in future Camunda releases without additional migration work

### Negative Consequences

- **Breaking change for existing users**: In-place upgrades from charts using ES7 require following a migration guide, creating operational friction and upgrade risk
- **Platform-specific complexity increase**: ES8's stricter security model required additional OpenShift tuning patches, adding platform-conditional maintenance burden that did not exist with ES7