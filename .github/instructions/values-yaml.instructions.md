---
applyTo: "**/values*.yaml"
---

# Values YAML — Scoped Instructions

## Overview

Values files define the public API of each Helm chart. The primary file is `values.yaml`;
additional overlay files extend or override it for specific purposes. Changes to `values.yaml`
affect all users — treat every field as a public API. Every defined property must be documented
with a `@param` or `@extra` annotation so `helm-docs` can generate accurate documentation.
Use camelCase for all field names. Provide sensible defaults (empty strings for optional
secrets, `{}` or `[]` for optional collections, `true`/`false` for feature toggles). Never
remove or rename an existing field without a deprecation notice — this is a breaking change.

### Chart Design Principles

Every values change must respect the chart design principles in `docs/index.md` (canonical):
minimal & common, user-driven, generic extensibility (`extraConfiguration`/`extraEnv`/`extraVolumes`
over opinionated integrations), 1:1 mapping to application config, reasonable production defaults.

---

## Critical Rules

### NEVER
- **NEVER** add a field without a corresponding documentation comment (`## @param` or `## @extra`) —
  `helm-docs` silently omits undocumented fields.
- **NEVER** rename or remove an existing field without checking for backward compatibility — it
  breaks existing user `values.yaml` files.
- **NEVER** use `snake_case` or `PascalCase` for field names — always use `camelCase`.
- **NEVER** put secrets (passwords, tokens) as default values — use empty string `""` and document
  the `existingSecret` / `existingSecretKey` pattern instead.
- **NEVER** add non-empty defaults for `resources:` without consulting the resource guidelines —
  defaults must be appropriate for production and documented.
- **NEVER** edit `values-digest.yaml` or `values.schema.json` manually — both are generated
  (see `AGENTS.md` → Generated Artifacts); manual edits are overwritten.

### ALWAYS
- **ALWAYS** follow the `@param <path> <conjunction> <description>` comment format.
- **ALWAYS** use the correct conjunction: `defines` (mandatory), `can be used` (optional),
  `if true` (toggles), `configuration` (section headers).
- **ALWAYS** document the full dotted path in `@param` even for nested fields.
- **ALWAYS** put hand-written validation constraints in `values.schema.extra.json`, then run
  `make helm.schema-update` (also part of `make precommit.chores`) — the committed
  `values.schema.json` is generated from `values.yaml` merged with that extra file.
- **ALWAYS** run `make helm.lint` after editing values to catch schema violations.
- **ALWAYS** check whether the new field needs to be exposed in `values-local.yaml` for
  local development.

---

## Core Patterns with Code Examples

### 1. File Hierarchy

| File | Purpose |
|------|---------|
| `values.yaml` | Primary values — heavily documented, default everything |
| `values-latest.yaml` | Overrides image tags to latest upstream versions |
| `values-enterprise.yaml` | Enterprise feature overrides (extra components, licenses) |
| `values-local.yaml` | Local dev overrides (reduced replicas, disabled auth, etc.) |
| `values-bitnami-legacy.yaml` | Legacy Bitnami sub-chart compatibility |
| `values-digest.yaml` | Image digest pins — **generated, do not edit manually** |
| `values.schema.json` | JSON Schema for Helm validation |

### 2. File Header Documentation Convention

```yaml
# Default values for Camunda Helm chart.
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

# Variable names should begin with a lowercase letter, and words should be separated with camelCase.
# Every defined property in values.yaml should be documented.
# Pattern: # [VarName] [conjunction] [definition]
#
# Conjunctions:
#   [defines]       for mandatory configuration
#   [can be used]   for optional configuration
#   [if true]       for toggles
#   [configuration] for section/group of variables
```

### 3. Section and Parameter Comments

```yaml
## @section Global parameters
## @extra global

global:
  ## License configuration.
  ## @extra global.license
  license:
    ## @extra global.license.secret configuration to provide the license secret.
    secret:
      ## @param global.license.secret.inlineSecret can be used to provide the license as a plain-text value for non-production usage.
      inlineSecret: ""
      ## @param global.license.secret.existingSecret can be used to reference an existing Kubernetes Secret.
      existingSecret: ""
      ## @param global.license.secret.existingSecretKey defines the key within the existing secret object.
      existingSecretKey: ""

  ## @param global.annotations Annotations can be used to define common annotations applied to all deployments.
  annotations: {}

  ## @extra global.labels can be used to apply immutable labels applied to all Camunda resources.
  labels:
    ## @param global.labels.app Name of the application.
    app: camunda-platform
```

### 4. Toggle Pattern

```yaml
## @param orchestration.enabled if true, the Orchestration (Zeebe) component will be deployed.
enabled: true
```

### 5. Image Block Pattern

```yaml
image:
  ## @param orchestration.image.registry can be used to set the image registry.
  registry: ""
  ## @param orchestration.image.repository defines the image repository to use.
  repository: camunda/camunda
  ## @param orchestration.image.tag defines the image tag to use.
  tag: 8.10.0
  ## @param orchestration.image.digest can be used to set the image digest, overriding the tag.
  digest: ""
  ## @param orchestration.image.pullSecrets can be used to configure image pull secrets.
  pullSecrets: []
```

### 6. Secret Block Pattern

Secrets always use the **canonical `secret:` block** (`inlineSecret` → `existingSecret` →
`existingSecretKey`, fixed order); do not invent a new layout — ADR-0068 standardizes this
interface and the constraints template validates against it. Certificate / CA references use
the `existingSecret` / `existingSecretKey` subset (no inline form).

Canonical block — real example `identity.externalDatabase.secret`, shown with its parent nesting:

```yaml
identity:
  externalDatabase:
    ## @extra identity.externalDatabase.secret configuration to provide the external database password secret.
    secret:
      ## @param identity.externalDatabase.secret.inlineSecret can be used to provide the password as a plain-text value for non-production usage.
      inlineSecret: ""
      ## @param identity.externalDatabase.secret.existingSecret can be used to reference an existing Kubernetes Secret containing the password.
      existingSecret: ""
      ## @param identity.externalDatabase.secret.existingSecretKey defines the key within the existing secret object.
      existingSecretKey: ""
```

- **Key order is fixed:** `inlineSecret` → `existingSecret` → `existingSecretKey` (matches
  `global.license.secret` and `global.documentStore.type.aws.*.secret` in `values.yaml`).
- **Multiple secret values** (e.g. an access key ID *and* a secret access key) → one named entry
  per value, each wrapping its **own** canonical `secret:` block. Reference:
  `global.documentStore.type.aws.{accessKeyId,secretAccessKey}.secret` in `values.yaml`.
- **Auxiliary (non-secret) config goes BESIDE the `secret:` block, never inside it.** Only the
  secret-source keys belong in `secret:`; a key alias, a keystore/cert `type` selector, a
  `proxyVerify` or `autoRollout` toggle, etc. are SIBLINGS of `secret:`. Real precedent —
  `global.tls.caBundle`, where `autoRollout` (and `image`) sit beside `secret:`:

```yaml
global:
  tls:
    caBundle:
      ## @extra global.tls.caBundle.secret configuration to provide the CA bundle secret.
      secret:
        ## @param global.tls.caBundle.secret.existingSecret can be used to reference an existing Kubernetes Secret containing the PEM-encoded CA bundle.
        existingSecret: ""
        ## @param global.tls.caBundle.secret.existingSecretKey defines the key within the existing secret object.
        existingSecretKey: "ca.crt"
      ## @param global.tls.caBundle.autoRollout if true, rolls Java components when the CA Secret changes.
      autoRollout: false
```

### 7. Component Structure Pattern

```yaml
connectors:
  ## @param connectors.enabled if true, the Connectors component will be deployed.
  enabled: true
  ## @param connectors.replicas defines the number of replicas for the Connectors Deployment.
  replicas: 1

  ## @param connectors.configuration can be used to provide a custom application.yaml configuration.
  configuration: ""

  ## @param connectors.extraConfiguration can be used to add additional configuration files to the Connectors ConfigMap.
  extraConfiguration: {}

  ## @extra connectors.podAnnotations can be used to define extra pod annotations for the Connectors pods.
  podAnnotations: {}
  ## @extra connectors.podLabels can be used to define extra labels for the Connectors pods.
  podLabels: {}
```

### 8. values-local.yaml — Minimal Local Dev Overrides

```yaml
# Local development overrides — reduces resource consumption and disables features
# not needed for local testing.
global:
  elasticsearch:
    enabled: true
  identity:
    auth:
      enabled: false

identity:
  enabled: false

identityKeycloak:
  enabled: false

orchestration:
  replicas: 1
  clusterSize: "1"
  partitionCount: "1"
  replicationFactor: "1"
  pvcSize: 10Gi
```

### 9. Integration Test Values Layer Resolution

The `deploy-camunda` Go CLI resolves integration test values in a fixed order (last wins):
`base` → `base-qa`/`base-upgrade` modifiers → `identity` → `persistence` → `platform` →
`features` → migrator → `image-tags`. Layers live in
`test/integration/scenarios/chart-full-setup/values/` per chart version.

The canonical reference — full resolution-order table, per-version layer availability, and
scenario-name → config derivation — is `docs/skills/integration-test-scenario-resolution.md`.
When adding a new integration test values file, check which chart versions it applies to there.

---

## Common Mistakes

For anything restating a Critical Rule or Pattern above, see those sections. Additional pitfalls:

1. **Using `## @extra` when `## @param` is needed** — `@extra` is for section headers or groups
   that don't have a value themselves. For fields with actual values, use `@param`.

2. **Inconsistent conjunction** — using `defines` for an optional field or `can be used` for a
   required field confuses users. Match the conjunction to the actual behaviour.

3. **Missing `values-local.yaml` update** — if a new feature is enabled by default and is
   expensive to run locally (e.g., requires external dependencies), add an override to
   `values-local.yaml`.

4. **Breaking the integration test layer system** — when a `base.yaml` change enables a feature
   by default, verify all identity/persistence/feature layer files don't conflict. Consult
   `docs/skills/integration-test-scenario-resolution.md` to understand which layer files exist per version.

---

## Resources

- Helm values best practices: <https://helm.sh/docs/chart_best_practices/values/>
- helm-docs annotation format: <https://github.com/norwoodj/helm-docs>
- Chart design principles: `docs/index.md`
- Values YAML policy (canonical): `docs/policies/values-yaml-policy.md`
- Standardized secret interface: `docs/adr/0068-standardize-existingsecret-input-interface-across-all-helm.md`
- Breaking changes policy: `docs/policies/breaking-changes.md`
- Primary values file: `charts/<version>/values.yaml`
- Schema file: `charts/<version>/values.schema.json`
- Integration test values: `test/integration/scenarios/chart-full-setup/values/`
- Integration test layer resolution: `docs/skills/integration-test-scenario-resolution.md`
- Lint: `make helm.lint chartPath=charts/camunda-platform-8.10`
