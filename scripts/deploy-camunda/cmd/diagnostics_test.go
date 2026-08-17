package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
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

	var execedPods []string
	newSrc := func(pod string) podDiagnosticsSource {
		return podDiagnosticsSource{
			GetPods:            func(_ context.Context, _, _ string) (string, error) { return "", nil },
			GetEvents:          func(_ context.Context, _, _ string) (string, error) { return "", nil },
			GetPVCs:            func(_ context.Context, _, _ string) (string, error) { return "", nil },
			DescribePVCs:       func(_ context.Context, _, _ string) (string, error) { return "", nil },
			GetNonReadyPods:    func(_ context.Context, _, _ string) ([]string, error) { return []string{pod}, nil },
			DescribePod:        func(_ context.Context, _, _, _ string) (string, error) { return "", nil },
			GetPodLogs:         func(_ context.Context, _, _, _ string, _ int) (string, error) { return "log", nil },
			GetPodLogsPrevious: func(_ context.Context, _, _, _ string, _ int) (string, error) { return "", nil },
			ExecInPod: func(_ context.Context, _, _, pod string, _ []string) (string, error) {
				execedPods = append(execedPods, pod)
				return "status yellow, 1 unassigned shard", nil
			},
		}
	}

	var buf bytes.Buffer
	printNamespaceDiagnostics(context.Background(), &buf, newSrc("elasticsearch-master-0"), "", "ns", 500, false)
	if out := buf.String(); !strings.Contains(out, "cluster state") || !strings.Contains(out, "1 unassigned shard") {
		t.Errorf("expected search cluster state to be captured, got:\n%s", out)
	}

	execedPods = nil
	buf.Reset()
	printNamespaceDiagnostics(context.Background(), &buf, newSrc("integration-zeebe-0"), "", "ns", 500, false)
	if len(execedPods) != 0 {
		t.Errorf("non-search pods must not be exec'd into, got %v", execedPods)
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
