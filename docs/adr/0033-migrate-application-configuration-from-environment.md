# Migrate application configuration from environment variables to ConfigMap-mounted files

- Status: accepted
- Date: 2024-04-02
- Decision-makers: Jesse Simpson

## Context and Problem Statement

Camunda 8 Self-Managed components rely on complex, nested configuration (Spring Boot YAML, log4j2.xml, TOML for Web Modeler) that environment variables cannot adequately express. Operators needed full control over application behavior — including logging configuration and multi-line structured settings — without building custom images or forking the Helm chart. The existing env-var-based approach forced flattening of hierarchical config into `SPRING_APPLICATION_JSON` or dot-notation variables, which was fragile, hard to review, and incompatible with Spring Boot's native config file precedence model.

## Decision Drivers

- **Expressiveness:** Environment variables cannot represent nested YAML structures, multi-document configs, or non-Spring formats (log4j2.xml, TOML) without brittle encoding workarounds.
- **Operator autonomy:** Operators should be able to supply arbitrary application configuration via Helm values without understanding chart internals or submitting upstream PRs.
- **Alignment with Spring Boot conventions:** Spring Boot's config file precedence (`application.yaml` in specific directories) is a well-understood model that ConfigMap-mounted files directly leverage.
- **Uniform pattern across components:** All components should follow the same configuration injection mechanism to reduce cognitive load for operators managing the full platform.

## Considered Options

- **Keep environment variables as the sole configuration mechanism:** Rejected because it cannot express complex nested config, makes logging overrides impossible without custom images, and diverges from how Spring Boot applications are typically configured in production.
- **Migrate all configuration including sensitive credentials to ConfigMaps:** Attempted but partially reverted — Identity client credentials with conditional logic caused authentication failures when moved from env vars to file-based config. The interaction between Helm templating conditionals and Spring property resolution proved unreliable for these specific values.
- **Incremental per-component migration:** Rejected in favor of a single coordinated change to avoid a prolonged period of inconsistency where operators would need to understand two different configuration patterns depending on the component.

## Decision Outcome

All Camunda 8 components received dedicated ConfigMap templates that hold their application configuration as mounted files. Deployments were updated with volume mounts and checksum annotations (to trigger rollouts on config changes). Identity retains a hybrid approach with a separate `configmap-env-vars.yaml` for sensitive client credentials that proved problematic in pure file-based config.

### Positive Consequences

- Operators can now override any application-level setting (logging, Spring profiles, feature flags) through Helm values without chart modifications or custom images.
- Configuration is auditable and diffable as structured files rather than flattened env var lists, improving change review quality.
- The uniform pattern across all components reduces onboarding time and enables shared documentation for configuration customization.

### Negative Consequences

- Template complexity increased significantly — every deployment now carries volume mounts, ConfigMap references, and checksum annotations, raising the maintenance burden for chart contributors.
- The hybrid approach for Identity (env vars for credentials, ConfigMap for application config) creates an inconsistency that operators must understand, and may confuse users who expect uniform behavior across components.