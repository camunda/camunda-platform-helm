# Standardize `<component>.extraConfiguration` as the Application Configuration Mechanism

- Status: proposed
- Date: 2026-05-06
- Decision-makers: Distribution team

## Context and Problem Statement

The Camunda Helm chart historically added abstraction layers over application-level configuration to simplify inconsistent component setups (e.g., Elasticsearch connections, OIDC wiring, log levels). Over time, this produced a third configuration layer sitting between the application's native config and Kubernetes deployment concerns. Critically, these Helm-level parameters were not a 1:1 mapping to native application properties — they were a simplified abstraction that required its own Helm-specific documentation and did not transfer to other deployment methods such as ECS, Docker, or Jar. Instead of having a single configuration reference for all deployment methods, operators were required to learn a Helm-specific configuration layer on top of the application's own.

The 8.8 re-architecture of the Orchestration Cluster (consolidating Operate, Tasklist, and Zeebe into a single application) significantly reduced the number of components requiring individual configuration, making a large portion of the existing per-component Helm abstraction keys redundant for the orchestration cluster. `<component>.extraConfiguration` existed in earlier releases but as an unordered map with inconsistent behavior across components (e.g., Optimize and Console used different merge strategies). In 8.9, it was unified across all components into an ordered list of file entries — making it consistent, Spring-native, and the documented recommended path for application configuration without Helm needing to understand the content.

As additional deployment methods (ECS, Docker, Jar) have become first-class targets too, the mismatch between Helm-specific configuration abstractions and deployment-method-agnostic application configuration has become increasingly visible. Knowledge of the Helm abstraction layer does not transfer to other deployment methods, and a new application feature that requires configuration currently demands changes to the Helm chart even when no Kubernetes-specific concern is involved.

## Decision Drivers

- **Separation of concerns**: Helm charts should configure Kubernetes infrastructure — pods, services, networking, storage — not application behavior. Application-level configuration has no Kubernetes-specific concern and should not require chart changes.
- **Portability**: Application configuration knowledge must transfer across all deployment methods (Helm, ECS, Docker, Jar). Helm-specific abstraction keys do not transfer and create a siloed knowledge requirement.
- **Application team autonomy**: Application teams must be able to introduce and document new configuration without requiring Helm chart changes in the common case.
- **Complexity reduction**: Accumulated abstraction layers increase upgrade risk and chart surface area. Removing them reduces the gap between chart values and Kubernetes primitives.
- **Established path**: `<component>.extraConfiguration` is already the documented recommended path as of 8.9, making this decision a formalization rather than a new direction.

## Considered Options

- **Option A — Status quo**: Keep all existing app config keys; continue adding new ones as needed. Rejected because it perpetuates the growing abstraction layer, increases chart complexity with every new application feature, and continues to require Helm chart changes for purely application-level concerns.
- **Option B — Big bang removal**: Remove all app config keys from values.yaml in a single release. Rejected because it constitutes a breaking change without a migration window, violating upgrade expectations for existing operators and providing no documented migration path.
- **Option C — Incremental deprecation with extraConfiguration as the standard (chosen)**: Freeze new additions immediately, deprecate existing keys in 8.10 with a migration path, and target removal in 8.11.

## Decision Outcome

`<component>.extraConfiguration` is adopted as the standard and only supported mechanism for passing application configuration through the Camunda Helm chart. It is an ordered list of file entries (each with a `file` name and `content` string, and an optional `springImport` flag) that are mounted individually into the container's config directory and loaded via Spring Boot's `spring.config.import` semantics — later entries override earlier ones for duplicate keys. Non-Spring files (e.g., Log4j2 XML) can be mounted without triggering Spring import by setting `springImport: false`. For Node.js and custom-loader components (Console, Optimize), entries are merged at template time into a single override file. See the [Camunda Helm application configuration docs](https://docs.camunda.io/docs/self-managed/deployment/helm/configure/application-configs/) for the full reference and examples.

The following rules apply from this decision forward:

- No new application configuration keys shall be added to the Helm chart. New application features that require configuration must be configured exclusively via `<component>.extraConfiguration`. This rule applies to chart version 8.10 and all subsequent versions; it does not apply retroactively to 8.7 or 8.8. Exceptions require explicit team consensus and must be justified by a Kubernetes-specific concern that cannot be expressed in static application configuration.
- All existing application-specific keys in values.yaml are deprecated as of Camunda 8.10, with documented migration hints mapping each deprecated key to the equivalent extraConfiguration pattern.
- Removal is targeted for Camunda 8.11. The keys in scope for removal are all values.yaml entries that configure application behavior rather than Kubernetes deployment concerns. Sequencing and any version-specific exceptions are tracked in product-hub#3562.
- `<component>.configuration` (full application config file override) remains available as an advanced escape hatch for operators who intentionally want to take full control of the application configuration file. It is not the recommended path and is not part of the standard mechanism.

### Positive Consequences

- Helm charts focus exclusively on Kubernetes infrastructure concerns: pods, services, secrets, networking, storage.
- Application configuration is consistent and portable across all deployment methods (Helm, ECS, Docker, Jar) — no deployment-method-specific configuration knowledge required.
- Application teams own their configuration surface and can introduce new settings without touching the Helm chart.
- Operators see fewer, clearer Helm values with an unambiguous pattern for application configuration.
- Upgrades reduce the risk that chart evolution silently overrides operator configuration, since operator-provided extraConfiguration entries are layered last.

### Negative Consequences

- Existing users relying on application-specific Helm values must migrate to extraConfiguration; the impact is mitigated by the deprecation cycle — keys are deprecated in 8.10 and removed no earlier than 8.11, with documented migration hints for each. Further investigations in a migration tool related to product-hub#3563 will be considered.
- Migration guidance and per-key mapping documentation is required before removal (owned by the Distribution team in coordination with application and docs teams).
- Shared configuration (e.g., TLS certificates, authentication settings, database connections) that was previously expressible once at a Helm global level must now be provided per component via extraConfiguration. Three factors mitigate this: (1) the 8.8 orchestration consolidation already eliminated the largest shared-config surface — Operate, Tasklist, and Zeebe are now a single deployment; (2) operators can reduce remaining repetition using YAML anchors in their values files; (3) operators already compose values from multiple `-f` files, so a dedicated `shared-auth.yaml` extra-values file is a natural pattern that factors out the repetition at the values layer without requiring chart-level abstraction.

## Links

- Builds on [ADR 0033 — Migrate application configuration from environment variables to ConfigMap-mounted files](0033-migrate-application-configuration-from-environment.md): established the ConfigMap-mounted config pattern this ADR standardizes around.
- Builds on [ADR 0061 — Introduce a unified configuration mechanism for Camunda Platform core components](0061-introduce-a-unified-configuration-mechanism-for-camunda.md): introduced the unified ConfigMap for the orchestration StatefulSet; this ADR extends that direction to the operator-facing values API.
- Coordinated via product-hub#3562 (removal scope and sequencing) and product-hub#3563 (migration tooling investigation).
- [Camunda Helm application configuration docs](https://docs.camunda.io/docs/self-managed/deployment/helm/configure/application-configs/)
