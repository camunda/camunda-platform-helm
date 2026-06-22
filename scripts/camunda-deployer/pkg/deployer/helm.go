package deployer

import (
	"context"
	"fmt"
	"path/filepath"
	"scripts/camunda-core/pkg/helm"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-deployer/pkg/types"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// Package-level function variables for helm operations. These default to the
// real implementations but can be swapped in tests to avoid shelling out to
// the helm binary.
var (
	helmRunCapturing = helm.RunCaptureStderr
	helmRepoAdd      = helm.RepoAdd
	helmRepoUpdate   = helm.RepoUpdate
	helmWaitFlag     = helm.WaitFlag
)

// helmUpgradeRetryDelay is the wait between retry attempts for transient API-server errors.
// Exposed as a var so tests can set it to zero without sleeping.
var helmUpgradeRetryDelay = 10 * time.Second

// helmRunWithRetry calls helmRunCapturing and retries once when helm reports a transient
// Kubernetes API-server error (e.g. 500 during pre-flight resource lookup).
// helm upgrade --install is idempotent, so retrying is always safe. We only retry on
// transient stderr patterns to avoid doubling the helm --timeout on genuine failures.
func helmRunWithRetry(ctx context.Context, args []string) (string, error) {
	for attempt := 1; attempt <= 2; attempt++ {
		stderr, err := helmRunCapturing(ctx, args, "")
		if err == nil {
			return stderr, nil
		}
		if attempt == 1 && helm.IsTransientHelmError(stderr) {
			logging.Logger.Warn().
				Err(err).
				Dur("retryDelay", helmUpgradeRetryDelay).
				Msg("helm upgrade --install hit a transient error, retrying")
			select {
			case <-ctx.Done():
				return stderr, err
			case <-time.After(helmUpgradeRetryDelay):
			}
			continue
		}
		return stderr, err
	}
	// unreachable
	return "", nil
}

// HelmError is a structured error for helm command failures that separates
// the high-level failure reason from the full command details. This allows
// consumers to display a short summary or the full details as needed.
type HelmError struct {
	// Reason is a short description of what failed (e.g. "helm upgrade --install failed")
	Reason string
	// Command is the full helm command that was executed
	Command string
	// Cause is the underlying error (e.g. exit status 1)
	Cause error
}

func (e *HelmError) Error() string {
	return fmt.Sprintf("%s: %v", e.Reason, e.Cause)
}

func (e *HelmError) Unwrap() error {
	return e.Cause
}

// ShortCommand returns the command with long file paths shortened to just their
// base filenames for readability. Full paths in -f values file args and chart
// paths are replaced with just the filename.
func (e *HelmError) ShortCommand() string {
	return shortenPaths(e.Command)
}

// shortenPaths replaces long absolute/relative file paths with just basenames.
// It handles both -f <path> patterns and standalone long paths.
func shortenPaths(cmd string) string {
	parts := strings.Fields(cmd)
	for i := range parts {
		// Shorten -f value file paths
		if i > 0 && parts[i-1] == "-f" && len(parts[i]) > 0 && (parts[i][0] == '/' || strings.Contains(parts[i], "/")) {
			parts[i] = filepath.Base(parts[i])
			continue
		}
		// Shorten chart path arguments (absolute paths that aren't flags)
		if len(parts[i]) > 0 && parts[i][0] == '/' && !strings.HasPrefix(parts[i], "--") && strings.Contains(parts[i], "/") {
			parts[i] = filepath.Base(parts[i])
		}
	}
	return strings.Join(parts, " ")
}

// upgradeInstall builds and executes helm upgrade --install with deployer's opinionated policies
func upgradeInstall(ctx context.Context, o types.Options) error {
	var args []string
	if o.Chart != "" {
		args = []string{
			"upgrade", "--install",
			o.ReleaseName,
			o.Chart,
			"-n", o.Namespace,
		}
	} else {
		args = []string{
			"upgrade", "--install",
			o.ReleaseName,
			filepath.Clean(o.ChartPath),
			"-n", o.Namespace,
		}
	}

	// When using a repository chart name, allow pinning the chart version
	if o.Chart != "" && strings.TrimSpace(o.Version) != "" {
		args = append(args, "--version", o.Version)
	}

	// Deployer policy: always create namespace
	args = append(args, "--create-namespace")

	// Kubernetes connection
	args = append(args, composeKubeArgs(o.Kubeconfig, o.KubeContext)...)

	// Deployment behavior
	if o.Wait {
		args = append(args, helmWaitFlag(ctx))
	}
	if o.Atomic {
		args = append(args, "--atomic")
	}
	if o.Timeout > 0 {
		args = append(args, "--timeout", fmt.Sprintf("%ds", int(o.Timeout.Seconds())))
	}

	// Deployer convention: set global.ingress.host for Camunda Platform
	if o.IngressHost != "" {
		args = append(args, "--set", "global.ingress.host="+o.IngressHost)
	}

	// Optional post-renderer
	if o.PostRendererPath != "" {
		args = append(args, "--post-renderer", o.PostRendererPath)
	}

	// Values files in order
	for _, v := range o.ValuesFiles {
		args = append(args, "-f", v)
	}

	// Set pairs - deployer uses map[string]string, format as key=value
	if len(o.SetPairs) > 0 {
		// Sort keys for determinism
		keys := make([]string, 0, len(o.SetPairs))
		for k := range o.SetPairs {
			keys = append(keys, k)
		}
		// Note: intentionally not sorting to preserve user order
		for k, v := range o.SetPairs {
			args = append(args, "--set", fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Extra args last (allow override)
	if len(o.ExtraArgs) > 0 {
		args = append(args, o.ExtraArgs...)
	}

	_, runErr := helmRunWithRetry(ctx, args)
	if runErr != nil {
		return &HelmError{
			Reason:  "helm upgrade --install failed",
			Command: "helm " + formatArgs(args),
			Cause:   runErr,
		}
	}
	return nil
}

// composeKubeArgs builds kubeconfig and context arguments
func composeKubeArgs(kubeconfig, context string) []string {
	var args []string
	if kubeconfig != "" {
		args = append(args, "--kubeconfig", kubeconfig)
	}
	if context != "" {
		args = append(args, "--kube-context", context)
	}
	return args
}

// formatArgs formats command arguments for error messages, escaping special characters
func formatArgs(args []string) string {
	var parts []string
	for _, arg := range args {
		// If arg contains spaces or special chars, quote it
		if strings.ContainsAny(arg, " \t\n\"'") {
			parts = append(parts, fmt.Sprintf("%q", arg))
		} else {
			parts = append(parts, arg)
		}
	}
	return strings.Join(parts, " ")
}

// companionRepoMu serializes helm repo add/update across concurrent companion
// deployments — those commands rewrite the shared repositories.yaml, so
// concurrent writers would race. No current scenario pairs two remote-repo
// companions, so contention is effectively zero; this guards future ones.
var companionRepoMu sync.Mutex

// deployCompanionCharts deploys all configured companion charts concurrently as
// separate Helm releases in the same namespace. Companions are independent of
// each other (only the main chart depends on them), so they install in
// parallel. The call blocks until every companion is ready; on the first
// failure the shared context is cancelled, terminating in-flight siblings, and
// the first error is returned. A single companion behaves identically to a
// serial deploy.
func deployCompanionCharts(ctx context.Context, o types.Options) error {
	g, gCtx := errgroup.WithContext(ctx)
	for i, cc := range o.CompanionCharts {
		g.Go(func() error {
			logging.Logger.Info().
				Str("chart", cc.ChartRef).
				Str("version", cc.Version).
				Str("release", cc.ReleaseName).
				Str("namespace", o.Namespace).
				Msg("Deploying companion chart")
			if err := deployCompanionChart(gCtx, cc, o); err != nil {
				return fmt.Errorf("companion chart [%d] %q failed: %w", i, cc.ReleaseName, err)
			}
			return nil
		})
	}
	return g.Wait()
}

// deployCompanionChart deploys a single companion chart as its own Helm release
// in the same namespace as the main Camunda chart. It uses helm upgrade --install
// with --wait to ensure the chart is fully ready before returning.
func deployCompanionChart(ctx context.Context, cc types.CompanionChart, o types.Options) error {
	// Ensure the Helm repo is registered when a repo-style chart ref is used.
	if cc.RepoName != "" && cc.RepoURL != "" {
		companionRepoMu.Lock()
		err := func() error {
			if err := helmRepoAdd(ctx, cc.RepoName, cc.RepoURL); err != nil {
				return fmt.Errorf("companion chart %q: repo add failed: %w", cc.ReleaseName, err)
			}
			if err := helmRepoUpdate(ctx); err != nil {
				return fmt.Errorf("companion chart %q: repo update failed: %w", cc.ReleaseName, err)
			}
			return nil
		}()
		companionRepoMu.Unlock()
		if err != nil {
			return err
		}
	}

	args := []string{
		"upgrade", "--install",
		cc.ReleaseName,
		cc.ChartRef,
		"-n", o.Namespace,
		"--create-namespace",
		helmWaitFlag(ctx),
	}

	// Pin chart version (required for remote charts)
	if cc.Version != "" {
		args = append(args, "--version", cc.Version)
	}

	// Use the same timeout as the main deployment
	if o.Timeout > 0 {
		args = append(args, "--timeout", fmt.Sprintf("%ds", int(o.Timeout.Seconds())))
	}

	// Kubernetes connection
	args = append(args, composeKubeArgs(o.Kubeconfig, o.KubeContext)...)

	// Values file (optional)
	if cc.ValuesFile != "" {
		args = append(args, "-f", cc.ValuesFile)
	}

	_, runErr := helmRunWithRetry(ctx, args)
	if runErr == nil {
		return nil
	}
	return &HelmError{
		Reason:  fmt.Sprintf("companion chart %q helm upgrade --install failed", cc.ReleaseName),
		Command: "helm " + formatArgs(args),
		Cause:   runErr,
	}
}
