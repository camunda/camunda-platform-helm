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
	"strconv"
	"strings"
	"testing"
)

// nodeIP is the InternalIP the fake cluster reports for its single node; it is
// the default external egress target.
const nodeIP = "172.18.0.2"

type state struct {
	policies      bool
	extraRules    bool
	connectorsOff bool
	installed     bool
	egress        bool
	egressExtra   bool
	allowedNs     bool
	selector      bool
	connectorsNs  bool
	egressAllowed bool
}

type splitState struct {
	installed bool
	egress    bool
}

type fakeCluster struct {
	enforcing        bool
	externalIP       string
	leakyExtra       bool
	insideBlocked    bool
	egressIgnored    bool
	dnsBlocked       bool
	leakyAllowed     bool
	stickyLabel      bool
	ignoreReplace    bool
	egressNsIgnored  bool
	splitEgressBlock bool
	splitLeaky       bool
	rollbackDrops    bool
	split            splitState
	splitRevisions   []splitState
	labeled          map[string]bool
	staleExecs       int
	existing         []string
	revisions        []state
	current          state
	calls            []string
}

func (f *fakeCluster) run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	if name == "helm" && slices.Contains(args, "netpol-allowed") {
		f.splitHelm(args)
		return "", nil
	}
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
			policies:      strings.Contains(joined, "global.networkPolicy.enabled=true") || strings.Contains(joined, "values-extrarules.yaml") || strings.Contains(joined, "values-egress") || strings.Contains(joined, "values-allowed-"),
			egress:        strings.Contains(joined, "values-egress.yaml") || strings.Contains(joined, "values-egress-allowed-namespaces.yaml"),
			egressAllowed: strings.Contains(joined, "values-egress-allowed-namespaces.yaml") && !f.egressNsIgnored,
			egressExtra:   strings.Contains(joined, "values-egress-extrarules.yaml"),
			extraRules:    strings.Contains(joined, "values-extrarules.yaml"),
			connectorsOff: strings.Contains(joined, "connectors.networkPolicy.enabled=false"),
			allowedNs:     strings.Contains(joined, "values-allowed-namespaces.yaml") || strings.Contains(joined, "values-allowed-component-override.yaml"),
			selector:      strings.Contains(joined, "values-allowed-selector.yaml"),
			connectorsNs:  !strings.Contains(joined, "values-allowed-component-override.yaml") || f.ignoreReplace,
		}
		f.revisions = append(f.revisions, f.current)
	case "rollback":
		f.current = f.revisions[0]
		if args[2] != "1" {
			f.current = f.revisions[len(f.revisions)-2]
		}
		if f.rollbackDrops {
			f.current = state{installed: true}
		}
		f.revisions = append(f.revisions, f.current)
	case "uninstall":
		f.current = state{}
		f.revisions = nil
	}
}

func (f *fakeCluster) splitHelm(args []string) {
	switch args[0] {
	case "install", "upgrade":
		f.split = splitState{installed: true, egress: slices.Contains(args, "global.networkPolicy.egress.enabled=true")}
		f.splitRevisions = append(f.splitRevisions, f.split)
	case "rollback":
		f.split = f.splitRevisions[len(f.splitRevisions)-2]
		f.splitRevisions = append(f.splitRevisions, f.split)
	case "uninstall":
		f.split = splitState{}
		f.splitRevisions = nil
	}
}

func (f *fakeCluster) kubectl(args []string) (string, error) {
	switch {
	case args[0] == "get" && args[1] == "namespace":
		if slices.Contains(f.existing, args[2]) {
			return "namespace/" + args[2], nil
		}
		return "", nil
	case args[0] == "label":
		f.label(args[2], args[3])
		return "", nil
	case args[0] == "get" && args[1] == "networkpolicy" && args[3] == "netpol-allowed":
		if f.split.installed {
			return "netpol-split-connectors", nil
		}
		return "", nil
	case args[0] == "get" && args[1] == "networkpolicy":
		return strings.Join(f.policyNames(), " "), nil
	case args[0] == "get" && args[1] == "nodes":
		return nodeIP, nil
	case args[0] == "get" && args[1] == "pod" && args[2] == "netpol-probe":
		return map[string]string{"netpol": "10.1.0.0", "netpol-outsider": "10.1.0.1", "netpol-allowed": "10.1.0.2"}[args[4]], nil
	case args[0] == "get" && args[1] == "pod" && strings.Contains(strings.Join(args, " "), "instance=netpol-split"):
		if !f.split.installed {
			return "", nil
		}
		if strings.Contains(args[len(args)-1], "metadata.name") {
			return "netpol-split-connectors-0", nil
		}
		return "10.2.0.2", nil
	case args[0] == "get" && args[1] == "pod" && strings.Contains(args[len(args)-1], "metadata.name"):
		return "netpol-zeebe-0", nil
	case args[0] == "get" && args[1] == "pod":
		if strings.Contains(strings.Join(args, " "), "component=connectors") {
			return "10.0.0.2", nil
		}
		return "10.0.0.1", nil
	case args[0] == "exec" && args[1] == "netpol-split-connectors-0":
		return "", f.splitExec(args)
	case args[0] == "exec" && args[1] != "netpol-probe":
		return "", f.egressExec(args)
	case args[0] == "exec":
		return "", f.exec(args)
	}
	return "", nil
}

func (f *fakeCluster) label(namespace, label string) {
	if f.labeled == nil {
		f.labeled = map[string]bool{}
	}
	if strings.HasSuffix(label, "-") {
		if !f.stickyLabel {
			delete(f.labeled, namespace)
		}
		return
	}
	f.labeled[namespace] = label == "camunda.io/tenant-of=netpol"
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
	if ip == "10.2.0.2" {
		return f.splitIngress(from)
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
	if f.current.extraRules && from == "netpol-allowed" && (port == "8080" || f.leakyExtra) {
		return true
	}
	clientPort := port == "8080" || (component == "zeebe" && port == "26500") || f.leakyAllowed
	if f.current.allowedNs && from == "netpol-allowed" && clientPort {
		return component == "zeebe" || f.current.connectorsNs
	}
	return f.current.selector && f.labeled[from] && clientPort
}

func (f *fakeCluster) egressExec(args []string) error {
	targets := map[string]string{"10.0.0.2": "connectors", "10.1.0.1": "outsider", "10.1.0.2": "allowed", "10.2.0.2": "allowed", nodeIP: "external"}
	if f.externalIP != "" {
		targets[f.externalIP] = "external"
	}
	target := targets[args[len(args)-2]]
	if args[5] == "nslookup" {
		target = "dns"
	}
	if f.dnsBlocked && target == "dns" && f.current.egress {
		return errors.New("blocked")
	}
	if !f.enforcing || f.egressIgnored || !f.current.egress || target == "connectors" || target == "dns" || ((f.current.egressExtra || f.current.egressAllowed) && target == "allowed") {
		return nil
	}
	return errors.New("blocked")
}

func (f *fakeCluster) splitIngress(from string) error {
	if !f.enforcing || from == "netpol-allowed" || (from == "netpol" && !f.current.egress) || f.splitLeaky {
		return nil
	}
	if from == "netpol" && f.current.egressAllowed {
		return nil
	}
	return errors.New("blocked")
}

func (f *fakeCluster) splitExec(args []string) error {
	ip, port := args[len(args)-2], args[len(args)-1]
	if f.enforcing && f.split.egress && f.splitEgressBlock {
		return errors.New("blocked")
	}
	if ip == "10.0.0.1" && f.allowed("netpol-allowed", "zeebe", port) {
		return nil
	}
	return errors.New("blocked")
}

func testRunner(f *fakeCluster) runner {
	cfg := config{chartDir: "/chart", scenarioDir: "/chart/s", namespace: "netpol", release: "netpol", namespace2: "netpol-allowed", release2: "netpol-split", timeout: "1m", externalPort: nodeExternalPort, retries: 3}
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

func TestRunFailsWhenEgressIsNotEnforced(t *testing.T) {
	err := testRunner(&fakeCluster{enforcing: true, egressIgnored: true}).run(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "S7") || !strings.Contains(err.Error(), "outsider-probe") {
		t.Fatalf("expected S7 failure, got %v", err)
	}
}

func TestRunFailsWhenEgressBlocksDNS(t *testing.T) {
	err := testRunner(&fakeCluster{enforcing: true, dnsBlocked: true}).run(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "S7") || !strings.Contains(err.Error(), "dns") {
		t.Fatalf("expected S7 DNS failure, got %v", err)
	}
}

func TestRunFailsWhenAllowedNamespacesOpenInternalPorts(t *testing.T) {
	err := testRunner(&fakeCluster{enforcing: true, leakyAllowed: true}).run(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "S10") || !strings.Contains(err.Error(), ":9600") {
		t.Fatalf("expected S10 failure on 9600, got %v", err)
	}
}

func TestRunFailsWhenLabelRemovalIsNotEnforced(t *testing.T) {
	err := testRunner(&fakeCluster{enforcing: true, stickyLabel: true}).run(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "S11 removing") || !strings.Contains(err.Error(), "netpol-outsider") {
		t.Fatalf("expected S11 label removal failure, got %v", err)
	}
}

func TestRunFailsWhenComponentListDoesNotReplaceGlobal(t *testing.T) {
	err := testRunner(&fakeCluster{enforcing: true, ignoreReplace: true}).run(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "S12") || !strings.Contains(err.Error(), "connectors:8080") {
		t.Fatalf("expected S12 failure on connectors, got %v", err)
	}
}

func TestRunLabelsAndUnlabelsTheOutsiderNamespace(t *testing.T) {
	f := &fakeCluster{enforcing: true}
	if err := testRunner(f).run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, want := range []string{
		"kubectl label namespace netpol-outsider camunda.io/tenant-of=netpol --overwrite",
		"kubectl label namespace netpol-outsider camunda.io/tenant-of- --overwrite",
	} {
		if !slices.Contains(f.calls, want) {
			t.Fatalf("missing %q", want)
		}
	}
	unlabel := slices.Index(f.calls, "kubectl label namespace netpol-outsider camunda.io/tenant-of- --overwrite")
	for _, c := range f.calls[unlabel:] {
		if strings.Contains(c, "values-egress-allowed-namespaces.yaml") {
			break
		}
		if strings.HasPrefix(c, "helm upgrade") && !strings.Contains(c, "values-allowed-component-override.yaml") {
			t.Fatalf("unexpected upgrade after label removal: %s", c)
		}
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
	env := map[string]string{"CHART_DIR": "/c", "RELEASE": "r", "EXTERNAL_TARGET": "203.0.113.10:8443"}
	cfg, err := parseConfig([]string{"-install-calico", "ns"}, func(k string) string { return env[k] }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.installCalico || cfg.namespace != "ns" || cfg.release != "r" || cfg.namespace2 != "ns-allowed" || cfg.release2 != "netpol-split" || cfg.scenarioDir != "/c/test/integration/scenarios/networkpolicy" || !strings.Contains(cfg.calicoManifest, calicoVersion) || cfg.externalHost != "203.0.113.10" || cfg.externalPort != 8443 {
		t.Fatalf("unexpected config %+v", cfg)
	}
	if _, err := parseConfig([]string{"a", "b"}, func(string) string { return "" }, nil); err == nil {
		t.Fatal("expected error for two namespaces")
	}
	def, err := parseConfig(nil, func(string) string { return "" }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if def.externalHost != "" || def.externalPort != nodeExternalPort {
		t.Fatalf("unexpected default external target %+v", def)
	}
}

func TestRunFailsWhenEgressAllowedNamespacesAreIgnored(t *testing.T) {
	err := testRunner(&fakeCluster{enforcing: true, egressNsIgnored: true}).run(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "S13") || !strings.Contains(err.Error(), "allowed-probe") {
		t.Fatalf("expected S13 failure, got %v", err)
	}
}

func TestRunFailsWhenSplitEgressBlocksTheHub(t *testing.T) {
	err := testRunner(&fakeCluster{enforcing: true, splitEgressBlock: true}).run(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "S14") || !strings.Contains(err.Error(), "netpol-split-connectors -> netpol/netpol-zeebe-broker:8080") {
		t.Fatalf("expected S14 cross-release failure, got %v", err)
	}
}

func TestRunFailsWhenSplitIngressLeaks(t *testing.T) {
	err := testRunner(&fakeCluster{enforcing: true, splitLeaky: true}).run(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "S14") || !strings.Contains(err.Error(), "netpol-outsider/netpol-probe") {
		t.Fatalf("expected S14 outsider failure, got %v", err)
	}
}

func TestRunFailsWhenRollbackDropsEnforcement(t *testing.T) {
	err := testRunner(&fakeCluster{enforcing: true, rollbackDrops: true}).run(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "S5") && !strings.HasPrefix(err.Error(), "S9") && !strings.HasPrefix(err.Error(), "S15") {
		t.Fatalf("expected rollback failure, got %v", err)
	}
}

func TestRunDeploysTheSplitReleaseInOrder(t *testing.T) {
	f := &fakeCluster{enforcing: true}
	if err := testRunner(f).run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	index := func(prefix, contains string) int {
		return slices.IndexFunc(f.calls, func(c string) bool { return strings.HasPrefix(c, prefix) && strings.Contains(c, contains) })
	}
	receiver := index("helm upgrade netpol /chart", "values-allowed-namespaces.yaml")
	install := index("helm install netpol-split /chart --namespace netpol-allowed", "values-split-hub.yaml")
	splitEgress := index("helm upgrade netpol-split", "global.networkPolicy.egress.enabled=true")
	hubEgress := slices.IndexFunc(f.calls, func(c string) bool {
		return strings.HasPrefix(c, "helm upgrade netpol /chart") && strings.Contains(c, "values-allowed-namespaces.yaml") && strings.Contains(c, "values-egress-allowed-namespaces.yaml")
	})
	if receiver < 0 || receiver >= install || install >= splitEgress || splitEgress >= hubEgress {
		t.Fatalf("unexpected order receiver=%d install=%d splitEgress=%d hubEgress=%d", receiver, install, splitEgress, hubEgress)
	}
	if !slices.Contains(f.calls, "helm uninstall netpol-split --namespace netpol-allowed --ignore-not-found --wait") {
		t.Fatal("cleanup did not uninstall the split release")
	}
}

func TestParseExternalTarget(t *testing.T) {
	cases := []struct {
		raw     string
		host    string
		port    int
		wantErr bool
	}{
		{raw: "", host: "", port: nodeExternalPort},
		{raw: "external.example.com", host: "external.example.com", port: defaultExternalPort},
		{raw: "external.example.com:8443", host: "external.example.com", port: 8443},
		{raw: "[2001:db8::1]:8443", host: "2001:db8::1", port: 8443},
		{raw: "2001:db8::1", host: "2001:db8::1", port: defaultExternalPort},
		{raw: "external.example.com:0", wantErr: true},
		{raw: "external.example.com:70000", wantErr: true},
		{raw: "external.example.com:http", wantErr: true},
		{raw: ":8443", wantErr: true},
		{raw: "a:b:c", wantErr: true},
	}
	for _, c := range cases {
		host, port, err := parseExternalTarget(c.raw)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseExternalTarget(%q): expected error", c.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseExternalTarget(%q): %v", c.raw, err)
			continue
		}
		if host != c.host || port != c.port {
			t.Errorf("parseExternalTarget(%q) = %q:%d, want %q:%d", c.raw, host, port, c.host, c.port)
		}
	}
}

func TestRunUsesExternalTargetOverride(t *testing.T) {
	f := &fakeCluster{enforcing: true, externalIP: "external.example.com"}
	r := testRunner(f)
	r.cfg.externalHost = "external.example.com"
	r.cfg.externalPort = defaultExternalPort
	if err := r.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if slices.ContainsFunc(f.calls, func(c string) bool { return strings.Contains(c, "get nodes") }) {
		t.Fatalf("override still resolved the node InternalIP: %v", f.calls)
	}
	if !slices.ContainsFunc(f.calls, func(c string) bool {
		return strings.Contains(c, "nc -z -w 2 external.example.com "+strconv.Itoa(defaultExternalPort))
	}) {
		t.Fatalf("external probe did not target the override: %v", f.calls)
	}
}
