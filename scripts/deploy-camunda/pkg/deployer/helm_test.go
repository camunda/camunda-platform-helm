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

package deployer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"scripts/deploy-camunda/pkg/types"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAppendHelmValueArgsKeepsDeterministicSetsAndExtraArgsLast(t *testing.T) {
	args := appendHelmValueArgs([]string{"template"}, types.Options{
		ValuesFiles: []string{"base.yaml"},
		SetPairs:    map[string]string{"z": "last-key", "a": "first-key"},
		ExtraArgs:   []string{"--set", "a=override"},
	})
	want := "template -f base.yaml --set a=first-key --set z=last-key --set a=override"
	if got := strings.Join(args, " "); got != want {
		t.Fatalf("args = %q, want %q", got, want)
	}
}

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
// reset the originals. The wait-flag detector is pinned to "--wait" so tests
// don't depend on the local Helm CLI version.
//
// helmRunCapturing is stubbed as a wrapper around runFn that returns empty stderr,
// so existing tests that verify error propagation are unaffected. To test retry
// behaviour, override helmRunCapturing directly after calling stubHelm.
func stubHelm(
	runFn func(ctx context.Context, args []string, workDir string) error,
	repoAddFn func(ctx context.Context, name, url string) error,
	repoUpdateFn func(ctx context.Context) error,
) func() {
	origCapturing, origAdd, origUpdate, origWait := helmRunCapturing, helmRepoAdd, helmRepoUpdate, helmWaitFlag
	origReadiness := newReadinessClient
	newReadinessClient = func(string, string) (readinessClient, error) {
		return nil, errors.New("no diagnostic client in Helm unit tests")
	}
	helmRunCapturing = func(ctx context.Context, args []string, workDir string) (string, error) {
		return "", runFn(ctx, args, workDir)
	}
	helmRepoAdd = repoAddFn
	helmRepoUpdate = repoUpdateFn
	helmWaitFlag = func(context.Context) string { return "--wait" }
	return func() {
		helmRunCapturing, helmRepoAdd, helmRepoUpdate, helmWaitFlag = origCapturing, origAdd, origUpdate, origWait
		newReadinessClient = origReadiness
	}
}

type fakeReadinessClient struct {
	pods     *corev1.PodList
	events   *corev1.EventList
	podErr   error
	eventErr error
}

func (client fakeReadinessClient) ListPods(ctx context.Context, _ string) (*corev1.PodList, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return client.pods, client.podErr
}

func (client fakeReadinessClient) ListEvents(context.Context, string) (*corev1.EventList, error) {
	return client.events, client.eventErr
}

func TestHelmFailureReadinessSummary(t *testing.T) {
	t.Setenv(imagePullGuardEnvVar, "off")
	started := time.Now().Add(-10 * time.Minute)
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "integration-identity", UID: "pod-uid", Labels: map[string]string{"app.kubernetes.io/instance": "integration"}},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Name: "identity", State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(started)},
			}}},
		},
	}
	event := corev1.Event{
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", UID: pod.UID, FieldPath: "spec.containers{identity}"},
		Reason:         "Unhealthy", Message: "Readiness probe failed: HTTP probe failed with statuscode: 503",
		LastTimestamp: metav1.Now(),
	}
	for _, test := range []struct {
		name       string
		mutate     func(*corev1.Pod, *corev1.Event)
		podErr     error
		eventErr   error
		companion  bool
		noWait     bool
		success    bool
		wantDetail bool
		wantEvent  bool
	}{
		{name: "running unready timeout", wantDetail: true, wantEvent: true},
		{name: "companion uses its own release", companion: true, wantDetail: true, wantEvent: true},
		{name: "events forbidden", eventErr: errors.New("forbidden"), wantDetail: true},
		{name: "pods forbidden", podErr: errors.New("forbidden")},
		{name: "different pod UID", mutate: func(_ *corev1.Pod, event *corev1.Event) { event.InvolvedObject.UID = "old-pod" }, wantDetail: true},
		{name: "event before current container", mutate: func(_ *corev1.Pod, event *corev1.Event) {
			event.LastTimestamp = metav1.NewTime(started.Add(-time.Minute))
		}, wantDetail: true},
		{name: "no event timestamp", mutate: func(_ *corev1.Pod, event *corev1.Event) { event.LastTimestamp = metav1.Time{} }, wantDetail: true},
		{name: "different container", mutate: func(_ *corev1.Pod, event *corev1.Event) { event.InvolvedObject.FieldPath = "spec.containers{sidecar}" }, wantDetail: true},
		{name: "liveness failure", mutate: func(_ *corev1.Pod, event *corev1.Event) { event.Message = "Liveness probe failed: HTTP 500" }, wantDetail: true},
		{name: "ready", mutate: func(pod *corev1.Pod, _ *corev1.Event) { pod.Status.ContainerStatuses[0].Ready = true }},
		{name: "other release", mutate: func(pod *corev1.Pod, _ *corev1.Event) { pod.Labels["app.kubernetes.io/instance"] = "other" }},
		{name: "legacy release label", mutate: func(pod *corev1.Pod, _ *corev1.Event) { pod.Labels = map[string]string{"release": "integration"} }, wantDetail: true, wantEvent: true},
		{name: "unlabelled pod", mutate: func(pod *corev1.Pod, _ *corev1.Event) { pod.Labels = nil }},
		{name: "terminating", mutate: func(pod *corev1.Pod, _ *corev1.Event) { now := metav1.Now(); pod.DeletionTimestamp = &now }},
		{name: "completed", mutate: func(pod *corev1.Pod, _ *corev1.Event) { pod.Status.Phase = corev1.PodSucceeded }},
		{name: "not waiting", noWait: true},
		{name: "success", success: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			currentPod, currentEvent := pod.DeepCopy(), event.DeepCopy()
			if test.mutate != nil {
				test.mutate(currentPod, currentEvent)
			}
			exitErr := errors.New("exit status 1")
			restore := stubHelm(func(context.Context, []string, string) error {
				if test.success {
					return nil
				}
				return exitErr
			}, nil, nil)
			t.Cleanup(restore)
			newReadinessClient = func(string, string) (readinessClient, error) {
				if test.success || test.noWait {
					t.Fatal("must not collect diagnostics")
				}
				return fakeReadinessClient{pods: podList(*currentPod), events: &corev1.EventList{Items: []corev1.Event{*currentEvent}}, podErr: test.podErr, eventErr: test.eventErr}, nil
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			opts := types.Options{ReleaseName: "integration", Namespace: "issue7210", Wait: !test.noWait}
			var err error
			if test.companion {
				opts.ReleaseName, opts.Wait = "main-release", false
				err = deployCompanionChart(ctx, types.CompanionChart{ReleaseName: "integration"}, opts)
			} else {
				err = upgradeInstall(ctx, opts)
			}
			if test.success {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var helmErr *HelmError
			if !errors.As(err, &helmErr) || !errors.Is(err, exitErr) {
				t.Fatalf("original error lost: %v", err)
			}
			if got := strings.Contains(err.Error(), "still not ready"); got != test.wantDetail {
				t.Fatalf("readiness detail=%t, error=%v", got, err)
			}
			if got := strings.Contains(err.Error(), "last probe failure"); got != test.wantEvent {
				t.Fatalf("probe event=%t, error=%v", got, err)
			}
			if test.wantDetail {
				for _, want := range []string{"issue7210", "integration-identity", "identity", "Running, Ready=false, restarts=0"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("missing %q: %v", want, err)
					}
				}
			}
		})
	}
}

func TestLastReadinessFailure(t *testing.T) {
	t.Parallel()
	now := time.Now()
	pod := corev1.Pod{ObjectMeta: metav1.ObjectMeta{UID: "uid"}}
	container := corev1.ContainerStatus{Name: "app", State: corev1.ContainerState{
		Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(now.Add(-time.Hour))},
	}}
	event := corev1.Event{
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", UID: pod.UID, FieldPath: "spec.containers{app}"},
		Reason:         "Unhealthy", Message: "Readiness probe failed: older",
		LastTimestamp: metav1.NewTime(now.Add(-time.Minute)),
	}
	latest := *event.DeepCopy()
	latest.Message = "Readiness probe failed: latest"
	latest.Series = &corev1.EventSeries{LastObservedTime: metav1.NewMicroTime(now)}
	eventTime := *event.DeepCopy()
	eventTime.EventTime = metav1.NewMicroTime(now.Add(-time.Second))
	for _, events := range []*corev1.EventList{
		{Items: []corev1.Event{latest, event, eventTime}},
		{Items: []corev1.Event{event, eventTime, latest}},
	} {
		if got := lastReadinessFailure(pod, container, events); got != latest.Message {
			t.Fatalf("latest probe failure = %q, want %q", got, latest.Message)
		}
	}
	if got := lastReadinessFailure(pod, container, nil); got != "" {
		t.Fatalf("missing events must not invent a failure: %q", got)
	}
}

func TestDeployCompanionChart(t *testing.T) {
	tests := []struct {
		name          string
		cc            types.CompanionChart
		opts          types.Options
		runErr        error
		repoAddErr    error
		repoUpdErr    error
		valuesContent string
		wantErr       string   // substring expected in error message; empty = no error
		wantArgs      []string // substrings expected in the helm run args
		notWantArgs   []string // substrings that must NOT appear in helm run args
		wantRepoAdd   bool     // expect helmRepoAdd to be called
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
		{
			name: "node selector and tolerations rendered as set-json",
			cc: types.CompanionChart{
				ChartRef:    "bitnami/redis",
				Version:     "18.0.0",
				ReleaseName: "redis",
			},
			opts: types.Options{
				Namespace:             "ns",
				CompanionNodeSelector: map[string]string{"pool": "companion"},
				CompanionTolerations:  []map[string]interface{}{{"key": "pool", "operator": "Equal", "value": "companion"}},
			},
			wantArgs: []string{
				"--set-json", `nodeSelector={"pool":"companion"}`,
				"--set-json", `tolerations=[{"key":"pool","operator":"Equal","value":"companion"}]`,
			},
		},
		{
			name: "elasticsearch storage class override",
			cc: types.CompanionChart{
				ChartRef:    "elastic/elasticsearch",
				Version:     "8.5.1",
				ReleaseName: "elasticsearch",
			},
			opts: types.Options{
				Namespace:             "ns",
				CompanionStorageClass: "hyperdisk-balanced",
			},
			wantArgs: []string{"--set", "volumeClaimTemplate.storageClassName=hyperdisk-balanced"},
		},
		{
			name: "postgresql companion storage class override",
			cc: types.CompanionChart{
				ChartRef:    "internal-postgresql",
				ReleaseName: "postgresql",
			},
			opts: types.Options{
				Namespace:             "ns",
				CompanionStorageClass: "hyperdisk-balanced",
			},
			wantArgs: []string{"--set", "persistence.storageClass=hyperdisk-balanced"},
		},
		{
			name: "keycloak postgresql storage class override",
			cc: types.CompanionChart{
				ChartRef:    "internal-keycloak-26",
				ReleaseName: "keycloak",
			},
			opts: types.Options{
				Namespace:             "ns",
				CompanionStorageClass: "hyperdisk-balanced",
			},
			wantArgs: []string{"--set", "postgresql.storage.storageClassName=hyperdisk-balanced", "postgresql.storage.size=4Gi"},
		},
		{
			name:          "keycloak raises an undersized hyperdisk volume",
			cc:            types.CompanionChart{ChartRef: "internal-keycloak-26", ReleaseName: "keycloak"},
			opts:          types.Options{Namespace: "ns", CompanionStorageClass: "hyperdisk-balanced"},
			valuesContent: "postgresql:\n  storage:\n    size: 2Gi\n",
			wantArgs:      []string{"postgresql.storage.size=4Gi"},
		},
		{
			name:          "keycloak preserves a larger hyperdisk volume",
			cc:            types.CompanionChart{ChartRef: "internal-keycloak-26", ReleaseName: "keycloak"},
			opts:          types.Options{Namespace: "ns", CompanionStorageClass: "hyperdisk-balanced"},
			valuesContent: "postgresql:\n  storage:\n    size: 8Gi\n",
			wantArgs:      []string{"postgresql.storage.storageClassName=hyperdisk-balanced"},
			notWantArgs:   []string{"postgresql.storage.size="},
		},
		{
			name:          "keycloak preserves a minimum-sized hyperdisk volume",
			cc:            types.CompanionChart{ChartRef: "internal-keycloak-26", ReleaseName: "keycloak"},
			opts:          types.Options{Namespace: "ns", CompanionStorageClass: "hyperdisk-balanced"},
			valuesContent: "postgresql:\n  storage:\n    size: 4096Mi\n",
			notWantArgs:   []string{"postgresql.storage.size="},
		},
		{
			name:          "keycloak rejects an invalid hyperdisk storage quantity",
			cc:            types.CompanionChart{ChartRef: "internal-keycloak-26", ReleaseName: "keycloak"},
			opts:          types.Options{Namespace: "ns", CompanionStorageClass: "hyperdisk-balanced"},
			valuesContent: "postgresql:\n  storage:\n    size: invalid\n",
			wantErr:       "parse keycloak postgresql storage size",
		},
		{
			name:    "keycloak reports unreadable storage values",
			cc:      types.CompanionChart{ChartRef: "internal-keycloak-26", ReleaseName: "keycloak", ValuesFile: "/nonexistent/6833-keycloak-values.yaml"},
			opts:    types.Options{Namespace: "ns", CompanionStorageClass: "hyperdisk-balanced"},
			wantErr: "read keycloak companion values",
		},
		{
			name:          "keycloak reports malformed storage values",
			cc:            types.CompanionChart{ChartRef: "internal-keycloak-26", ReleaseName: "keycloak"},
			opts:          types.Options{Namespace: "ns", CompanionStorageClass: "hyperdisk-balanced"},
			valuesContent: "postgresql: [",
			wantErr:       "parse keycloak companion values",
		},
		{
			name: "keycloak preserves the configured storage class without an override",
			cc: types.CompanionChart{
				ChartRef:    "internal-keycloak-26",
				ReleaseName: "keycloak",
				ValuesFile:  "/tmp/keycloak-values.yaml",
			},
			opts:        types.Options{Namespace: "ns"},
			wantArgs:    []string{"-f", "/tmp/keycloak-values.yaml"},
			notWantArgs: []string{"--set", "storageClassName"},
		},
		{
			name: "unmarshalable tolerations value propagates as HelmError",
			cc: types.CompanionChart{
				ChartRef:    "bitnami/redis",
				Version:     "18.0.0",
				ReleaseName: "redis",
			},
			opts: types.Options{
				Namespace:            "ns",
				CompanionTolerations: []map[string]interface{}{{"bad": make(chan int)}},
			},
			wantErr: "marshal tolerations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valuesContent != "" {
				tt.cc.ValuesFile = filepath.Join(t.TempDir(), "values.yaml")
				if err := os.WriteFile(tt.cc.ValuesFile, []byte(tt.valuesContent), 0o600); err != nil {
					t.Fatal(err)
				}
			}
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

func TestUpgradeInstall_WaitFlagDelegatesToHelmWaitFlag(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		wantArg  string
		wantWait bool
	}{
		{name: "helm v3 returns --wait", flag: "--wait", wantArg: "--wait", wantWait: true},
		{name: "helm v4 returns --wait=legacy", flag: "--wait=legacy", wantArg: "--wait=legacy", wantWait: true},
		{name: "wait disabled emits no flag", flag: "--wait=legacy", wantWait: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedArgs []string
			restore := stubHelm(
				func(ctx context.Context, args []string, workDir string) error {
					capturedArgs = args
					return nil
				},
				func(ctx context.Context, name, url string) error { return nil },
				func(ctx context.Context) error { return nil },
			)
			defer restore()

			origWait := helmWaitFlag
			helmWaitFlag = func(context.Context) string { return tt.flag }
			defer func() { helmWaitFlag = origWait }()

			err := upgradeInstall(context.Background(), types.Options{
				ChartPath:   "/charts/x",
				ReleaseName: "r",
				Namespace:   "ns",
				Wait:        tt.wantWait,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			argsStr := strings.Join(capturedArgs, " ")
			if tt.wantWait {
				if !containsArg(capturedArgs, tt.wantArg) {
					t.Errorf("args = %q, want exact arg %q", argsStr, tt.wantArg)
				}
				other := "--wait"
				if tt.wantArg == "--wait" {
					other = "--wait=legacy"
				}
				if containsArg(capturedArgs, other) {
					t.Errorf("args = %q, must not contain exact arg %q", argsStr, other)
				}
			} else {
				if containsArg(capturedArgs, "--wait") || containsArg(capturedArgs, "--wait=legacy") {
					t.Errorf("args = %q, should not contain any --wait variant when Wait=false", argsStr)
				}
			}
		})
	}
}

func TestDeployCompanionChart_UsesHelmWaitFlag(t *testing.T) {
	var capturedArgs []string
	restore := stubHelm(
		func(ctx context.Context, args []string, workDir string) error {
			capturedArgs = args
			return nil
		},
		func(ctx context.Context, name, url string) error { return nil },
		func(ctx context.Context) error { return nil },
	)
	defer restore()

	origWait := helmWaitFlag
	helmWaitFlag = func(context.Context) string { return "--wait=legacy" }
	defer func() { helmWaitFlag = origWait }()

	err := deployCompanionChart(context.Background(), types.CompanionChart{
		ChartRef:    "/charts/local",
		ReleaseName: "local",
	}, types.Options{Namespace: "ns"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !containsArg(capturedArgs, "--wait=legacy") {
		t.Errorf("args = %q, want exact arg --wait=legacy", argsStr)
	}
	if containsArg(capturedArgs, "--wait") {
		t.Errorf("args = %q, must not contain bare --wait alongside --wait=legacy", argsStr)
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
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

func TestUpgradeInstall_TransientRetry(t *testing.T) {
	// First call returns a transient API-server 500; second call succeeds.
	// helmUpgradeRetryDelay is patched to zero so the test doesn't actually sleep.
	origDelay := helmUpgradeRetryDelay
	helmUpgradeRetryDelay = 0
	defer func() { helmUpgradeRetryDelay = origDelay }()

	restore := stubHelm(
		func(ctx context.Context, args []string, workDir string) error { return nil },
		func(ctx context.Context, name, url string) error { return nil },
		func(ctx context.Context) error { return nil },
	)
	defer restore()

	attempts := 0
	helmRunCapturing = func(ctx context.Context, args []string, workDir string) (string, error) {
		attempts++
		if attempts == 1 {
			return "Error: unable to continue with install: Internal Server Error", fmt.Errorf("exit status 1")
		}
		return "", nil
	}

	err := upgradeInstall(context.Background(), types.Options{
		ReleaseName: "integration",
		ChartPath:   "/charts/camunda-platform-8.9",
		Namespace:   "ns",
	})

	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 helm invocations (1 transient + 1 retry), got %d", attempts)
	}
}

func TestUpgradeInstall_NoRetryOnNonTransient(t *testing.T) {
	restore := stubHelm(
		func(ctx context.Context, args []string, workDir string) error { return nil },
		func(ctx context.Context, name, url string) error { return nil },
		func(ctx context.Context) error { return nil },
	)
	defer restore()

	attempts := 0
	helmRunCapturing = func(ctx context.Context, args []string, workDir string) (string, error) {
		attempts++
		return "Error: render error in \"camunda/templates/foo.yaml\": template: undefined variable", fmt.Errorf("exit status 1")
	}

	err := upgradeInstall(context.Background(), types.Options{
		ReleaseName: "integration",
		ChartPath:   "/charts/camunda-platform-8.9",
		Namespace:   "ns",
	})

	if err == nil {
		t.Fatal("expected error for non-transient failure, got nil")
	}
	if attempts != 1 {
		t.Errorf("expected exactly 1 helm invocation for non-transient error, got %d", attempts)
	}
}

func TestDeployCompanionCharts_DeploysInParallel(t *testing.T) {
	restore := stubHelm(
		func(ctx context.Context, args []string, workDir string) error { return nil },
		func(ctx context.Context, name, url string) error { return nil },
		func(ctx context.Context) error { return nil },
	)
	defer restore()

	var inFlight, maxInFlight, total int32
	helmRunCapturing = func(ctx context.Context, args []string, workDir string) (string, error) {
		atomic.AddInt32(&total, 1)
		cur := atomic.AddInt32(&inFlight, 1)
		for {
			prev := atomic.LoadInt32(&maxInFlight)
			if cur <= prev || atomic.CompareAndSwapInt32(&maxInFlight, prev, cur) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		return "", nil
	}

	err := deployCompanionCharts(context.Background(), types.Options{
		Namespace: "ns",
		CompanionCharts: []types.CompanionChart{
			{ChartRef: "/charts/a", ReleaseName: "a"},
			{ChartRef: "/charts/b", ReleaseName: "b"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("expected both companions to deploy, got %d invocations", total)
	}
	if maxInFlight < 2 {
		t.Errorf("expected companions to overlap (maxInFlight>=2), got %d — deploys ran serially", maxInFlight)
	}
}

func TestDeployCompanionCharts_ErrorCancelsSiblings(t *testing.T) {
	restore := stubHelm(
		func(ctx context.Context, args []string, workDir string) error { return nil },
		func(ctx context.Context, name, url string) error { return nil },
		func(ctx context.Context) error { return nil },
	)
	defer restore()

	var siblingCancelled atomic.Bool
	helmRunCapturing = func(ctx context.Context, args []string, workDir string) (string, error) {
		if containsArg(args, "fails") {
			return "", fmt.Errorf("boom")
		}
		// Slow sibling: block until the failing companion cancels the group ctx.
		select {
		case <-ctx.Done():
			siblingCancelled.Store(true)
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
			return "", nil
		}
	}

	err := deployCompanionCharts(context.Background(), types.Options{
		Namespace: "ns",
		CompanionCharts: []types.CompanionChart{
			{ChartRef: "/charts/fails", ReleaseName: "fails"},
			{ChartRef: "/charts/slow", ReleaseName: "slow"},
		},
	})
	if err == nil {
		t.Fatal("expected error from failing companion, got nil")
	}
	if !strings.Contains(err.Error(), "companion chart") {
		t.Errorf("error = %q, want it to mention the failing companion chart", err.Error())
	}
	if !siblingCancelled.Load() {
		t.Error("expected the in-flight sibling to observe context cancellation")
	}
}

func TestDeployCompanionCharts_SingleCompanionSucceeds(t *testing.T) {
	restore := stubHelm(
		func(ctx context.Context, args []string, workDir string) error { return nil },
		func(ctx context.Context, name, url string) error { return nil },
		func(ctx context.Context) error { return nil },
	)
	defer restore()

	var calls int32
	helmRunCapturing = func(ctx context.Context, args []string, workDir string) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "", nil
	}

	err := deployCompanionCharts(context.Background(), types.Options{
		Namespace: "ns",
		CompanionCharts: []types.CompanionChart{
			{ChartRef: "/charts/only", ReleaseName: "only"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 companion deploy, got %d", calls)
	}
}

func TestDeployCompanionCharts_RepoRegistrationSerialized(t *testing.T) {
	var inFlight, maxRepoInFlight int32
	repoOp := func() {
		cur := atomic.AddInt32(&inFlight, 1)
		for {
			prev := atomic.LoadInt32(&maxRepoInFlight)
			if cur <= prev || atomic.CompareAndSwapInt32(&maxRepoInFlight, prev, cur) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
	}
	restore := stubHelm(
		func(ctx context.Context, args []string, workDir string) error { return nil },
		func(ctx context.Context, name, url string) error { repoOp(); return nil },
		func(ctx context.Context) error { return nil },
	)
	defer restore()

	err := deployCompanionCharts(context.Background(), types.Options{
		Namespace: "ns",
		CompanionCharts: []types.CompanionChart{
			{ChartRef: "x/a", ReleaseName: "a", RepoName: "x", RepoURL: "https://x"},
			{ChartRef: "y/b", ReleaseName: "b", RepoName: "y", RepoURL: "https://y"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maxRepoInFlight != 1 {
		t.Errorf("repo registration must be serialized (maxRepoInFlight=1), got %d", maxRepoInFlight)
	}
}
