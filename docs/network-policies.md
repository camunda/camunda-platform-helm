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

## Isolation boundary

The policies select only the pods of the components listed above, so the isolation
boundary is those pods.

| Isolated | Not isolated |
|---|---|
| Ingress to the component pods from outside the release namespace, beyond `allowedNamespaces` and `extraRules` | Traffic between pods in the release namespace: every pod in the namespace reaches every component pod port, including the broker ports 26501 and 26502 |
| Egress from the component pods, when `global.networkPolicy.egress.enabled` is on | Pods the chart does not render, such as companion charts, init jobs from other charts or sidecars in their own pods. See [Companion charts](#companion-charts) |
| | Components with `networkPolicy.enabled: false`, or egress for components with `networkPolicy.egress.enabled: false` |
| | Host-network pods and node traffic: most CNIs always allow kubelet probes from the node |
| | Layer-7 traffic: a rule allows a port, not a path, method or hostname |

The chart does not render a default-deny policy for the namespace. Add one yourself to
isolate pods the chart does not select.

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
brief denials until both have synced. See [Upgrade choreography](#upgrade-choreography)
for a single release.

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

## Failure modes

| Symptom | Likely cause | Fix |
|---|---|---|
| Policies render but nothing is blocked | The CNI does not enforce `NetworkPolicy`, for example kindnet or flannel without a policy engine | Use a CNI that enforces policies |
| Pods fail to start with name-resolution errors after egress is on | `allowDNS` points at `kube-dns` in `kube-system`, but the cluster DNS uses other labels or another namespace | Set `allowDNS: false` and add a DNS rule in `global.networkPolicy.egress.extraRules` |
| Readiness stays false, logs show timeouts to the database, Elasticsearch, OpenSearch or OIDC provider | Egress is on without a rule for that dependency | Add it to egress `allowedNamespaces` (in cluster) or `extraRules` with an `ipBlock` (outside the cluster) |
| Ingress controller returns 502 or 504 | The ingress controller namespace is not allowed | Add it to `global.networkPolicy.ingress.allowedNamespaces` |
| Prometheus targets are down | The management port is not a client port | Add an entry with `ports`. See [Monitoring](#monitoring) |
| A component list seems to drop the global entries | A component `allowedNamespaces` replaces the global list, including `[]` | Repeat the global entries in the component list |
| `helm template` fails on an `allowedNamespaces` entry | The entry sets both or neither of `name` and `namespaceSelector`, or an empty `namespaceSelector` | Fix the entry. See [Entry forms](#entry-forms) |
| A label-selected namespace keeps or loses access unexpectedly | The `namespaceSelector` follows namespace labels live, without an upgrade | Check the labels with `kubectl get ns --show-labels` |
| An `ipBlock` rule for an external endpoint stops working | The IP behind the hostname changed | Widen the CIDR or use the provider's published ranges |

## Upgrade choreography

A single release moves through these steps. Each step is one `helm upgrade` and can be
rolled back on its own:

1. Install or upgrade with `global.networkPolicy.enabled: false`. No policy exists.
2. Add `allowedNamespaces` or `extraRules` for every caller outside the release
   namespace: ingress controller, monitoring, other Camunda releases.
3. Set `global.networkPolicy.enabled: true`. Ingress is enforced; egress stays open.
4. Add the egress destinations from the [Egress](#egress) table as egress
   `allowedNamespaces` or `extraRules`.
5. Set `global.networkPolicy.egress.enabled: true`.

`helm rollback` to the revision of step 1 deletes the policies; rollback to the
revision of step 3 removes only the egress rules. Both are covered by the enforcement
scenarios in `scripts/networkpolicy-enforcement` (S5, rollback to the first revision
deletes the policies; S9, rollback removes the egress rules).

## Argo CD and GitOps

The policies are plain chart resources with no Helm hooks, so Argo CD and Flux apply them
like any other resource, in the same sync as the workloads. Within one Application:

- The policies have no `argocd.argoproj.io/sync-wave` annotation and land in wave 0,
  together with the Deployments and the StatefulSet. A pod can start before its policy
  exists; it is briefly unrestricted, not briefly blocked.
- The chart has no value for a sync-wave annotation on the policies. To have the
  policies in place before new pods start, enable them in a sync of their own first,
  as in [Upgrade choreography](#upgrade-choreography).
- With egress on and a dependency missing from the rules, the sync stays `Progressing`
  until the health check times out. Check the [Failure modes](#failure-modes) table.

Across Applications, follow [Upgrade order](#upgrade-order): sync the receiving
Application first, by hand or with an app-of-apps sync wave on the child Applications.

## Service meshes and other CNIs

This section is general guidance and has not been validated on a cluster. The
enforcement scenarios run on kind with Calico only.

- **Istio**: with sidecar injection, the application traffic leaves the pod through the
  Envoy sidecar on the same ports, so the policies still apply. The sidecar also needs
  egress to `istiod` (port 15012) in `istio-system`, and ingress on the sidecar ports
  15090 and 15021 for metrics and health checks if they are scraped from outside the
  namespace; add them through `extraRules`. In ambient mode, traffic between pods is
  carried by `ztunnel` on port 15008, which `NetworkPolicy` port rules do not match as
  expected; prefer Istio `AuthorizationPolicy` for layer-7 rules and test the layer-4
  policies before enforcing egress.
- **Cilium**: enforces `NetworkPolicy` natively. With kube-proxy replacement, a policy
  matches the backend pod, not the Service IP, which is what the chart rules expect.
  Cilium identifies host traffic as `host` and allows it by default.
- **Calico**: enforces `NetworkPolicy` natively. A Calico `GlobalNetworkPolicy` with a
  lower order can override the chart policies; check for one when a rule seems ignored.

## OpenShift

This section is general guidance and has not been validated on an OpenShift cluster.

- OpenShift SDN and OVN-Kubernetes both enforce `NetworkPolicy`.
- The OpenShift router runs in `openshift-ingress`. Allow it with
  `global.networkPolicy.ingress.allowedNamespaces: [openshift-ingress]`. On clusters
  where the router uses host networking, also allow the namespace label
  `policy-group.network.openshift.io/ingress: ""` through a `namespaceSelector` entry.
- Cluster monitoring runs in `openshift-monitoring`, user-workload monitoring in
  `openshift-user-workload-monitoring`. Allow the one that scrapes the release on the
  management ports. See [Monitoring](#monitoring).
- Cluster DNS runs in `openshift-dns` with the label
  `dns.operator.openshift.io/daemonset-dns: default` on port 5353. The default
  `allowDNS` rule does not match it: set `allowDNS: false` and add:

  ```yaml
  global:
    networkPolicy:
      egress:
        allowDNS: false
        extraRules:
          - to:
              - namespaceSelector:
                  matchLabels:
                    kubernetes.io/metadata.name: openshift-dns
            ports:
              - protocol: UDP
                port: 5353
              - protocol: TCP
                port: 5353
  ```

- The rendered policies need no extra security context constraints.

## Companion charts

The policies cover only the components rendered by this chart. Companion charts installed
as separate releases render no policy from these values:

| Companion | Chart in this repository | Covered | What to do |
|---|---|---|---|
| Elasticsearch | `charts/elasticsearch-21` | No | Use the chart's own `networkPolicy` values, or add a policy allowing the Camunda namespace on 9200 |
| Keycloak | `charts/keycloak-24`, `charts/internal-keycloak-26` | No | Allow Identity and the OIDC clients on the Keycloak HTTP port |
| PostgreSQL for Identity | `charts/identity-postgresql-15`, `charts/identity-postgresql-16` | No | Allow Identity on 5432 |
| PostgreSQL for Camunda Hub | `charts/web-modeler-postgresql-14`, `charts/web-modeler-postgresql-15` | No | Allow the Camunda Hub REST API on 5432 |
| Internal PostgreSQL | `charts/internal-postgresql` | No | Allow the components that use it on 5432 |
| Operators (ECK, CloudNativePG) | none | No | Follow the operator documentation |

When a companion runs in the release namespace, the Camunda components reach it through
the release-namespace rule. When it runs in its own namespace, add it to egress
`allowedNamespaces`. See [Database or Elasticsearch in its own namespace](#database-or-elasticsearch-in-its-own-namespace).

## Day-2 debugging

List the policies of a release and check their rules:

```bash
kubectl -n <namespace> get networkpolicy -l app.kubernetes.io/instance=<release>
kubectl -n <namespace> describe networkpolicy <name>
```

Check that a policy selects the expected pods:

```bash
kubectl -n <namespace> get networkpolicy <name> -o jsonpath='{.spec.podSelector.matchLabels}'
kubectl -n <namespace> get pods -l <label>=<value>
```

Probe a port from another namespace with a short-lived pod. A timeout means blocked; a
connection refused or an HTTP response means allowed:

```bash
kubectl -n <other-namespace> run np-probe --rm -it --restart=Never --image=busybox -- \
  nc -zv -w 3 <service>.<namespace>.svc.cluster.local 8080
```

Check the namespace labels a `namespaceSelector` entry relies on:

```bash
kubectl get namespace <other-namespace> --show-labels
```

Compare the rendered policies with what is in the cluster:

```bash
helm get manifest <release> -n <namespace> | yq 'select(.kind == "NetworkPolicy")'
```

To rule the policies out, set `global.networkPolicy.enabled: false` and upgrade. The
policies are deleted; turn them on again after the fix. CNI-specific tools, such as
`calicoctl` or `cilium monitor --type drop`, show which policy dropped a packet.

## Helpers and tests

The shared helpers `camundaPlatform.networkPolicyEnabled`,
`camundaPlatform.networkPolicyIngressEnabled`,
`camundaPlatform.networkPolicyEgressEnabled`,
`camundaPlatform.networkPolicyAllowedNamespacesList`,
`camundaPlatform.networkPolicyAllowedNamespaceRules` and `camundaPlatform.networkPolicy` live in
`templates/common/_helpers.tpl`. Unit and golden tests are in `test/unit/common/networkpolicy_test.go`.
