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

package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/matrix"

	"github.com/spf13/cobra"
)

// newDiagnosticsCommand creates the `diagnostics` parent command.
func newDiagnosticsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagnostics",
		Short: "Namespace diagnostics helpers",
	}
	cmd.AddCommand(newDiagnosticsPrintCommand())
	return cmd
}

// newDiagnosticsPrintCommand creates `diagnostics print`: the in-log fast path
// for the failed-pods-info CI action. It prints pods, events, PVC state, and —
// for every non-ready pod — describe + current/previous logs. Non-ready is
// determined by the Ready condition (via kube.GetNonReadyPods), so a `0/N Running`
// pod (the common deploy-timeout shape) is captured, not just crashed ones.
// --include-ready widens the describe+logs loop to every pod (kube.GetPodNames),
// covering a Ready pod that is serving errors rather than crashing.
func newDiagnosticsPrintCommand() *cobra.Command {
	var namespaces []string
	var kubeContext string
	var tail int
	var includeReady bool

	cmd := &cobra.Command{
		Use:   "print",
		Short: "Print namespace diagnostics (pods, events, PVCs, Services, endpoints, pod describe+logs) to stdout",
		Long: `Print a best-effort namespace diagnostics dump for CI logs.

Backs the failed-pods-info GitHub action. Structured diagnostics are also
uploaded as the diagnostics-* artifact by the matrix runner; this command is the
human-readable in-log fast path for the common failure shapes (crash, image
pull, scheduling, and volume-mount/init hangs).

By default only non-ready pods are described and their logs collected. Pass
--include-ready to cover every pod in the namespace, which is what a
Ready-but-misbehaving component (an HTTP 500 from a running pod) requires.

--namespace may be repeated (or comma-separated). A topology scenario spreads
one logical cluster over several namespaces, and the component that explains a
failure is often not in the namespace whose test failed: an orchestration leg
can fail purely because Identity or Web Modeler is unhealthy in the hub
namespace. Passing every namespace of the topology captures all of them in one
dump.

All calls are best-effort: errors are printed inline and never abort the dump.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			if err := logging.Setup(logging.Options{
				LevelString:  flags.LogLevel,
				ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
			}); err != nil {
				return err
			}

			return matrix.PrintDiagnostics(ctx, kubeContext, namespaces, tail, includeReady)
		},
	}

	f := cmd.Flags()
	f.StringSliceVarP(&namespaces, "namespace", "n", nil, "Namespace to inspect; repeat or comma-separate to cover every namespace of a multi-namespace topology (required)")
	f.StringVar(&kubeContext, "kube-context", "", "Kubernetes context")
	f.IntVar(&tail, "tail", 500, "Log tail lines per pod")
	f.BoolVar(&includeReady, "include-ready", false, "Describe and collect logs for every pod, not just non-ready ones")
	f.StringVarP(&flags.LogLevel, "log-level", "l", "info", "Log level")

	return cmd
}
