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

// kubectlPerCallTimeout caps a single kubectl/helm invocation. Without a
// per-call cap, an unreachable apiserver or hung admission webhook blocks
// an entire poll tick on one command — destroying the diagnose-while-
// running property the watcher exists to provide.
const kubectlPerCallTimeout = 30 * time.Second

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
// the error so the caller can surface it without a separate field. Each
// call gets its own timeout-bound child context so a single hung
// invocation cannot block the watch loop indefinitely.
func runCmd(ctx context.Context, name string, args ...string) ([]byte, error) {
	callCtx, cancel := context.WithTimeout(ctx, kubectlPerCallTimeout)
	defer cancel()
	cmd := exec.CommandContext(callCtx, name, args...)
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

// AllPodsReady reports whether every pod in the snapshot has reached a
// terminal-ready state: phase=Running with the Ready condition true, or
// phase=Succeeded (e.g. one-shot Jobs). Returns false when no pods are
// present, since an empty namespace means the install has not started.
func (s Snapshot) AllPodsReady() bool {
	if len(s.Pods) == 0 {
		return false
	}
	var list struct {
		Items []struct {
			Status struct {
				Phase      string `json:"phase"`
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(s.Pods, &list); err != nil {
		return false
	}
	if len(list.Items) == 0 {
		return false
	}
	for _, p := range list.Items {
		switch p.Status.Phase {
		case "Succeeded":
			continue
		case "Running":
			ready := false
			for _, c := range p.Status.Conditions {
				if c.Type == "Ready" && c.Status == "True" {
					ready = true
					break
				}
			}
			if !ready {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// InstallComplete reports whether the watched install reached a successful
// terminal state. Pod readiness alone is not enough while Helm is still
// pending: companion/pre-install pods can all be ready before the main release
// has created its workloads.
func (s Snapshot) InstallComplete() bool {
	if !s.AllPodsReady() {
		return false
	}
	if s.Release == "" {
		return true
	}
	return s.HelmReleaseDeployed()
}

// HelmReleaseDeployed reports whether `helm status -o json` says the watched
// release is deployed. Missing/erroring Helm status is treated as incomplete so
// the watch loop keeps collecting snapshots and can see later failing pods.
func (s Snapshot) HelmReleaseDeployed() bool {
	if len(s.HelmStatus) == 0 {
		return false
	}
	var status struct {
		Info struct {
			Status string `json:"status"`
		} `json:"info"`
	}
	if err := json.Unmarshal(s.HelmStatus, &status); err != nil {
		return false
	}
	return status.Info.Status == "deployed"
}
