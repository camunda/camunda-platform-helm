// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kube

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"scripts/camunda-core/pkg/executil"
)

// ExecRunner runs commands in pods via the kubectl binary.
//
// kubectl rather than the client-go remotecommand API: the Client does not
// retain a rest.Config, and the lifecycle scripts this sits alongside already
// shell out to kubectl, so the auth path is the one already proven in CI.
type ExecRunner struct {
	KubeContext string
}

// ExecInPod runs command under sh -c in the first ready pod matching selector
// and returns its stdout.
func (e ExecRunner) ExecInPod(ctx context.Context, namespace, selector, container, command string) (string, error) {
	args := []string{"exec", "-n", namespace}
	if e.KubeContext != "" {
		args = append([]string{"--context", e.KubeContext}, args...)
	}

	pod, err := e.firstReadyPod(ctx, namespace, selector)
	if err != nil {
		return "", err
	}
	args = append(args, pod)
	if container != "" {
		args = append(args, "-c", container)
	}
	args = append(args, "--", "sh", "-c", command)

	out, err := executil.RunCommandCapture(ctx, "kubectl", args, nil, "")
	if err != nil {
		return "", fmt.Errorf("exec in %s/%s: %w", namespace, pod, err)
	}
	return string(out), nil
}

// firstReadyPod resolves a selector to a pod name, preferring a ready pod so a
// probe does not run against a container that is still starting.
func (e ExecRunner) firstReadyPod(ctx context.Context, namespace, selector string) (string, error) {
	client, err := NewClient("", e.KubeContext)
	if err != nil {
		return "", err
	}
	pods, err := client.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return "", fmt.Errorf("list pods %q in %s: %w", selector, namespace, err)
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pod matches %q in %s", selector, namespace)
	}
	for _, p := range pods.Items {
		for _, c := range p.Status.Conditions {
			if c.Type == "Ready" && c.Status == "True" {
				return p.Name, nil
			}
		}
	}
	names := make([]string, 0, len(pods.Items))
	for _, p := range pods.Items {
		names = append(names, p.Name)
	}
	return "", fmt.Errorf("no ready pod matches %q in %s (found %s)", selector, namespace, strings.Join(names, ", "))
}
