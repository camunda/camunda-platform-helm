# Introduce Identity Migration as a Dedicated Helm Job Separate from the Identity Deployment

- Status: accepted
- Date: 2025-07-19
- Decision-makers: Hamza Masood

## Context and Problem Statement

The Camunda 8.8 platform requires identity data migration (Phase 1) as part of the evolution of the Identity service. Previously, migration logic would have been embedded within the Identity deployment itself, coupling schema/data migration lifecycle to the runtime service lifecycle. A dedicated migration mechanism was needed to ensure migrations run to completion before the Identity service starts serving traffic, without blocking rollbacks or complicating the deployment resource.

## Decision Drivers

- **Deployment independence**: Migrations must complete successfully before the Identity service starts, but should not block unrelated components or cause restart loops if they fail.
- **Idempotency and observability**: A Kubernetes Job provides built-in completion tracking, retry semantics, and clear success/failure signals that a container init-hook or deployment sidecar does not.
- **Separation of concerns**: Decoupling migration logic from the Identity runtime allows independent versioning, testing, and rollback of each concern.
- **Helm lifecycle ordering**: Jobs with appropriate hooks or weights can be sequenced before deployments within a single Helm release.

## Considered Options

- **Init container on the Identity Deployment** — rejected because a failing migration would cause the entire deployment to enter CrashLoopBackOff, obscuring the root cause and blocking the runtime pod from being inspectable.
- **Embedding migration in Identity application startup** — rejected because it couples migration execution to every pod replica start, risks concurrent migration runs, and makes rollback of the application independent of migration state impossible.
- **Standalone Job (chosen)** — provides clear lifecycle, single-execution semantics, and separation from the runtime deployment.

## Decision Outcome

Identity migration was introduced as a dedicated Kubernetes Job (`identity-migration/job.yaml`) with its own helpers, ConfigMap, and values-driven configuration. The Identity deployment and core helpers were updated to reference shared configuration, ensuring the migration Job and Identity Deployment agree on connection parameters without duplicating secrets or environment wiring.

### Positive Consequences

- Migration failures surface as failed Jobs with clear status, independent of the Identity runtime health.
- The Identity Deployment only starts once the migration is confirmed complete, preventing serving on an inconsistent schema.
- Future migration phases can extend the Job template without touching the Identity Deployment manifest.

### Negative Consequences

- Adds a new template surface area (helpers, ConfigMap, Job) that must be maintained in lockstep with Identity schema changes across chart versions.
- Operators must now monitor an additional Job resource during upgrades, increasing operational awareness requirements.