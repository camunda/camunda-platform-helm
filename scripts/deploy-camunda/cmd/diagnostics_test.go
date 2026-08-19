package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestPrintNamespaceDiagnostics(t *testing.T) {
	t.Parallel()

	var describedPods []string
	src := podDiagnosticsSource{
		GetPods: func(_ context.Context, _, _ string) (string, error) {
			return "pod-a 0/1 Running\npod-b 1/1 Running", nil
		},
		GetEvents:    func(_ context.Context, _, _ string) (string, error) { return "e1\ne2\ne3", nil },
		GetPVCs:      func(_ context.Context, _, _ string) (string, error) { return "pvc-a Pending", nil },
		DescribePVCs: func(_ context.Context, _, _ string) (string, error) { return "Name: pvc-a\nStatus: Pending", nil },
		// The key behaviour: a 0/1 Running pod (not crashed) must be treated as
		// non-ready and described — the regression the old grep filter caused.
		GetNonReadyPods: func(_ context.Context, _, _ string) ([]string, error) { return []string{"pod-a"}, nil },
		DescribePod: func(_ context.Context, _, _, pod string) (string, error) {
			describedPods = append(describedPods, pod)
			return "Name: " + pod + "\nEvents: FailedMount", nil
		},
		GetPodLogs:         func(_ context.Context, _, _, pod string, _ int) (string, error) { return pod + " current log", nil },
		GetPodLogsPrevious: func(_ context.Context, _, _, _ string, _ int) (string, error) { return "", nil },
	}

	var buf bytes.Buffer
	printNamespaceDiagnostics(context.Background(), &buf, src, "", "ns", 500, false)
	out := buf.String()

	for _, want := range []string{"Pods", "Events", "PersistentVolumeClaims", "PVC describe", "Non-ready pod: pod-a", "FailedMount", "pod-a current log"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if len(describedPods) != 1 || describedPods[0] != "pod-a" {
		t.Errorf("expected pod-a described exactly once, got %v", describedPods)
	}
	// pod-b is ready; it must not be described.
	if strings.Contains(out, "Non-ready pod: pod-b") {
		t.Errorf("ready pod-b should not be described:\n%s", out)
	}
}

func TestPrintNamespaceDiagnosticsErrorsAreInline(t *testing.T) {
	t.Parallel()
	boom := func(_ context.Context, _, _ string) (string, error) { return "", fmt.Errorf("boom") }
	src := podDiagnosticsSource{
		GetPods:         boom,
		GetEvents:       boom,
		GetPVCs:         boom,
		DescribePVCs:    boom,
		GetNonReadyPods: func(_ context.Context, _, _ string) ([]string, error) { return nil, fmt.Errorf("list failed") },
	}
	var buf bytes.Buffer
	// Must not panic and must surface the errors rather than aborting.
	printNamespaceDiagnostics(context.Background(), &buf, src, "", "ns", 10, false)
	out := buf.String()
	if !strings.Contains(out, "(error: boom)") || !strings.Contains(out, "(error: list failed)") {
		t.Errorf("expected inline errors, got:\n%s", out)
	}
}

func TestPrintNamespaceDiagnosticsIncludeReady(t *testing.T) {
	t.Parallel()

	var describedPods []string
	nonReadyCalls := 0
	src := podDiagnosticsSource{
		GetPods:      func(_ context.Context, _, _ string) (string, error) { return "keycloak 1/1 Running", nil },
		GetEvents:    func(_ context.Context, _, _ string) (string, error) { return "", nil },
		GetPVCs:      func(_ context.Context, _, _ string) (string, error) { return "", nil },
		DescribePVCs: func(_ context.Context, _, _ string) (string, error) { return "", nil },
		GetNonReadyPods: func(_ context.Context, _, _ string) ([]string, error) {
			nonReadyCalls++
			return nil, nil
		},
		GetPodNames: func(_ context.Context, _, _ string) ([]string, error) {
			return []string{"orchestration-0", "keycloak-0"}, nil
		},
		DescribePod: func(_ context.Context, _, _, pod string) (string, error) {
			describedPods = append(describedPods, pod)
			return "Name: " + pod, nil
		},
		GetPodLogs: func(_ context.Context, _, _, pod string, _ int) (string, error) {
			return pod + ": Unexpected error when handling authentication request", nil
		},
		GetPodLogsPrevious: func(_ context.Context, _, _, _ string, _ int) (string, error) { return "", nil },
	}

	var buf bytes.Buffer
	printNamespaceDiagnostics(context.Background(), &buf, src, "", "ns", 500, true)
	out := buf.String()

	// Every pod is Ready, so the default mode would have collected no logs at all.
	for _, want := range []string{
		"Pod: keycloak-0 — describe",
		"Pod: orchestration-0 — describe",
		"Unexpected error when handling authentication request",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if len(describedPods) != 2 {
		t.Errorf("expected both pods described, got %v", describedPods)
	}
	// Sorted, so keycloak-0 precedes orchestration-0.
	if describedPods[0] != "keycloak-0" {
		t.Errorf("expected sorted pod order, got %v", describedPods)
	}
	if nonReadyCalls != 0 {
		t.Errorf("include-ready must not call GetNonReadyPods, got %d calls", nonReadyCalls)
	}
	if strings.Contains(out, "Non-ready pod:") {
		t.Errorf("include-ready output should not use the non-ready label:\n%s", out)
	}
}

func TestPrintNamespaceDiagnosticsFallsBackToPerContainerLogs(t *testing.T) {
	t.Parallel()

	var fetched []string
	src := podDiagnosticsSource{
		GetPods:         func(_ context.Context, _, _ string) (string, error) { return "optimize 0/1 Init:Error", nil },
		GetEvents:       func(_ context.Context, _, _ string) (string, error) { return "", nil },
		GetPVCs:         func(_ context.Context, _, _ string) (string, error) { return "", nil },
		DescribePVCs:    func(_ context.Context, _, _ string) (string, error) { return "", nil },
		GetNonReadyPods: func(_ context.Context, _, _ string) ([]string, error) { return []string{"optimize"}, nil },
		DescribePod:     func(_ context.Context, _, _, pod string) (string, error) { return "Name: " + pod, nil },
		// kubectl logs --all-containers fails as a whole while a container is still
		// waiting to start, which is what hid the failing init container's output.
		GetPodLogs: func(_ context.Context, _, _, _ string, _ int) (string, error) {
			return "", fmt.Errorf("exit status 1")
		},
		GetPodLogsPrevious: func(_ context.Context, _, _, _ string, _ int) (string, error) { return "", nil },
		GetPodContainers: func(_ context.Context, _, _, _ string) ([]string, error) {
			return []string{"migration", "optimize"}, nil
		},
		GetPodContainerLogs: func(_ context.Context, _, _, _, container string, _ int) (string, error) {
			fetched = append(fetched, container)
			if container == "optimize" {
				return "", fmt.Errorf("waiting to start: PodInitializing")
			}
			return "migration failed to reach elasticsearch", nil
		},
	}

	var buf bytes.Buffer
	printNamespaceDiagnostics(context.Background(), &buf, src, "", "ns", 500, false)
	out := buf.String()

	if len(fetched) != 2 || fetched[0] != "migration" {
		t.Errorf("expected init container fetched first, got %v", fetched)
	}
	for _, want := range []string{"logs [migration]", "migration failed to reach elasticsearch", "logs [optimize]", "PodInitializing"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintNamespaceDiagnosticsFallbackWhenContainerListUnavailable(t *testing.T) {
	t.Parallel()

	src := podDiagnosticsSource{
		GetPods:         func(_ context.Context, _, _ string) (string, error) { return "", nil },
		GetEvents:       func(_ context.Context, _, _ string) (string, error) { return "", nil },
		GetPVCs:         func(_ context.Context, _, _ string) (string, error) { return "", nil },
		DescribePVCs:    func(_ context.Context, _, _ string) (string, error) { return "", nil },
		GetNonReadyPods: func(_ context.Context, _, _ string) ([]string, error) { return []string{"pod-a"}, nil },
		DescribePod:     func(_ context.Context, _, _, _ string) (string, error) { return "", nil },
		GetPodLogs: func(_ context.Context, _, _, _ string, _ int) (string, error) {
			return "", fmt.Errorf("original failure")
		},
		GetPodLogsPrevious: func(_ context.Context, _, _, _ string, _ int) (string, error) { return "", nil },
		GetPodContainers: func(_ context.Context, _, _, _ string) ([]string, error) {
			return nil, fmt.Errorf("pod gone")
		},
		GetPodContainerLogs: func(_ context.Context, _, _, _, _ string, _ int) (string, error) {
			t.Fatal("must not fetch per-container logs without a container list")
			return "", nil
		},
	}

	var buf bytes.Buffer
	printNamespaceDiagnostics(context.Background(), &buf, src, "", "ns", 500, false)
	if out := buf.String(); !strings.Contains(out, "(error: original failure)") {
		t.Errorf("expected the original logs error to be reported, got:\n%s", out)
	}
}

func TestPrintNamespaceDiagnosticsCapturesSearchClusterState(t *testing.T) {
	t.Parallel()

	type call struct {
		pod     string
		script  string
		timeout time.Duration
	}
	var calls []call
	newSrc := func(pod string, exec func(string) (string, error)) podDiagnosticsSource {
		return podDiagnosticsSource{
			GetPods:            func(_ context.Context, _, _ string) (string, error) { return "", nil },
			GetEvents:          func(_ context.Context, _, _ string) (string, error) { return "", nil },
			GetPVCs:            func(_ context.Context, _, _ string) (string, error) { return "", nil },
			DescribePVCs:       func(_ context.Context, _, _ string) (string, error) { return "", nil },
			GetNonReadyPods:    func(_ context.Context, _, _ string) ([]string, error) { return []string{pod}, nil },
			DescribePod:        func(_ context.Context, _, _, _ string) (string, error) { return "", nil },
			GetPodLogs:         func(_ context.Context, _, _, _ string, _ int) (string, error) { return "log", nil },
			GetPodLogsPrevious: func(_ context.Context, _, _, _ string, _ int) (string, error) { return "", nil },
			ExecInPod: func(_ context.Context, _, _, pod string, command []string, timeout time.Duration) (string, error) {
				script := command[len(command)-1]
				calls = append(calls, call{pod: pod, script: script, timeout: timeout})
				return exec(script)
			},
		}
	}

	// One exec per endpoint, so a slow endpoint cannot starve the others.
	var buf bytes.Buffer
	calls = nil
	src := newSrc("elasticsearch-master-0", func(string) (string, error) { return "status yellow", nil })
	printNamespaceDiagnostics(context.Background(), &buf, src, "", "ns", 500, false)

	if len(calls) != len(searchClusterQueries) {
		t.Fatalf("expected one exec per query (%d), got %d", len(searchClusterQueries), len(calls))
	}
	for i, query := range searchClusterQueries {
		if !strings.Contains(calls[i].script, query) {
			t.Errorf("exec %d does not query %q: %s", i, query, calls[i].script)
		}
		if calls[i].timeout != searchClusterExecTimeout {
			t.Errorf("exec %d timeout = %v, want %v", i, calls[i].timeout, searchClusterExecTimeout)
		}
	}
	out := buf.String()
	for _, want := range []string{"cluster state [_cluster/health]", "cluster state [_cat/indices]", "cluster state [_cluster/allocation/explain]"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing section %q:\n%s", want, out)
		}
	}

	// A failing exec must not hide the output it captured, nor the other queries.
	buf.Reset()
	calls = nil
	failFirst := newSrc("opensearch-master-0", func(script string) (string, error) {
		if strings.Contains(script, "_cluster/health") {
			return "partial health output", fmt.Errorf("signal: killed")
		}
		return "later query ran", nil
	})
	printNamespaceDiagnostics(context.Background(), &buf, failFirst, "", "ns", 500, false)
	out = buf.String()
	for _, want := range []string{"partial health output", "(error: signal: killed)", "later query ran"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}

	// Non-search pods are never exec'd into.
	buf.Reset()
	calls = nil
	printNamespaceDiagnostics(context.Background(), &buf,
		newSrc("integration-zeebe-0", func(string) (string, error) { return "", nil }), "", "ns", 500, false)
	if len(calls) != 0 {
		t.Errorf("non-search pods must not be exec'd into, got %v", calls)
	}
}

func TestSearchClusterPreludeSupportsSecureOpenSearch(t *testing.T) {
	t.Parallel()

	// The opensearch-self-signed scenarios (osss, osot) serve HTTPS with the
	// security plugin, so the probe must try https first and send admin
	// credentials from the container environment.
	var schemeLine string
	for _, line := range strings.Split(searchClusterPrelude, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "for scheme in ") {
			schemeLine = strings.TrimSpace(line)
			break
		}
	}
	if schemeLine == "" {
		t.Fatal("prelude has no scheme probe loop")
	}
	schemes := strings.Fields(strings.TrimSuffix(strings.TrimPrefix(schemeLine, "for scheme in "), "; do"))
	if len(schemes) != 2 || schemes[0] != "https" || schemes[1] != "http" {
		t.Errorf("scheme probe order = %v, want [https http]", schemes)
	}
	for _, want := range []string{
		"OPENSEARCH_INITIAL_ADMIN_PASSWORD",
		"-u admin:",
		"ELASTIC_PASSWORD",
		"-u elastic:",
		"-k",
	} {
		if !strings.Contains(searchClusterPrelude, want) {
			t.Errorf("prelude missing %q", want)
		}
	}
	// Credentials must never be echoed into CI logs.
	for _, line := range strings.Split(searchClusterPrelude, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "echo") && strings.Contains(line, "PASSWORD") {
			t.Errorf("prelude echoes a credential: %s", line)
		}
	}
}

func TestSearchQueryLabel(t *testing.T) {
	t.Parallel()
	for query, want := range map[string]string{
		"_cluster/health?level=indices&pretty": "_cluster/health",
		"_cat/indices?v&s=health:desc,index":   "_cat/indices",
		"_cluster/allocation/explain":          "_cluster/allocation/explain",
	} {
		if got := searchQueryLabel(query); got != want {
			t.Errorf("searchQueryLabel(%q) = %q, want %q", query, got, want)
		}
	}
}

func TestIsSearchPod(t *testing.T) {
	t.Parallel()
	for pod, want := range map[string]bool{
		"elasticsearch-master-0":        true,
		"opensearch-cluster-master-0":   true,
		"integration-zeebe-0":           false,
		"integration-optimize-79c-hgcw": false,
	} {
		if got := isSearchPod(pod); got != want {
			t.Errorf("isSearchPod(%q) = %v, want %v", pod, got, want)
		}
	}
}

func TestLastLines(t *testing.T) {
	t.Parallel()
	if got := lastLines("a\nb\nc\nd", 2); got != "c\nd" {
		t.Errorf("lastLines tail = %q", got)
	}
	if got := lastLines("a\nb", 5); got != "a\nb" {
		t.Errorf("short input should be unchanged, got %q", got)
	}
	if got := lastLines("", 3); got != "" {
		t.Errorf("empty input should stay empty, got %q", got)
	}
}

// TestPrintNamespaceDiagnosticsIncludesServiceState covers the shape that pod-only
// diagnostics cannot explain: every pod is Ready, but a Service selects nothing, so
// traffic goes nowhere. The dump must carry the Service ports (including
// appProtocol) and the resolved Endpoints.
func TestPrintNamespaceDiagnosticsIncludesServiceState(t *testing.T) {
	src := podDiagnosticsSource{
		GetPods:   func(_ context.Context, _, _ string) (string, error) { return "pod-a 1/1 Running", nil },
		GetEvents: func(_ context.Context, _, _ string) (string, error) { return "", nil },
		GetServices: func(_ context.Context, _, _ string) (string, error) {
			return "camunda-platform-zeebe-gateway ClusterIP 10.0.0.1 26500/TCP", nil
		},
		GetServicesYAML: func(_ context.Context, _, _ string) (string, error) {
			return "  ports:\n  - appProtocol: kubernetes.io/h2c\n    name: grpc\n    port: 26500\n  selector:\n    app.kubernetes.io/component: postgresql", nil
		},
		GetEndpoints: func(_ context.Context, _, _ string) (string, error) {
			return "camunda-platform-zeebe-gateway <none>", nil
		},
		GetNonReadyPods: func(_ context.Context, _, _ string) ([]string, error) { return nil, nil },
	}

	var buf bytes.Buffer
	printNamespaceDiagnostics(context.Background(), &buf, src, "", "camunda-platform", 10, false)
	out := buf.String()

	for _, want := range []string{
		"===== Services =====",
		"===== Service spec (YAML) =====",
		"===== Endpoints =====",
		"appProtocol: kubernetes.io/h2c",
		"app.kubernetes.io/component: postgresql",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("diagnostics dump missing %q\n---\n%s", want, out)
		}
	}
}

// TestPrintNamespaceDiagnosticsToleratesUnsetCollectors guards the struct-of-funcs
// source against a caller that has not wired every collector: the dump must degrade
// to a labelled section rather than panicking part-way through.
func TestPrintNamespaceDiagnosticsToleratesUnsetCollectors(t *testing.T) {
	src := podDiagnosticsSource{
		GetPods:         func(_ context.Context, _, _ string) (string, error) { return "pod-a 1/1 Running", nil },
		GetNonReadyPods: func(_ context.Context, _, _ string) ([]string, error) { return nil, nil },
	}

	var buf bytes.Buffer
	printNamespaceDiagnostics(context.Background(), &buf, src, "", "camunda-platform", 10, false)
	out := buf.String()

	if !strings.Contains(out, "===== Services =====") {
		t.Errorf("expected a Services section even when unset\n---\n%s", out)
	}
	if !strings.Contains(out, "not collected") {
		t.Errorf("expected unset collectors to be labelled\n---\n%s", out)
	}
}
