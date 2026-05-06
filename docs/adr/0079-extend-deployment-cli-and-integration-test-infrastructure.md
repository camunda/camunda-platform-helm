# Extend deployment CLI and integration test infrastructure to support EKS as a first-class platform

- Status: accepted
- Date: 2026-02-11
- Decision-makers: Eamonn Moloney

## Context and Problem Statement

The `deploy-camunda` CLI and integration test infrastructure were designed around a single Kubernetes platform (GKE), with platform-specific configuration (TLS, ingress, service monitors, secrets) embedded in a flat directory structure. As Camunda 8 Self-Managed adoption grew on AWS, EKS-specific deployment issues were only discovered post-release because CI validation ran exclusively on GKE. The architecture needed to accommodate multiple cloud platforms without sacrificing clarity or independent evolution of platform-specific concerns.

## Decision Drivers

- **Customer risk reduction** — EKS is a primary deployment target for customers, yet had zero pre-release integration test coverage
- **Maintainability over cleverness** — Platform differences (ingress controllers, TLS provisioning, cleanup hooks) are significant enough to warrant explicit separation rather than runtime conditionals
- **Single deployment interface** — Teams should not need to learn separate tools per cloud provider; one CLI with platform awareness reduces cognitive overhead
- **Cross-version consistency** — All supported chart versions (8.6–8.9) must have equivalent multi-cloud coverage to prevent version-specific regressions

## Considered Options

- **Single platform-agnostic configuration with runtime conditionals** — Rejected because platform differences in TLS, DNS, and cleanup are substantial enough that conditional logic would obscure intent and complicate debugging
- **EKS support as a separate tool or workflow** — Rejected to avoid fragmenting the deployment interface and duplicating orchestration logic across tools
- **Support only on latest chart version, backport later** — Rejected because customers run older versions on EKS today, and delayed coverage leaves existing releases exposed

## Decision Outcome

Platform-specific integration test configurations were refactored from a flat structure into explicit `common/gke/` and `common/eks/` subdirectories, establishing a filesystem convention for multi-cloud support. The `deploy-camunda` CLI was extended to resolve platform-appropriate configurations (TLS, service monitors, cleanup, base-layer) based on the target cluster type. External secret definitions were updated across all supported chart versions to include EKS credential paths.

### Positive Consequences

- **Reduced customer risk** — EKS deployment issues are now caught in CI before release, closing the largest gap in pre-release validation coverage
- **Clear platform boundaries** — The `common/{platform}/` convention makes platform-specific concerns discoverable and independently evolvable without merge conflicts
- **Extensible pattern** — Adding a third platform (AKS, etc.) follows the established directory convention with no architectural changes required

### Negative Consequences

- **Increased maintenance surface** — Every new platform multiplies scenario configuration files; 71 files changed in this single commit, and future platform additions will have similar breadth
- **Structural duplication** — GKE and EKS common directories will share conceptual overlap (both need TLS, service monitors) that could drift, requiring periodic alignment reviews