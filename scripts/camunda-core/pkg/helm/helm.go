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

package helm

import (
	"context"
	"fmt"
	"scripts/camunda-core/pkg/executil"
	"scripts/camunda-core/pkg/logging"
	"strings"
	"sync"
	"time"
)

func Run(ctx context.Context, args []string, workDir string) error {
	return executil.RunCommand(ctx, "helm", args, nil, workDir)
}

// RunCaptureStderr runs a helm command like Run but also returns the accumulated
// stderr output so callers can classify transient errors for retry decisions.
func RunCaptureStderr(ctx context.Context, args []string, workDir string) (string, error) {
	return executil.RunCommandCaptureStderr(ctx, "helm", args, nil, workDir)
}

// IsTransientHelmError reports whether the stderr text from a failed helm command
// indicates a transient infrastructure error that is safe to retry.
func IsTransientHelmError(stderr string) bool {
	if stderr == "" {
		return false
	}
	lower := strings.ToLower(stderr)
	transientHints := []string{
		"internal server error",
		"server is currently unable to handle the request",
		"connection reset by peer",
		"i/o timeout",
		"tls handshake timeout",
		"service unavailable",
		"too many requests",
		"etcdserver: request timed out",
		"net/http: request canceled",
		"eof",
	}
	for _, hint := range transientHints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

var (
	waitFlagOnce  sync.Once
	waitFlagValue string
)

// WaitFlag returns the appropriate --wait argument for the installed Helm CLI.
// Helm v4 redefined --wait; --wait=legacy preserves v3 behavior. Detection runs
// once per process and falls back to "--wait" if `helm version` fails.
func WaitFlag(ctx context.Context) string {
	waitFlagOnce.Do(func() {
		waitFlagValue = detectWaitFlag(ctx)
	})
	return waitFlagValue
}

func detectWaitFlag(ctx context.Context) string {
	out, err := executil.RunCommandCapture(ctx, "helm", []string{"version", "--short"}, nil, "")
	if err != nil {
		logging.Logger.Warn().Err(err).Msg("helm version detection failed; defaulting wait flag to --wait")
		return "--wait"
	}
	version := strings.TrimSpace(string(out))
	flag := waitFlagFromOutput(out)
	logging.Logger.Info().
		Str("helmVersion", version).
		Str("waitFlag", flag).
		Msg("detected helm CLI version")
	return flag
}

func waitFlagFromOutput(out []byte) string {
	if strings.HasPrefix(strings.TrimSpace(string(out)), "v4") {
		return "--wait=legacy"
	}
	return "--wait"
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
// --force-update makes this idempotent: without it Helm fails with
// "repository name (%s) already exists" whenever the name is registered under
// a URL that differs by so much as a trailing slash.
func RepoAdd(ctx context.Context, name, url string) error {
	args := []string{"repo", "add", name, url, "--force-update"}
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
