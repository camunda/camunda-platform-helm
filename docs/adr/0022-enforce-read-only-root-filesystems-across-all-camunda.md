# Enforce read-only root filesystems across all Camunda platform components

- Status: accepted
- Date: 2023-10-09
- Decision-makers: Jesse Simpson

## Context and Problem Statement

The Camunda 8 Self-Managed Helm chart deployed containers with writable root filesystems by default, which violates the principle of least privilege and fails common security benchmarks (CIS, Pod Security Standards). Organizations running Camunda in regulated environments or with enforced Pod Security Admission policies could not deploy without manual overrides. A platform-wide decision was needed to establish read-only root filesystems as the default security posture for all components.

## Decision Drivers

- **Security compliance**: Many production Kubernetes environments enforce restricted Pod Security Standards that require `readOnlyRootFilesystem: true`
- **Defense in depth**: Preventing filesystem writes at the container level limits the blast radius of container escapes or supply-chain compromises
- **Consistency across components**: A uniform security posture simplifies auditing and reduces per-component configuration drift
- **Deployment compatibility**: The chart must work out-of-the-box in hardened clusters without requiring users to patch security contexts

## Considered Options

- **Opt-in via values override** — Let users enable read-only root filesystems themselves. Rejected because it shifts security burden to operators and leaves the default posture weak.
- **Read-only for new components only** — Apply only to recently added services. Rejected because inconsistency creates confusion and leaves legacy components exposed.
- **Read-only with no writable volumes** — Rejected because several components (Zeebe, Operate, Optimize) require temporary or data directories at runtime; emptyDir volumes are necessary.

## Decision Outcome

All Camunda platform component containers (Identity, Operate, Optimize, Tasklist, Zeebe, Zeebe Gateway, Connectors, Web Modeler) now set `readOnlyRootFilesystem: true` in their security contexts by default. Where components require runtime write access (tmp directories, caches, data volumes), explicit emptyDir or persistent volume mounts are added at the required paths.

### Positive Consequences

- The chart passes restricted Pod Security Admission checks without user intervention, enabling deployment in hardened environments
- Reduces attack surface uniformly across all components — a compromised process cannot modify container binaries or inject persistent malware
- Establishes a security-first default that aligns with Kubernetes best practices and simplifies compliance documentation

### Negative Consequences

- Increases template complexity: each component must explicitly declare writable volume mounts for any path requiring write access, making future component additions slightly more involved
- Potential for subtle runtime failures if an application version introduces new write paths not covered by the mounted volumes, requiring ongoing maintenance as upstream applications evolve