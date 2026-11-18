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
	"slices"
	"strings"
	"testing"
)

type openshiftFake struct {
	enforcing     bool
	leakyRouter   bool
	chartKubeDNS  bool
	projectFix    bool
	egressFix     bool
	values        string
	routerLabeled bool
	calls         []string
}

func (f *openshiftFake) run(_ context.Context, name string, args ...string) (string, error) {
	joined := strings.Join(args, " ")
	f.calls = append(f.calls, name+" "+joined)
	if name == "helm" {
		if args[0] == "install" || args[0] == "upgrade" {
			f.values = joined
		}
		return "", nil
	}
	switch {
	case args[0] == "label" && args[2] == "netpol-router":
		f.routerLabeled = true
	case args[0] == "apply" || args[0] == "delete":
		on := args[0] == "apply"
		if strings.HasSuffix(joined, "openshift-project-policies.yaml") {
			f.projectFix = on
		}
		if strings.HasSuffix(joined, "openshift-egress-policies.yaml") {
			f.egressFix = on
		}
	case args[0] == "get" && args[1] == "networkpolicy":
		return strings.Join(f.policyNames(), " "), nil
	case args[0] == "get" && args[1] == "nodes":
		return "172.18.0.2", nil
	case args[0] == "get" && args[1] == "service":
		return "10.96.5.53", nil
	case args[0] == "get" && args[1] == "pod" && args[2] == "netpol-probe":
		return map[string]string{"netpol": "10.1.0.0", "netpol-outsider": "10.1.0.1", "netpol-allowed": "10.1.0.2"}[args[4]], nil
	case args[0] == "get" && args[1] == "pod" && strings.Contains(args[len(args)-1], "metadata.name"):
		return "netpol-zeebe-0", nil
	case args[0] == "get" && args[1] == "pod":
		if strings.Contains(joined, "component=connectors") {
			return "10.0.0.2", nil
		}
		return "10.0.0.1", nil
	case args[0] == "exec" && args[1] == "netpol-probe":
		return "", f.ingress(args[3], args[len(args)-1])
	case args[0] == "exec":
		return "", f.egress(args)
	}
	return "", nil
}

func (f *openshiftFake) has(file string) bool {
	return strings.Contains(f.values, file)
}

func (f *openshiftFake) policyNames() []string {
	var names []string
	if f.projectFix {
		names = append(names, projectPolicies...)
	}
	if f.egressFix {
		names = append(names, egressPolicies...)
	}
	if f.values != "" {
		names = append(names, "netpol-connectors", "netpol-zeebe")
	}
	return names
}

func (f *openshiftFake) ingress(from, port string) error {
	open := !f.enforcing || from == "netpol"
	if from == "netpol-router" && f.routerLabeled {
		open = open || f.projectFix || f.leakyRouter ||
			(f.has("values-openshift-router.yaml") && port == "8080") ||
			(f.has("values-openshift-router-allowed.yaml") && (port == "8080" || port == "26500"))
	}
	if open {
		return nil
	}
	return errors.New("blocked")
}

func (f *openshiftFake) egress(args []string) error {
	egressOn := f.enforcing && (f.has("values-egress.yaml") || f.has("values-openshift-egress-dns.yaml"))
	if !egressOn {
		return nil
	}
	dnsAllowed := f.has("values-openshift-egress-dns.yaml")
	if slices.Contains(args, "nslookup") {
		if args[len(args)-1] == "10.96.5.53" {
			if dnsAllowed || f.chartKubeDNS {
				return nil
			}
			return errors.New("blocked")
		}
		if f.has("values-egress.yaml") || f.egressFix {
			return nil
		}
		return errors.New("blocked")
	}
	switch args[len(args)-2] {
	case "10.0.0.2":
		return nil
	case "10.1.0.2":
		if dnsAllowed {
			return nil
		}
	}
	return errors.New("blocked")
}

func openshiftRunner(f *openshiftFake) runner {
	cfg := config{chartDir: "/chart", scenarioDir: "/chart/s", namespace: "netpol", release: "netpol", namespace2: "netpol-allowed", release2: "netpol-split", timeout: "1m", retries: 3, env: envOpenShift}
	return runner{cfg: cfg, command: f.run, log: func(string, ...any) {}}
}

func TestOpenShiftRunPasses(t *testing.T) {
	f := &openshiftFake{enforcing: true}
	if err := openshiftRunner(f).run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !slices.Contains(f.calls, "kubectl label namespace netpol-router "+routerLabelOVN+" "+routerLabelSDN) {
		t.Fatalf("router namespace not labelled: %v", f.calls)
	}
	if !slices.Contains(f.calls, "kubectl apply --namespace netpol-dns -f /chart/s/openshift-dns.yaml") {
		t.Fatalf("OpenShift-shaped DNS not created: %v", f.calls)
	}
	for _, ns := range []string{"netpol-router", "netpol-dns"} {
		if !slices.ContainsFunc(f.calls, func(c string) bool { return strings.HasPrefix(c, "kubectl delete namespace "+ns) }) {
			t.Fatalf("%s not cleaned up", ns)
		}
	}
}

func TestOpenShiftRunFailsOnNonEnforcingCNI(t *testing.T) {
	err := openshiftRunner(&openshiftFake{}).run(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "S6/openshift") {
		t.Fatalf("expected S6/openshift failure, got %v", err)
	}
}

func TestOpenShiftRunFailsWhenRouterIsAlwaysOpen(t *testing.T) {
	err := openshiftRunner(&openshiftFake{enforcing: true, leakyRouter: true}).run(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "S2/openshift") {
		t.Fatalf("expected S2/openshift failure, got %v", err)
	}
}

func TestOpenShiftRunFailsWhenAllowDNSReachesOpenShiftDNS(t *testing.T) {
	err := openshiftRunner(&openshiftFake{enforcing: true, chartKubeDNS: true}).run(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "S7/openshift chart egress allowDNS") {
		t.Fatalf("expected S7/openshift failure, got %v", err)
	}
}

func TestParseConfigEnv(t *testing.T) {
	cfg, err := parseConfig([]string{"-env", "openshift"}, func(string) string { return "" }, nil)
	if err != nil || cfg.env != envOpenShift {
		t.Fatalf("unexpected %+v %v", cfg, err)
	}
	if _, err := parseConfig([]string{"-env", "eks"}, func(string) string { return "" }, nil); err == nil {
		t.Fatal("expected error for unknown env")
	}
}
