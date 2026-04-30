package deployer

import (
	"context"
	"errors"
	"fmt"
	"scripts/camunda-deployer/pkg/types"
	"strings"
	"testing"
	"time"
)

func TestHelmError_Error(t *testing.T) {
	err := &HelmError{
		Reason:  "helm upgrade --install failed",
		Command: "helm upgrade --install integration /path/to/chart -n ns --wait",
		Cause:   fmt.Errorf("exit status 1"),
	}

	got := err.Error()
	want := "helm upgrade --install failed: exit status 1"
	if got != want {
		t.Errorf("HelmError.Error() = %q, want %q", got, want)
	}
}

func TestHelmError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("exit status 1")
	err := &HelmError{
		Reason:  "helm upgrade --install failed",
		Command: "helm upgrade --install integration /path/to/chart -n ns --wait",
		Cause:   cause,
	}

	if !errors.Is(err, cause) {
		t.Error("HelmError.Unwrap: expected errors.Is to find cause")
	}
}

func TestHelmError_ShortCommand(t *testing.T) {
	err := &HelmError{
		Reason: "helm upgrade --install failed",
		Command: "helm upgrade --install integration /Users/user/workspaces/camunda-platform-helm/charts/camunda-platform-8.9 " +
			"-n distribution-89-esarm-inst --create-namespace --kube-context gke_camunda-distribution_europe-west1-b_distro-ci " +
			"--wait --atomic --timeout 300s " +
			"-f /var/folders/5n/00q4pg1s0fv11255p5zzlbg40000gn/T/camunda-values-elasticsearch-arm-4206716148/base.yaml " +
			"-f /var/folders/5n/00q4pg1s0fv11255p5zzlbg40000gn/T/camunda-values-elasticsearch-arm-4206716148/keycloak.yaml " +
			"-f /var/folders/5n/00q4pg1s0fv11255p5zzlbg40000gn/T/camunda-values-elasticsearch-arm-4206716148/elasticsearch.yaml",
		Cause: fmt.Errorf("exit status 1"),
	}

	short := err.ShortCommand()

	// Chart path should be shortened
	if !strings.Contains(short, "camunda-platform-8.9") {
		t.Errorf("ShortCommand: expected shortened chart name in output, got: %s", short)
	}
	if strings.Contains(short, "/Users/user/workspaces") {
		t.Errorf("ShortCommand: expected long chart path to be shortened, got: %s", short)
	}

	// Values file paths should be shortened
	if strings.Contains(short, "/var/folders") {
		t.Errorf("ShortCommand: expected long values paths to be shortened, got: %s", short)
	}
	if !strings.Contains(short, "base.yaml") || !strings.Contains(short, "keycloak.yaml") || !strings.Contains(short, "elasticsearch.yaml") {
		t.Errorf("ShortCommand: expected base filenames to be preserved, got: %s", short)
	}

	// Non-path args should be preserved
	if !strings.Contains(short, "--kube-context") || !strings.Contains(short, "gke_camunda-distribution_europe-west1-b_distro-ci") {
		t.Errorf("ShortCommand: expected non-path args to be preserved, got: %s", short)
	}
}

func TestShortenPaths_NoAbsolutePaths(t *testing.T) {
	cmd := "helm upgrade --install integration camunda/camunda-platform -n ns --version 13.5.0 --wait"
	got := shortenPaths(cmd)
	if got != cmd {
		t.Errorf("shortenPaths: should not modify command without absolute paths: got %q, want %q", got, cmd)
	}
}

// stubHelm replaces the package-level helm function variables with test stubs
// and returns a restore function that must be called (typically via defer) to
// reset the originals.
func stubHelm(
	runFn func(ctx context.Context, args []string, workDir string) error,
	repoAddFn func(ctx context.Context, name, url string) error,
	repoUpdateFn func(ctx context.Context) error,
) func() {
	origRun, origAdd, origUpdate := helmRun, helmRepoAdd, helmRepoUpdate
	helmRun = runFn
	helmRepoAdd = repoAddFn
	helmRepoUpdate = repoUpdateFn
	return func() {
		helmRun, helmRepoAdd, helmRepoUpdate = origRun, origAdd, origUpdate
	}
}

func TestDeployCompanionChart(t *testing.T) {
	tests := []struct {
		name        string
		cc          types.CompanionChart
		opts        types.Options
		runErr      error
		repoAddErr  error
		repoUpdErr  error
		wantErr     string   // substring expected in error message; empty = no error
		wantArgs    []string // substrings expected in the helm run args
		notWantArgs []string // substrings that must NOT appear in helm run args
		wantRepoAdd bool     // expect helmRepoAdd to be called
	}{
		{
			name: "remote chart with version and repo registration",
			cc: types.CompanionChart{
				ChartRef:    "opensearch/opensearch",
				Version:     "3.6.0",
				ReleaseName: "opensearch",
				ValuesFile:  "/tmp/values.yaml",
				RepoName:    "opensearch",
				RepoURL:     "https://opensearch-project.github.io/helm-charts/",
			},
			opts: types.Options{
				Namespace:   "test-ns",
				Kubeconfig:  "/tmp/kubeconfig",
				KubeContext: "test-ctx",
				Timeout:     5 * time.Minute,
			},
			wantArgs: []string{
				"upgrade", "--install",
				"opensearch",
				"opensearch/opensearch",
				"-n", "test-ns",
				"--create-namespace",
				"--wait",
				"--version", "3.6.0",
				"--timeout", "300s",
				"--kubeconfig", "/tmp/kubeconfig",
				"--kube-context", "test-ctx",
				"-f", "/tmp/values.yaml",
			},
			wantRepoAdd: true,
		},
		{
			name: "local chart path without version or repo",
			cc: types.CompanionChart{
				ChartRef:    "/charts/local-chart",
				ReleaseName: "local",
			},
			opts: types.Options{
				Namespace: "default",
			},
			wantArgs: []string{
				"upgrade", "--install",
				"local",
				"/charts/local-chart",
				"-n", "default",
				"--create-namespace",
				"--wait",
			},
			notWantArgs: []string{"--version"},
			wantRepoAdd: false,
		},
		{
			name: "no values file omits -f flag",
			cc: types.CompanionChart{
				ChartRef:    "bitnami/redis",
				Version:     "18.0.0",
				ReleaseName: "redis",
			},
			opts: types.Options{
				Namespace: "cache-ns",
			},
			notWantArgs: []string{"-f"},
			wantRepoAdd: false,
		},
		{
			name: "no timeout omits --timeout flag",
			cc: types.CompanionChart{
				ChartRef:    "bitnami/redis",
				Version:     "18.0.0",
				ReleaseName: "redis",
			},
			opts: types.Options{
				Namespace: "cache-ns",
			},
			notWantArgs: []string{"--timeout"},
		},
		{
			name: "repo add error propagates",
			cc: types.CompanionChart{
				ChartRef:    "opensearch/opensearch",
				Version:     "3.6.0",
				ReleaseName: "opensearch",
				RepoName:    "opensearch",
				RepoURL:     "https://example.com/charts",
			},
			opts:        types.Options{Namespace: "ns"},
			repoAddErr:  fmt.Errorf("network timeout"),
			wantErr:     "repo add failed",
			wantRepoAdd: true,
		},
		{
			name: "repo update error propagates",
			cc: types.CompanionChart{
				ChartRef:    "opensearch/opensearch",
				Version:     "3.6.0",
				ReleaseName: "opensearch",
				RepoName:    "opensearch",
				RepoURL:     "https://example.com/charts",
			},
			opts:        types.Options{Namespace: "ns"},
			repoUpdErr:  fmt.Errorf("index fetch failed"),
			wantErr:     "repo update failed",
			wantRepoAdd: true,
		},
		{
			name: "helm run error wraps as HelmError",
			cc: types.CompanionChart{
				ChartRef:    "opensearch/opensearch",
				Version:     "3.6.0",
				ReleaseName: "opensearch",
			},
			opts:    types.Options{Namespace: "ns"},
			runErr:  fmt.Errorf("exit status 1"),
			wantErr: "helm upgrade --install failed",
		},
		{
			name: "repo fields partially set skips repo registration",
			cc: types.CompanionChart{
				ChartRef:    "opensearch/opensearch",
				Version:     "3.6.0",
				ReleaseName: "opensearch",
				RepoName:    "opensearch",
				// RepoURL intentionally empty
			},
			opts:        types.Options{Namespace: "ns"},
			wantRepoAdd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedArgs []string
			repoAddCalled := false

			restore := stubHelm(
				func(ctx context.Context, args []string, workDir string) error {
					capturedArgs = args
					return tt.runErr
				},
				func(ctx context.Context, name, url string) error {
					repoAddCalled = true
					return tt.repoAddErr
				},
				func(ctx context.Context) error {
					return tt.repoUpdErr
				},
			)
			defer restore()

			err := deployCompanionChart(context.Background(), tt.cc, tt.opts)

			// Check error expectations
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check repo add was/wasn't called
			if tt.wantRepoAdd != repoAddCalled {
				t.Errorf("helmRepoAdd called = %v, want %v", repoAddCalled, tt.wantRepoAdd)
			}

			// Check expected args are present
			argsStr := strings.Join(capturedArgs, " ")
			for _, want := range tt.wantArgs {
				if !strings.Contains(argsStr, want) {
					t.Errorf("args = %q, missing expected substring %q", argsStr, want)
				}
			}

			// Check unwanted args are absent
			for _, notWant := range tt.notWantArgs {
				if strings.Contains(argsStr, notWant) {
					t.Errorf("args = %q, should not contain %q", argsStr, notWant)
				}
			}
		})
	}
}

func TestDeployCompanionChart_HelmErrorType(t *testing.T) {
	restore := stubHelm(
		func(ctx context.Context, args []string, workDir string) error {
			return fmt.Errorf("exit status 1")
		},
		func(ctx context.Context, name, url string) error { return nil },
		func(ctx context.Context) error { return nil },
	)
	defer restore()

	err := deployCompanionChart(context.Background(), types.CompanionChart{
		ChartRef:    "opensearch/opensearch",
		Version:     "3.6.0",
		ReleaseName: "opensearch",
	}, types.Options{Namespace: "ns"})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var helmErr *HelmError
	if !errors.As(err, &helmErr) {
		t.Fatalf("expected *HelmError, got %T", err)
	}
	if !strings.Contains(helmErr.Reason, "companion chart") {
		t.Errorf("HelmError.Reason = %q, want it to mention companion chart", helmErr.Reason)
	}
	if !strings.Contains(helmErr.Command, "helm upgrade --install") {
		t.Errorf("HelmError.Command = %q, want it to contain full command", helmErr.Command)
	}
}
