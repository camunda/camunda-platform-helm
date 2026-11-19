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
	"strconv"
	"time"
)

type endpoint struct {
	namespace string
	release   string
	component string
}

type crossReach struct {
	from  endpoint
	to    endpoint
	ports []int
	want  bool
}

func (e endpoint) String() string {
	if e.release == "" {
		return e.namespace + "/netpol-probe"
	}
	return e.namespace + "/" + e.release + "-" + e.component
}

func (r runner) hub(component string) endpoint {
	return endpoint{namespace: r.cfg.namespace, release: r.cfg.release, component: component}
}

func (r runner) splitConnectors() endpoint {
	return endpoint{namespace: r.cfg.namespace2, release: r.cfg.release2, component: connectors}
}

func (r runner) probe(s source) endpoint {
	return endpoint{namespace: r.sourceNamespace(s)}
}

func (r runner) endpointPod(ctx context.Context, e endpoint) (string, error) {
	if e.release == "" {
		return "netpol-probe", nil
	}
	return r.componentPodIn(ctx, e.namespace, e.release, e.component)
}

func (r runner) endpointIP(ctx context.Context, e endpoint) (string, error) {
	if e.release == "" {
		return r.jsonpath(ctx, "probe IP", "pod", "netpol-probe", "--namespace", e.namespace, "-o", "jsonpath={.status.podIP}")
	}
	return r.podIPIn(ctx, e.namespace, e.release, e.component)
}

func (r runner) crossReachable(ctx context.Context, from, to endpoint, port int) (bool, error) {
	pod, err := r.endpointPod(ctx, from)
	if err != nil {
		return false, err
	}
	ip, err := r.endpointIP(ctx, to)
	if err != nil {
		return false, err
	}
	_, err = r.kubectl(ctx, "exec", pod, "--namespace", from.namespace, "--", "nc", "-z", "-w", "2", ip, strconv.Itoa(port))
	return err == nil, nil
}

func (r runner) expectCross(ctx context.Context, c crossReach) error {
	for _, port := range c.ports {
		if err := r.eventually(ctx, func() error {
			got, err := r.crossReachable(ctx, c.from, c.to, port)
			if err != nil {
				return err
			}
			if got != c.want {
				return fmt.Errorf("%s -> %s:%d reachable=%t, expected %t", c.from, c.to, port, got, c.want)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r runner) eventually(ctx context.Context, check func() error) error {
	var lastErr error
	for attempt := 0; attempt < r.cfg.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return errors.Join(lastErr, ctx.Err())
			case <-time.After(r.cfg.retryInterval):
			}
		}
		if lastErr = check(); lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func (r runner) deploySplit(verb string, extra ...string) func(context.Context) error {
	return func(ctx context.Context) error {
		hubGateway := r.cfg.release + "-zeebe-gateway." + r.cfg.namespace + ".svc"
		args := []string{verb, r.cfg.release2, r.cfg.chartDir, "--namespace", r.cfg.namespace2, "--wait", "--timeout", r.cfg.timeout}
		args = append(args, r.values("values-split-hub.yaml")...)
		args = append(args,
			"--set", "connectors.env[0].name=CAMUNDA_CLIENT_GRPCADDRESS",
			"--set", "connectors.env[0].value=http://"+hubGateway+":26500",
			"--set", "connectors.env[1].name=CAMUNDA_CLIENT_RESTADDRESS",
			"--set", "connectors.env[1].value=http://"+hubGateway+":8080",
		)
		return r.helm(ctx, append(args, extra...)...)
	}
}

func (r runner) rollbackPrevious(ctx context.Context) error {
	if err := r.helm(ctx, "rollback", r.cfg.release2, "--namespace", r.cfg.namespace2, "--wait", "--timeout", r.cfg.timeout); err != nil {
		return err
	}
	return r.helm(ctx, "rollback", r.cfg.release, "--namespace", r.cfg.namespace, "--wait", "--timeout", r.cfg.timeout)
}

func (r runner) splitReaches() []crossReach {
	return []crossReach{
		{from: r.splitConnectors(), to: r.hub(zeebe), ports: []int{8080, 26500}, want: true},
		{from: r.splitConnectors(), to: r.hub(zeebe), ports: []int{26501, 26502}, want: false},
		{from: r.hub(zeebe), to: r.splitConnectors(), ports: []int{8080}, want: true},
		{from: r.probe(allowed), to: r.splitConnectors(), ports: []int{8080}, want: true},
		{from: r.probe(outsider), to: r.splitConnectors(), ports: []int{8080}, want: false},
	}
}

func (r runner) splitSteps() []step {
	both := []string{"zeebe", "connectors"}
	enforced := append(all(inside, true), all(outsider, false)...)
	return []step{
		{name: "S13 egress allowedNamespaces opens the allowed namespace only", action: r.deploy("upgrade", r.values("values-egress-allowed-namespaces.yaml")...), policies: both, reaches: enforced, egress: []egress{
			{targetConnectors, true}, {targetDNS, true}, {targetAllowed, true}, {targetOutsider, false}, {targetExternal, false},
		}},
		{name: "S14 two releases split across namespaces reach each other only", action: r.then(
			r.deploy("upgrade", r.values("values-allowed-namespaces.yaml")...),
			r.deploySplit("install"),
			r.deploySplit("upgrade", "--set", "global.networkPolicy.egress.enabled=true"),
			r.deploy("upgrade", r.values("values-allowed-namespaces.yaml", "values-egress-allowed-namespaces.yaml")...),
		), policies: both, splitPolicies: []string{"connectors"}, reaches: enforced, cross: r.splitReaches(), egress: []egress{
			{targetDNS, true}, {targetAllowed, true}, {targetOutsider, false}, {targetExternal, false},
		}},
		{name: "S15 rollback of both releases keeps ingress enforced and reopens egress", action: r.rollbackPrevious, policies: both, splitPolicies: []string{"connectors"}, reaches: enforced, cross: r.splitReaches(), egress: egressOpen()},
	}
}
