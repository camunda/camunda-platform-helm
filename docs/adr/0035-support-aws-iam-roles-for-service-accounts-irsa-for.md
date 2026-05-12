# Support AWS IAM Roles for Service Accounts (IRSA) for OpenSearch authentication

- Status: accepted
- Date: 2024-04-04
- Decision-makers: Hamza Masood

## Context and Problem Statement

Camunda platform components (Operate, Optimize, Tasklist, Zeebe) connecting to OpenSearch required static username/password credentials configured via Helm values. For AWS-hosted deployments using Amazon OpenSearch Service, this forced operators to manage long-lived credentials rather than leveraging AWS's native identity federation through IAM Roles for Service Accounts (IRSA), which provides automatic credential rotation and eliminates secret management overhead.

## Decision Drivers

- **Cloud-native security posture**: Eliminating static credentials in favor of short-lived, automatically rotated AWS STS tokens reduces attack surface
- **Operational simplicity on AWS**: IRSA removes the need to provision, rotate, and distribute OpenSearch credentials across multiple components
- **Conditional compatibility**: The solution must remain backward-compatible — clusters using username/password authentication must continue to work without modification

## Considered Options

- **Static credentials only (status quo)** — Rejected because it forces AWS customers into an inferior security model and increases operational burden for secret rotation
- **External secrets operator integration** — Rejected as it adds an external dependency and still relies on static secrets at rest, merely automating their injection
- **AWS IRSA with conditional credential rendering** — Selected; leverages Kubernetes-native annotation-based identity without additional tooling

## Decision Outcome

AWS IRSA support was added as a configurable option across all components that connect to OpenSearch. When IRSA is enabled, the templates conditionally omit username/password environment variables and instead rely on the AWS SDK credential chain via annotated service accounts. A bug in the existing OpenSearch configuration rendering was also corrected as part of this change.

### Positive Consequences

- Components deployed on EKS can authenticate to Amazon OpenSearch Service without any stored secrets, aligning with AWS security best practices
- The conditional templating approach means zero impact on existing non-AWS deployments
- Uniform IRSA configuration across Operate, Tasklist, Zeebe, and Optimize ensures consistent authentication behavior

### Negative Consequences

- Increased conditional complexity in Helm templates — each component's configmap and deployment now branches on IRSA enablement, making template logic harder to reason about
- Tighter implicit coupling to AWS SDK behavior — debugging authentication failures requires understanding both Kubernetes service account annotations and AWS STS token exchange mechanics