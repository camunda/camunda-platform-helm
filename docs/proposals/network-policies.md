# Solution Proposal — Optional Least-Privilege NetworkPolicies for Camunda 8 Self-Managed

- Status: proposed (PDP **Define** stage)
- Epic: [product-hub#3519](https://github.com/camunda/product-hub/issues/3519)
- Target charts: **8.10** (primary) + **8.9** (backport)
- Decision-makers: Distribution team (Eng DRI), Docs DRI

> This is a solution proposal for DRI review, not an implementation. No chart
> code is changed by this document. Once the approach is ratified, the
> normative decision should be captured in a human-authored ADR under
> `docs/adr/` (agents do not author ADRs), and the work broken into
> Implement-stage tickets.

## Context and problem statement

Enterprise and regulated customers (e.g. HSBC, Telstra) moving to Zero-Trust
require least-privilege Kubernetes `NetworkPolicy` resources so Camunda pods
only communicate with the components they need. Today there is no official,
tested baseline shipped with the chart: customers either run with allow-all
defaults or maintain hand-crafted, consultant-authored policies that drift and
break across upgrades.

This epic ships **opt-in, disabled-by-default** NetworkPolicies maintained and
tested alongside the Helm chart, plus documentation describing the traffic
matrix and how to adapt the policies to single- and multi-namespace topologies.

## Verified codebase facts

- **8.10 and 8.9 are both unified**: `templates/orchestration/` bundles the
  Zeebe gateway + Operate + Tasklist in one StatefulSet. Both charts share the
  same component template directories: `orchestration`, `identity`,
  `web-modeler`, `connectors`, `console`, `optimize`, `common`,
  `service-monitor` (plus `camunda-hub` in 8.10). The backport is therefore
  structurally identical and low-risk.
- **No NetworkPolicy templates or `networkPolicy` values keys exist** in either
  chart today.
- Gating convention to reuse verbatim:
  `{{- if and .Values.<component>.enabled .Values.<feature>.enabled -}}`
  (see `templates/orchestration/poddisruptionbudget.yaml`, `serviceaccount.yaml`).
- Reusable pod selectors already exist and MUST be reused so policy selectors
  never drift from the workloads they target: `camundaPlatform.matchLabels`
  (`templates/common/_helpers.tpl`), `orchestration.matchLabels`, and per-component
  `*.matchLabels` (e.g. `webModeler.restapi.matchLabels`).
- External datastores (`global.elasticsearch`, `global.opensearch`, Keycloak via
  `global.identity.keycloak.url`, PostgreSQL) may be **in-cluster or external**.
  Egress to external endpoints cannot use a pod selector and must be
  CIDR-parameterized.
- Tests use terratest + golden snapshots under `test/unit/<component>/golden/`,
  registered in `goldenfiles_test.go`, regenerated via
  `make go.update-golden-only chartPath=...` (never hand-edited). Pattern to
  copy: `test/unit/orchestration/poddisruptionbudget_test.go`.

### Default ports

| Component | In-cluster listen ports (targetPort) |
|-----------|--------------------------------------|
| Orchestration | http/REST + webapps **8080**, gRPC gateway **26500**, management **9600**, broker command **26501**, broker internal **26502** |
| Identity | http **8084**, metrics **8082** |
| Keycloak | `global.identity.keycloak.url.port` (configurable; bundled default 8080) |
| Web-Modeler restapi | app **8081**, management **8091** |
| Web-Modeler websockets | **8060** |
| Connectors | **8080** |
| Optimize | app **8090**, management **8092** |
| Console | http + management (service-mapped) |
| Elasticsearch / OpenSearch | **9200** (egress target) |
| PostgreSQL | **5432** (egress target) |
| kube-dns | **53/UDP + 53/TCP** (egress, all pods) |

## Traffic matrix (least-privilege baseline)

Initiator → target (egress on the initiator, ingress on the target). To be
validated against live nightly clusters during Implement.

**Ingress to each component:**

- **Orchestration** — gRPC 26500 from connectors and external clients
  (Ingress/Gateway); REST 8080 from web-modeler, connectors, optimize, console,
  and the Operate/Tasklist UI via Ingress/Gateway; broker 26501/26502 from
  orchestration peers (intra-StatefulSet); management 9600 from monitoring.
- **Identity** — 8084 from orchestration, web-modeler, optimize, connectors,
  console, Ingress/Gateway; metrics 8082 from monitoring.
- **Keycloak** — keycloak port from identity and every auth client
  (orchestration, web-modeler, optimize, connectors, console) plus Ingress/Gateway.
- **Web-Modeler restapi** — 8081 from Ingress/Gateway and websockets; 8091 from
  monitoring.
- **Web-Modeler websockets** — 8060 from Ingress/Gateway.
- **Connectors** — 8080 from orchestration and Ingress/Gateway.
- **Optimize** — 8090 from Ingress/Gateway; 8092 from monitoring.
- **Console** — http/management from Ingress/Gateway.

**Egress from each component** (DNS → kube-dns required by all):

- **Orchestration** → ES/OS 9200, Keycloak.
- **Identity** → Keycloak, PostgreSQL 5432.
- **Keycloak** → PostgreSQL 5432.
- **Web-Modeler restapi** → PostgreSQL 5432, Keycloak/Identity, orchestration REST 8080.
- **Connectors** → orchestration gRPC 26500 + REST 8080, ES/OS, Keycloak, and
  **external connector endpoints (internet)** — customer-configurable; the
  default policy must not silently break outbound connector calls.
- **Optimize** → ES/OS 9200, Keycloak/Identity, orchestration.
- **Console** → orchestration, Identity/Keycloak.

When `global.<store>.enabled` (bundled), egress uses a
`namespaceSelector`/`podSelector`; when the store is external, egress uses
`ipBlock.cidr` from the new values described below.

## Decision drivers

- **Small configuration surface.** The issue explicitly asks to avoid
  over-parameterizing policy rules.
- **Selectors must not drift.** Policies have to track workload labels exactly,
  or enabling them silently breaks traffic.
- **Opt-in, zero default impact.** Disabled by default; existing deployments
  must be unaffected.
- **Version-scoped, low-risk backport.** 8.9 shares 8.10's layout.

## Considered options

### Configuration surface

- **Per-component toggles** (`<component>.networkPolicy.enabled` each). Rejected:
  larger surface, more value sprawl, no real benefit over a single switch plus
  per-component escape hatches.
- **Single global toggle + extras (chosen).** One `networkPolicy.enabled` master
  switch, optional default-deny, external-endpoint CIDR config, and per-component
  `extraIngress`/`extraEgress` escape hatches.

### Packaging

- **Single monolithic `network-policies.yaml`.** Rejected: does not match the
  chart's per-component layout and renders policies for disabled components.
- **Per-component template files (chosen).** One policy template per component,
  each gated by the global switch AND the component's own `.enabled`, rendered
  conditionally with the component.

## Proposed design

### 1. Configuration surface

New top-level `networkPolicy:` section in `values.yaml`:

```yaml
networkPolicy:
  enabled: false                 # master switch; every policy gated by
                                 # `and <component>.enabled .Values.networkPolicy.enabled`
  defaultDeny:
    enabled: false               # optional default-deny-all (ingress+egress) over all Camunda pods
  ingress:
    ingressControllerNamespaceSelector: {}   # trusted ingress source, reused across components
    monitoringNamespaceSelector: {}          # Prometheus/ServiceMonitor scraping source
  external:                      # egress ipBlock rules when datastores are NOT bundled
    elasticsearch: { cidrs: [], ports: [9200] }
    opensearch:    { cidrs: [], ports: [9200] }
    keycloak:      { cidrs: [], ports: [] }
    postgresql:    { cidrs: [], ports: [5432] }
  extraIngress: []               # raw NetworkPolicyIngressRule entries merged into generated rules
  extraEgress: []                # raw NetworkPolicyEgressRule entries merged into generated rules
```

### 2. Packaging — per-component templates reusing existing selectors

- `templates/orchestration/networkpolicy.yaml`
- `templates/identity/networkpolicy.yaml` (+ Keycloak policy)
- `templates/web-modeler/networkpolicy-restapi.yaml`, `networkpolicy-websockets.yaml`
- `templates/connectors/networkpolicy.yaml`
- `templates/optimize/networkpolicy.yaml`
- `templates/console/networkpolicy.yaml`
- `templates/common/networkpolicy-default-deny.yaml` (optional default-deny)

Each `podSelector` and cross-component `from`/`to` rule reuses the existing
`*.matchLabels` helpers. Shared rule fragments (DNS egress, "from monitoring",
"from ingress controller") live in a new `templates/common/_networkpolicy.tpl`
helper to avoid duplication across the 7+ policy files.

### 3. Single- vs multi-namespace

Default rules target same-namespace, release-scoped pods. For multi-namespace
topologies, the `*NamespaceSelector` values and per-component
`extraIngress`/`extraEgress` widen scope without adding a separate toggle.

### 4. Versions

Implement on **8.10** first, then backport identically to **8.9** (same unified
layout). Each chart gets its own templates, values entries, tests, and goldens,
keeping diffs version-scoped.

## Implementation phases (Implement-stage breakdown)

1. **8.10 templates + values** — add the `networkPolicy.*` schema, the
   `_networkpolicy.tpl` helpers, per-component `networkpolicy.yaml` templates,
   and the default-deny template. Reuse existing `matchLabels` helpers and the
   `and ...enabled` gating idiom.
2. **8.10 tests** — terratest golden tests per template (copy
   `poddisruptionbudget_test.go`), registered in `goldenfiles_test.go` with
   `SetValues: networkPolicy.enabled=true`; goldens via
   `make go.update-golden-only chartPath=charts/camunda-platform-8.10`. Add a
   negative test asserting nothing renders by default.
3. **8.9 backport** — replicate phases 1–2 under `charts/camunda-platform-8.9/`.
4. **Docs** — new "Network security / NetworkPolicies" page in `camunda-docs`
   under `docs/self-managed/deployment/helm/...` (and the matching
   `versioned_docs/version-8.9/...`): traffic matrix, enable/customize via Helm,
   single- vs multi-namespace, external-endpoint CIDR config, and limitations
   (connectors→internet, external datastores). Include an annotated example
   values block aligned to the shipped policies.
5. **Cross-repo coordination** — Helm PR(s) first (8.10 then 8.9), Docs PR last,
   all linked to each other and to the epic.

## Open questions for DRI review

1. Confirm the traffic matrix against live nightly clusters — especially
   management ingress (9600/8092) for ServiceMonitor scraping and broker
   26501/26502 intra-cluster traffic — before locking defaults.
2. Connectors → internet egress: ship as a documented-required `extraEgress`
   example, or a permissive default egress scoped to connectors only?
   (Recommendation: documented example, no permissive default.)
3. Default-deny: ship the optional default-deny policy in v1, or defer to a
   follow-up to reduce blast radius?
4. Web-Modeler/Console value paths: 8.10 exposes both `webModeler.*`/`console.*`
   and the SaaS-hub-merged `camundaHub.*` paths — confirm policies cover both.

## Positive consequences

- Turns "lock down Camunda network traffic" into a configuration switch plus a
  review step, with a vendor-supported, tested baseline.
- Selectors reuse existing helpers, so policies stay correct across upgrades.
- Opt-in default means zero impact on existing deployments.

## Negative consequences

- Ongoing maintenance burden: the traffic matrix must be revalidated whenever
  component ports or dependencies change; golden tests mitigate but do not
  eliminate drift.
- External-endpoint egress (datastores, connector targets) cannot be fully
  defaulted and remains a customer responsibility — a documentation and support
  surface even with the baseline shipped.

## Validation criteria

- `make helm.template chartPath=charts/camunda-platform-8.10`
  with `--set networkPolicy.enabled=true` renders valid `NetworkPolicy` objects;
  the default render produces none.
- `make go.test chartPath=charts/camunda-platform-8.10` (and `-8.9`) green;
  goldens regenerated via `make go.update-golden-only`, never hand-edited.
- `make helm.lint chartPath=...` clean (after `make helm.dependency-update chartPath=...`).
- End-to-end: deploy to a GKE nightly namespace with policies enabled, run the SM
  smoke suite (`c8e2e` / `render-e2e-env.sh`) to confirm no required traffic is
  blocked; nightly matrix green for both 8.10 and 8.9.
