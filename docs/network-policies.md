---
id: network-policies
title: Network policies
---

# Network policies

The 8.11 chart (`charts/camunda-platform-8.11`) can render one Kubernetes
`NetworkPolicy` per Camunda component, with ingress rules, egress rules or both. The
feature is opt-in: with the default values the chart renders no `NetworkPolicy`, and
egress rules stay off until `global.networkPolicy.egress.enabled` is set.

Policies are enforced only when the cluster CNI supports them (for example Calico or
Cilium).

## Values

| Key | Default | Description |
|---|---|---|
| `global.networkPolicy.enabled` | `false` | Master switch. |
| `global.networkPolicy.ingress.enabled` | `true` | Renders the ingress policies when the master switch is on. |
| `global.networkPolicy.ingress.extraRules` | `[]` | `NetworkPolicyIngressRule` entries appended to every rendered policy. Rendered with `tpl`. |
| `global.networkPolicy.ingress.allowedNamespaces` | `[]` | Namespaces allowed to reach the client ports of every component. See [Allowing other namespaces](#allowing-other-namespaces). |
| `<component>.networkPolicy.ingress.allowedNamespaces` | unset | Replaces the global list for that component. Unset inherits the global list; `[]` allows no other namespace. |
| `global.networkPolicy.egress.enabled` | `false` | Adds egress rules to the rendered policies. |
| `global.networkPolicy.egress.allowDNS` | `true` | Allows egress to `kube-dns` in `kube-system` on port 53, UDP and TCP. |
| `global.networkPolicy.egress.extraRules` | `[]` | `NetworkPolicyEgressRule` entries appended to every policy. Rendered with `tpl`. |
| `global.networkPolicy.egress.allowedNamespaces` | `[]` | Namespaces every component may send traffic to. See [Egress to other namespaces](#egress-to-other-namespaces). |
| `<component>.networkPolicy.egress.allowedNamespaces` | unset | Replaces the global egress list for that component. Unset inherits the global list; `[]` allows no other namespace. |
| `<component>.networkPolicy.egress.enabled` | `true` | Per-component egress toggle. |
| `<component>.networkPolicy.egress.extraRules` | `[]` | `NetworkPolicyEgressRule` entries appended to that component only, after the global ones. Rendered with `tpl`. |
| `orchestration.networkPolicy.enabled` | `true` | Per-component toggle. |
| `connectors.networkPolicy.enabled` | `true` | Per-component toggle. |
| `identity.networkPolicy.enabled` | `true` | Per-component toggle. |
| `optimize.networkPolicy.enabled` | `true` | Per-component toggle. |
| `camundaHub.restapi.networkPolicy.enabled` | `true` | Per-component toggle. |
| `camundaHub.websockets.networkPolicy.enabled` | `true` | Per-component toggle. |

`<component>` is `orchestration`, `connectors`, `identity`, `optimize`,
`camundaHub.restapi` or `camundaHub.websockets`.

A policy is rendered for a component only when the component itself is enabled. Each
component keeps one policy; its `policyTypes` lists `Ingress` when the ingress rules are
on and `Egress` when both `global.networkPolicy.egress.enabled` and the component
`networkPolicy.egress.enabled` are on. With ingress off and egress off for a component, no
policy is rendered for it.

## Rendered policies

Each policy selects the component pods by their match labels and allows ingress from
any pod in the release namespace on the pod ports:

| Component | Template | Ports |
|---|---|---|
| Orchestration Cluster | `templates/orchestration/networkpolicy.yaml` | `orchestration.service.{httpPort,commandPort,internalPort,managementPort,grpcPort}` |
| Connectors | `templates/connectors/networkpolicy.yaml` | `connectors.service.serverPort` |
| Identity | `templates/identity/networkpolicy.yaml` | 8084, 8082 |
| Optimize | `templates/optimize/networkpolicy.yaml` | 8090, 8092 |
| Camunda Hub REST API | `templates/web-modeler/networkpolicy-restapi.yaml` | 8081, 8091 |
| Camunda Hub WebSockets | `templates/web-modeler/networkpolicy-websockets.yaml` | 8060 |

Traffic from outside the namespace, such as an ingress controller or Prometheus, is
blocked unless it is allowed through `allowedNamespaces` or `extraRules`:

```yaml
global:
  networkPolicy:
    enabled: true
    ingress:
      extraRules:
        - from:
            - namespaceSelector:
                matchLabels:
                  kubernetes.io/metadata.name: ingress-nginx
```

## Allowing other namespaces

`allowedNamespaces` lets pods in other namespaces reach a component without writing
`NetworkPolicyIngressRule` entries by hand. Each entry renders one ingress rule, placed
after the release-namespace rule and before `global.networkPolicy.ingress.extraRules`.
Rules are additive, so the result is the union of the release namespace, the listed
namespaces and the `extraRules`.

An allowed namespace reaches only the component client ports, not the cluster-internal
or management ports:

| Component | Default client ports |
|---|---|
| Orchestration Cluster | `orchestration.service.httpPort` (8080), `orchestration.service.grpcPort` (26500) |
| Connectors | `connectors.service.serverPort` (8080) |
| Identity | 8084 |
| Optimize | 8090 |
| Camunda Hub REST API | 8081 |
| Camunda Hub WebSockets | 8060 |

The broker ports 26501 and 26502 and the management ports (9600, 8082, 8092, 8091) are
never opened by default. Open a management port only through an entry `ports` list.

### Entry forms

Each entry is a string or an object:

```yaml
global:
  networkPolicy:
    enabled: true
    ingress:
      allowedNamespaces:
        # Namespace name, matched on kubernetes.io/metadata.name. Rendered with tpl.
        - camunda-hub
        - "{{ .Release.Namespace }}-clients"
        # Object with name: the same as a string, plus optional podSelector and ports.
        - name: monitoring
          podSelector:
            matchLabels:
              app.kubernetes.io/name: prometheus
          ports:
            - protocol: TCP
              port: 9600
        # Object with namespaceSelector: any namespace carrying the labels.
        - namespaceSelector:
            matchLabels:
              camunda.io/tenant-of: "{{ .Release.Namespace }}"
```

- An object sets exactly one of `name` or `namespaceSelector`; the chart fails otherwise.
- An empty `namespaceSelector` (`{}`) would match every namespace and fails the render.
- `podSelector` is set in the same peer as the namespace selector, so both must match.
- `ports` replaces the default client ports for that entry and is passed through as
  `NetworkPolicyPort` entries, including `endPort`.
- Strings and object values are rendered with `tpl`.

### Inherit or replace

A component without `networkPolicy.ingress.allowedNamespaces` uses the global list. A
component that sets the key, including to `[]`, replaces the global list for its own
policy; the lists are not merged. `[]` narrows that component to the release namespace
and its `extraRules`:

```yaml
global:
  networkPolicy:
    enabled: true
    ingress:
      allowedNamespaces:
        - camunda-hub
connectors:
  networkPolicy:
    ingress:
      allowedNamespaces: []
```

## Split-namespace layouts

### Hub and Orchestration Cluster in different namespaces

With `global.topology.mode` splitting Camunda Hub and the Orchestration Cluster across
namespaces, the receiving release allows the calling namespace. On the Orchestration
Cluster release:

```yaml
global:
  networkPolicy:
    enabled: true
    ingress:
      allowedNamespaces:
        - camunda-hub
```

The Hub components then reach the Orchestration Cluster on 8080 and 26500 only.

With egress on, the Hub release also needs the egress half of the pair, and the
Orchestration Cluster release needs it for any call back into the Hub namespace. On the
Hub release:

```yaml
global:
  networkPolicy:
    enabled: true
    egress:
      enabled: true
      allowedNamespaces:
        - name: camunda-orchestration
          ports:
            - protocol: TCP
              port: 8080
            - protocol: TCP
              port: 26500
```

Without `ports`, the egress entry allows every port in that namespace; the ingress
policy of the Orchestration Cluster still limits the Hub to 8080 and 26500.

### Monitoring

Prometheus scrapes the management port, which is not a client port. Allow it explicitly
on the component that is scraped:

```yaml
orchestration:
  networkPolicy:
    ingress:
      allowedNamespaces:
        - name: monitoring
          podSelector:
            matchLabels:
              app.kubernetes.io/name: prometheus
          ports:
            - protocol: TCP
              port: 9600
```

Setting the component list replaces the global one, so repeat any global entries that
the Orchestration Cluster still needs.

### Ingress controller

```yaml
global:
  networkPolicy:
    ingress:
      allowedNamespaces: [ingress-nginx]
```

### Tenant namespaces

Label each namespace that belongs to a release and select it by label. Labelling or
unlabelling a namespace takes effect without a `helm upgrade`:

```yaml
global:
  networkPolicy:
    ingress:
      allowedNamespaces:
        - namespaceSelector:
            matchLabels:
              camunda.io/tenant-of: "{{ .Release.Namespace }}"
```

```bash
kubectl label namespace tenant-a camunda.io/tenant-of=camunda
```

### Connectors in another namespace

A Connectors release in `camunda-connectors` calling an Orchestration Cluster in
`camunda` needs both directions. On the Orchestration Cluster release, the ingress half,
plus the egress half when Connectors exposes inbound webhooks the cluster calls:

```yaml
orchestration:
  networkPolicy:
    ingress:
      allowedNamespaces: [camunda-connectors]
    egress:
      allowedNamespaces: [camunda-connectors]
```

On the Connectors release, typically with `orchestration.enabled: false`:

```yaml
global:
  networkPolicy:
    enabled: true
    ingress:
      allowedNamespaces: [camunda]
    egress:
      enabled: true
      allowedNamespaces:
        - name: camunda
          ports:
            - protocol: TCP
              port: 8080
            - protocol: TCP
              port: 26500
```

### Database or Elasticsearch in its own namespace

An in-cluster datastore, such as an ECK Elasticsearch cluster or a CloudNativePG
PostgreSQL cluster, is reached through egress `allowedNamespaces`. Narrow each entry to
the datastore pods and port:

```yaml
global:
  networkPolicy:
    egress:
      allowedNamespaces:
        - name: elastic-system
          podSelector:
            matchLabels:
              elasticsearch.k8s.elastic.co/cluster-name: camunda
          ports:
            - protocol: TCP
              port: 9200
        - name: cnpg-system
          podSelector:
            matchLabels:
              cnpg.io/cluster: camunda-db
          ports:
            - protocol: TCP
              port: 5432
```

The datastore namespace must also allow the Camunda namespace in its own ingress
policies, if it has any. A managed service outside the cluster, such as Amazon RDS or
Elastic Cloud, has no namespace: add it with an `ipBlock` in
`global.networkPolicy.egress.extraRules`.

### Multi-tenant Optimize

With one Optimize release per tenant, each Optimize release reaches the shared
Elasticsearch on 9200 and the Orchestration Cluster on 8080:

```yaml
optimize:
  networkPolicy:
    egress:
      allowedNamespaces:
        - name: elastic-system
          ports:
            - protocol: TCP
              port: 9200
        - name: camunda
          ports:
            - protocol: TCP
              port: 8080
```

The Orchestration Cluster release allows the tenant namespaces with the selector form,
so a new tenant only needs the namespace label:

```yaml
orchestration:
  networkPolicy:
    ingress:
      allowedNamespaces:
        - namespaceSelector:
            matchLabels:
              camunda.io/tenant-of: "{{ .Release.Namespace }}"
          ports:
            - protocol: TCP
              port: 8080
```

Never open the broker ports 26501 and 26502 to a tenant namespace.

### Upgrade order

When a release starts calling another, upgrade the receiving release first, so its
ingress policy allows the caller before the caller starts sending traffic, then turn
egress on last, on the calling release. With Argo CD, put the receiving release in an
earlier sync wave. Sync waves order resources inside one Application only, so when the
two releases are separate Applications there is no ordering between them: expect
brief denials until both have synced.

### Migrating extraRules to allowedNamespaces

An ingress rule that selects a namespace by name:

```yaml
global:
  networkPolicy:
    ingress:
      extraRules:
        - from:
            - namespaceSelector:
                matchLabels:
                  kubernetes.io/metadata.name: camunda-hub
```

becomes:

```yaml
global:
  networkPolicy:
    ingress:
      allowedNamespaces:
        - camunda-hub
```

The `extraRules` form without `ports` opens every port to that namespace; the
`allowedNamespaces` form opens only the client ports. Add a `ports` list to the entry
when the old rule relied on another port. Remove the `extraRules` entry in the same
upgrade; keeping both is harmless but leaves the wider rule in place.

## Egress

With egress on, each policy allows:

1. Any pod in the release namespace (`podSelector: {}`).
2. With `allowDNS`, the `kube-dns` pods (`k8s-app: kube-dns`) in `kube-system` on port 53
   over UDP and TCP. Clusters whose DNS uses other labels or another namespace set
   `allowDNS: false` and add their own rule.
3. The resolved egress `allowedNamespaces` list, one rule per entry.
4. `global.networkPolicy.egress.extraRules`, then the component `extraRules`.

Everything else is blocked. The chart does not generate `ipBlock` rules for the
external endpoints it is configured with: a `NetworkPolicy` matches pods, namespaces and
CIDRs, not hostnames, and the IP behind a hostname such as a managed database or an
OIDC provider can change. Add those destinations through `extraRules`, or in-cluster
namespaces through `allowedNamespaces`. When egress is on with an external dependency
and no egress `extraRules` or egress `allowedNamespaces` on any component, `helm install`
prints a warning.

| Component | Egress outside the release namespace |
|---|---|
| Orchestration Cluster | Secondary storage (Elasticsearch, OpenSearch or RDBMS), OIDC provider, document store (S3, GCS, Azure), backup store, exporters |
| Connectors | OIDC provider, and any endpoint an outbound connector calls (REST, SaaS APIs, SMTP, message brokers) |
| Identity | OIDC provider or Keycloak, its PostgreSQL database |
| Optimize | Elasticsearch or OpenSearch, OIDC provider |
| Camunda Hub REST API | Its PostgreSQL database, OIDC provider, SMTP server |
| Camunda Hub WebSockets | None besides the REST API |

Example for an external PostgreSQL in `10.20.0.0/16` and an OIDC provider reached
over HTTPS:

```yaml
global:
  networkPolicy:
    enabled: true
    egress:
      enabled: true
      extraRules:
        - to:
            - ipBlock:
                cidr: 10.20.0.0/16
          ports:
            - protocol: TCP
              port: 5432
        - to:
            - ipBlock:
                cidr: 203.0.113.0/24
          ports:
            - protocol: TCP
              port: 443
```

### Egress to other namespaces

`global.networkPolicy.egress.allowedNamespaces` and the component
`networkPolicy.egress.allowedNamespaces` take the same entry forms as the ingress lists
and follow the same inherit-or-replace rule. Unlike ingress, an egress entry has no
default ports: an entry without `ports` allows every port in the selected namespace.
Add a `ports` list to narrow it:

```yaml
global:
  networkPolicy:
    egress:
      enabled: true
      allowedNamespaces:
        - camunda-hub
        - name: elastic-system
          ports:
            - protocol: TCP
              port: 9200
```

Each entry renders one egress rule after the DNS rule and before the egress
`extraRules`.

### Connectors

Outbound connectors call endpoints that are defined in process models, not in the chart
values, so a fixed list of destinations rarely holds. Either keep egress off for
Connectors:

```yaml
connectors:
  networkPolicy:
    egress:
      enabled: false
```

or allow HTTPS to any address and keep the rest blocked:

```yaml
connectors:
  networkPolicy:
    egress:
      extraRules:
        - to:
            - ipBlock:
                cidr: 0.0.0.0/0
          ports:
            - protocol: TCP
              port: 443
```

### Migrating from ingress-only policies

Egress is off by default, so an upgrade does not change existing ingress-only policies.
To turn egress on, list the external dependencies of each component from the table
above, add them as `extraRules` or egress `allowedNamespaces`, then set `global.networkPolicy.egress.enabled: true`.
The policies keep their names and are updated in place; `helm rollback` to a revision
without egress removes the egress rules again.

## Isolation boundary

The chart renders one policy per component and selects only that component's pods.
What is isolated, and what is not, follows from the rendered rules:

| Component | Ingress when on | Egress when on | Not isolated by the chart |
|---|---|---|---|
| Orchestration Cluster | Release namespace on all five ports; `allowedNamespaces` on 8080 and 26500; `extraRules` | Release namespace, DNS, egress `allowedNamespaces`, `extraRules` | Broker-to-broker traffic inside the release namespace; any pod in the namespace can reach 26501, 26502 and 9600 |
| Connectors | Release namespace on 8080; `allowedNamespaces` on 8080; `extraRules` | Same as above | Inbound webhooks from the internet need an ingress controller rule; outbound connector targets need `extraRules` |
| Identity | Release namespace on 8084 and 8082; `allowedNamespaces` on 8084; `extraRules` | Same as above | Keycloak and its database, which are not part of the chart |
| Optimize | Release namespace on 8090 and 8092; `allowedNamespaces` on 8090; `extraRules` | Same as above | Elasticsearch or OpenSearch |
| Camunda Hub REST API | Release namespace on 8081 and 8091; `allowedNamespaces` on 8081; `extraRules` | Same as above | Its PostgreSQL database, the Hub web app pods |
| Camunda Hub WebSockets | Release namespace on 8060; `allowedNamespaces` on 8060; `extraRules` | Same as above | — |

Properties that hold for every policy:

- **Ingress is default-deny for the selected pods.** Once a policy with `Ingress` in
  `policyTypes` selects a pod, only the listed peers and ports reach it.
- **The release namespace is trusted.** The first ingress rule and the first egress rule
  use `podSelector: {}`, so every pod in the namespace, including pods the chart does not
  manage, can reach every listed port. Policies isolate the release from other
  namespaces, not components from each other. Use a dedicated namespace per release.
- **Egress is allow-all until enabled.** With `global.networkPolicy.egress.enabled: false`
  the policies have no `Egress` type and Kubernetes does not restrict outbound traffic.
- **Pods without a policy are open.** Init jobs, Helm test pods, companion charts and any
  extra workloads in the namespace are not selected and stay unrestricted, unless another
  policy in the namespace selects them.
- **`extraRules` are global.** `global.networkPolicy.ingress.extraRules` and
  `global.networkPolicy.egress.extraRules` are appended to every component; an `extraRules`
  entry without `ports` opens every port. There is no per-component ingress `extraRules`.
- **Policies are additive.** A `NetworkPolicy` from another source that selects the same
  pods widens, never narrows, what the chart allows. A namespace-wide default-deny policy
  added by a platform team still applies to pods the chart does not select.
- **Node and host traffic depends on the CNI.** Most CNIs always allow traffic from the
  local node, so kubelet probes keep working. Traffic from other nodes, `hostNetwork` pods
  and cloud load balancers is not.

## Failure modes

| Symptom | Likely cause | Fix |
|---|---|---|
| Pods log `UnknownHostException` or `Temporary failure in name resolution` right after egress is turned on | Cluster DNS is not `k8s-app: kube-dns` in `kube-system` (for example NodeLocal DNSCache on `169.254.20.10`, OpenShift `openshift-dns`) | Set `allowDNS: false` and add an egress `extraRules` entry for the actual DNS pods or IP on port 53 UDP and TCP |
| The UI or API returns 502 or 504 through the ingress; pods are healthy | The ingress controller namespace is not allowed | Add it to `global.networkPolicy.ingress.allowedNamespaces` |
| Cloud load balancer marks backends unhealthy (GKE NEG, AWS ALB in IP mode) | Health checks come from load balancer CIDRs, not from a pod | Add an `ipBlock` ingress rule for the provider's health-check ranges |
| Readiness or liveness probes fail only with policies on | CNI does not allow node-to-pod traffic by default, or probes run from a different node | Allow the node CIDR with an `ipBlock` ingress rule on the management port |
| Prometheus targets show `context deadline exceeded` | Management ports are not client ports and are never opened by `allowedNamespaces` defaults | Add a `monitoring` entry with `ports` (see [Monitoring](#monitoring)) |
| An operator (ECK, CloudNativePG, Keycloak operator, cert-manager webhook) loses control of its resources, or the operator reports connection timeouts | The operator pod in its own namespace cannot reach the workload, or Camunda egress blocks the operator-managed service | Allow the operator namespace on the target port on the receiving side; allow the datastore namespace in egress `allowedNamespaces` |
| Hub, Connectors or Optimize in another release cannot reach the Orchestration Cluster; `UNAVAILABLE` on gRPC or connection timeouts | Cross-release traffic: one half of the ingress/egress pair is missing | Add ingress `allowedNamespaces` on the receiver and egress `allowedNamespaces` on the caller |
| Login loops or `401` after enabling egress | The OIDC provider (issuer, JWKS endpoint) is unreachable | Add an egress `extraRules` entry for the provider address on 443 |
| Exporter, backup or document store errors after enabling egress | S3, GCS, Azure or Elasticsearch endpoints are outside the cluster | Add `ipBlock` egress rules, or route through an egress gateway |
| Zeebe brokers fail to form a cluster | A namespace-wide default-deny from another source blocks traffic, or brokers run in several namespaces | Keep all brokers in the release namespace; the chart does not support cross-namespace broker ports |

A blocked connection usually shows as a timeout, not as `connection refused`: most CNIs
drop denied packets. A refused connection points to a missing listener, not a policy.

## Upgrade choreography

Order matters because a policy takes effect as soon as the API server accepts it, while
the pods that need the new access may not exist yet. Apply each step on its own
`helm upgrade` and wait for the release to be healthy before the next one.

**Turning policies on for the first time:**

1. Upgrade with `global.networkPolicy.enabled: true` and all required ingress
   `allowedNamespaces` and `extraRules` (ingress controller, monitoring, calling
   releases). Egress stays off.
2. Verify the UI, APIs, metrics and cross-release callers work.
3. Add egress `extraRules` and egress `allowedNamespaces` for every external dependency
   while `egress.enabled` is still `false`. They are not rendered yet, so this upgrade is
   a no-op; it lets you review the values diff on its own.
4. Set `global.networkPolicy.egress.enabled: true`. For a risky component, enable egress
   globally but keep `<component>.networkPolicy.egress.enabled: false` until it is
   verified.

**Adding a new caller across releases:** upgrade the receiving release first so its
ingress allows the caller, then the calling release with its egress entry. Removing a
caller is the reverse: remove the caller's traffic, then remove the receiver's entry.

**Chart version upgrades:** upgrade with the same network policy values first. Change
policy values in a separate upgrade, so a failure can be attributed either to the new
chart version or to the policy change.

**Rollbacks:** `helm rollback` restores the policies of the target revision together
with the workloads. Rolling back to a revision before policies were enabled deletes them
and reopens the namespace. Rolling back a receiving release to a revision without an
`allowedNamespaces` entry cuts off callers that still send traffic: roll back the caller
first, or keep the receiver's entry.

**Emergency disable:** `helm upgrade --reuse-values --set global.networkPolicy.enabled=false`
removes every chart policy. To keep ingress isolation while removing egress restrictions,
set `global.networkPolicy.egress.enabled=false` instead.

## Argo CD and GitOps

The policies are plain chart resources without hooks or sync-wave annotations, so Argo
CD applies them in the same wave as the workloads (`NetworkPolicy` is synced before
`Deployment` and `StatefulSet` within a wave by Argo's kind ordering).

- **Receiver first.** When releases are separate Applications, give the receiving
  Application an earlier wave in the parent app-of-apps or ApplicationSet with
  progressive sync, so its ingress rules exist before the caller's pods start.
- **Within one Application**, sync waves cannot be set per policy through chart values.
  `global.annotations` applies to every chart resource, so do not use it for
  `argocd.argoproj.io/sync-wave`. Use a Kustomize patch or a post-renderer on the
  `NetworkPolicy` kind if you need a separate wave.
- **Policies managed outside the chart** (for example a namespace default-deny in a
  platform repository) should sync in an earlier wave than any Camunda Application, and
  the Camunda values must already allow everything that the default-deny removes.
- **Self-heal and pruning.** Argo prunes policies removed from the values on the next
  sync. Manual `kubectl edit` changes to a policy are reverted by self-heal; make
  debugging changes in Git, or pause automatic sync while debugging.
- **Health.** Argo has no health check for `NetworkPolicy`, so a sync reports healthy
  even when a policy blocks traffic. Add a post-sync smoke test that calls the client
  ports from the expected namespaces.

## Service mesh and CNI-specific policy

Kubernetes `NetworkPolicy` is enforced at L3/L4 by the CNI. A service mesh or an
extended policy engine operates alongside it:

- **Istio (sidecar mode).** The sidecar proxies traffic, but packets still go from pod
  IP to pod IP, so the chart policies keep applying. Allow the mesh's own ports in
  addition: the control plane (`istiod` in `istio-system`, 15012 and 15017) in egress,
  and port 15008 (HBONE) or the sidecar ports if traffic is tunnelled. Mesh mTLS does not
  replace these policies; `AuthorizationPolicy` adds L7 identity checks on top.
- **Istio ambient mode.** Traffic between pods is carried by ztunnel on port 15008.
  Ingress rules that list only the application ports drop it: allow 15008 from the
  release and the allowed namespaces, or rely on Istio `AuthorizationPolicy` and keep the
  chart policies off for that release.
- **Cilium.** Cilium enforces Kubernetes `NetworkPolicy` and its own
  `CiliumNetworkPolicy`; both apply and are additive. Use `CiliumNetworkPolicy` for
  what the chart cannot express, such as `toFQDNs` egress to managed databases or OIDC
  providers, instead of `ipBlock` ranges. With Cilium's host firewall or
  `policy-enforcement: always`, also allow the `host` and `remote-node` entities for
  probes.
- **Calico.** Calico `GlobalNetworkPolicy` with a lower `order` is evaluated before
  Kubernetes policies and can deny traffic the chart allows. Check tiered policies
  first when the chart policy looks correct.
- **Egress gateways.** When outbound traffic goes through a mesh egress gateway or a
  Cilium egress gateway, allow the gateway namespace in egress `allowedNamespaces`
  instead of the external CIDRs.

When the mesh is the single source of truth for traffic control, keep
`global.networkPolicy.enabled: false` and enforce the same boundaries with mesh
policies; running both means every change must be made twice.

## Components not covered

The chart renders policies only for the six components listed in
[Rendered policies](#rendered-policies). The following are deployed separately and are
not selected by any chart policy. Restrict them with their own chart settings or your own
policies, and allow Camunda traffic to them in egress:

| Component | Typical source | Port Camunda calls | Who calls it |
|---|---|---|---|
| Elasticsearch | ECK operator or Elastic Helm chart | 9200 | Orchestration Cluster, Optimize |
| OpenSearch | OpenSearch operator or Helm chart, or a managed service | 9200 or 443 | Orchestration Cluster, Optimize |
| PostgreSQL for Identity, Camunda Hub or RDBMS secondary storage | CloudNativePG, Bitnami-style chart or a managed service | 5432 | Identity, Camunda Hub REST API, Orchestration Cluster |
| Keycloak | Keycloak operator or Helm chart | 8080 or 443 | Identity, every component for token validation, browsers |
| Other OIDC providers | Entra ID, Okta and others, outside the cluster | 443 | Every component, browsers |
| Other Camunda Hub pods | Pods other than the REST API and WebSockets, if deployed | — | Not selected; unrestricted |
| Ingress controller | ingress-nginx, Gateway API controllers | — | Calls every component on its client port |
| Prometheus and ServiceMonitor targets | kube-prometheus-stack | — | Scrapes management ports |
| Chart jobs and Helm test pods | This chart | — | Not selected; unrestricted |

## Debugging: which rule blocked me?

Kubernetes does not log policy decisions, so find the blocking rule by elimination.

1. **List the policies that select the pod.** A connection is allowed only if at least
   one policy selecting the destination allows it in ingress, and one selecting the
   source allows it in egress.

   ```bash
   kubectl get networkpolicy -n camunda
   kubectl describe networkpolicy -n camunda <release>-orchestration
   kubectl get pod -n camunda <pod> --show-labels
   ```

   Compare the pod labels with `spec.podSelector` of every policy in both namespaces,
   including policies not created by the chart.

2. **Check the source namespace label.** `allowedNamespaces` by name matches on
   `kubernetes.io/metadata.name`; the selector form matches your own labels.

   ```bash
   kubectl get namespace camunda-hub --show-labels
   ```

3. **Reproduce from the source namespace.**

   ```bash
   kubectl run np-probe -n camunda-hub --rm -it --restart=Never --image=busybox -- \
     nc -vz -w 3 <orchestration-service>.camunda.svc 26500
   ```

   A timeout means a policy drop; `connection refused` means the port has no listener.
   Run the same probe from the release namespace: success there and failure from the
   other namespace points to `allowedNamespaces`, failure everywhere points to egress on
   the source or a CNI-level policy.

4. **Isolate the direction.** Temporarily set the source's
   `networkPolicy.egress.enabled: false`. If the probe succeeds, the source egress is the
   blocker; otherwise it is the destination ingress.

5. **Read the rendered values.** Compare the effective policy with what you expect:

   ```bash
   helm get values -n camunda <release> --all | yq '.global.networkPolicy'
   helm get manifest -n camunda <release> | yq 'select(.kind == "NetworkPolicy")'
   ```

   Remember that a component-level `allowedNamespaces`, even `[]`, replaces the global
   list.

6. **Use CNI tooling to see the drop.**

   - Calico:

     ```bash
     calicoctl get networkpolicy,globalnetworkpolicy -A -o wide
     kubectl get globalnetworkpolicies.crd.projectcalico.org
     ```

     Kubernetes policies appear as `knp.default.<name>`. Enable flow logs (Calico
     Enterprise) or a `Log` action in a `GlobalNetworkPolicy` that precedes the deny to
     see which policy matched.

   - Cilium:

     ```bash
     cilium monitor --type drop
     hubble observe --namespace camunda --verdict DROPPED
     cilium endpoint list
     cilium policy get
     ```

     Hubble prints the source, destination, port and `Policy denied` reason.

   - Other CNIs: check node logs or the cloud provider's network policy logging (for
     example GKE Dataplane V2 network policy logging).

**Common patterns:**

- Works in the release namespace, fails from another: missing `allowedNamespaces` entry,
  or a management port that is not a client port.
- Worked before an upgrade that set a component list: the component `allowedNamespaces`
  replaced the global one.
- Fails only for some pods of a StatefulSet: pods on nodes where the CNI agent is not
  ready, or a stale policy cache; check the CNI agent pods.
- Fails only after egress was enabled: DNS, OIDC provider or external datastore.
- Fails to a hostname whose IP changed: `ipBlock` rules are static; use a wider CIDR,
  an egress gateway or an FQDN-capable policy engine.

## Helpers and tests

The shared helpers `camundaPlatform.networkPolicyEnabled`,
`camundaPlatform.networkPolicyIngressEnabled`,
`camundaPlatform.networkPolicyEgressEnabled`,
`camundaPlatform.networkPolicyAllowedNamespacesList`,
`camundaPlatform.networkPolicyAllowedNamespaceRules` and `camundaPlatform.networkPolicy` live in
`templates/common/_helpers.tpl`. Unit and golden tests are in `test/unit/common/networkpolicy_test.go`.
