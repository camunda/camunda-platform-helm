# Introduce a unified configuration mechanism for Camunda Platform core components

- Status: accepted
- Date: 2025-07-29
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

Camunda Platform's core components (Zeebe, Operate, Tasklist, etc.) each maintained separate configuration files and ConfigMaps within the Helm chart, leading to configuration drift, duplication, and difficulty reasoning about cross-component settings. As the platform converged toward a unified orchestration model in 8.8+, a single configuration surface was needed to reduce operational complexity and ensure consistency across components sharing a StatefulSet.

## Decision Drivers

- **Maintainability**: Reducing duplicated configuration blocks across multiple templates that had to be kept in sync manually
- **Operational simplicity**: Providing a single ConfigMap as the source of truth for core component configuration rather than scattered per-component files
- **Alignment with unified deployment model**: The move toward a single StatefulSet for core components required a matching unified configuration approach
- **Schema-driven validation**: Enabling values.schema.json to enforce constraints on the unified config surface

## Considered Options

- **Per-component ConfigMaps with shared helpers**: Retaining separate ConfigMaps but extracting common values into helper templates. Rejected because it preserves the multi-resource sprawl and makes it harder to reason about the final merged configuration.
- **Kustomize overlays or post-render patching**: Rejected due to added toolchain complexity and poor discoverability for operators using standard `helm template` workflows.

## Decision Outcome

A unified ConfigMap (`configmap-unified.yaml`) and corresponding application configuration file (`_application-unified.yaml`) were introduced for the core StatefulSet. The Helm helpers in both `camunda/_helpers.tpl` and `core/_helpers.tpl` were refactored to feed into this single configuration resource, and the StatefulSet was updated to mount it. The values schema was extended to validate the unified configuration structure.

### Positive Consequences

- Single source of truth for core component configuration eliminates cross-component drift
- Simplified StatefulSet definition with one ConfigMap mount instead of multiple
- Easier onboarding and debugging — operators inspect one resource to understand runtime config

### Negative Consequences

- Breaking change for users who previously customized individual per-component ConfigMap values; migration guidance is required
- Initial implementation complexity in the helper templates to merge previously independent configuration paths into a coherent unified structure