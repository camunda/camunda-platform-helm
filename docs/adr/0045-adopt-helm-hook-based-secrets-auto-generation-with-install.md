# Adopt Helm-hook-based secrets auto-generation with install-time immutability

- Status: accepted
- Date: 2024-09-02
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Helm chart required users to either manually provision secrets before installation or suffered from secrets being regenerated on every `helm upgrade`, causing credential rotation that broke running services. There was no stable default path for users who did not bring their own secret management solution, creating poor developer experience and operational fragility during upgrades.

## Decision Drivers

- **Upgrade stability** — secrets must remain immutable across `helm upgrade` to prevent auth failures and service disruption
- **Low-friction defaults** — new users should get a working deployment without pre-provisioning secrets externally
- **Incremental rollout** — the mechanism should be validated on the alpha channel before propagating to stable versions
- **Flexibility** — users who bring their own secrets (via external-secrets-operator, Vault, etc.) must not be blocked or conflicted

## Considered Options

- **Require users to always provide secrets externally** — rejected due to high operational burden and poor out-of-box experience for evaluation and development clusters
- **Regenerate secrets on every upgrade** — rejected because it causes authentication failures, breaks inter-component trust, and forces service restarts
- **Use a Kubernetes operator for secret lifecycle management** — rejected as too heavyweight a dependency for a Helm chart default; appropriate for advanced users but not a sensible baseline
- **Generate secrets via init containers at runtime** — rejected due to race conditions between components and lack of persistence guarantees

## Decision Outcome

Secrets are auto-generated using Helm hook-weighted templates that execute only during `helm install`, producing per-component Kubernetes Secrets that persist immutably across subsequent upgrades. Each component (Connectors, Console, Elasticsearch, OpenSearch, Operate, Optimize, Tasklist, Zeebe) receives its own secret template with customizable key names, and the feature is gated behind the alpha chart version for incremental validation.

### Positive Consequences

- Users get a functional deployment with zero secret pre-provisioning, dramatically improving time-to-first-deploy
- Upgrades become non-destructive by default — no credential rotation unless explicitly triggered
- Per-component secret isolation improves least-privilege posture and allows independent rotation

### Negative Consequences

- Helm hook-managed resources have different lifecycle semantics (not removed by `helm uninstall` by default), adding cognitive overhead for operators unfamiliar with hook behavior
- Per-version test customization via the new `vars/files` pattern increases CI maintenance surface as each chart version diverges