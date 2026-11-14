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

type state struct {
	policies      bool
	extraRules    bool
	connectorsOff bool
	installed     bool
}

type fakeCluster struct {
	enforcing     bool
	leakyExtra    bool
	insideBlocked bool
	staleExecs    int
	existing      []string
	revisions     []state
	current       state
	calls         []string
}

func (f *fakeCluster) run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	if name == "helm" {
		f.helm(args)
		return "", nil
	}
	return f.kubectl(args)
}

func (f *fakeCluster) helm(args []string) {
	joined := strings.Join(args, " ")
	switch args[0] {
	case "install", "upgrade":
		f.current = state{
			installed:     true,
			policies:      strings.Contains(joined, "global.networkPolicy.enabled=true") || strings.Contains(joined, "values-extrarules.yaml"),
			extraRules:    strings.Contains(joined, "values-extrarules.yaml"),
			connectorsOff: strings.Contains(joined, "connectors.networkPolicy.enabled=false"),
		}
		f.revisions = append(f.revisions, f.current)
	case "rollback":
		f.current = f.revisions[0]
	case "uninstall":
		f.current = state{}
		f.revisions = nil
	}
}

func (f *fakeCluster) kubectl(args []string) (string, error) {
	switch {
	case args[0] == "get" && args[1] == "namespace":
		if slices.Contains(f.existing, args[2]) {
			return "namespace/" + args[2], nil
		}
		return "", nil
	case args[0] == "get" && args[1] == "networkpolicy":
		return strings.Join(f.policyNames(), " "), nil
	case args[0] == "get" && args[1] == "pod":
		if strings.Contains(strings.Join(args, " "), "component=connectors") {
			return "10.0.0.2", nil
		}
		return "10.0.0.1", nil
	case args[0] == "exec":
		return "", f.exec(args)
	}
	return "", nil
}

func (f *fakeCluster) policyNames() []string {
	var names []string
	if !f.current.policies {
		return names
	}
	if !f.current.connectorsOff {
		names = append(names, "netpol-connectors")
	}
	return append(names, "netpol-zeebe")
}

func (f *fakeCluster) exec(args []string) error {
	from, ip, port := args[3], args[len(args)-2], args[len(args)-1]
	if f.staleExecs > 0 {
		f.staleExecs--
		return errors.New("stale")
	}
	component := "zeebe"
	if ip == "10.0.0.2" {
		component = "connectors"
	}
	if f.insideBlocked && from == "netpol" {
		return errors.New("blocked")
	}
	if f.allowed(from, component, port) {
		return nil
	}
	return errors.New("blocked")
}

func (f *fakeCluster) allowed(from, component, port string) bool {
	if !f.enforcing || !slices.Contains(f.policyNames(), "netpol-"+component) || from == "netpol" {
		return true
	}
	return f.current.extraRules && from == "netpol-allowed" && (port == "8080" || f.leakyExtra)
}

func testRunner(f *fakeCluster) runner {
	cfg := config{chartDir: "/chart", scenarioDir: "/chart/s", namespace: "netpol", release: "netpol", timeout: "1m", retries: 3}
	return runner{cfg: cfg, command: f.run, log: func(string, ...any) {}}
}

func TestRunPassesOnEnforcingCluster(t *testing.T) {
	f := &fakeCluster{enforcing: true}
	if err := testRunner(f).run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !slices.ContainsFunc(f.calls, func(c string) bool { return strings.HasPrefix(c, "kubectl delete namespace netpol-allowed") }) {
		t.Fatalf("cleanup did not delete namespaces: %v", f.calls)
	}
}

func TestRunFailsOnNonEnforcingCNI(t *testing.T) {
	err := testRunner(&fakeCluster{}).run(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "S2") {
		t.Fatalf("expected S2 failure, got %v", err)
	}
}

func TestRunFailsWhenExtraRulesOpenTooMuch(t *testing.T) {
	err := testRunner(&fakeCluster{enforcing: true, leakyExtra: true}).run(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "S3") || !strings.Contains(err.Error(), ":9600") {
		t.Fatalf("expected S3 failure on 9600, got %v", err)
	}
}

func TestRunChecksInsidePrecondition(t *testing.T) {
	err := testRunner(&fakeCluster{enforcing: true, insideBlocked: true}).run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "S1") {
		t.Fatalf("expected failure, got %v", err)
	}
}

func TestExpectReachRetriesUntilConverged(t *testing.T) {
	f := &fakeCluster{staleExecs: 2}
	if err := testRunner(f).expectReach(context.Background(), "netpol", zeebe, 8080, true); err != nil {
		t.Fatalf("expected convergence: %v", err)
	}
}

func TestRunRefusesExistingNamespace(t *testing.T) {
	f := &fakeCluster{enforcing: true, existing: []string{"netpol-outsider"}}
	err := testRunner(f).run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected refusal, got %v", err)
	}
	for _, c := range f.calls {
		if strings.Contains(c, "delete") || strings.HasPrefix(c, "helm") {
			t.Fatalf("unexpected mutation after refusal: %s", c)
		}
	}
}

func TestRunInstallsCalico(t *testing.T) {
	f := &fakeCluster{enforcing: true}
	r := testRunner(f)
	r.cfg.installCalico = true
	r.cfg.calicoManifest = "calico.yaml"
	if err := r.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !slices.Contains(f.calls, "kubectl apply --server-side -f calico.yaml") {
		t.Fatalf("Calico not applied: %v", f.calls[:5])
	}
}

func TestParseConfig(t *testing.T) {
	env := map[string]string{"CHART_DIR": "/c", "RELEASE": "r"}
	cfg, err := parseConfig([]string{"-install-calico", "ns"}, func(k string) string { return env[k] }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.installCalico || cfg.namespace != "ns" || cfg.release != "r" || cfg.scenarioDir != "/c/test/integration/scenarios/networkpolicy" || !strings.Contains(cfg.calicoManifest, calicoVersion) {
		t.Fatalf("unexpected config %+v", cfg)
	}
	if _, err := parseConfig([]string{"a", "b"}, func(string) string { return "" }, nil); err == nil {
		t.Fatal("expected error for two namespaces")
	}
}
