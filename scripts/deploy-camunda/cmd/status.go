package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"scripts/prepare-helm-values/pkg/env"
	"strings"
	"time"

	"github.com/jwalton/gchalk"
	"github.com/spf13/cobra"
)

// StatusResult represents the status of a deployment.
type StatusResult struct {
	Release     string      `json:"release"`
	Namespace   string      `json:"namespace"`
	Status      string      `json:"status"`
	Revision    string      `json:"revision,omitempty"`
	Updated     string      `json:"updated,omitempty"`
	Chart       string      `json:"chart,omitempty"`
	AppVersion  string      `json:"appVersion,omitempty"`
	Pods        []PodStatus `json:"pods,omitempty"`
	Error       string      `json:"error,omitempty"`
}

// PodStatus represents the status of a pod.
type PodStatus struct {
	Name   string `json:"name"`
	Ready  string `json:"ready"`
	Status string `json:"status"`
	Age    string `json:"age"`
}

// newStatusCommand creates the status subcommand.
func newStatusCommand() *cobra.Command {
	var statusFlags struct {
		watch        bool
		watchInterval int
		showPods     bool
		outputFormat string
	}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check the status of a Camunda deployment",
		Long: `Check the status of a Camunda Platform deployment.

This command queries Helm and Kubernetes to show the current state of the
deployment, including release status and pod readiness.

EXAMPLES:
  # Check status using active config profile
  deploy-camunda status

  # Check status with pod details
  deploy-camunda status --pods

  # Watch status with live updates
  deploy-camunda status --watch

  # Output as JSON
  deploy-camunda status --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Setup logging
			if err := logging.Setup(logging.Options{
				LevelString:  flags.LogLevel,
				ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
			}); err != nil {
				return err
			}

			// Load .env file
			if flags.EnvFile != "" {
				_ = env.Load(flags.EnvFile)
			} else {
				_ = env.Load(".env")
			}

			// Load config
			cfgPath, err := config.ResolvePath(configFile)
			if err != nil {
				return fmt.Errorf("failed to resolve config path: %w", err)
			}

			rc, err := config.Read(cfgPath, true)
			if err != nil {
				return fmt.Errorf("failed to read config: %w", err)
			}

			// Apply active deployment defaults
			if err := config.ApplyActiveDeployment(rc, rc.Current, &flags); err != nil {
				return err
			}

			// Require namespace and release for status check
			if flags.Namespace == "" {
				return fmt.Errorf("namespace not set; provide --namespace or set in config")
			}
			if flags.Release == "" {
				// Use default release name
				flags.Release = "integration"
			}

			// Get output format
			outputJSON := statusFlags.outputFormat == "json"

			if statusFlags.watch {
				return watchStatus(cmd.Context(), flags.Namespace, flags.Release, statusFlags.showPods, statusFlags.watchInterval, outputJSON)
			}

			result := getStatus(cmd.Context(), flags.Namespace, flags.Release, statusFlags.showPods)
			return printStatus(result, outputJSON)
		},
	}

	cmd.Flags().BoolVar(&statusFlags.watch, "watch", false,
		"Continuously watch deployment status")
	cmd.Flags().IntVar(&statusFlags.watchInterval, "interval", 5,
		"Watch interval in seconds (used with --watch)")
	cmd.Flags().BoolVar(&statusFlags.showPods, "pods", false,
		"Show detailed pod status")
	cmd.Flags().StringVarP(&statusFlags.outputFormat, "output", "o", "text",
		"Output format: text (default) or json")

	return cmd
}

// getStatus retrieves the status of a deployment.
func getStatus(ctx context.Context, namespace, release string, showPods bool) *StatusResult {
	result := &StatusResult{
		Release:   release,
		Namespace: namespace,
	}

	// Get Helm release status
	helmCmd := exec.CommandContext(ctx, "helm", "status", release, "--namespace", namespace, "--output", "json")
	output, err := helmCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Status = "not-found"
			result.Error = strings.TrimSpace(string(exitErr.Stderr))
		} else {
			result.Status = "error"
			result.Error = err.Error()
		}
		return result
	}

	// Parse Helm JSON output
	var helmStatus struct {
		Info struct {
			Status       string `json:"status"`
			LastDeployed string `json:"last_deployed"`
		} `json:"info"`
		Version int `json:"version"`
		Chart   struct {
			Metadata struct {
				Name       string `json:"name"`
				Version    string `json:"version"`
				AppVersion string `json:"appVersion"`
			} `json:"metadata"`
		} `json:"chart"`
	}

	if err := json.Unmarshal(output, &helmStatus); err != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("failed to parse helm status: %v", err)
		return result
	}

	result.Status = helmStatus.Info.Status
	result.Revision = fmt.Sprintf("%d", helmStatus.Version)
	result.Updated = helmStatus.Info.LastDeployed
	result.Chart = fmt.Sprintf("%s-%s", helmStatus.Chart.Metadata.Name, helmStatus.Chart.Metadata.Version)
	result.AppVersion = helmStatus.Chart.Metadata.AppVersion

	// Get pod status if requested
	if showPods {
		result.Pods = getPodStatus(ctx, namespace, release)
	}

	return result
}

// getPodStatus retrieves the status of pods for a release.
func getPodStatus(ctx context.Context, namespace, release string) []PodStatus {
	labelSelector := fmt.Sprintf("app.kubernetes.io/instance=%s", release)
	kubectlCmd := exec.CommandContext(ctx, "kubectl", "get", "pods",
		"--namespace", namespace,
		"--selector", labelSelector,
		"--output", "json")

	output, err := kubectlCmd.Output()
	if err != nil {
		return nil
	}

	var podList struct {
		Items []struct {
			Metadata struct {
				Name              string    `json:"name"`
				CreationTimestamp time.Time `json:"creationTimestamp"`
			} `json:"metadata"`
			Status struct {
				Phase             string `json:"phase"`
				ContainerStatuses []struct {
					Ready bool `json:"ready"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &podList); err != nil {
		return nil
	}

	pods := make([]PodStatus, 0, len(podList.Items))
	for _, pod := range podList.Items {
		readyCount := 0
		totalCount := len(pod.Status.ContainerStatuses)
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				readyCount++
			}
		}

		age := time.Since(pod.Metadata.CreationTimestamp).Round(time.Second)
		ageStr := formatDuration(age)

		pods = append(pods, PodStatus{
			Name:   pod.Metadata.Name,
			Ready:  fmt.Sprintf("%d/%d", readyCount, totalCount),
			Status: pod.Status.Phase,
			Age:    ageStr,
		})
	}

	return pods
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// printStatus outputs the status result.
func printStatus(result *StatusResult, outputJSON bool) error {
	if outputJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Pretty print
	styleHead := func(s string) string { return logging.Emphasize(s, gchalk.Bold) }
	styleKey := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	styleVal := func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }
	styleStatus := func(s string) string {
		switch s {
		case "deployed":
			return logging.Emphasize(s, gchalk.Green)
		case "pending-install", "pending-upgrade", "pending-rollback":
			return logging.Emphasize(s, gchalk.Yellow)
		case "failed", "not-found", "error":
			return logging.Emphasize(s, gchalk.Red)
		default:
			return s
		}
	}

	var b strings.Builder
	b.WriteString(styleHead("Deployment Status"))
	b.WriteString("\n\n")

	maxKey := 12
	fmt.Fprintf(&b, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Release")), styleVal(result.Release))
	fmt.Fprintf(&b, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Namespace")), styleVal(result.Namespace))
	fmt.Fprintf(&b, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Status")), styleStatus(result.Status))

	if result.Error != "" {
		fmt.Fprintf(&b, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Error")), logging.Emphasize(result.Error, gchalk.Red))
	}

	if result.Revision != "" {
		fmt.Fprintf(&b, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Revision")), styleVal(result.Revision))
	}
	if result.Updated != "" {
		fmt.Fprintf(&b, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Updated")), styleVal(result.Updated))
	}
	if result.Chart != "" {
		fmt.Fprintf(&b, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Chart")), styleVal(result.Chart))
	}
	if result.AppVersion != "" {
		fmt.Fprintf(&b, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "App Version")), styleVal(result.AppVersion))
	}

	// Pod status table
	if len(result.Pods) > 0 {
		b.WriteString("\n")
		b.WriteString(styleHead("Pods"))
		b.WriteString("\n")

		// Find max name length for alignment
		maxNameLen := 4 // "NAME"
		for _, pod := range result.Pods {
			if len(pod.Name) > maxNameLen {
				maxNameLen = len(pod.Name)
			}
		}

		// Header
		fmt.Fprintf(&b, "  %-*s  %-8s  %-12s  %s\n",
			maxNameLen, styleKey("NAME"), styleKey("READY"), styleKey("STATUS"), styleKey("AGE"))

		// Rows
		for _, pod := range result.Pods {
			statusColor := gchalk.Green
			if pod.Status != "Running" {
				statusColor = gchalk.Yellow
			}
			if pod.Status == "Failed" || pod.Status == "Error" {
				statusColor = gchalk.Red
			}

			fmt.Fprintf(&b, "  %-*s  %-8s  %-12s  %s\n",
				maxNameLen, pod.Name, pod.Ready,
				logging.Emphasize(pod.Status, statusColor), pod.Age)
		}
	}

	logging.Logger.Info().Msg(b.String())
	return nil
}

// watchStatus continuously monitors deployment status.
func watchStatus(ctx context.Context, namespace, release string, showPods bool, interval int, outputJSON bool) error {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	// Initial status check
	result := getStatus(ctx, namespace, release, showPods)
	if err := printStatus(result, outputJSON); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Clear screen for TTY, print separator for non-TTY
			if logging.IsTerminal(os.Stdout.Fd()) && !outputJSON {
				fmt.Print("\033[H\033[2J")
			} else if !outputJSON {
				fmt.Println("---")
			}

			result := getStatus(ctx, namespace, release, showPods)
			if err := printStatus(result, outputJSON); err != nil {
				return err
			}
		}
	}
}

