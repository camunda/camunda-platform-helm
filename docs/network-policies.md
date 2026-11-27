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

## December proof runbook (GKE/distroci)

The KIND + Calico evidence proves the chart renders the policies and that an enforcing CNI
acts on them. The December proof carries that to GKE/distroci, whose datapath differs
(Dataplane V2, private nodes, node-local DNS) and where the chart runs with the real
Camunda images. It has three parts, run in this order: the canary gate **G0**, the runner
leg **S1–S12 (13 runs; S13–S15 not yet written)**, then the real-image leg **D1–D4**.

### G0 — deny-all canary gate

No Camunda run is trusted until the cluster proves it enforces a deny-all policy on its
own. G0 is independent of the chart and runs once the Calico addon from
`camunda/team-distribution#960` has merged and been applied (decision log, 18 Nov).

1. In one namespace, start a client pod and a server pod listening on a known port, and
   confirm the client reaches the server.
2. Apply a deny-all ingress `NetworkPolicy` (`podSelector: {}`, `policyTypes: [Ingress]`)
   to the server. Confirm the client is blocked within 30 seconds.
3. Remove the policy and confirm the client reaches the server again.
4. Repeat 1–3 for egress: confirm the server reaches the client, apply a deny-all egress
   policy, confirm the connection is blocked within 30 seconds, remove it and confirm it
   is reachable again.

If any part of G0 fails, stop. The cluster does not enforce `NetworkPolicy` on the path
the chart relies on, and neither of the legs below can be trusted.

### Runner leg — S1–S12 (13 runs; S13–S15 not yet written)

`scripts/networkpolicy-enforcement/` drives the chart through a chain-linked sequence of
installs, upgrades, rollbacks and label changes; each run builds on the revision the
previous one left behind. The recorded evidence is **S1–S12 (13 runs; S13–S15 not yet
written)** — 13 chain-linked runs. On KIND, create the cluster from `kind-calico.yaml`
(which disables the default CNI) and let the runner apply Calico v3.30.3 with
`-install-calico`:

```bash
kind create cluster --config charts/camunda-platform-8.11/test/integration/scenarios/networkpolicy/kind-calico.yaml
make test.networkpolicy-enforcement networkPolicyArgs=-install-calico
```

The make target builds the runner, runs its Go tests, then executes the sequence against
`charts/camunda-platform-8.11`. The runner refuses to start if any of its namespaces
(`<ns>`, `<ns>-outsider`, `<ns>-allowed`) already exist, and it cleans them up when it
finishes. On GKE/distroci the cluster CNI already enforces policies, so omit
`-install-calico` and run the binary directly:

```bash
CHART_DIR=$PWD/charts/camunda-platform-8.11 \
EXTERNAL_TARGET=203.0.113.10:443 \
  scripts/networkpolicy-enforcement/networkpolicy-enforcement netpol
```

The runs are:

| Run | What it proves |
|---|---|
| S1 | install with policies off — nothing renders, the outsider reaches every port |
| S2 | upgrade enables policies — in-namespace traffic works, the outsider is blocked |
| S3 | `ingress.extraRules` open exactly the allowed namespace on 8080 |
| S4 | `connectors.networkPolicy.enabled=false` removes only the connectors policy |
| S5 | `helm rollback` deletes the policies and reopens the ports |
| S6 | fresh install with policies on — pods reach Ready, egress still open |
| S7 | egress on blocks everything but the release namespace and DNS |
| S8 | egress `extraRules` open the allowed namespace on 8080 |
| S9 | rollback removes the egress rules |
| S10 | ingress `allowedNamespaces` open the client ports (8080, 26500) only |
| S11 | a `namespaceSelector` entry follows the namespace label |
| S11 | removing the label blocks the namespace again without an upgrade |
| S12 | an empty component list replaces the global list for that component |

The S11 step runs twice — the label applied, then removed — which is why S1–S12 is 13
runs, not 12.

The external-egress probe in S7, S8 and the rollback runs dials a target outside the
cluster to prove egress is blocked. `EXTERNAL_TARGET` selects it: a host or `host:port`,
with IPv6 hosts bracketed (`[2001:db8::1]:443`). Unset, it keeps the default — the first
node's `InternalIP` on port 6443 — which is what KIND serves. GKE does not serve the API
there, so set `EXTERNAL_TARGET` to a public IP on 443 or the `kubernetes` service
endpoint. A value without a port uses 443.

### Real-image leg — D1–D4

The runner uses busybox stand-ins. The real-image leg installs the full 8.11 chart with
the real Camunda images on distroci and re-checks enforcement against the real pods,
with Identity and Keycloak on so that the proof includes a real browser login.
**This leg is executed by the human operator from a Mac**, not by the agent hosts, which
have no Docker credentials for the private registry. The variables and commands are:

```bash
NS=ws-e45976-real
REL=netpol
CHART=./charts/camunda-platform-8.11
SC=$CHART/test/integration/scenarios/networkpolicy
CTX=distro-ci
DOMAIN=ci.distro.ultrawombat.com
export CAMUNDA_HOSTNAME=$NS.$DOMAIN
```

`DOMAIN` is the distroci entry of `ValidIngressBaseDomains` in
`scripts/deploy-camunda/config/config.go` (`ci.distro.ultrawombat.com`; the other entry,
`distribution.aws.camunda.cloud`, is the EKS zone). Without `--ingress-subdomain`,
deploy-camunda joins the namespace and the base domain into `CAMUNDA_HOSTNAME`, so the
host is `$NS.$DOMAIN`; the companion Keycloak below needs the same value.

The topology is one broker plus Connectors, Elasticsearch, and Identity with Keycloak
and its database; Optimize and Camunda Hub (Web Modeler) are off. Two values layers
produce it:

- `--identity keycloak` (`values/identity/keycloak.yaml`) turns on Identity, OIDC and the
  `/auth/` ingress route, and sets `publicIssuerUrl` to
  `https://$CAMUNDA_HOSTNAME/auth/realms/camunda-platform`.
- `--features networkpolicy` (`values/features/networkpolicy.yaml`) turns on ingress
  policies and turns off `optimize` and `camundaHub`, which `base.yaml` and the keycloak
  layer turn on. deploy-camunda merges feature layers after the identity layer.

Do not layer `$SC/values-real.yaml` on top: it sets `identity.enabled: false`, which
removes Identity and its policy, and leaves only `netpol-zeebe` and `netpol-connectors`.

Pre-checks:

```bash
deploy-camunda --help   # confirm the --extra-values syntax (comma-separated or repeatable)
deploy-camunda doctor
```

`--extra-values` takes comma-separated files or is repeated; `doctor` confirms the
config, the kube context, the Docker credentials and the scenario's `$PLACEHOLDER`s
before the first install. The keycloak layer reads `$RDBMS_POSTGRESQL_USERNAME` and
`$RDBMS_POSTGRESQL_PASSWORD`; set them in the `.env` file or the shell.

Credentials pre-check. The identity/keycloak layer and the Keycloak companion read the
secret `integration-test-credentials` in `$NS`. Check that it exists and list its key
names only; never print the values:

```bash
kubectl --context $CTX -n $NS get secret integration-test-credentials \
  -o go-template='{{range $k, $v := .data}}{{$k}}{{"\n"}}{{end}}'
```

The rendered chart and the companions read these keys:

| Key | Read by |
|---|---|
| `identity-keycloak-admin-password` | the chart's Identity, and the Keycloak companion (`auth.passwordSecretKey`) |
| `identity-admin-client-token` | Identity (`global.identity.auth.admin`) |
| `identity-admin-client-password` | `base.yaml` admin client |
| `identity-user-password` | `base.yaml` demo user |
| `identity-connectors-client-token` | Connectors and Identity |
| `identity-orchestration-client-token` | Orchestration and Identity |

The console and Optimize client tokens are not read, because those components are off.
If the secret or a key is missing, stop and create the secret before D1.

Keycloak dependencies. The registry declares the Keycloak dependencies in
`charts/camunda-platform-8.11/test/ci/registry/dependencies/`; `alwaysgreen.yaml` is the
keycloak scenario that pins the issuer and uses all three:

| Dependency | Chart | Release | Values file |
|---|---|---|---|
| `keycloak-fixed-issuer` | `charts/internal-keycloak-26` | `keycloak` | `test/integration/companion-values/keycloak-fixed-issuer.yaml` (reads `CAMUNDA_HOSTNAME`; Keycloak's own PostgreSQL is a sub-component of this release) |
| `postgresql` | `charts/internal-postgresql` | `postgresql` | `test/integration/companion-values/postgresql.yaml` (reads `RDBMS_POSTGRESQL_USERNAME`, `RDBMS_POSTGRESQL_PASSWORD`; holds the Identity database) |
| `elasticsearch` | `elastic/elasticsearch` 8.5.1 from `https://helm.elastic.co` | `elasticsearch` | `test/integration/companion-values/elasticsearch-9.yaml` |

`keycloak-fixed-issuer` is required rather than `keycloak`: it pins `KC_HOSTNAME` to
`https://$CAMUNDA_HOSTNAME/auth`, so the issuer is the same for the browser and for the
pods. The single-deploy command has no flag for companion charts: deploy-camunda installs
them only for a registry scenario selected by name (`matrix run`, or `--scenario <name>`
for a scenario in the registry). `chart-full-setup` is not a registry scenario, and
`--scenario alwaysgreen` would also layer the `alwaysgreen` node pool. Install the three
companions into `$NS` once before D1, from the repository root:

```bash
kubectl --context $CTX create namespace $NS
helm repo add elastic https://helm.elastic.co
for v in keycloak-fixed-issuer postgresql; do
  envsubst < test/integration/companion-values/$v.yaml > /tmp/$v.yaml
done
helm --kube-context $CTX -n $NS install keycloak charts/internal-keycloak-26 -f /tmp/keycloak-fixed-issuer.yaml
helm --kube-context $CTX -n $NS install postgresql charts/internal-postgresql -f /tmp/postgresql.yaml
helm --kube-context $CTX -n $NS install elasticsearch elastic/elasticsearch --version 8.5.1 \
  -f test/integration/companion-values/elasticsearch-9.yaml
```

D1 — baseline with policies off (no `--features networkpolicy`):

```bash
deploy-camunda --chart-path $CHART --namespace $NS --release $REL --kube-context $CTX \
  --scenario chart-full-setup --identity keycloak --persistence elasticsearch \
  --ingress-base-domain $DOMAIN
```

D1 renders Optimize and Camunda Hub, because only the networkpolicy feature layer turns
them off.

D2 — ingress policies:

```bash
deploy-camunda --chart-path $CHART --namespace $NS --release $REL --kube-context $CTX \
  --scenario chart-full-setup --identity keycloak --persistence elasticsearch \
  --features networkpolicy --ingress-base-domain $DOMAIN \
  --extra-values $SC/values-extrarules.yaml
```

D3 — egress policies:

```bash
deploy-camunda --chart-path $CHART --namespace $NS --release $REL --kube-context $CTX \
  --scenario chart-full-setup --identity keycloak --persistence elasticsearch \
  --features networkpolicy --ingress-base-domain $DOMAIN \
  --extra-values $SC/values-egress.yaml,$SC/values-egress-extrarules.yaml
```

D4 — allowed namespaces:

```bash
deploy-camunda --chart-path $CHART --namespace $NS --release $REL --kube-context $CTX \
  --scenario chart-full-setup --identity keycloak --persistence elasticsearch \
  --features networkpolicy --ingress-base-domain $DOMAIN \
  --extra-values $SC/values-allowed-namespaces.yaml
```

With `--features networkpolicy` the expected policy list is one per enabled component:
`netpol-zeebe` (orchestration), `netpol-connectors` and `netpol-identity`. Keycloak,
PostgreSQL and Elasticsearch are separate releases and get no policy.

The policies block the ingress controller's namespace unless it is allowed (see
[Ingress controller](#ingress-controller)), so the browser login through
`https://$CAMUNDA_HOSTNAME` can fail on D2–D4 while `/auth/` (Keycloak, no policy) still
answers. If it does, add the Contour namespace to `ingress.allowedNamespaces` in a
local values file; in D4 that list must also keep `$NS-allowed`, because Helm replaces
lists.

After each D step:

- All pods in `$NS` are Ready, with no restarts caused by a policy.
- `kubectl --context $CTX get networkpolicy -n $NS` matches the expected list below.
- Probe pods in `$NS-outsider` and `$NS-allowed` reach or are blocked exactly as in the
  table. Create both namespaces once before D1 and apply `$SC/probe.yaml` into each:

  ```bash
  for n in $NS-outsider $NS-allowed; do
    kubectl --context $CTX create namespace $n
    kubectl --context $CTX apply -n $n -f $SC/probe.yaml
  done
  ```

- The browser login at `https://$CAMUNDA_HOSTNAME/orchestration` redirects to Keycloak
  and back, and the ID token's `iss` claim is
  `https://$CAMUNDA_HOSTNAME/auth/realms/camunda-platform`.
- The e2e smoke suite passes after D2–D4 (the D1 baseline needs no policy check).

| Step | Policies in `$NS` | Ingress from `$NS-outsider` | Ingress from `$NS-allowed` | Egress from the release pods |
|---|---|---|---|---|
| D1 | none | reaches every port | reaches every port | open |
| D2 | `netpol-zeebe`, `netpol-connectors`, `netpol-identity`, `Ingress` | blocked on every port | 8080 on every component; zeebe and connectors listen there, identity (8084) does not | open |
| D3 | same three, `Ingress` + `Egress` | blocked | blocked — no ingress rule for `$NS-allowed` in this step | `$NS-allowed` 8080 only; `$NS-outsider` and external blocked; DNS intact |
| D4 | same three, `Ingress` | blocked | zeebe 8080 and 26500, connectors 8080, identity 8084; 9600/26501/26502 blocked | open |

Not verified until a deploy: the browser login, DNS for `$CAMUNDA_HOSTNAME`, the TLS
certificate, the Contour namespace to allow, and the `iss` claim. The policy list and the
issuer values are from `helm template` only.

### Catalog entry

The deploy-camunda registry entry for this proof
(`charts/camunda-platform-8.11/test/ci/registry/scenarios/networkpolicy.yaml`, tier 2,
gke/distroci) stays disabled and lands **after the #7924–#7928 chain merges to main**.
Until then the proof is driven by the runner and the D1–D4 commands above, not by the
matrix.

## Helpers and tests

The shared helpers `camundaPlatform.networkPolicyEnabled`,
`camundaPlatform.networkPolicyIngressEnabled`,
`camundaPlatform.networkPolicyEgressEnabled`,
`camundaPlatform.networkPolicyAllowedNamespacesList`,
`camundaPlatform.networkPolicyAllowedNamespaceRules` and `camundaPlatform.networkPolicy` live in
`templates/common/_helpers.tpl`. Unit and golden tests are in `test/unit/common/networkpolicy_test.go`.
