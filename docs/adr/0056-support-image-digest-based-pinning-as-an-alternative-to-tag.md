# Support image digest-based pinning as an alternative to tag-based references across all Helm chart components

- Status: accepted
- Date: 2025-06-04
- Decision-makers: Daniel Rodriguez

## Context and Problem Statement

The Camunda Platform Helm charts exclusively supported `image.tag` for specifying container image versions. This prevented security-conscious organizations, air-gapped environments, and compliance-driven deployments from using immutable, content-addressable image digests (sha256 references) — a requirement for guaranteeing that exactly the audited image binary is deployed, regardless of tag mutability.

## Decision Drivers

- **Security and compliance:** Tag-based references are mutable; a compromised registry can serve different content under the same tag. Digest pinning eliminates this attack vector.
- **Backward compatibility:** Existing deployments using `image.tag` must continue working without modification.
- **Consistency across supported versions:** Customers on older chart versions (8.5–8.7) have the same compliance requirements as those on 8.8, so the capability cannot be limited to the latest release.
- **Maintainability of per-component architecture:** The existing pattern of per-component helper templates must be preserved to allow independent component-level overrides.

## Considered Options

- **Single global image helper:** Rejected because it would break the per-component override pattern that allows individual components to reference different registries or repositories.
- **Digest-only support without tag fallback:** Rejected because it would break backward compatibility for all existing deployments.
- **Implement only in the latest chart version (8.8):** Rejected because customers on supported older versions face identical compliance requirements and cannot upgrade chart versions independently of platform upgrades.
- **Validation of digest format in templates:** Not implemented — deferred to Kubernetes image pull validation to keep template logic simple and avoid maintaining regex patterns across versions.

## Decision Outcome

The image reference helper templates in every component's `_helpers.tpl` were extended with digest-aware logic: when `image.digest` is set, the rendered reference uses the `@sha256:...` format and takes precedence over `image.tag`. This was applied uniformly across chart versions 8.5 through 8.8, preserving the per-component template architecture while enabling digest pinning at any granularity (global or per-component).

### Positive Consequences

- Enables immutable image pinning for security-hardened and air-gapped deployments without requiring manual template overrides.
- Maintains full backward compatibility — existing `image.tag` configurations are unaffected.
- Consistent behavior across all supported chart versions reduces cognitive load for platform teams managing multiple environments at different versions.

### Negative Consequences

- The breadth of change (56 files across 4 chart versions) increases the ongoing maintenance burden; any future change to image reference logic must be replicated across all supported versions.
- Setting both `image.digest` and `image.tag` silently ignores the tag with no warning, which could confuse operators who expect both to be validated or who set digest unintentionally.