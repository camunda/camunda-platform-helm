// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

const (
	routerLabelOVN = "policy-group.network.openshift.io/ingress="
	routerLabelSDN = "network.openshift.io/policy-group=ingress"
)

var projectPolicies = []string{"allow-from-openshift-ingress", "allow-from-same-namespace", "deny-by-default"}

var egressPolicies = []string{"allow-egress-dns", "allow-egress-same-namespace", "deny-all-egress"}

func (r runner) prepareOpenShift(ctx context.Context) error {
	namespace := r.sourceNamespace(router)
	if _, err := r.kubectl(ctx, "label", "namespace", namespace, routerLabelOVN, routerLabelSDN); err != nil {
		return fmt.Errorf("label router namespace %s: %w", namespace, err)
	}
	dns := r.sourceNamespace(dnsHost)
	if _, err := r.kubectl(ctx, "apply", "--namespace", dns, "-f", filepath.Join(r.cfg.scenarioDir, "openshift-dns.yaml")); err != nil {
		return fmt.Errorf("create OpenShift-shaped DNS in %s: %w", dns, err)
	}
	if _, err := r.kubectl(ctx, "wait", "--for=condition=Ready", "pod/dns-default", "--namespace", dns, "--timeout", r.cfg.timeout); err != nil {
		return fmt.Errorf("wait for OpenShift-shaped DNS in %s: %w", dns, err)
	}
	return nil
}

func (r runner) applyFixture(file string) func(context.Context) error {
	return func(ctx context.Context) error {
		_, err := r.kubectl(ctx, "apply", "--namespace", r.cfg.namespace, "-f", filepath.Join(r.cfg.scenarioDir, file))
		return err
	}
}

func (r runner) deleteFixture(file string) func(context.Context) error {
	return func(ctx context.Context) error {
		_, err := r.kubectl(ctx, "delete", "--namespace", r.cfg.namespace, "--wait", "-f", filepath.Join(r.cfg.scenarioDir, file))
		return err
	}
}

func (r runner) retry(ctx context.Context, attempt func() error) error {
	var lastErr error
	for i := 0; i < r.cfg.retries; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return errors.Join(lastErr, ctx.Err())
			case <-time.After(r.cfg.retryInterval):
			}
		}
		if lastErr = attempt(); lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func (r runner) expectOpenShiftDNS(want bool) func(context.Context) error {
	return func(ctx context.Context) error {
		server, err := r.jsonpath(ctx, "OpenShift-shaped DNS service IP", "service", "dns-default", "--namespace", r.sourceNamespace(dnsHost), "-o", "jsonpath={.spec.clusterIP}")
		if err != nil {
			return err
		}
		return r.retry(ctx, func() error {
			pod, err := r.componentPod(ctx, zeebe)
			if err != nil {
				return err
			}
			_, err = r.kubectl(ctx, "exec", pod, "--namespace", r.cfg.namespace, "--", "nslookup", "-timeout=2", "kubernetes.default.svc.cluster.local", server)
			if got := err == nil; got != want {
				return fmt.Errorf("egress %s -> OpenShift-shaped DNS %s:53 (pod port 5353) reachable=%t, expected %t", zeebe, server, got, want)
			}
			return nil
		})
	}
}

func (r runner) openshiftSteps() []step {
	both := []string{"zeebe", "connectors"}
	enforced := append(all(inside, true), all(outsider, false)...)
	clientPorts := []int{8080, 26500}
	internalPorts := []int{9600, 26501, 26502}
	return []step{
		{
			name:     "S6/openshift install with policies on beside the project-template policies",
			action:   r.then(r.applyFixture("openshift-project-policies.yaml"), r.deploy("install", append(r.values(), "--set", "global.networkPolicy.enabled=true")...)),
			policies: both, foreign: projectPolicies,
			reaches: append(enforced, all(router, true)...),
			egress:  egressOpen(),
			finding: "[OpenShift 4.x] project-template policies coexist with the chart's: names do not collide (chart policies carry the release prefix) and NetworkPolicy is additive, so allow-from-openshift-ingress opens every component port to the Router namespace; no chart value can narrow it. [kind-limited] the Router is a plain pod in a labelled namespace, not the hostNetwork router.",
		},
		{
			name:     "S2/openshift without the project template the chart policies block the Router",
			action:   r.deleteFixture("openshift-project-policies.yaml"),
			policies: both,
			reaches:  append(enforced, all(router, false)...),
			finding:  "[OpenShift 4.x] on projects without allow-from-openshift-ingress, Routes to Camunda fail once global.networkPolicy.enabled=true until the Router namespace is admitted by ingress.allowedNamespaces or ingress.extraRules.",
		},
		{
			name:     "S3/openshift extraRules admit the Router by its policy-group label on 8080 only",
			action:   r.deploy("upgrade", r.values("values-openshift-router.yaml")...),
			policies: both,
			reaches: append(all(outsider, false),
				reach{from: router, component: zeebe, ports: []int{8080}, want: true},
				reach{from: router, component: zeebe, ports: []int{9600, 26500, 26501, 26502}, want: false},
				reach{from: router, component: connectors, ports: []int{8080}, want: true},
			),
			finding: "[OpenShift 4.x] namespaceSelector policy-group.network.openshift.io/ingress=\"\" (OVN-Kubernetes) selects the Router namespace; network.openshift.io/policy-group=ingress was the OpenShiftSDN form. [kind-limited] with HostNetwork endpoint publishing OVN-Kubernetes classifies Router traffic as policy-group.network.openshift.io/host-network, which kind cannot reproduce.",
		},
		{
			name:     "S10/openshift ingress allowedNamespaces admit the Router by namespaceSelector on the client ports",
			action:   r.deploy("upgrade", r.values("values-openshift-router-allowed.yaml")...),
			policies: both,
			reaches: append(all(outsider, false),
				reach{from: router, component: zeebe, ports: clientPorts, want: true},
				reach{from: router, component: zeebe, ports: internalPorts, want: false},
				reach{from: router, component: connectors, ports: componentPorts[connectors], want: true},
			),
			finding: "[OpenShift 4.x] a namespaceSelector entry in global.networkPolicy.ingress.allowedNamespaces is enough for Routes, no extraRules needed. It opens the default client ports, including Zeebe gRPC 26500; HTTP Routes need only 8080, so set ports on the entry to keep 26500 closed unless a passthrough gRPC Route is used. A plain name entry (openshift-ingress) also works because OpenShift 4.x sets kubernetes.io/metadata.name.",
		},
		{
			name:     "S7/openshift chart egress allowDNS does not reach the OpenShift DNS shape",
			action:   r.deploy("upgrade", r.values("values-egress.yaml")...),
			policies: both,
			reaches:  enforced,
			egress:   []egress{{targetConnectors, true}, {targetDNS, true}, {targetOutsider, false}, {targetAllowed, false}, {targetExternal, false}},
			checks:   []func(context.Context) error{r.expectOpenShiftDNS(false)},
			finding:  "[OpenShift 4.x] global.networkPolicy.egress.allowDNS allows kube-system pods labelled k8s-app=kube-dns on port 53. OpenShift DNS runs in openshift-dns (label dns.operator.openshift.io/daemonset-dns=default) and listens on 5353 behind the 53 Service, and policies match the pod port, so with egress.enabled=true every lookup fails on OpenShift. [kind-limited] the OpenShift DNS shape is a CoreDNS pod in <namespace>-dns queried explicitly; on kind the pods still resolve through kube-dns.",
		},
		{
			name:     "S13/openshift egress allowedNamespaces admit openshift-dns on 5353 and the allowed namespace",
			action:   r.deploy("upgrade", r.values("values-openshift-egress-dns.yaml")...),
			policies: both,
			reaches:  enforced,
			egress:   []egress{{targetConnectors, true}, {targetDNS, false}, {targetAllowed, true}, {targetOutsider, false}, {targetExternal, false}},
			checks:   []func(context.Context) error{r.expectOpenShiftDNS(true)},
			finding:  "[OpenShift 4.x] set egress.allowDNS=false and add an egress.allowedNamespaces entry {name: openshift-dns, podSelector dns.operator.openshift.io/daemonset-dns=default, ports 5353 UDP/TCP}; no extraRules needed. allowDNS=false removes the kube-system rule, so kube-dns is blocked, as intended on OpenShift.",
		},
		{
			name:     "S7/openshift an admin egress default-deny does not narrow the chart egress rules",
			action:   r.applyFixture("openshift-egress-policies.yaml"),
			policies: both, foreign: egressPolicies,
			reaches: enforced,
			egress:  []egress{{targetConnectors, true}, {targetDNS, true}, {targetAllowed, true}, {targetOutsider, false}, {targetExternal, false}},
			checks:  []func(context.Context) error{r.expectOpenShiftDNS(true)},
			finding: "[OpenShift 4.x] a namespace-scoped deny-all-egress NetworkPolicy coexists with the chart's (no name collision) but NetworkPolicies are additive: chart egress.allowedNamespaces still open what the admin denied, and the admin's kube-dns allow re-opens kube-dns. Admins who must cap egress need AdminNetworkPolicy or EgressFirewall. [kind-limited] AdminNetworkPolicy, BaselineAdminNetworkPolicy and EgressFirewall are not exercised.",
		},
	}
}
