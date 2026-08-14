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
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"

	"github.com/spf13/cobra"
)

// errNotCollected marks a section whose collector was not wired on the source.
var errNotCollected = fmt.Errorf("not collected")

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
	GetServices         func(ctx context.Context, kubeContext, namespace string) (string, error)
	GetServicesYAML     func(ctx context.Context, kubeContext, namespace string) (string, error)
	GetEndpoints        func(ctx context.Context, kubeContext, namespace string) (string, error)
	GetNonReadyPods     func(ctx context.Context, kubeContext, namespace string) ([]string, error)
	GetPodNames         func(ctx context.Context, kubeContext, namespace string) ([]string, error)
	DescribePod         func(ctx context.Context, kubeContext, namespace, pod string) (string, error)
	GetPodLogs          func(ctx context.Context, kubeContext, namespace, pod string, tail int) (string, error)
	GetPodLogsPrevious  func(ctx context.Context, kubeContext, namespace, pod string, tail int) (string, error)
	GetPodContainers    func(ctx context.Context, kubeContext, namespace, pod string) ([]string, error)
	GetPodContainerLogs func(ctx context.Context, kubeContext, namespace, pod, container string, tail int) (string, error)
	ExecInPod           func(ctx context.Context, kubeContext, namespace, pod string, command []string, timeout time.Duration) (string, error)
}

func defaultPodDiagnosticsSource() podDiagnosticsSource {
	return podDiagnosticsSource{
		GetPods:             kube.GetPods,
		GetEvents:           kube.GetEvents,
		GetPVCs:             kube.GetPVCs,
		DescribePVCs:        kube.DescribePVCs,
		GetServices:         kube.GetServices,
		GetServicesYAML:     kube.GetServicesYAML,
		GetEndpoints:        kube.GetEndpoints,
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
		Short: "Print namespace diagnostics (pods, events, PVCs, Services, endpoints, pod describe+logs) to stdout",
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
	// emitPartial keeps whatever was captured before an error, which emit discards.
	// A timed-out exec still carries the endpoints it managed to query.
	emitPartial := func(title string, out string, err error) {
		section(title)
		if out != "" {
			fmt.Fprintln(w, out)
		}
		if err != nil {
			fmt.Fprintf(w, "(error: %v)\n", err)
		}
		if out == "" && err == nil {
			fmt.Fprintln(w, "(none)")
		}
	}

	// collect tolerates an unset collector so a partially-populated source degrades
	// to a "(not collected)" section instead of panicking mid-dump.
	collect := func(fn func(context.Context, string, string) (string, error)) (string, error) {
		if fn == nil {
			return "", errNotCollected
		}
		return fn(ctx, kubeContext, namespace)
	}

	pods, podsErr := collect(src.GetPods)
	emit("Pods", pods, podsErr)

	events, eventsErr := collect(src.GetEvents)
	emit(fmt.Sprintf("Events (last %d)", diagnosticsEventsTail), lastLines(events, diagnosticsEventsTail), eventsErr)

	pvcs, pvcsErr := collect(src.GetPVCs)
	emit("PersistentVolumeClaims", pvcs, pvcsErr)

	pvcDesc, pvcDescErr := collect(src.DescribePVCs)
	emit("PVC describe", pvcDesc, pvcDescErr)

	services, servicesErr := collect(src.GetServices)
	emit("Services", services, servicesErr)

	serviceYAML, serviceYAMLErr := collect(src.GetServicesYAML)
	emit("Service spec (YAML)", serviceYAML, serviceYAMLErr)

	endpoints, endpointsErr := collect(src.GetEndpoints)
	emit("Endpoints", endpoints, endpointsErr)

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
			emitSearchClusterState(ctx, emitPartial, src, kubeContext, namespace, pod, podLabel)
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

// searchClusterQueries is queried one exec per entry.
var searchClusterQueries = []string{
	"_cluster/health?level=indices&pretty",
	"_cat/indices?v&s=health:desc,index",
	"_cluster/allocation/explain?pretty",
}

// searchClusterExecTimeout must exceed the worst-case curl budget in one exec —
// two 8s scheme probes plus an 8s query — plus kubectl exec startup, so kubectl
// does not cancel the exec before curl reports its own failure.
const searchClusterExecTimeout = 40 * time.Second

// searchClusterPrelude sets BASE, CURL, TLS and AUTH for the query appended after
// it. It probes https before http because the companion chart's scheme varies per
// scenario, and reads credentials from the container environment: ELASTIC_PASSWORD
// for Elasticsearch, OPENSEARCH_INITIAL_ADMIN_PASSWORD for OpenSearch with the
// security plugin. Both are passed via curl -u and never echoed.
//
// TLS verification is skipped: the self-signed node certificate is issued for the
// masterService DNS name, not the loopback address this probe connects to.
const searchClusterPrelude = `
set -u
CURL="curl -sS --max-time 8"
TLS="-k"
AUTH=""
if [ -n "${ELASTIC_PASSWORD:-}" ]; then AUTH="-u elastic:${ELASTIC_PASSWORD}"; fi
if [ -n "${OPENSEARCH_INITIAL_ADMIN_PASSWORD:-}" ]; then AUTH="-u admin:${OPENSEARCH_INITIAL_ADMIN_PASSWORD}"; fi
BASE=""
for scheme in https http; do
  if $CURL $TLS $AUTH "$scheme://localhost:9200/" >/dev/null 2>&1; then
    BASE="$scheme://localhost:9200"
    break
  fi
done
if [ -z "$BASE" ]; then
  echo "(no reachable endpoint on localhost:9200 over https or http)"
  exit 0
fi
`

// emitSearchClusterState runs one exec per query so a slow endpoint cannot consume
// the budget of the others, and emits each result through emitPartial so a
// timed-out exec still reports what it captured.
func emitSearchClusterState(
	ctx context.Context,
	emitPartial func(string, string, error),
	src podDiagnosticsSource,
	kubeContext, namespace, pod, podLabel string,
) {
	for _, query := range searchClusterQueries {
		script := searchClusterPrelude + "\n$CURL $TLS $AUTH \"$BASE/" + query + "\"\n"
		out, err := src.ExecInPod(
			ctx, kubeContext, namespace, pod,
			[]string{"sh", "-c", script},
			searchClusterExecTimeout,
		)
		emitPartial(podLabel+pod+" — cluster state ["+searchQueryLabel(query)+"]", out, err)
	}
}

// searchQueryLabel drops the query string from an endpoint path.
func searchQueryLabel(query string) string {
	if i := strings.IndexByte(query, '?'); i >= 0 {
		return query[:i]
	}
	return query
}

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
