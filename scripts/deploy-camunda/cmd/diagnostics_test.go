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
	printNamespaceDiagnostics(context.Background(), &buf, src, "", "ns", 500)
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
	printNamespaceDiagnostics(context.Background(), &buf, src, "", "ns", 10)
	out := buf.String()
	if !strings.Contains(out, "(error: boom)") || !strings.Contains(out, "(error: list failed)") {
		t.Errorf("expected inline errors, got:\n%s", out)
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
