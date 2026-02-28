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
	cmdCtx, cancel := context.WithTimeout(ctx, diagnosticTimeout)
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

// GetEvents returns the output of `kubectl get events -n <namespace> --sort-by=.lastTimestamp`.
func GetEvents(ctx context.Context, kubeContext, namespace string) (string, error) {
	args := append(kubectlBaseArgs(kubeContext), "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
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
