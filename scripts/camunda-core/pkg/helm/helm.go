package helm

import (
	"context"
	"fmt"
	"scripts/camunda-core/pkg/executil"
	"scripts/camunda-core/pkg/logging"
	"strings"
	"time"
)

func Run(ctx context.Context, args []string, workDir string) error {
	return executil.RunCommand(ctx, "helm", args, nil, workDir)
}

func DependencyUpdate(ctx context.Context, chartPath string) error {
	// Clean up any temporary chart directories before dependency update
	// This is needed because if you are not logged into docker, helm will leave these junk tmpcharts and tgz files in the chart path.
	// Once you are logged in, if you run helm package, the junk is included in the package and quickly exceeds the 1MB limit for
	// k8s secrets.
	if err := cleanTempCharts(ctx, chartPath); err != nil {
		// Non-fatal: log warning but continue (temp charts cleanup is best-effort)
		logging.Logger.Warn().Err(err).Str("chartPath", chartPath).Msg("failed to clean temporary charts (non-fatal)")
	}

	args := []string{"dependency", "update"}
	var err error
	for attempt := 1; attempt <= 2; attempt++ {
		err = Run(ctx, args, chartPath)
		if err == nil {
			return nil
		}
		// Always retry once on failure. The `helm dependency update` command is idempotent,
		// so retrying is always safe. We retry unconditionally because the error returned by
		// exec.Cmd is typically just "exit status 1" — the actual failure details (e.g., OCI
		// rate-limit, network timeout) are in stderr which is logged but not part of err.Error().
		// The isTransientHelmError check cannot reliably detect transient failures from exit codes alone.
		if attempt == 1 {
			logging.Logger.Warn().
				Err(err).
				Int("attempt", attempt).
				Str("chartPath", chartPath).
				Msg("helm dependency update failed, retrying once (command is idempotent)")
			select {
			case <-ctx.Done():
				return fmt.Errorf("helm dependency update failed: command: helm %s (in %s): %w", strings.Join(args, " "), chartPath, ctx.Err())
			case <-time.After(3 * time.Second):
			}
			continue
		}
		return fmt.Errorf("helm dependency update failed: command: helm %s (in %s): %w", strings.Join(args, " "), chartPath, err)
	}
	return nil
}

// RepoAdd registers a Helm chart repository (helm repo add).
// If the repo already exists, Helm treats this as a no-op update.
func RepoAdd(ctx context.Context, name, url string) error {
	args := []string{"repo", "add", name, url}
	if err := Run(ctx, args, ""); err != nil {
		return fmt.Errorf("helm repo add %s %s failed: %w", name, url, err)
	}
	return nil
}

// RepoUpdate runs helm repo update to fetch the latest chart index.
func RepoUpdate(ctx context.Context) error {
	args := []string{"repo", "update"}
	if err := Run(ctx, args, ""); err != nil {
		return fmt.Errorf("helm repo update failed: %w", err)
	}
	return nil
}

func cleanTempCharts(ctx context.Context, chartPath string) error {
	tmpChartDirArgs := []string{".", "-maxdepth", "1", "-type", "d", "-name", "tmpcharts-*", "-exec", "rm", "-rf", "{}", "+"}
	if err := executil.RunCommand(ctx, "find", tmpChartDirArgs, nil, chartPath); err != nil {
		return fmt.Errorf("remove tmpcharts-*: %w", err)
	}

	tmpChartTgzArgs := []string{".", "-maxdepth", "1", "-type", "f", "-name", "*.tgz", "-exec", "rm", "-rf", "{}", "+"}
	if err := executil.RunCommand(ctx, "find", tmpChartTgzArgs, nil, chartPath); err != nil {
		return fmt.Errorf("remove *.tgz: %w", err)
	}

	return nil
}
