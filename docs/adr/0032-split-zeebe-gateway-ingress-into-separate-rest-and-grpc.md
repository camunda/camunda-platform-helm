# Split Zeebe Gateway ingress into separate REST and gRPC resources to support multi-protocol exposure

- Status: accepted
- Date: 2024-03-21
- Decision-makers: Nicolas Pepin-Perreault

## Context and Problem Statement

Zeebe Gateway historically exposed only a gRPC API through a single Kubernetes ingress resource. With the introduction of a REST API on the gateway, the chart needed to serve two fundamentally different protocols (HTTP/2 for gRPC, HTTP/1.1 for REST) that require incompatible ingress annotations, TLS configurations, and load balancer behaviors. A single ingress resource cannot correctly express protocol-specific backend annotations for both gRPC and REST simultaneously.

## Decision Drivers

- **Protocol incompatibility at the ingress layer**: gRPC requires HTTP/2 backend annotations (e.g., `nginx.ingress.kubernetes.io/backend-protocol: "GRPC"`), which break standard REST traffic if applied to the same ingress resource.
- **Independent configurability**: Operations teams need to assign different hostnames, TLS certificates, and rate-limiting policies to the REST and gRPC endpoints independently.
- **Forward-looking architecture**: Connectors and other components are migrating toward REST-first communication with Zeebe, requiring a first-class REST endpoint in the chart's topology.
- **Cloud load balancer correctness**: Major cloud providers (GKE, EKS) configure L7 balancers per-ingress; mixing protocols in one resource produces undefined behavior.

## Considered Options

- **Single ingress with path-based routing (prior state)**: Simpler to maintain but cannot accommodate protocol-specific annotations. Rejected because gRPC backend annotations applied globally would corrupt REST routing and vice versa.
- **Single ingress with annotation merging or conditional logic**: Helm template conditionals could toggle annotations, but this creates fragile, hard-to-test combinations and still cannot express two different backend protocols on one ingress resource in most controllers.
- **Two dedicated ingress resources (chosen)**: Each protocol gets its own resource with independent annotations, hosts, and TLS configuration, matching the Kubernetes ingress model's design intent.

## Decision Outcome

The `zeebeGateway.ingress` values key was restructured into `zeebeGateway.ingress.rest` and `zeebeGateway.ingress.grpc` sub-keys, each controlling a dedicated ingress resource rendered from its own template (`ingress-rest.yaml`, `ingress-grpc.yaml`). The deployment and service templates were extended to expose the REST port, and helper templates were updated so that Connectors and other internal consumers resolve the correct REST endpoint.

### Positive Consequences

- Each protocol has fully independent ingress configuration (annotations, TLS, hostname), enabling correct cloud load balancer behavior without workarounds.
- Connectors can now reference the Zeebe REST endpoint natively, eliminating the need for manual service URL overrides when REST-based communication is adopted.
- The chart's ingress model now scales cleanly if additional protocols or API versions are added to the gateway in the future.

### Negative Consequences

- **Breaking change for existing users**: The values schema migration from `zeebeGateway.ingress.*` to `.rest`/`.grpc` sub-keys requires all consumers to update their values files, increasing upgrade friction.
- **Increased template surface area**: Two ingress templates, expanded test golden files (87 files changed), and more complex helper logic increase the maintenance and review burden for chart contributors.