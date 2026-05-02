package agentwatch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// Snapshot bundles the cluster state we hand to the agent on each tick.
// Fields are raw JSON so the agent sees exactly what `kubectl` and
// `helm` produced, with no intermediate transformation that might hide a
// signal.
type Snapshot struct {
	Timestamp     time.Time       `json:"timestamp"`
	Namespace     string          `json:"namespace"`
	Release       string          `json:"release"`
	Pods          json.RawMessage `json:"pods"`
	Events        json.RawMessage `json:"events"`
	PVCs          json.RawMessage `json:"pvcs,omitempty"`
	HelmStatus    json.RawMessage `json:"helm_status,omitempty"`
	HelmStatusErr string          `json:"helm_status_error,omitempty"`
}

// SnapshotOptions configures a snapshot collection.
type SnapshotOptions struct {
	Namespace   string
	Release     string
	KubeContext string
	// SkipHelmStatus omits `helm status` from the snapshot. Useful in tests
	// or when Helm credentials are not available locally.
	SkipHelmStatus bool
}

// GatherSnapshot runs `kubectl get pods,events,pvcs` and `helm status` and
// returns the combined snapshot. A non-fatal helm error is recorded in
// Snapshot.HelmStatusErr rather than failing the whole collection — the
// agent can still reason about pod/event state without helm metadata.
func GatherSnapshot(ctx context.Context, opts SnapshotOptions) (Snapshot, error) {
	if opts.Namespace == "" {
		return Snapshot{}, fmt.Errorf("namespace is required")
	}

	snap := Snapshot{
		Timestamp: time.Now().UTC(),
		Namespace: opts.Namespace,
		Release:   opts.Release,
	}

	pods, err := runKubectlJSON(ctx, opts.KubeContext, opts.Namespace, "pods")
	if err != nil {
		return Snapshot{}, fmt.Errorf("kubectl get pods: %w", err)
	}
	snap.Pods = pods

	events, err := runKubectlJSON(ctx, opts.KubeContext, opts.Namespace, "events")
	if err != nil {
		return Snapshot{}, fmt.Errorf("kubectl get events: %w", err)
	}
	snap.Events = events

	if pvcs, err := runKubectlJSON(ctx, opts.KubeContext, opts.Namespace, "pvc"); err == nil {
		snap.PVCs = pvcs
	}

	if !opts.SkipHelmStatus && opts.Release != "" {
		if status, err := runHelmStatus(ctx, opts.KubeContext, opts.Namespace, opts.Release); err == nil {
			snap.HelmStatus = status
		} else {
			snap.HelmStatusErr = err.Error()
		}
	}

	return snap, nil
}

// runKubectlJSON shells out to `kubectl get <resource> -n <ns> -o json` and
// returns the raw JSON bytes.
func runKubectlJSON(ctx context.Context, kubeContext, namespace, resource string) (json.RawMessage, error) {
	args := []string{"get", resource, "-n", namespace, "-o", "json"}
	if kubeContext != "" {
		args = append([]string{"--context", kubeContext}, args...)
	}
	out, err := runCmd(ctx, "kubectl", args...)
	if err != nil {
		return nil, err
	}
	// json.RawMessage requires valid JSON; sanity-check the prefix.
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, fmt.Errorf("kubectl get %s returned non-JSON output", resource)
	}
	return json.RawMessage(trimmed), nil
}

// runHelmStatus runs `helm status <release> -n <ns> -o json`.
func runHelmStatus(ctx context.Context, kubeContext, namespace, release string) (json.RawMessage, error) {
	args := []string{"status", release, "-n", namespace, "-o", "json"}
	if kubeContext != "" {
		args = append(args, "--kube-context", kubeContext)
	}
	out, err := runCmd(ctx, "helm", args...)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(bytes.TrimSpace(out)), nil
}

// runCmd executes a command and returns its stdout. Stderr is folded into
// the error so the caller can surface it without a separate field.
func runCmd(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("%s %v failed: %w; stderr: %s",
			name, args, err, truncate(stderr.String(), 1000))
	}
	return stdout.Bytes(), nil
}

// MarshalJSON ensures Snapshot serializes deterministically for the agent
// prompt (json.Marshal already sorts struct fields by declaration order in
// Go, so this is the default; we keep the explicit method as a hook for
// future redaction or summarization).
func (s Snapshot) MarshalIndent() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}
