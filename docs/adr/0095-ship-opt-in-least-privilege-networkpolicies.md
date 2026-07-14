# Ship opt-in least-privilege NetworkPolicies as a first-class chart feature

- Status: proposed
- Date: 2026-07-13
- Decision-makers: Distribution team
- Builds on: [ADR 0094](0094-remove-bundled-bitnami-subcharts-from-the-8-10-chart.md)

## Context and Problem Statement

Enterprise and regulated customers (e.g. HSBC, Telstra) moving to Zero-Trust
require least-privilege Kubernetes `NetworkPolicy` resources so Camunda pods
only communicate with the components they need. The chart ships none today:
customers either run with allow-all defaults or maintain hand-crafted,
consultant-authored policies that drift and break across upgrades, and there is
no official, tested traffic baseline. This is the [PDP #3519](https://github.com/camunda/product-hub/issues/3519)
epic (Define→Implement).

The problem is not "add a config knob" — it is defining a chart-wide,
cross-component networking contract: which components may talk to which, on
which ports, and how egress to customer-owned datastores is expressed. Per
`docs/maintainer-guide.md`, a change that "alters a shared contract (auth,
storage, **networking**)" requires an ADR.

The available signal is deliberately minimal. The Define-stage solution proposal
(closed PR [#6459](https://github.com/camunda/camunda-platform-helm/pull/6459))
predates [ADR 0094](0094-remove-bundled-bitnami-subcharts-from-the-8-10-chart.md)
and assumed bundled datastores and 8.9/8.10 structural parity; it is reference
only, not the basis for this decision.

### Applicability by version

- **8.10 (chart 15.x, primary).** Per [ADR 0094](0094-remove-bundled-bitnami-subcharts-from-the-8-10-chart.md)
  the chart bundles no datastores: Elasticsearch/OpenSearch, Keycloak, and
  PostgreSQL are always external. Datastore egress is therefore CIDR/`ipBlock`
  based, never pod-selector based. Workloads: the `orchestration` StatefulSet
  plus `identity`, `connectors`, `optimize`, and Web Modeler `restapi`/`websockets`
  Deployments (no standalone `console`/`webapp` pod, per [ADR 0085](0085-consolidate-web-modeler-into-a-single-container.md)
  and the Camunda Hub consolidation).
- **8.9 (chart 14.x, committed backport).** 8.9 is the current GA version and
  the designated Helm 3→4 migration landing point; 8.10 is in alpha, and the
  enterprise customers driving this epic run GA or older. The backport is
  therefore committed, not optional — an 8.10-only feature would reach no
  production deployment until 8.10 GA plus enterprise upgrade lag. It is a
  distinct design, not a copy of 8.10: 8.9 still bundles the Bitnami
  Elasticsearch/Keycloak/PostgreSQL subcharts (each already shipping its own
  `networkPolicy.enabled`, default off) and has standalone `console` and
  Web Modeler `webapp` workloads, so datastore egress needs a bundled
  (pod-selector) branch in addition to the external (CIDR) branch. Designed and
  implemented via [#6593](https://github.com/camunda/camunda-platform-helm/issues/6593)
  after the 8.10 traffic matrix is validated; the values surface stays identical
  to 8.10 wherever possible.

## Decision Drivers

- **Networking is a shared contract.** A policy must track the exact ports and
  workload labels of every component; a wrong or drifted selector silently
  breaks traffic. This is inherently cross-component and needs one governing
  decision, not per-component ad-hoc choices.
- **Small configuration surface.** The epic explicitly asks to avoid
  over-parameterizing policy rules.
- **Zero default impact.** Existing deployments must be unaffected; the feature
  is opt-in and disabled by default.
- **Selectors must not drift.** Policies must reuse the existing
  `<component>.matchLabels` helpers so they cannot diverge from the workloads
  they target (consistent with [ADR 0034](0034-remove-image-version-tags-from-kubernetes-matchlabels.md)).
- **External-infrastructure-first.** Post-0094 the chart owns the workloads and
  the customer owns datastores; egress policy must reflect that boundary rather
  than assume in-cluster datastore pods.

## Considered Options

### Configuration surface

- **Per-component toggles only** (`<component>.networkPolicy.enabled` each, no
  global switch) — rejected: no single audit/enable point, and shared inputs
  (ingress-controller/monitoring selectors, external CIDRs) would be duplicated
  per component.
- **Single monolithic top-level block** (everything under `networkPolicy.*`) —
  rejected: does not match the chart's per-component layout and cannot cheaply
  gate on each component's own `.enabled`.
- **Hybrid: `global.networkPolicy.*` master switch + shared config, plus
  per-component `<component>.networkPolicy.*` (chosen)** — one enable/audit
  point and one home for shared inputs, while per-component blocks follow the
  established `<component>.podDisruptionBudget.enabled` idiom and gate naturally
  with each component.

### Packaging

- **Single `network-policies.yaml`** rendering all components — rejected: renders
  policies for disabled components and fights the per-component template layout.
- **Per-component template files reusing existing selectors (chosen)** — one
  policy template per component, gated by the global switch AND the component's
  own `.enabled`, with shared rule fragments in a `templates/common/_networkpolicy.tpl`
  helper.

### Datastore egress

- **Pod-selector egress to bundled datastore pods** — rejected for 8.10: there
  are no bundled datastores post-0094.
- **CIDR/`ipBlock` egress from explicit external-endpoint config (chosen)** —
  matches the external-infrastructure boundary; the customer supplies datastore
  CIDRs.

### Backport scope

- **8.10 only** — rejected: 8.10 is in alpha and the enterprise customers
  driving the epic run the GA version or older; the feature would reach no
  production deployment until 8.10 GA plus enterprise upgrade lag.
- **Backport to 8.9 and 8.8** — rejected: 8.8 predates the Helm 3→4 migration
  landing point and would triple the bundled-datastore design surface for a
  version customers are already expected to leave.
- **Backport to 8.9, after 8.10 validation (chosen)** — 8.9 is the current GA
  and migration landing point; the 8.10 implementation validates the model on a
  policy-enforcing cluster first, then 8.9 adds the bundled-datastore branch.

### Default-deny

- **Defer default-deny to a follow-up** — rejected: without a deny baseline the
  allow-rules are inert on most CNIs, so v1 would not deliver least-privilege.
- **Ship an optional default-deny in v1 (chosen)** — a separate, off-by-default
  toggle so the least-privilege story is complete when opted into, with zero
  default blast radius.

## Decision Outcome

The chart ships opt-in, disabled-by-default `NetworkPolicy` resources as a
first-class feature, starting with 8.10. Normative constraints:

1. The feature MUST be gated by a master switch `global.networkPolicy.enabled`
   (default `false`). With it off, the chart MUST render zero `NetworkPolicy`
   objects and existing render output MUST be byte-identical.
2. Each component policy MUST gate on
   `and .Values.<component>.enabled .Values.global.networkPolicy.enabled .Values.<component>.networkPolicy.enabled`,
   where `<component>.networkPolicy.enabled` defaults to `true` (so the master
   switch alone enables the full set).
3. Every `podSelector` and cross-component `from`/`to` selector MUST reuse the
   existing `<component>.matchLabels` helpers — no hand-written label maps. A
   cross-component rule MUST be conditional on the referenced component's
   `.enabled`.
4. Policy rules MUST match container (target) ports, verified against each
   component's workload template — not Service ports.
5. Datastore egress (Elasticsearch/OpenSearch, Keycloak, PostgreSQL) MUST be
   expressed as `ipBlock` rules from `global.networkPolicy.external.<store>.{cidrs,ports}`;
   on 8.10 it MUST NOT assume in-cluster datastore pods. The 8.9 backport MUST
   additionally provide pod-selector egress to the bundled datastore pods when
   the corresponding subchart is enabled (`elasticsearch.enabled`,
   `identityKeycloak.enabled`, `identityPostgresql.enabled`,
   `webModelerPostgresql.enabled`), so the default bundled topology works with
   policies and default-deny enabled.
6. Shared rule fragments (DNS egress to kube-dns on 53/UDP+TCP, "from monitoring",
   "from ingress controller", external-endpoint egress) MUST live in a single
   `templates/common/_networkpolicy.tpl` helper and be reused across component
   templates.
7. An optional default-deny (Ingress+Egress) policy MUST be provided behind
   `global.networkPolicy.defaultDeny.enabled` (default `false`), scoped to the
   release's Camunda pods via `camundaPlatform.matchLabels` so co-located
   non-Camunda workloads are untouched.
8. Each component policy MUST expose `extraIngress`/`extraEgress` escape hatches
   appended verbatim to the generated rules. Connectors outbound-internet egress
   MUST NOT be a permissive default; it MUST be shipped as a documented
   `extraEgress` example. Web Modeler SMTP egress is handled the same way.
9. Multi-namespace topologies MUST be served by the
   `ingressControllerNamespaceSelector`/`monitoringNamespaceSelector` values and
   the per-component escape hatches — no separate multi-namespace toggle.
10. Every template MUST have golden + negative unit tests; goldens MUST be
    regenerated via `make go.update-golden-only` and never hand-edited.

All new `networkPolicy` keys are Tier 2 (infrastructure/connectivity) per
[ADR 0091](0091-adopt-component-extraconfiguration-as-the-standard-application-configuration-mechanism.md)
— allowed in `values.yaml` and backport-eligible.

Applicability: 8.10 (chart 15.x) first, landing across foundation +
per-component + CI-validation PRs tracked by
[#6585–#6591](https://github.com/camunda/camunda-platform-helm/issues/6585);
docs in [#6592](https://github.com/camunda/camunda-platform-helm/issues/6592);
the committed 8.9 backport (including the bundled-datastore egress branch,
console, and Web Modeler webapp policies) designed and implemented via
[#6593](https://github.com/camunda/camunda-platform-helm/issues/6593) after
8.10 validation.
The traffic matrix MUST be validated on a policy-enforcing cluster before the
defaults are locked. This assumes the CI cluster used for
[#6591](https://github.com/camunda/camunda-platform-helm/issues/6591) enforces
NetworkPolicies (Dataplane V2 or Calico); a canary check (deny-all, confirm
traffic is actually blocked) MUST precede trusting a green run, since a
non-enforcing cluster accepts policy objects without acting on them.

### Positive Consequences

- Turns "lock down Camunda network traffic" into a configuration switch plus a
  review step, with a vendor-supported, tested baseline (drives the epic's
  time-to-secure-install and support-load goals).
- Selector reuse and golden tests keep policies correct across upgrades; drift
  surfaces as a failing test rather than as silently blocked traffic.
- Opt-in, disabled-by-default means zero impact on existing deployments.
- The external-endpoint egress model matches the post-0094 ownership boundary,
  so the networking story is consistent with how 8.10 already handles datastores.

### Negative Consequences

- Ongoing maintenance burden: the traffic matrix must be revalidated whenever a
  component's ports or dependencies change; golden tests mitigate but do not
  eliminate drift.
- External-endpoint egress (datastore CIDRs, connector targets, SMTP) cannot be
  defaulted and remains a customer responsibility — a documentation and support
  surface even with the baseline shipped.
- Enforcement is CNI-dependent. On clusters without NetworkPolicy enforcement the
  policies are inert; the feature can give a false sense of security if operators
  do not confirm their CNI enforces. Docs must call this out.
- The committed 8.9 backport carries extra design cost: the bundled-datastore
  egress branch and the console/webapp policies are 8.9-only code, partially
  throwaway given the Bitnami deprecation path
  ([ADR 0083](0083-deprecate-bitnami-subcharts-in-camunda-platform-helm-chart.md)),
  and version parity is still delayed until 8.10 validation completes.

## Links

- Builds on [ADR 0094](0094-remove-bundled-bitnami-subcharts-from-the-8-10-chart.md) — the 8.10 external-infrastructure boundary that makes datastore egress CIDR-based.
- Relates to [ADR 0091](0091-adopt-component-extraconfiguration-as-the-standard-application-configuration-mechanism.md) — Tier 1/2 key classification (these keys are Tier 2).
- Relates to [ADR 0080](0080-adopt-kubernetes-gateway-api-as-a-first-class-routing.md) — ingress/Gateway API routing, the trusted north-south source these policies admit.
- Epic [product-hub#3519](https://github.com/camunda/product-hub/issues/3519); implementation issues #6585–#6593.
