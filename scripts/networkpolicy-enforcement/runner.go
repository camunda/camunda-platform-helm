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
	"fmt"
	"path/filepath"
	"slices"
)

const (
	zeebe      = "zeebe-broker"
	connectors = "connectors"
)

var componentPorts = map[string][]int{
	zeebe:      {8080, 9600, 26500, 26501, 26502},
	connectors: {8080},
}

type source int

const (
	inside source = iota
	outsider
	allowed
)

type reach struct {
	from      source
	component string
	ports     []int
	want      bool
}

type target int

const (
	targetInside target = iota
	targetOutsider
	targetAllowed
	targetConnectors
	targetDNS
	targetExternal
)

const probePort = 8080

func (t target) String() string {
	return [...]string{"inside-probe", "outsider-probe", "allowed-probe", "connectors", "dns", "external"}[t]
}

type egress struct {
	to   target
	want bool
}

type step struct {
	name          string
	action        func(context.Context) error
	policies      []string
	splitPolicies []string
	reaches       []reach
	egress        []egress
	cross         []crossReach
}

type runner struct {
	cfg     config
	command commandFunc
	log     func(string, ...any)
}

func newRunner(cfg config, command commandFunc) runner {
	return runner{cfg: cfg, command: command, log: func(format string, args ...any) { fmt.Printf(format+"\n", args...) }}
}

func (r runner) namespaces() []string {
	return []string{r.cfg.namespace, r.cfg.namespace + "-outsider", r.cfg.namespace + "-allowed"}
}

func (r runner) sourceNamespace(s source) string {
	return r.namespaces()[s]
}

func (r runner) run(ctx context.Context) error {
	if err := r.refuseExisting(ctx); err != nil {
		return err
	}
	if r.cfg.installCalico {
		if err := r.installCalico(ctx); err != nil {
			return err
		}
	}
	defer r.cleanup(context.WithoutCancel(ctx))
	for _, namespace := range r.namespaces() {
		if err := r.createProbeNamespace(ctx, namespace); err != nil {
			return err
		}
	}
	for _, s := range r.steps() {
		if err := r.runStep(ctx, s); err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
		r.log("PASS %s", s.name)
	}
	return nil
}

func (r runner) refuseExisting(ctx context.Context) error {
	for _, namespace := range r.namespaces() {
		exists, err := r.namespaceExists(ctx, namespace)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("namespace %s already exists, refusing to run", namespace)
		}
	}
	return nil
}

func (r runner) cleanup(ctx context.Context) {
	_ = r.helm(ctx, "uninstall", r.cfg.release2, "--namespace", r.cfg.namespace2, "--ignore-not-found", "--wait")
	_ = r.helm(ctx, "uninstall", r.cfg.release, "--namespace", r.cfg.namespace, "--ignore-not-found", "--wait")
	for _, namespace := range r.namespaces() {
		_, _ = r.kubectl(ctx, "delete", "namespace", namespace, "--ignore-not-found", "--wait=false")
	}
}

func (r runner) runStep(ctx context.Context, s step) error {
	if err := s.action(ctx); err != nil {
		return err
	}
	if err := r.assertPolicies(ctx, r.cfg.namespace, r.cfg.release, s.policies); err != nil {
		return err
	}
	if s.splitPolicies != nil {
		if err := r.assertPolicies(ctx, r.cfg.namespace2, r.cfg.release2, s.splitPolicies); err != nil {
			return err
		}
	}
	if err := r.checkReaches(ctx, s); err != nil {
		return err
	}
	for _, c := range s.cross {
		if err := r.expectCross(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func (r runner) checkReaches(ctx context.Context, s step) error {
	for _, c := range s.reaches {
		if err := r.checkReach(ctx, c); err != nil {
			return err
		}
	}
	for _, e := range s.egress {
		if err := r.expectEgress(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

func (r runner) checkReach(ctx context.Context, c reach) error {
	for _, port := range c.ports {
		if !c.want {
			if err := r.expectReach(ctx, r.sourceNamespace(inside), c.component, port, true); err != nil {
				return fmt.Errorf("precondition: %w", err)
			}
		}
		if err := r.expectReach(ctx, r.sourceNamespace(c.from), c.component, port, c.want); err != nil {
			return err
		}
	}
	return nil
}

func (r runner) values(files ...string) []string {
	args := []string{"--values", filepath.Join(r.cfg.scenarioDir, "values-slim.yaml")}
	for _, f := range files {
		args = append(args, "--values", filepath.Join(r.cfg.scenarioDir, f))
	}
	return args
}

func (r runner) deploy(verb string, extra ...string) func(context.Context) error {
	return func(ctx context.Context) error {
		args := []string{verb, r.cfg.release, r.cfg.chartDir, "--namespace", r.cfg.namespace, "--wait", "--timeout", r.cfg.timeout}
		return r.helm(ctx, append(args, extra...)...)
	}
}

func (r runner) rollback(ctx context.Context) error {
	return r.helm(ctx, "rollback", r.cfg.release, "1", "--namespace", r.cfg.namespace, "--wait", "--timeout", r.cfg.timeout)
}

func (r runner) reinstall(ctx context.Context) error {
	if err := r.helm(ctx, "uninstall", r.cfg.release, "--namespace", r.cfg.namespace, "--wait"); err != nil {
		return err
	}
	return r.deploy("install", append(r.values(), "--set", "global.networkPolicy.enabled=true")...)(ctx)
}

func all(from source, want bool) []reach {
	return []reach{
		{from: from, component: zeebe, ports: componentPorts[zeebe], want: want},
		{from: from, component: connectors, ports: componentPorts[connectors], want: want},
	}
}

func egressOpen() []egress {
	return []egress{{targetConnectors, true}, {targetDNS, true}, {targetOutsider, true}, {targetAllowed, true}, {targetExternal, true}}
}

func (r runner) steps() []step {
	return slices.Concat(r.ingressSteps(), r.egressSteps(), r.allowedNamespaceSteps(), r.splitSteps())
}

func (r runner) ingressSteps() []step {
	both := []string{"zeebe", "connectors"}
	enforced := append(all(inside, true), all(outsider, false)...)
	return []step{
		{name: "S1 install with policies off", action: r.deploy("install", r.values()...), reaches: append(all(inside, true), all(outsider, true)...)},
		{name: "S2 upgrade enables policies", action: r.deploy("upgrade", append(r.values(), "--set", "global.networkPolicy.enabled=true")...), policies: both, reaches: enforced},
		{name: "S3 extraRules open the allowed namespace on 8080 only", action: r.deploy("upgrade", r.values("values-extrarules.yaml")...), policies: both, reaches: append(all(outsider, false),
			reach{from: allowed, component: zeebe, ports: []int{8080}, want: true},
			reach{from: allowed, component: zeebe, ports: []int{9600, 26500, 26501, 26502}, want: false},
			reach{from: allowed, component: connectors, ports: []int{8080}, want: true},
		)},
		{name: "S4 component toggle removes only the connectors policy", action: r.deploy("upgrade", append(r.values("values-extrarules.yaml"), "--set", "connectors.networkPolicy.enabled=false")...), policies: []string{"zeebe"}, reaches: []reach{
			{from: outsider, component: zeebe, ports: componentPorts[zeebe], want: false},
			{from: outsider, component: connectors, ports: componentPorts[connectors], want: true},
		}},
		{name: "S5 rollback to the first revision deletes the policies", action: r.rollback, reaches: all(outsider, true)},
	}
}

func (r runner) egressSteps() []step {
	both := []string{"zeebe", "connectors"}
	enforced := append(all(inside, true), all(outsider, false)...)
	return []step{
		{name: "S6 fresh install with policies on leaves egress open", action: r.reinstall, policies: both, reaches: enforced, egress: egressOpen()},
		{name: "S7 egress blocks everything but the namespace and DNS", action: r.deploy("upgrade", r.values("values-egress.yaml")...), policies: both, reaches: enforced, egress: []egress{
			{targetConnectors, true}, {targetDNS, true}, {targetOutsider, false}, {targetAllowed, false}, {targetExternal, false},
		}},
		{name: "S8 egress extraRules open the allowed namespace on 8080", action: r.deploy("upgrade", r.values("values-egress.yaml", "values-egress-extrarules.yaml")...), policies: both, reaches: enforced, egress: []egress{
			{targetConnectors, true}, {targetDNS, true}, {targetAllowed, true}, {targetOutsider, false}, {targetExternal, false},
		}},
		{name: "S9 rollback removes the egress rules", action: r.rollback, policies: both, reaches: enforced, egress: egressOpen()},
	}
}

func (r runner) allowedNamespaceSteps() []step {
	both := []string{"zeebe", "connectors"}
	clientPorts := []int{8080, 26500}
	internalPorts := []int{9600, 26501, 26502}
	tenantLabel := "camunda.io/tenant-of=" + r.cfg.namespace
	return []step{
		{name: "S10 allowedNamespaces opens the client ports to the allowed namespace", action: r.deploy("upgrade", r.values("values-allowed-namespaces.yaml")...), policies: both, reaches: append(append(all(inside, true), all(outsider, false)...),
			reach{from: allowed, component: zeebe, ports: clientPorts, want: true},
			reach{from: allowed, component: zeebe, ports: internalPorts, want: false},
			reach{from: allowed, component: connectors, ports: componentPorts[connectors], want: true},
		)},
		{name: "S11 a namespaceSelector entry follows the namespace label", action: r.then(r.labelNamespace(r.sourceNamespace(outsider), tenantLabel), r.deploy("upgrade", r.values("values-allowed-selector.yaml")...)), policies: both, reaches: []reach{
			{from: outsider, component: zeebe, ports: clientPorts, want: true},
			{from: outsider, component: zeebe, ports: internalPorts, want: false},
			{from: allowed, component: zeebe, ports: clientPorts, want: false},
		}},
		{name: "S11 removing the namespace label blocks it again without an upgrade", action: r.labelNamespace(r.sourceNamespace(outsider), "camunda.io/tenant-of-"), policies: both, reaches: all(outsider, false)},
		{name: "S12 an empty component list replaces the global list", action: r.deploy("upgrade", r.values("values-allowed-component-override.yaml")...), policies: both, reaches: []reach{
			{from: allowed, component: zeebe, ports: clientPorts, want: true},
			{from: allowed, component: connectors, ports: componentPorts[connectors], want: false},
		}},
	}
}

func (r runner) then(actions ...func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		for _, action := range actions {
			if err := action(ctx); err != nil {
				return err
			}
		}
		return nil
	}
}
