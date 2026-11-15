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

## Helpers and tests

The shared helpers `camundaPlatform.networkPolicyEnabled`,
`camundaPlatform.networkPolicyIngressEnabled`,
`camundaPlatform.networkPolicyEgressEnabled`,
`camundaPlatform.networkPolicyAllowedNamespacesList`,
`camundaPlatform.networkPolicyAllowedNamespaceRules` and `camundaPlatform.networkPolicy` live in
`templates/common/_helpers.tpl`. Unit and golden tests are in `test/unit/common/networkpolicy_test.go`.
