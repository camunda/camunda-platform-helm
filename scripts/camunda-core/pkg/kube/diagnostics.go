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

package kube

import (
	"context"
	"fmt"
	"scripts/camunda-core/pkg/executil"
	"strings"
	"time"
)

// diagnosticTimeout is the per-command timeout for kubectl diagnostic calls.
const diagnosticTimeout = 10 * time.Second

// kubectlBaseArgs returns the common connection args (--context) for kubectl.
func kubectlBaseArgs(kubeContext string) []string {
	var args []string
	if kubeContext != "" {
		args = append(args, "--context", kubeContext)
	}
	return args
}

// runKubectl executes a kubectl command with a child timeout context and returns stdout.
// On error it returns empty string and the error — callers treat diagnostics as best-effort.
func runKubectl(ctx context.Context, args []string) (string, error) {
	return runKubectlTimeout(ctx, args, diagnosticTimeout)
}

// runKubectlTimeout is runKubectl with an explicit per-command budget, for calls
// that legitimately outlast diagnosticTimeout such as an in-pod exec that has to
// negotiate TLS before it can query anything.
func runKubectlTimeout(ctx context.Context, args []string, timeout time.Duration) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := executil.RunCommandBuffered(cmdCtx, "kubectl", args, nil, "")
	if err != nil {
		// Still return any partial output captured before the error.
		if output != nil && len(output.Stdout) > 0 {
			return strings.Join(output.Stdout, "\n"), err
		}
		return "", err
	}
	return strings.Join(output.Stdout, "\n"), nil
}

// GetPods returns the output of `kubectl get pods -n <namespace> -o wide`.
func GetPods(ctx context.Context, kubeContext, namespace string) (string, error) {
	args := append(kubectlBaseArgs(kubeContext), "get", "pods", "-n", namespace, "-o", "wide")
	return runKubectl(ctx, args)
}

// GetPodNames returns all pod names in the namespace.
func GetPodNames(ctx context.Context, kubeContext, namespace string) ([]string, error) {
	args := append(kubectlBaseArgs(kubeContext),
		"get", "pods", "-n", namespace,
		"--no-headers",
		"-o", "custom-columns=NAME:.metadata.name",
	)
	output, err := runKubectl(ctx, args)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		names = append(names, line)
	}

	return names, nil
}

// GetEvents returns the output of `kubectl get events -n <namespace> --sort-by=.lastTimestamp`.
func GetEvents(ctx context.Context, kubeContext, namespace string) (string, error) {
	args := append(kubectlBaseArgs(kubeContext), "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
	return runKubectl(ctx, args)
}

// GetPVCs returns the output of `kubectl get pvc -n <namespace> -o wide`.
// PVC state (bound/pending, capacity, storage class) is the key evidence for
// volume-mount and provisioning failures.
func GetPVCs(ctx context.Context, kubeContext, namespace string) (string, error) {
	args := append(kubectlBaseArgs(kubeContext), "get", "pvc", "-n", namespace, "-o", "wide")
	return runKubectl(ctx, args)
}

// GetServices returns the output of `kubectl get svc -n <namespace> -o wide`.
func GetServices(ctx context.Context, kubeContext, namespace string) (string, error) {
	args := append(kubectlBaseArgs(kubeContext), "get", "svc", "-n", namespace, "-o", "wide")
	return runKubectl(ctx, args)
}

// GetServicesYAML returns `kubectl get svc -n <namespace> -o yaml`. YAML is used
// rather than describe because describe omits appProtocol; the YAML carries both
// the per-port appProtocol and the selector, which together explain a Service that
// resolves to no backends.
func GetServicesYAML(ctx context.Context, kubeContext, namespace string) (string, error) {
	args := append(kubectlBaseArgs(kubeContext), "get", "svc", "-n", namespace, "-o", "yaml")
	return runKubectl(ctx, args)
}

// GetEndpoints returns the output of `kubectl get endpoints -n <namespace>`, which
// shows which pods actually back each Service.
func GetEndpoints(ctx context.Context, kubeContext, namespace string) (string, error) {
	args := append(kubectlBaseArgs(kubeContext), "get", "endpoints", "-n", namespace)
	return runKubectl(ctx, args)
}

// DescribePVCs returns the output of `kubectl describe pvc -n <namespace>`, which
// includes the events explaining why a claim is stuck (e.g., waiting for a consumer,
// provisioning errors, multi-attach conflicts).
func DescribePVCs(ctx context.Context, kubeContext, namespace string) (string, error) {
	args := append(kubectlBaseArgs(kubeContext), "describe", "pvc", "-n", namespace)
	return runKubectl(ctx, args)
}

// GetPodLogs returns the last tailLines of logs from all containers in a pod.
func GetPodLogs(ctx context.Context, kubeContext, namespace, pod string, tailLines int) (string, error) {
	args := append(kubectlBaseArgs(kubeContext),
		"logs", pod, "-n", namespace,
		"--tail", fmt.Sprintf("%d", tailLines),
		"--all-containers",
	)
	return runKubectl(ctx, args)
}

// GetPodLogsPrevious returns the last tailLines of logs from the previous
// (crashed) container instance. Empty when the pod never restarted.
func GetPodLogsPrevious(ctx context.Context, kubeContext, namespace, pod string, tailLines int) (string, error) {
	args := append(kubectlBaseArgs(kubeContext),
		"logs", pod, "-n", namespace,
		"--tail", fmt.Sprintf("%d", tailLines),
		"--all-containers", "--previous",
	)
	return runKubectl(ctx, args)
}

// DescribePod returns the output of `kubectl describe pod <pod> -n <namespace>`.
// The Events section is the key evidence for scheduling, mount, and image-pull
// failures on a pod that never became ready.
func DescribePod(ctx context.Context, kubeContext, namespace, pod string) (string, error) {
	args := append(kubectlBaseArgs(kubeContext), "describe", "pod", pod, "-n", namespace)
	return runKubectl(ctx, args)
}

// GetPodContainers returns the pod's init containers followed by its regular
// containers. Init containers come first so a caller iterating the list reaches
// the one that blocked startup before the containers that never ran.
func GetPodContainers(ctx context.Context, kubeContext, namespace, pod string) ([]string, error) {
	args := append(kubectlBaseArgs(kubeContext),
		"get", "pod", pod, "-n", namespace,
		"-o", "jsonpath={range .spec.initContainers[*]}{.name}{\"\\n\"}{end}{range .spec.containers[*]}{.name}{\"\\n\"}{end}",
	)
	output, err := runKubectl(ctx, args)
	if err != nil {
		return nil, err
	}
	var containers []string
	for _, line := range strings.Split(output, "\n") {
		if name := strings.TrimSpace(line); name != "" {
			containers = append(containers, name)
		}
	}
	return containers, nil
}

// GetPodContainerLogs returns the last tailLines of logs from a single container.
// `kubectl logs --all-containers` fails as a whole when any one container has yet
// to start, so a pod stuck in Init:Error yields nothing; fetching per container
// recovers the init container's output.
func GetPodContainerLogs(ctx context.Context, kubeContext, namespace, pod, container string, tailLines int) (string, error) {
	args := append(kubectlBaseArgs(kubeContext),
		"logs", pod, "-n", namespace,
		"-c", container,
		"--tail", fmt.Sprintf("%d", tailLines),
	)
	return runKubectl(ctx, args)
}

// ExecInPod runs command in the pod's default container under its own timeout and
// returns whatever stdout was captured, including on timeout or a non-zero exit.
func ExecInPod(ctx context.Context, kubeContext, namespace, pod string, command []string, timeout time.Duration) (string, error) {
	args := append(kubectlBaseArgs(kubeContext), "exec", pod, "-n", namespace, "--")
	args = append(args, command...)
	return runKubectlTimeout(ctx, args, timeout)
}

// GetNonReadyPods returns the names of pods that are not fully ready.
// It uses a field-selector to find non-Running pods and also parses output to
// catch Running-but-not-Ready pods (e.g., readiness probe failing).
func GetNonReadyPods(ctx context.Context, kubeContext, namespace string) ([]string, error) {
	// Get all pods in a parseable format: NAME READY STATUS
	args := append(kubectlBaseArgs(kubeContext),
		"get", "pods", "-n", namespace,
		"--no-headers",
		"-o", "custom-columns=NAME:.metadata.name,READY:.status.conditions[?(@.type=='Ready')].status,PHASE:.status.phase",
	)
	output, err := runKubectl(ctx, args)
	if err != nil {
		return nil, err
	}

	var nonReady []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		podName := fields[0]
		readyStatus := fields[1]
		// A pod is non-ready if its Ready condition is not "True"
		if !strings.EqualFold(readyStatus, "True") {
			nonReady = append(nonReady, podName)
		}
	}
	return nonReady, nil
}
