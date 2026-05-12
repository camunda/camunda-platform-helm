# Separate Tasklist and Operate importer into its own Kubernetes deployment

- Status: accepted
- Date: 2025-09-05
- Decision-makers: Hamza Masood

## Context and Problem Statement

The Camunda 8 orchestration component bundles multiple responsibilities including the importers for Tasklist and Operate. Running importers within the same deployment as the core orchestration engine creates resource contention and scaling constraints — the importer workload has different resource profiles and availability requirements than the main orchestration process. Separating the importer into its own deployment allows independent scaling and lifecycle management.

## Decision Drivers

- **Independent scalability** — importers have variable load patterns distinct from the orchestration engine and need to scale independently
- **Fault isolation** — importer failures or resource exhaustion should not impact the core orchestration deployment's availability
- **Deployment flexibility** — operators need the ability to tune resources, replicas, and restart policies for importers without affecting the main engine
- **Operational clarity** — separate deployments provide clearer monitoring boundaries and easier debugging

## Considered Options

- **Keep importers embedded in the orchestration deployment** — rejected because it couples unrelated scaling concerns and makes resource tuning a compromise between two workloads
- **Sidecar container within the same pod** — rejected because it still shares pod-level resource limits and restart behavior, offering only partial isolation
- **Separate Helm subchart** — rejected as over-engineered for what is fundamentally the same application binary with different configuration flags

## Decision Outcome

The Tasklist and Operate importer is deployed as a separate Kubernetes Deployment resource with its own ConfigMap, managed within the existing orchestration chart templates. The importer shares the same container image as the orchestration component but receives distinct configuration via a dedicated configmap, allowing it to run only the importer functionality.

### Positive Consequences

- Importers can be scaled horizontally without over-provisioning the orchestration engine
- Resource limits and requests can be tuned independently, improving cluster utilization
- Rolling updates to importer configuration do not trigger restarts of the core orchestration pods

### Negative Consequences

- Increased operational surface area — operators must now monitor and manage an additional deployment
- Configuration drift risk between the orchestration and importer configmaps requires careful values management