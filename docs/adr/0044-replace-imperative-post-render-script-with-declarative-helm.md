# Replace imperative post-render script with declarative Helm values overlay for OpenShift compatibility

- Status: accepted
- Date: 2024-08-08
- Decision-makers: Hamza Masood

## Context and Problem Statement

OpenShift deployments required restrictive Security Context Constraints (SCCs) that were applied via a post-render shell script (`openshift/patch.sh`). Post-render scripts operate outside Helm's declarative model — they mutate rendered manifests imperatively, are invisible to `helm template` output, difficult to test with standard Helm tooling, and introduce fragile shell execution into the deployment path. The team needed a mechanism for platform-specific configuration that remained within Helm's native capabilities.

## Decision Drivers

- **Maintainability:** Post-render scripts are opaque to standard Helm linting, testing, and template rendering workflows, making them expensive to maintain and debug.
- **Declarative consistency:** All other environment-specific configurations were already managed through values overlays; OpenShift was the sole exception requiring imperative intervention.
- **CI reliability:** Shell script execution during Helm operations introduced a class of failures (path issues, permission errors, script drift) that values-based configuration eliminates entirely.
- **Auditability:** Security-sensitive SCC configurations should be visible in standard `helm template` output for review and policy enforcement.

## Considered Options

- **Keep the post-render script** — rejected due to ongoing maintenance burden, testing difficulty, and incompatibility with declarative GitOps workflows.
- **Kustomize overlays for OpenShift patches** — rejected because it introduces an additional tool dependency when Helm values files are sufficient and native to the existing toolchain.
- **Upstream chart changes to expose SCC knobs as first-class values** — this was effectively accomplished, making the existing `securityContext` and `podSecurityContext` parameters sufficient for OpenShift without post-processing.

## Decision Outcome

OpenShift compatibility is now achieved entirely through a declarative `openshift/values.yaml` file that configures restrictive SCCs via standard Helm values overrides. The post-render script was removed, CI workflows were updated to pass the values file via `-f`, and a vendored `web-modeler-postgresql-14` subchart was added to provide an OpenShift-compatible PostgreSQL configuration with proper security contexts.

### Positive Consequences

- OpenShift configuration is now visible in `helm template` output, enabling standard review, diff, and policy-as-code workflows without executing external scripts.
- CI pipelines are simplified — no shell script invocation, no path dependencies, no script-specific failure modes.
- The deployment model is fully declarative and consistent across all target platforms, improving onboarding and reducing operational surprises.

### Negative Consequences

- The vendored `web-modeler-postgresql-14` subchart adds 50+ files to the repository, increasing the surface area for dependency updates and version drift.
- Any future OpenShift-specific patches that cannot be expressed as Helm values overrides will require introducing a new mechanism, as the post-renderer escape hatch has been removed.