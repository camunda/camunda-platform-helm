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
	"slices"
	"strconv"
	"strings"
	"time"
)

func (r runner) kubectl(ctx context.Context, args ...string) (string, error) {
	return r.command(ctx, "kubectl", args...)
}

func (r runner) helm(ctx context.Context, args ...string) error {
	if _, err := r.command(ctx, "helm", args...); err != nil {
		return fmt.Errorf("helm %s: %w", args[0], err)
	}
	return nil
}

func (r runner) namespaceExists(ctx context.Context, namespace string) (bool, error) {
	out, err := r.kubectl(ctx, "get", "namespace", namespace, "--ignore-not-found", "-o", "name")
	if err != nil {
		return false, fmt.Errorf("check namespace %s: %w", namespace, err)
	}
	return out != "", nil
}

func (r runner) installCalico(ctx context.Context) error {
	if _, err := r.kubectl(ctx, "apply", "--server-side", "-f", r.cfg.calicoManifest); err != nil {
		return fmt.Errorf("apply Calico manifest: %w", err)
	}
	if _, err := r.kubectl(ctx, "rollout", "status", "daemonset/calico-node", "--namespace", "kube-system", "--timeout", r.cfg.timeout); err != nil {
		return fmt.Errorf("wait for calico-node: %w", err)
	}
	if _, err := r.kubectl(ctx, "wait", "--for=condition=Ready", "nodes", "--all", "--timeout", r.cfg.timeout); err != nil {
		return fmt.Errorf("wait for nodes: %w", err)
	}
	return nil
}

func (r runner) createProbeNamespace(ctx context.Context, namespace string) error {
	if _, err := r.kubectl(ctx, "create", "namespace", namespace); err != nil {
		return fmt.Errorf("create namespace %s: %w", namespace, err)
	}
	if _, err := r.kubectl(ctx, "apply", "--namespace", namespace, "-f", filepath.Join(r.cfg.scenarioDir, "probe.yaml")); err != nil {
		return fmt.Errorf("create probe in %s: %w", namespace, err)
	}
	if _, err := r.kubectl(ctx, "wait", "--for=condition=Ready", "pod/netpol-probe", "--namespace", namespace, "--timeout", r.cfg.timeout); err != nil {
		return fmt.Errorf("wait for probe in %s: %w", namespace, err)
	}
	return nil
}

func (r runner) labelNamespace(namespace, label string) func(context.Context) error {
	return func(ctx context.Context) error {
		if _, err := r.kubectl(ctx, "label", "namespace", namespace, label, "--overwrite"); err != nil {
			return fmt.Errorf("label namespace %s: %w", namespace, err)
		}
		return nil
	}
}

func (r runner) policies(ctx context.Context, namespace string) ([]string, error) {
	out, err := r.kubectl(ctx, "get", "networkpolicy", "--namespace", namespace, "-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return nil, fmt.Errorf("list NetworkPolicies: %w", err)
	}
	names := strings.Fields(out)
	slices.Sort(names)
	return names, nil
}

func (r runner) assertPolicies(ctx context.Context, namespace, release string, want []string) error {
	got, err := r.policies(ctx, namespace)
	if err != nil {
		return err
	}
	expected := make([]string, 0, len(want))
	for _, name := range want {
		expected = append(expected, release+"-"+name)
	}
	slices.Sort(expected)
	if !slices.Equal(got, expected) {
		return fmt.Errorf("NetworkPolicies are %v, expected %v", got, expected)
	}
	return nil
}

func (r runner) podIP(ctx context.Context, component string) (string, error) {
	return r.podIPIn(ctx, r.cfg.namespace, r.cfg.release, component)
}

func (r runner) podIPIn(ctx context.Context, namespace, release, component string) (string, error) {
	selector := "app.kubernetes.io/instance=" + release + ",app.kubernetes.io/component=" + component
	ip, err := r.kubectl(ctx, "get", "pod", "--namespace", namespace, "-l", selector, "-o", "jsonpath={.items[0].status.podIP}")
	if err != nil || ip == "" {
		return "", errors.Join(fmt.Errorf("no pod IP for %s", component), err)
	}
	return ip, nil
}

func (r runner) reachable(ctx context.Context, from, ip string, port int) bool {
	_, err := r.kubectl(ctx, "exec", "netpol-probe", "--namespace", from, "--", "nc", "-z", "-w", "2", ip, strconv.Itoa(port))
	return err == nil
}

func (r runner) expectReach(ctx context.Context, from, component string, port int, want bool) error {
	var lastErr error
	for attempt := 0; attempt < r.cfg.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return errors.Join(lastErr, ctx.Err())
			case <-time.After(r.cfg.retryInterval):
			}
		}
		ip, err := r.podIP(ctx, component)
		if err != nil {
			lastErr = err
			continue
		}
		if r.reachable(ctx, from, ip, port) == want {
			return nil
		}
		lastErr = fmt.Errorf("%s -> %s:%d reachable=%t, expected %t", from, component, port, !want, want)
	}
	return lastErr
}

func (r runner) jsonpath(ctx context.Context, what string, args ...string) (string, error) {
	out, err := r.kubectl(ctx, append([]string{"get"}, args...)...)
	if err != nil || out == "" {
		return "", errors.Join(fmt.Errorf("no %s", what), err)
	}
	return out, nil
}

func (r runner) componentPod(ctx context.Context, component string) (string, error) {
	return r.componentPodIn(ctx, r.cfg.namespace, r.cfg.release, component)
}

func (r runner) componentPodIn(ctx context.Context, namespace, release, component string) (string, error) {
	selector := "app.kubernetes.io/instance=" + release + ",app.kubernetes.io/component=" + component
	return r.jsonpath(ctx, "pod for "+component, "pod", "--namespace", namespace, "-l", selector, "-o", "jsonpath={.items[0].metadata.name}")
}

func (r runner) egressTarget(ctx context.Context, t target) (string, int, error) {
	switch t {
	case targetConnectors:
		ip, err := r.podIP(ctx, connectors)
		return ip, componentPorts[connectors][0], err
	case targetExternal:
		return r.externalTarget(ctx)
	default:
		ip, err := r.jsonpath(ctx, "probe IP", "pod", "netpol-probe", "--namespace", r.sourceNamespace(source(t-targetInside)), "-o", "jsonpath={.status.podIP}")
		return ip, probePort, err
	}
}

// externalTarget resolves the destination of the external egress probe. An
// EXTERNAL_TARGET host overrides the default, the first node's InternalIP.
func (r runner) externalTarget(ctx context.Context) (string, int, error) {
	if r.cfg.externalHost != "" {
		return r.cfg.externalHost, r.cfg.externalPort, nil
	}
	ip, err := r.jsonpath(ctx, "node InternalIP", "nodes", "-o", `jsonpath={.items[0].status.addresses[?(@.type=="InternalIP")].address}`)
	return ip, r.cfg.externalPort, err
}

func (r runner) egressReachable(ctx context.Context, pod string, t target) (bool, error) {
	if t == targetDNS {
		_, err := r.kubectl(ctx, "exec", pod, "--namespace", r.cfg.namespace, "--", "nslookup", "-timeout=2", "kubernetes.default.svc.cluster.local")
		return err == nil, nil
	}
	ip, port, err := r.egressTarget(ctx, t)
	if err != nil {
		return false, err
	}
	_, err = r.kubectl(ctx, "exec", pod, "--namespace", r.cfg.namespace, "--", "nc", "-z", "-w", "2", ip, strconv.Itoa(port))
	return err == nil, nil
}

func (r runner) expectEgress(ctx context.Context, e egress) error {
	var lastErr error
	for attempt := 0; attempt < r.cfg.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return errors.Join(lastErr, ctx.Err())
			case <-time.After(r.cfg.retryInterval):
			}
		}
		pod, err := r.componentPod(ctx, zeebe)
		if err != nil {
			lastErr = err
			continue
		}
		got, err := r.egressReachable(ctx, pod, e.to)
		if err != nil {
			lastErr = err
			continue
		}
		if got == e.want {
			return nil
		}
		lastErr = fmt.Errorf("egress %s -> %s reachable=%t, expected %t", zeebe, e.to, got, e.want)
	}
	return lastErr
}
