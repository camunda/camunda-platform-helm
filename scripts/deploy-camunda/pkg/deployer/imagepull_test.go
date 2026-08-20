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
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"scripts/deploy-camunda/pkg/types"
)

// waitingPod builds a pod whose named container is stuck in Waiting.
func waitingPod(name, container, image, reason, message string, init bool) corev1.Pod {
	status := corev1.ContainerStatus{
		Name:  container,
		Image: image,
		State: corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{Reason: reason, Message: message},
		},
	}
	pod := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if init {
		pod.Status.InitContainerStatuses = []corev1.ContainerStatus{status}
	} else {
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{status}
	}
	return pod
}

// Verbatim kubelet message from job 96141556284.
const childManifest404 = `rpc error: code = NotFound desc = failed to pull and unpack image ` +
	`"registry.camunda.cloud/vendor-ee/postgresql:15.18.0-debian-12-r17": failed to copy: ` +
	`httpReadSeeker: failed open: content at https://registry.camunda.cloud/v2/vendor-ee/postgresql/` +
	`manifests/sha256:1a9ef74da62314cdc642c7d9635f50c6a00fd96241172ed35c0b2867c4d51bb8 not found: not found`

func TestTerminalImagePullFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pod     corev1.Pod
		wantHit bool
		wantCtr string
	}{
		{
			name:    "missing child manifest is terminal",
			pod:     waitingPod("integration-postgresql-0", "postgresql", "reg/postgresql:15", "ImagePullBackOff", childManifest404, false),
			wantHit: true,
			wantCtr: "postgresql",
		},
		{
			name:    "ErrImagePull with manifest unknown is terminal",
			pod:     waitingPod("p", "c", "reg/i:1", "ErrImagePull", "manifest unknown: manifest unknown", false),
			wantHit: true,
			wantCtr: "c",
		},
		{
			name:    "init container is inspected too",
			pod:     waitingPod("p", "wait-for-es", "reg/os-shell:12", "ImagePullBackOff", childManifest404, true),
			wantHit: true,
			wantCtr: "wait-for-es",
		},
		{
			name:    "unauthorized is not terminal",
			pod:     waitingPod("p", "c", "reg/i:1", "ErrImagePull", "unauthorized: authentication required", false),
			wantHit: false,
		},
		{
			name:    "throttled pull is not terminal",
			pod:     waitingPod("p", "c", "reg/i:1", "ErrImagePull", "toomanyrequests: rate limit exceeded", false),
			wantHit: false,
		},
		{
			name:    "unrelated waiting reason is ignored",
			pod:     waitingPod("p", "c", "reg/i:1", "CrashLoopBackOff", "back-off restarting failed container", false),
			wantHit: false,
		},
		{
			name:    "running pod is ignored",
			pod:     corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p"}},
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := terminalImagePullFailure(&tt.pod)
			if ok != tt.wantHit {
				t.Fatalf("terminalImagePullFailure() ok = %v, want %v", ok, tt.wantHit)
			}
			if !tt.wantHit {
				return
			}
			if got.Container != tt.wantCtr {
				t.Errorf("container = %q, want %q", got.Container, tt.wantCtr)
			}
			if got.Pod != tt.pod.Name {
				t.Errorf("pod = %q, want %q", got.Pod, tt.pod.Name)
			}
		})
	}
}

func TestImagePullFailureErrorNamesEverything(t *testing.T) {
	t.Parallel()
	f := &ImagePullFailure{
		Pod: "integration-postgresql-0", Container: "postgresql",
		Image:  "registry.camunda.cloud/vendor-ee/postgresql:15.18.0-debian-12-r17",
		Reason: "ImagePullBackOff", Message: childManifest404,
	}
	msg := f.Error()
	for _, want := range []string{f.Pod, f.Container, f.Image, f.Reason, "not found"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q:\n%s", want, msg)
		}
	}
}

// noSleep runs the poll loop without wall-clock delay, and ends it after
// maxTicks so a test can never hang.
func noSleep(maxTicks int) func(context.Context, time.Duration) error {
	ticks := 0
	return func(ctx context.Context, _ time.Duration) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		ticks++
		if ticks > maxTicks {
			return errors.New("tick budget exhausted")
		}
		return nil
	}
}

func podList(pods ...corev1.Pod) *corev1.PodList { return &corev1.PodList{Items: pods} }

func TestWatchTerminalImagePull(t *testing.T) {
	t.Parallel()

	broken := waitingPod("integration-postgresql-0", "postgresql", "reg/pg:15", "ImagePullBackOff", childManifest404, false)
	healthy := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "integration-postgresql-0"}}

	t.Run("fires once the threshold is met", func(t *testing.T) {
		t.Parallel()
		calls := 0
		deps := imagePullWatchDeps{
			list: func(context.Context, string) (*corev1.PodList, error) {
				calls++
				return podList(broken), nil
			},
			sleep: noSleep(10), threshold: 2,
		}
		got := watchTerminalImagePull(context.Background(), deps, "ns")
		if got == nil {
			t.Fatal("expected a failure, got nil")
		}
		if calls != 2 {
			t.Errorf("expected to abort on the 2nd confirmation, got %d polls", calls)
		}
	})

	t.Run("a single observation is not enough", func(t *testing.T) {
		t.Parallel()
		calls := 0
		deps := imagePullWatchDeps{
			list: func(context.Context, string) (*corev1.PodList, error) {
				calls++
				if calls == 1 {
					return podList(broken), nil
				}
				return podList(healthy), nil
			},
			sleep: noSleep(4), threshold: 2,
		}
		if got := watchTerminalImagePull(context.Background(), deps, "ns"); got != nil {
			t.Fatalf("expected no abort after the state recovered, got %v", got)
		}
	})

	t.Run("listing errors do not abort", func(t *testing.T) {
		t.Parallel()
		deps := imagePullWatchDeps{
			list: func(context.Context, string) (*corev1.PodList, error) {
				return nil, errors.New("namespace not found")
			},
			sleep: noSleep(3), threshold: 2,
		}
		if got := watchTerminalImagePull(context.Background(), deps, "ns"); got != nil {
			t.Fatalf("a transient API error must not be read as an image failure, got %v", got)
		}
	})

	t.Run("a listing error breaks the streak", func(t *testing.T) {
		t.Parallel()
		calls := 0
		deps := imagePullWatchDeps{
			list: func(context.Context, string) (*corev1.PodList, error) {
				calls++
				if calls == 2 {
					return nil, errors.New("api server timeout")
				}
				return podList(broken), nil
			},
			sleep: noSleep(3), threshold: 2,
		}
		if got := watchTerminalImagePull(context.Background(), deps, "ns"); got != nil {
			t.Fatalf("observations either side of a blind poll are not consecutive, got %v", got)
		}
	})

	t.Run("empty namespace is a no-op", func(t *testing.T) {
		t.Parallel()
		deps := imagePullWatchDeps{
			list:  func(context.Context, string) (*corev1.PodList, error) { t.Fatal("must not poll"); return nil, nil },
			sleep: noSleep(1), threshold: 1,
		}
		if got := watchTerminalImagePull(context.Background(), deps, ""); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})
}

// fakeLister satisfies podLister.
type fakeLister struct{ pods *corev1.PodList }

func (f fakeLister) ListPods(context.Context, string) (*corev1.PodList, error) { return f.pods, nil }

// TestUpgradeInstall_AbortsOnTerminalImagePull asserts the wait ends early and
// the error names the image rather than the killed process.
func TestUpgradeInstall_AbortsOnTerminalImagePull(t *testing.T) {
	broken := waitingPod("integration-postgresql-0", "postgresql",
		"registry.camunda.cloud/vendor-ee/postgresql:15.18.0-debian-12-r17",
		"ImagePullBackOff", childManifest404, false)

	origLister := newPodLister
	newPodLister = func(string, string) (podLister, error) {
		return fakeLister{pods: podList(broken)}, nil
	}
	defer func() { newPodLister = origLister }()

	origInterval := imagePullGuardInterval
	imagePullGuardInterval = time.Millisecond
	defer func() { imagePullGuardInterval = origInterval }()

	restore := stubHelm(
		func(ctx context.Context, args []string, workDir string) error { return nil },
		func(ctx context.Context, name, url string) error { return nil },
		func(ctx context.Context) error { return nil },
	)
	defer restore()

	helmRunCapturing = func(ctx context.Context, args []string, workDir string) (string, error) {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("signal: killed")
		case <-time.After(10 * time.Second):
			return "", fmt.Errorf("test timed out: the guard never cancelled the wait")
		}
	}

	err := upgradeInstall(context.Background(), types.Options{
		ReleaseName: "integration",
		ChartPath:   "/charts/camunda-platform-8.7",
		Namespace:   "ns",
		Wait:        true,
		Timeout:     20 * time.Minute,
	})
	if err == nil {
		t.Fatal("expected the install to fail")
	}

	var helmErr *HelmError
	if !errors.As(err, &helmErr) {
		t.Fatalf("expected a *HelmError so matrix logging keeps working, got %T", err)
	}
	if !strings.Contains(helmErr.Reason, "unresolvable container image") {
		t.Errorf("reason should name the real cause, got %q", helmErr.Reason)
	}

	var pullErr *ImagePullFailure
	if !errors.As(err, &pullErr) {
		t.Fatalf("expected the cause to be an *ImagePullFailure, got %v", helmErr.Cause)
	}
	if !strings.Contains(err.Error(), "vendor-ee/postgresql") {
		t.Errorf("error should name the image, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "signal: killed") {
		t.Errorf("the killed-process error must not leak to the user, got %q", err.Error())
	}
}

// TestUpgradeInstall_NoGuardWhenNotWaiting asserts no client is built when the
// install does not wait.
func TestUpgradeInstall_NoGuardWhenNotWaiting(t *testing.T) {
	origLister := newPodLister
	newPodLister = func(string, string) (podLister, error) {
		t.Fatal("guard must not start when the install does not wait")
		return nil, nil
	}
	defer func() { newPodLister = origLister }()

	restore := stubHelm(
		func(ctx context.Context, args []string, workDir string) error { return nil },
		func(ctx context.Context, name, url string) error { return nil },
		func(ctx context.Context) error { return nil },
	)
	defer restore()

	if err := upgradeInstall(context.Background(), types.Options{
		ReleaseName: "integration",
		ChartPath:   "/charts/camunda-platform-8.7",
		Namespace:   "ns",
		Wait:        false,
	}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestImagePullGuardDisabledByEnv(t *testing.T) {
	for _, v := range []string{"off", "false", "0", "no", "OFF"} {
		t.Setenv(imagePullGuardEnvVar, v)
		if imagePullGuardEnabled() {
			t.Errorf("%s=%q should disable the guard", imagePullGuardEnvVar, v)
		}
	}
	for _, v := range []string{"", "on", "true", "anything"} {
		t.Setenv(imagePullGuardEnvVar, v)
		if !imagePullGuardEnabled() {
			t.Errorf("%s=%q should leave the guard enabled", imagePullGuardEnvVar, v)
		}
	}
}

func TestNoopGuardStopIsSafe(t *testing.T) {
	t.Parallel()
	g := noopImagePullGuard()
	if got := g.Stop(); got != nil {
		t.Fatalf("noop guard must report no failure, got %v", got)
	}
	if got := g.Stop(); got != nil {
		t.Fatalf("Stop must be idempotent, got %v", got)
	}
}
