---
id: code-style
title: Code Style
---

## Deciding what to expose in `values.yaml`

Only expose parameters in the `values.yaml` that are essential for end users or platform operators to configure directly.

`values.yaml` entries typically fall into three main categories:

- **Kubernetes configuration**
  - Examples include service definitions, ingress configuration, persistence options, resource requests/limits, affinity and tolerations, pod labels/annotations, and autoscaling settings.
- **Operational app settings**
  - These cover database connections and credentials, administrative user accounts and passwords, hostnames or URLs, and log level controls.
- **Opinionated "sane defaults" for UX**
  - Adding an admin user or `firstUser` to Identity, default redirect URLs for Keycloak/OIDC authentication flows, and disabling multitenancy by default.

Anything that falls outside these groups must have clear user demand or product management approval. This ensures the chart interface remains minimal and meaningful.

:::note
Keep purely internal or experimental application options out of the `values.yaml`. They belong in application configuration.

For example, database credentials or connection settings may be exposed after explicit approval based on real user requirements, while feature toggles or minor exporter configurations remain purely application-level. Deep application-internal tuning options are not included in the groups mentioned above. Therefore, they must not be included in the Helm chart. Here is an [example issue](https://github.com/camunda/camunda/issues/37725) of application internals that do not belong in the `values.yaml`.
:::

## Configuration Mechanisms

Applications are configured through Helm using a combination of ConfigMaps, Secrets, and Environment Variables.

- **ConfigMaps:** Default location for component configuration. All non-sensitive application settings should reside here.
- **Environment Variables:** Used only as a temporary workaround and must be justified. Prefer declarative configuration through ConfigMaps.
- **Secrets:** All sensitive data (passwords, tokens, credentials) must follow the standard Kubernetes Secret design pattern.
  - Inline secret: defining the secret as a string literal.
  - Referenced secret: defining the referenced Kubernetes secret.
  - Secret key: defining the secret key inside the Kubernetes secret.

:::note
Please refer to this [example secret configuration in `values.yaml`](https://github.com/camunda/camunda-platform-helm/blob/b776066d71511e63e4f421977c9c59ae4507ae6e/charts/camunda-platform-8.8/values.yaml#L205-L211).
:::

## Global vs. Component-Specific Configuration

**Do not** introduce new global values unless absolutely necessary. All configuration should remain component-level by default. Components evolve independently across versions. Global keys can obscure version-specific behavior and create tightly coupled dependencies.

Global values are exceptions, allowed only when the following criteria are met:

| **Condition** | **Definition** | **Example** |
|---|---|---|
| **Shared semantic meaning** | The parameter represents the same conceptual feature or function across multiple components. | License key applied consistently to all components. |
| **Stable behavior across versions** | The parameter's meaning and format remain consistent across versions and components. | Common ingress configuration shared across services. |
| **Simplifies usage** | Adding a global parameter reduces configuration complexity, without hiding necessary component-level differences. | Shared document store settings used by multiple components. |

### Overwriting global values

Global overrides should generally be avoided. The preferred approach is explicit configuration at the component level, managed via Helm anchors to minimize duplication and maintain consistency. If it is necessary to overwrite the global value, then the component-level value should overwrite the global value.

## Helm Defaults vs. Application Defaults

The Helm chart should respect and propagate application-level defaults. The Helm chart should not override application defaults without strong justification.

Good practice:

- Allow upstream application defaults to handle standard behavior.
- Propose upstream changes through the `values.yaml` if defaults are insufficient (e.g., timeout values, resource limits) instead of overriding them locally in the template.

Here is the layered override chain, taking a Kubernetes ConfigMap as an example:

```
Helm values.yaml -> ConfigMap -> Pod Config -> Unzipped JAR -> application.yaml -> Application code
```

The Helm chart should only interact with exposed application defaults, leaving internal default logic to the owning application team.
