# Migrate Bitnami Helm chart dependencies from HTTP repository to OCI registry

- Status: accepted
- Date: 2024-09-17
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform Helm charts depend on several Bitnami sub-charts (PostgreSQL, Keycloak, common library, etc.) which were previously referenced via Bitnami's traditional HTTP Helm repository. Bitnami has been migrating toward OCI-based distribution as the canonical method for consuming their charts, and the legacy HTTP repository is being deprecated. This change needed to be applied consistently across all maintained chart versions (8.2, 8.3, 8.4, alpha, latest) and their nested sub-charts.

## Decision Drivers

- **Supply chain reliability**: OCI registries provide more robust artifact distribution with content-addressable storage and better caching semantics than HTTP index-based repositories
- **Ecosystem alignment**: Helm 3.8+ natively supports OCI registries, and Bitnami is deprecating the HTTP repository — staying on the old mechanism risks future breakage
- **Consistency across chart versions**: All maintained chart versions should use the same dependency resolution mechanism to reduce operational divergence
- **Maintainability**: A single dependency format across all charts simplifies CI/CD tooling and dependency update automation

## Considered Options

- **Keep HTTP repository references** — Rejected because Bitnami is actively deprecating this distribution channel, creating a ticking time bomb for dependency resolution failures
- **Pin vendored copies of Bitnami charts** — Rejected because it would increase maintenance burden and prevent receiving upstream security patches
- **Migrate to OCI registry references** — Selected as the forward-compatible, officially supported approach

## Decision Outcome

All Bitnami chart dependencies across every maintained Camunda Platform Helm chart version were migrated from `https://charts.bitnami.com/bitnami` repository references to `oci://registry-1.docker.io/bitnamicharts` OCI references in their respective `Chart.yaml` files. This applies to both top-level charts and nested sub-charts (e.g., the identity sub-chart's own Bitnami dependencies).

### Positive Consequences

- **Future-proofed dependency resolution**: Charts will continue to resolve dependencies as Bitnami completes their HTTP repository deprecation
- **Improved artifact integrity**: OCI distribution provides content-addressable layers and cryptographic verification out of the box
- **Uniform dependency mechanism**: All chart versions now use the same approach, simplifying CI tooling and `helm dependency update` workflows

### Negative Consequences

- **Requires Helm 3.8+**: Any consumer or CI environment using older Helm versions will be unable to resolve OCI-based dependencies, raising the minimum tooling requirement
- **Broad cross-version change surface**: Touching 11 files across 5+ chart versions in a single refactor increases the risk of subtle version-specific regressions