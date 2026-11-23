# OpenShift-shaped NetworkPolicy scenarios

`make test.networkpolicy-enforcement networkPolicyArgs="-install-calico -env openshift"` runs the
runner on kind with Calico against an OpenShift-shaped environment. No OpenShift cluster is used.

What the runner adds for `-env openshift`:

- A `<namespace>-router` namespace labelled `policy-group.network.openshift.io/ingress=""` (OVN-Kubernetes)
  and `network.openshift.io/policy-group=ingress` (OpenShiftSDN), standing in for `openshift-ingress`.
- A `<namespace>-dns` namespace with `openshift-dns.yaml`: a CoreDNS pod labelled
  `dns.operator.openshift.io/daemonset-dns=default` that listens on 5353 behind a port-53 Service,
  standing in for `openshift-dns`.
- `openshift-project-policies.yaml`: the `deny-by-default`, `allow-from-same-namespace` and
  `allow-from-openshift-ingress` policies of the OpenShift 4.x multitenant project template.
- `openshift-egress-policies.yaml`: an admin egress default-deny with DNS and same-namespace egress allowed.
- `values-openshift-router.yaml`: the Router entry for `global.networkPolicy.ingress.extraRules`.
- `values-openshift-router-allowed.yaml`: the Router entry for `global.networkPolicy.ingress.allowedNamespaces`.
- `values-openshift-egress-dns.yaml`: `egress.allowDNS: false` plus `egress.allowedNamespaces` entries for
  the OpenShift DNS shape on 5353 and for `<namespace>-allowed`.

## Results

Confirmed by `-install-calico -env openshift` on kind with Calico v3.30.3 against the chain tip (509bb4aa2): all seven scenarios passed, exit 0.

| Scenario | Result | Finding | Applies to | Change vs the S1–S6 base |
|---|---|---|---|---|
| S6/openshift install with policies on beside the project-template policies | PASS | Names do not collide (chart policies carry the release prefix). Policies are additive, so `allow-from-openshift-ingress` opens every component port to the Router and no chart value can narrow it. Egress stays open while `egress.enabled` is false. | OpenShift 4.x | Unchanged; now also asserts egress is open by default. |
| S2/openshift without the project template the chart policies block the Router | PASS | Once `global.networkPolicy.enabled=true`, Routes fail until the Router namespace is admitted by `ingress.allowedNamespaces` or `ingress.extraRules`. | OpenShift 4.x | Was "must be listed in `extraRules`"; `allowedNamespaces` is now the first-class fix. |
| S3/openshift extraRules admit the Router by its policy-group label on 8080 only | PASS | `policy-group.network.openshift.io/ingress: ""` selects the Router namespace (the OpenShiftSDN form was `network.openshift.io/policy-group: ingress`). | OpenShift 4.x | Unchanged. |
| S10/openshift ingress allowedNamespaces admit the Router by namespaceSelector on the client ports | PASS | A `namespaceSelector` entry in `ingress.allowedNamespaces` is enough for Routes. It opens the default client ports, including Zeebe gRPC 26500; set `ports` on the entry to keep 26500 closed unless a passthrough gRPC Route is used. A name entry (`openshift-ingress`) also works because OpenShift 4.x sets `kubernetes.io/metadata.name`. | OpenShift 4.x | New: `allowedNamespaces` did not exist on the base. |
| S7/openshift chart egress allowDNS does not reach the OpenShift DNS shape | PASS | `egress.allowDNS` allows only `kube-system` pods labelled `k8s-app=kube-dns` on port 53. OpenShift DNS runs in `openshift-dns` and listens on 5353 behind the port-53 Service; policies match the pod port, so with `egress.enabled=true` and the defaults, every lookup fails on OpenShift. | OpenShift 4.x | New: the base had no chart egress; it recorded that the chart could not relax an admin default-deny. |
| S13/openshift egress allowedNamespaces admit openshift-dns on 5353 and the allowed namespace | PASS | Set `egress.allowDNS: false` and add an `egress.allowedNamespaces` entry `{name: openshift-dns, podSelector: {matchLabels: {dns.operator.openshift.io/daemonset-dns: default}}, ports: [5353/UDP, 5353/TCP]}`. No `extraRules` needed. `kube-dns` is then blocked, as intended. | OpenShift 4.x | New. |
| S7/openshift an admin egress default-deny does not narrow the chart egress rules | PASS | A namespace-scoped `deny-all-egress` coexists with the chart's policies (no name collision), but NetworkPolicies are additive: chart `egress.allowedNamespaces` still open what the admin denied. Admins who must cap egress need AdminNetworkPolicy or EgressFirewall. | OpenShift 4.x | Changed: on the base the chart rendered no egress, so the admin deny was the only egress policy. The chain tip's egress rules now widen it. |

## Limits of kind

- The Router is a regular pod. With HostNetwork endpoint publishing, OVN-Kubernetes classifies Router
  traffic as `policy-group.network.openshift.io/host-network`. kind cannot reproduce this.
- The OpenShift DNS shape is a CoreDNS pod in `<namespace>-dns`, queried by Service IP. Pods on kind still
  resolve through `kube-dns`, so S7/openshift does not show Camunda failing to start; on OpenShift it would.
- AdminNetworkPolicy, BaselineAdminNetworkPolicy, OVN EgressFirewall and SCC interactions are not exercised.
- Calico enforces standard NetworkPolicy. OVN-Kubernetes semantics are assumed equivalent for these policies.

## Follow-up

- Cilium under `-env openshift`: the multi-CNI runner code is on an unpublished branch. Add the Cilium
  variant once that branch lands.
