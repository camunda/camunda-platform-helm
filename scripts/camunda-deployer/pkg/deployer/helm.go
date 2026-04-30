package deployer

import (
	"context"
	"fmt"
	"path/filepath"
	"scripts/camunda-core/pkg/helm"
	"scripts/camunda-deployer/pkg/types"
	"strings"
)

// Package-level function variables for helm operations. These default to the
// real implementations but can be swapped in tests to avoid shelling out to
// the helm binary.
var (
	helmRun        = helm.Run
	helmRepoAdd    = helm.RepoAdd
	helmRepoUpdate = helm.RepoUpdate
)

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
		args = append(args, "--wait")
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

	// Execute via thin helm wrapper
	err := helmRun(ctx, args, "")
	if err != nil {
		return &HelmError{
			Reason:  "helm upgrade --install failed",
			Command: "helm " + formatArgs(args),
			Cause:   err,
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

// deployCompanionChart deploys a single companion chart as its own Helm release
// in the same namespace as the main Camunda chart. It uses helm upgrade --install
// with --wait to ensure the chart is fully ready before returning.
func deployCompanionChart(ctx context.Context, cc types.CompanionChart, o types.Options) error {
	// Ensure the Helm repo is registered when a repo-style chart ref is used.
	if cc.RepoName != "" && cc.RepoURL != "" {
		if err := helmRepoAdd(ctx, cc.RepoName, cc.RepoURL); err != nil {
			return fmt.Errorf("companion chart %q: repo add failed: %w", cc.ReleaseName, err)
		}
		if err := helmRepoUpdate(ctx); err != nil {
			return fmt.Errorf("companion chart %q: repo update failed: %w", cc.ReleaseName, err)
		}
	}

	args := []string{
		"upgrade", "--install",
		cc.ReleaseName,
		cc.ChartRef,
		"-n", o.Namespace,
		"--create-namespace",
		"--wait",
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

	err := helmRun(ctx, args, "")
	if err != nil {
		return &HelmError{
			Reason:  fmt.Sprintf("companion chart %q helm upgrade --install failed", cc.ReleaseName),
			Command: "helm " + formatArgs(args),
			Cause:   err,
		}
	}
	return nil
}
