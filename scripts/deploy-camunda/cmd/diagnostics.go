package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"

	"github.com/spf13/cobra"
)

// diagnosticsEventsTail bounds the events dump so the in-log output stays readable.
const diagnosticsEventsTail = 40

// podDiagnosticsSource is the set of namespace-inspection calls the print command
// needs. It is a struct of funcs (defaulting to the kube package) so tests can
// inject fakes without a cluster or a fake kubectl on PATH.
type podDiagnosticsSource struct {
	GetPods             func(ctx context.Context, kubeContext, namespace string) (string, error)
	GetEvents           func(ctx context.Context, kubeContext, namespace string) (string, error)
	GetPVCs             func(ctx context.Context, kubeContext, namespace string) (string, error)
	DescribePVCs        func(ctx context.Context, kubeContext, namespace string) (string, error)
	GetNonReadyPods     func(ctx context.Context, kubeContext, namespace string) ([]string, error)
	GetPodNames         func(ctx context.Context, kubeContext, namespace string) ([]string, error)
	DescribePod         func(ctx context.Context, kubeContext, namespace, pod string) (string, error)
	GetPodLogs          func(ctx context.Context, kubeContext, namespace, pod string, tail int) (string, error)
	GetPodLogsPrevious  func(ctx context.Context, kubeContext, namespace, pod string, tail int) (string, error)
	GetPodContainers    func(ctx context.Context, kubeContext, namespace, pod string) ([]string, error)
	GetPodContainerLogs func(ctx context.Context, kubeContext, namespace, pod, container string, tail int) (string, error)
	ExecInPod           func(ctx context.Context, kubeContext, namespace, pod string, command []string) (string, error)
}

func defaultPodDiagnosticsSource() podDiagnosticsSource {
	return podDiagnosticsSource{
		GetPods:             kube.GetPods,
		GetEvents:           kube.GetEvents,
		GetPVCs:             kube.GetPVCs,
		DescribePVCs:        kube.DescribePVCs,
		GetNonReadyPods:     kube.GetNonReadyPods,
		GetPodNames:         kube.GetPodNames,
		DescribePod:         kube.DescribePod,
		GetPodLogs:          kube.GetPodLogs,
		GetPodLogsPrevious:  kube.GetPodLogsPrevious,
		GetPodContainers:    kube.GetPodContainers,
		GetPodContainerLogs: kube.GetPodContainerLogs,
		ExecInPod:           kube.ExecInPod,
	}
}

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
	var namespace, kubeContext string
	var tail int
	var includeReady bool

	cmd := &cobra.Command{
		Use:   "print",
		Short: "Print namespace diagnostics (pods, events, PVCs, pod describe+logs) to stdout",
		Long: `Print a best-effort namespace diagnostics dump for CI logs.

Backs the failed-pods-info GitHub action. Structured diagnostics are also
uploaded as the diagnostics-* artifact by the matrix runner; this command is the
human-readable in-log fast path for the common failure shapes (crash, image
pull, scheduling, and volume-mount/init hangs).

By default only non-ready pods are described and their logs collected. Pass
--include-ready to cover every pod in the namespace, which is what a
Ready-but-misbehaving component (an HTTP 500 from a running pod) requires.

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
			if namespace == "" {
				return fmt.Errorf("--namespace is required")
			}

			printNamespaceDiagnostics(ctx, os.Stdout, defaultPodDiagnosticsSource(), kubeContext, namespace, tail, includeReady)
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&namespace, "namespace", "n", "", "Namespace to inspect (required)")
	f.StringVar(&kubeContext, "kube-context", "", "Kubernetes context")
	f.IntVar(&tail, "tail", 500, "Log tail lines per pod")
	f.BoolVar(&includeReady, "include-ready", false, "Describe and collect logs for every pod, not just non-ready ones")
	f.StringVarP(&flags.LogLevel, "log-level", "l", "info", "Log level")

	return cmd
}

// printNamespaceDiagnostics writes the diagnostics dump to w. It is pure
// orchestration over src so it can be unit-tested with a fake source.
// includeReady selects src.GetPodNames over src.GetNonReadyPods for the
// describe+logs loop.
func printNamespaceDiagnostics(ctx context.Context, w io.Writer, src podDiagnosticsSource, kubeContext, namespace string, tail int, includeReady bool) {
	section := func(title string) { fmt.Fprintf(w, "\n===== %s =====\n", title) }
	emit := func(title string, out string, err error) {
		section(title)
		if err != nil {
			fmt.Fprintf(w, "(error: %v)\n", err)
			return
		}
		if out == "" {
			fmt.Fprintln(w, "(none)")
			return
		}
		fmt.Fprintln(w, out)
	}

	pods, podsErr := src.GetPods(ctx, kubeContext, namespace)
	emit("Pods", pods, podsErr)

	events, eventsErr := src.GetEvents(ctx, kubeContext, namespace)
	emit(fmt.Sprintf("Events (last %d)", diagnosticsEventsTail), lastLines(events, diagnosticsEventsTail), eventsErr)

	pvcs, pvcsErr := src.GetPVCs(ctx, kubeContext, namespace)
	emit("PersistentVolumeClaims", pvcs, pvcsErr)

	pvcDesc, pvcDescErr := src.DescribePVCs(ctx, kubeContext, namespace)
	emit("PVC describe", pvcDesc, pvcDescErr)

	listPods, listTitle, podLabel := src.GetNonReadyPods, "Non-ready pods", "Non-ready pod: "
	if includeReady {
		listPods, listTitle, podLabel = src.GetPodNames, "All pods", "Pod: "
	}

	targets, err := listPods(ctx, kubeContext, namespace)
	if err != nil {
		section(listTitle)
		fmt.Fprintf(w, "(error: %v)\n", err)
		return
	}
	sort.Strings(targets)
	if len(targets) == 0 {
		section(listTitle)
		fmt.Fprintln(w, "(none)")
		return
	}
	for _, pod := range targets {
		desc, descErr := src.DescribePod(ctx, kubeContext, namespace, pod)
		emit(podLabel+pod+" — describe", desc, descErr)

		podLogs, logsErr := src.GetPodLogs(ctx, kubeContext, namespace, pod, tail)
		if logsErr != nil && src.GetPodContainers != nil && src.GetPodContainerLogs != nil {
			emitPerContainerLogs(ctx, emit, src, kubeContext, namespace, pod, podLabel, tail, logsErr)
		} else {
			emit(podLabel+pod+" — logs", podLogs, logsErr)
		}

		// --previous surfaces the prior crash; empty/erroring when the pod never restarted.
		if prev, err := src.GetPodLogsPrevious(ctx, kubeContext, namespace, pod, tail); err == nil && prev != "" {
			emit(podLabel+pod+" — previous logs", prev, nil)
		}

		if isSearchPod(pod) && src.ExecInPod != nil {
			state, stateErr := src.ExecInPod(ctx, kubeContext, namespace, pod, []string{"sh", "-c", searchClusterStateScript})
			emit(podLabel+pod+" — cluster state", state, stateErr)
		}
	}
}

// emitPerContainerLogs recovers logs one container at a time after the
// all-containers fetch failed, and reports the original error when the pod's
// container list cannot be read either.
func emitPerContainerLogs(
	ctx context.Context,
	emit func(string, string, error),
	src podDiagnosticsSource,
	kubeContext, namespace, pod, podLabel string,
	tail int,
	allContainersErr error,
) {
	containers, err := src.GetPodContainers(ctx, kubeContext, namespace, pod)
	if err != nil || len(containers) == 0 {
		emit(podLabel+pod+" — logs", "", allContainersErr)
		return
	}
	for _, container := range containers {
		logs, logsErr := src.GetPodContainerLogs(ctx, kubeContext, namespace, pod, container, tail)
		emit(podLabel+pod+" — logs ["+container+"]", logs, logsErr)
	}
}

// searchClusterStateScript queries the Elasticsearch/OpenSearch HTTP API from
// inside the pod. Both expose these endpoints identically, and CI runs them with
// security disabled, so no credentials are needed. Shard-level state is what
// distinguishes "still recovering" from "cannot reach this status on this
// topology" when a readiness probe gates on cluster health.
const searchClusterStateScript = `
for q in '_cluster/health?level=indices&pretty' '_cat/indices?v&s=health:desc,index' '_cluster/allocation/explain?pretty'; do
  echo "--- GET /$q"
  curl -sS --max-time 10 "http://localhost:9200/$q" || echo "(query failed)"
  echo
done
`

// isSearchPod reports whether the pod is an Elasticsearch or OpenSearch node,
// matched on name because the companion charts are separate Helm releases whose
// labels are not shared with the Camunda chart.
func isSearchPod(pod string) bool {
	return strings.Contains(pod, "elasticsearch") || strings.Contains(pod, "opensearch")
}

// lastLines returns the last n lines of s. Short inputs are returned unchanged.
func lastLines(s string, n int) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
