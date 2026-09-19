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

// Verbatim kubelet and scheduler messages.
const (
	backoffMessage       = "back-off 5m0s restarting failed container=zeebe pod=camunda-zeebe-0_ns(9f2)"
	missingKeyMessage    = `couldn't find key smtp-password in Secret ns/camunda-credentials`
	invalidImageMessage  = `Failed to apply default image tag "reg/camunda/zeebe:8.8:latest": couldn't parse image reference`
	unschedulableMessage = "0/6 nodes are available: 6 Insufficient cpu. preemption: 0/6 nodes are available."
)

// waitingStatus builds a single container status stuck in Waiting.
func waitingStatus(container, reason, message string) corev1.ContainerStatus {
	return corev1.ContainerStatus{
		Name:  container,
		Image: "reg/" + container + ":1",
		State: corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{Reason: reason, Message: message},
		},
	}
}

// withContainer appends another app container status to a pod.
func withContainer(pod corev1.Pod, status corev1.ContainerStatus) corev1.Pod {
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, status)
	return pod
}

// crashingPod builds a pod whose named container is in CrashLoopBackOff after
// restarts restarts.
func crashingPod(name, container string, restarts int32, message string, init bool) corev1.Pod {
	pod := waitingPod(name, container, "reg/"+container+":1", "CrashLoopBackOff", message, init)
	statuses := pod.Status.ContainerStatuses
	if init {
		statuses = pod.Status.InitContainerStatuses
	}
	statuses[0].RestartCount = restarts
	return pod
}

// silentCrashingPod is a crash loop whose kubelet message is empty, leaving the
// last exit code as the only evidence of what happened.
func silentCrashingPod(name, container string, restarts, exitCode int32, reason string) corev1.Pod {
	pod := crashingPod(name, container, restarts, "", false)
	pod.Status.ContainerStatuses[0].LastTerminationState = corev1.ContainerState{
		Terminated: &corev1.ContainerStateTerminated{ExitCode: exitCode, Reason: reason},
	}
	return pod
}

// scheduledPod builds a pod carrying a PodScheduled condition.
func scheduledPod(name string, status corev1.ConditionStatus, reason, message string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodScheduled, Status: status, Reason: reason, Message: message},
			},
		},
	}
}

func healthyPod(name string) corev1.Pod {
	return corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func TestTerminalCrashLoop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		pod          corev1.Pod
		wantHit      bool
		wantCtr      string
		wantRestarts int32
		wantMsg      string
	}{
		{
			name:         "restart count at the threshold is terminal",
			pod:          crashingPod("camunda-zeebe-0", "zeebe", 3, backoffMessage, false),
			wantHit:      true,
			wantCtr:      "zeebe",
			wantRestarts: 3,
			wantMsg:      backoffMessage,
		},
		{
			name:         "restart count above the threshold is terminal",
			pod:          crashingPod("camunda-zeebe-0", "zeebe", 11, backoffMessage, false),
			wantHit:      true,
			wantCtr:      "zeebe",
			wantRestarts: 11,
			wantMsg:      backoffMessage,
		},
		{
			name:    "a container restarting below the threshold is not terminal",
			pod:     crashingPod("camunda-zeebe-0", "zeebe", 2, backoffMessage, false),
			wantHit: false,
		},
		{
			name:    "a first restart is not terminal",
			pod:     crashingPod("camunda-zeebe-0", "zeebe", 1, backoffMessage, false),
			wantHit: false,
		},
		{
			name:         "init container crash loops are inspected too",
			pod:          crashingPod("camunda-operate-0", "wait-for-elasticsearch", 4, backoffMessage, true),
			wantHit:      true,
			wantCtr:      "wait-for-elasticsearch",
			wantRestarts: 4,
			wantMsg:      backoffMessage,
		},
		{
			name:         "an empty kubelet message falls back to the last exit code",
			pod:          silentCrashingPod("camunda-zeebe-0", "zeebe", 5, 137, "OOMKilled"),
			wantHit:      true,
			wantCtr:      "zeebe",
			wantRestarts: 5,
			wantMsg:      "137",
		},
		{
			name:    "an unpullable image is not a crash loop",
			pod:     waitingPod("p", "c", "reg/i:1", "ImagePullBackOff", childManifest404, false),
			wantHit: false,
		},
		{
			name:    "a running pod is not a crash loop",
			pod:     healthyPod("p"),
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := terminalCrashLoop(&tt.pod)
			if ok != tt.wantHit {
				t.Fatalf("terminalCrashLoop() ok = %v, want %v", ok, tt.wantHit)
			}
			if !tt.wantHit {
				return
			}
			if got.Pod != tt.pod.Name {
				t.Errorf("pod = %q, want %q", got.Pod, tt.pod.Name)
			}
			if got.Container != tt.wantCtr {
				t.Errorf("container = %q, want %q", got.Container, tt.wantCtr)
			}
			if got.Reason != "CrashLoopBackOff" {
				t.Errorf("reason = %q, want %q", got.Reason, "CrashLoopBackOff")
			}
			if got.RestartCount != tt.wantRestarts {
				t.Errorf("restart count = %d, want %d", got.RestartCount, tt.wantRestarts)
			}
			if !strings.Contains(got.Message, tt.wantMsg) {
				t.Errorf("message = %q, want it to contain %q", got.Message, tt.wantMsg)
			}
		})
	}
}

// TestCrashLoopRestartThreshold pins the default and proves a test can drive it
// without waiting for real restarts.
func TestCrashLoopRestartThreshold(t *testing.T) {
	if crashLoopRestartThreshold != 3 {
		t.Fatalf("default crash loop threshold = %d, want 3", crashLoopRestartThreshold)
	}

	orig := crashLoopRestartThreshold
	defer func() { crashLoopRestartThreshold = orig }()

	pod := crashingPod("camunda-zeebe-0", "zeebe", 1, backoffMessage, false)

	crashLoopRestartThreshold = 1
	if _, ok := terminalCrashLoop(&pod); !ok {
		t.Errorf("threshold 1 should make a single restart terminal")
	}

	crashLoopRestartThreshold = 10
	if _, ok := terminalCrashLoop(&pod); ok {
		t.Errorf("threshold 10 should leave a single restart alone")
	}
}

func TestTerminalConfigError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pod        corev1.Pod
		wantHit    bool
		wantCtr    string
		wantReason string
	}{
		{
			name:       "a missing secret key is terminal",
			pod:        waitingPod("camunda-connectors-0", "connectors", "reg/connectors:1", "CreateContainerConfigError", missingKeyMessage, false),
			wantHit:    true,
			wantCtr:    "connectors",
			wantReason: "CreateContainerConfigError",
		},
		{
			name:       "an unparseable image reference is terminal",
			pod:        waitingPod("camunda-zeebe-0", "zeebe", "reg/zeebe:8.8:latest", "InvalidImageName", invalidImageMessage, false),
			wantHit:    true,
			wantCtr:    "zeebe",
			wantReason: "InvalidImageName",
		},
		{
			name:       "init container config errors are inspected too",
			pod:        waitingPod("camunda-operate-0", "wait-for-elasticsearch", "reg/os-shell:12", "CreateContainerConfigError", missingKeyMessage, true),
			wantHit:    true,
			wantCtr:    "wait-for-elasticsearch",
			wantReason: "CreateContainerConfigError",
		},
		{
			name:    "a retryable create error is not terminal",
			pod:     waitingPod("p", "c", "reg/i:1", "CreateContainerError", "context deadline exceeded", false),
			wantHit: false,
		},
		{
			name:    "a container still being created is not terminal",
			pod:     waitingPod("p", "c", "reg/i:1", "ContainerCreating", "", false),
			wantHit: false,
		},
		{
			name:    "a pod waiting on its init container is not terminal",
			pod:     waitingPod("p", "c", "reg/i:1", "PodInitializing", "", false),
			wantHit: false,
		},
		{
			name:    "a running pod is not terminal",
			pod:     healthyPod("p"),
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := terminalConfigError(&tt.pod)
			if ok != tt.wantHit {
				t.Fatalf("terminalConfigError() ok = %v, want %v", ok, tt.wantHit)
			}
			if !tt.wantHit {
				return
			}
			if got.Pod != tt.pod.Name {
				t.Errorf("pod = %q, want %q", got.Pod, tt.pod.Name)
			}
			if got.Container != tt.wantCtr {
				t.Errorf("container = %q, want %q", got.Container, tt.wantCtr)
			}
			if got.Reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", got.Reason, tt.wantReason)
			}
			if got.Message == "" {
				t.Errorf("message must carry the kubelet detail, got empty")
			}
		})
	}
}

func TestTerminalUnschedulable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pod     corev1.Pod
		wantHit bool
	}{
		{
			name:    "an unschedulable pod is terminal",
			pod:     scheduledPod("camunda-zeebe-0", corev1.ConditionFalse, corev1.PodReasonUnschedulable, unschedulableMessage),
			wantHit: true,
		},
		{
			name:    "a scheduler error is not terminal",
			pod:     scheduledPod("camunda-zeebe-0", corev1.ConditionFalse, "SchedulerError", "error getting node"),
			wantHit: false,
		},
		{
			name:    "a scheduled pod is not terminal",
			pod:     scheduledPod("camunda-zeebe-0", corev1.ConditionTrue, "", ""),
			wantHit: false,
		},
		{
			name:    "a briefly pending pod with no conditions is not terminal",
			pod:     corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "camunda-zeebe-0"}, Status: corev1.PodStatus{Phase: corev1.PodPending}},
			wantHit: false,
		},
		{
			name: "an unready but scheduled pod is not terminal",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "camunda-zeebe-0"},
				Status: corev1.PodStatus{Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionFalse, Reason: "ContainersNotReady"},
				}},
			},
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := terminalUnschedulable(&tt.pod)
			if ok != tt.wantHit {
				t.Fatalf("terminalUnschedulable() ok = %v, want %v", ok, tt.wantHit)
			}
			if !tt.wantHit {
				return
			}
			if got.Pod != tt.pod.Name {
				t.Errorf("pod = %q, want %q", got.Pod, tt.pod.Name)
			}
			if got.Container != "" {
				t.Errorf("container must be empty for a pod-level failure, got %q", got.Container)
			}
			if got.Reason != corev1.PodReasonUnschedulable {
				t.Errorf("reason = %q, want %q", got.Reason, corev1.PodReasonUnschedulable)
			}
			if !strings.Contains(got.Message, "Insufficient cpu") {
				t.Errorf("message must carry the scheduler detail, got %q", got.Message)
			}
		})
	}
}

// TestPodFailureErrorNamesEverything asserts the operator learns what failed
// without running a second command.
func TestPodFailureErrorNamesEverything(t *testing.T) {
	t.Parallel()

	t.Run("a crash loop names the container and the restarts", func(t *testing.T) {
		t.Parallel()
		f := &PodFailure{
			Pod: "camunda-zeebe-0", Container: "zeebe",
			Reason: "CrashLoopBackOff", Message: backoffMessage, RestartCount: 11,
		}
		msg := f.Error()
		for _, want := range []string{f.Pod, f.Container, f.Reason, "11", "back-off"} {
			if !strings.Contains(msg, want) {
				t.Errorf("error message missing %q:\n%s", want, msg)
			}
		}
	})

	t.Run("an unschedulable pod names the pod and the scheduler message", func(t *testing.T) {
		t.Parallel()
		f := &PodFailure{
			Pod: "camunda-zeebe-0", Reason: corev1.PodReasonUnschedulable, Message: unschedulableMessage,
		}
		msg := f.Error()
		for _, want := range []string{f.Pod, f.Reason, "Insufficient cpu"} {
			if !strings.Contains(msg, want) {
				t.Errorf("error message missing %q:\n%s", want, msg)
			}
		}
		if strings.Contains(msg, `container ""`) {
			t.Errorf("an empty container must not be rendered:\n%s", msg)
		}
	})

	t.Run("a config error names the container and the missing key", func(t *testing.T) {
		t.Parallel()
		f := &PodFailure{
			Pod: "camunda-connectors-0", Container: "connectors",
			Reason: "CreateContainerConfigError", Message: missingKeyMessage,
		}
		msg := f.Error()
		for _, want := range []string{f.Pod, f.Container, f.Reason, "smtp-password"} {
			if !strings.Contains(msg, want) {
				t.Errorf("error message missing %q:\n%s", want, msg)
			}
		}
	})
}

// TestAbortReason asserts each failure kind replaces the generic Helm reason
// with one that names the actual problem.
func TestAbortReason(t *testing.T) {
	t.Parallel()

	const imageReason = "helm upgrade --install aborted early: unresolvable container image"
	if got := (&ImagePullFailure{}).abortReason(); got != imageReason {
		t.Errorf("image pull abort reason = %q, want %q", got, imageReason)
	}

	tests := []struct {
		reason   string
		wantPart string
	}{
		{reason: "CrashLoopBackOff", wantPart: "crash loop"},
		{reason: "CreateContainerConfigError", wantPart: "container configuration"},
		{reason: "InvalidImageName", wantPart: "container configuration"},
		{reason: corev1.PodReasonUnschedulable, wantPart: "cannot be scheduled"},
	}

	seen := map[string]bool{imageReason: true}
	for _, tt := range tests {
		got := (&PodFailure{Reason: tt.reason}).abortReason()
		if !strings.HasPrefix(got, "helm upgrade --install aborted early: ") {
			t.Errorf("%s abort reason = %q, want the helm abort prefix", tt.reason, got)
		}
		if !strings.Contains(got, tt.wantPart) {
			t.Errorf("%s abort reason = %q, want it to contain %q", tt.reason, got, tt.wantPart)
		}
		if got == imageReason {
			t.Errorf("%s must not reuse the image pull reason", tt.reason)
		}
		seen[got] = true
	}
	// Image pull, crash loop, config error and unschedulable are four kinds but
	// the two config reasons deliberately share a reason, so three new + one.
	if len(seen) != 4 {
		t.Errorf("expected 4 distinct abort reasons, got %d: %v", len(seen), seen)
	}
}

// TestStreakKey asserts repeat observations of the same problem are countable.
func TestStreakKey(t *testing.T) {
	t.Parallel()

	pull := &ImagePullFailure{Pod: "p", Container: "c", Image: "reg/i:1"}
	if got, want := pull.streakKey(), "p/c/reg/i:1"; got != want {
		t.Errorf("image pull streak key = %q, want %q", got, want)
	}

	early := &PodFailure{Pod: "p", Container: "c", Reason: "CrashLoopBackOff", RestartCount: 3}
	later := &PodFailure{Pod: "p", Container: "c", Reason: "CrashLoopBackOff", RestartCount: 9}
	if early.streakKey() != later.streakKey() {
		t.Errorf("a climbing restart count must not break the streak: %q vs %q",
			early.streakKey(), later.streakKey())
	}

	config := &PodFailure{Pod: "p", Container: "c", Reason: "CreateContainerConfigError"}
	if early.streakKey() == config.streakKey() {
		t.Errorf("different failure kinds on one container must not share a key: %q", config.streakKey())
	}

	otherPod := &PodFailure{Pod: "q", Container: "c", Reason: "CrashLoopBackOff"}
	if early.streakKey() == otherPod.streakKey() {
		t.Errorf("different pods must not share a key: %q", otherPod.streakKey())
	}
}

// TestTerminalPodStateFailureOrder pins the detector order so a pod with more
// than one problem always reports the same one.
func TestTerminalPodStateFailureOrder(t *testing.T) {
	t.Parallel()

	t.Run("an unpullable image outranks a crash loop", func(t *testing.T) {
		t.Parallel()
		pod := withContainer(
			crashingPod("camunda-zeebe-0", "zeebe", 9, backoffMessage, false),
			waitingStatus("exporter", "ImagePullBackOff", childManifest404))
		got, ok := terminalPodStateFailure(&pod)
		if !ok {
			t.Fatal("expected a failure, got none")
		}
		var pull *ImagePullFailure
		if !errors.As(got, &pull) {
			t.Fatalf("expected an *ImagePullFailure, got %T (%v)", got, got)
		}
	})

	t.Run("a config error outranks a crash loop", func(t *testing.T) {
		t.Parallel()
		pod := withContainer(
			crashingPod("camunda-zeebe-0", "zeebe", 9, backoffMessage, false),
			waitingStatus("exporter", "CreateContainerConfigError", missingKeyMessage))
		got, ok := terminalPodStateFailure(&pod)
		if !ok {
			t.Fatal("expected a failure, got none")
		}
		var failure *PodFailure
		if !errors.As(got, &failure) {
			t.Fatalf("expected a *PodFailure, got %T (%v)", got, got)
		}
		if failure.Reason != "CreateContainerConfigError" {
			t.Errorf("reason = %q, want CreateContainerConfigError", failure.Reason)
		}
	})

	t.Run("a crash loop outranks an unschedulable pod", func(t *testing.T) {
		t.Parallel()
		crashing := waitingStatus("zeebe", "CrashLoopBackOff", backoffMessage)
		crashing.RestartCount = 9
		pod := withContainer(
			scheduledPod("camunda-zeebe-0", corev1.ConditionFalse, corev1.PodReasonUnschedulable, unschedulableMessage),
			crashing)
		got, ok := terminalPodStateFailure(&pod)
		if !ok {
			t.Fatal("expected a failure, got none")
		}
		var failure *PodFailure
		if !errors.As(got, &failure) {
			t.Fatalf("expected a *PodFailure, got %T (%v)", got, got)
		}
		if failure.Reason != "CrashLoopBackOff" {
			t.Errorf("reason = %q, want CrashLoopBackOff", failure.Reason)
		}
	})

	t.Run("a healthy pod reports nothing", func(t *testing.T) {
		t.Parallel()
		pod := healthyPod("camunda-zeebe-0")
		if got, ok := terminalPodStateFailure(&pod); ok {
			t.Fatalf("expected no failure, got %v", got)
		}
	})
}

// TestFirstTerminalFailureIsDeterministic asserts the streak key stays stable
// when several pods fail in different ways.
func TestFirstTerminalFailureIsDeterministic(t *testing.T) {
	t.Parallel()

	unschedulable := scheduledPod("camunda-zeebe-2", corev1.ConditionFalse, corev1.PodReasonUnschedulable, unschedulableMessage)
	crashing := crashingPod("camunda-connectors-0", "connectors", 9, backoffMessage, false)
	configError := waitingPod("camunda-operate-0", "operate", "reg/operate:1", "CreateContainerConfigError", missingKeyMessage, false)

	orders := map[string]*corev1.PodList{
		"as listed":  podList(unschedulable, crashing, configError),
		"reversed":   podList(configError, crashing, unschedulable),
		"interposed": podList(crashing, unschedulable, configError),
	}
	for name, pods := range orders {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := firstTerminalFailure(pods)
			if got == nil {
				t.Fatal("expected a failure, got nil")
			}
			var failure *PodFailure
			if !errors.As(got, &failure) {
				t.Fatalf("expected a *PodFailure, got %T (%v)", got, got)
			}
			if failure.Pod != "camunda-connectors-0" {
				t.Errorf("pod = %q, want the alphabetically first affected pod camunda-connectors-0", failure.Pod)
			}
		})
	}

	t.Run("a healthy namespace yields a nil interface", func(t *testing.T) {
		t.Parallel()
		if got := firstTerminalFailure(podList(healthyPod("a"), healthyPod("b"))); got != nil {
			t.Fatalf("expected nil, got %v (%T)", got, got)
		}
	})
}

// TestWatchTerminalPodFailure drives the existing poll loop over the new failure
// kinds and asserts the consecutive-observation streak is honoured.
func TestWatchTerminalPodFailure(t *testing.T) {
	t.Parallel()

	crashing := crashingPod("camunda-zeebe-0", "zeebe", 9, backoffMessage, false)
	recovered := healthyPod("camunda-zeebe-0")

	t.Run("two consecutive observations abort", func(t *testing.T) {
		t.Parallel()
		calls := 0
		deps := imagePullWatchDeps{
			list: func(context.Context, string) (*corev1.PodList, error) {
				calls++
				return podList(crashing), nil
			},
			sleep: noSleep(10), threshold: 2,
		}
		got := watchTerminalImagePull(context.Background(), deps, "ns")
		if got == nil {
			t.Fatal("expected a failure, got nil")
		}
		var failure *PodFailure
		if !errors.As(got, &failure) {
			t.Fatalf("expected a *PodFailure, got %T (%v)", got, got)
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
					return podList(crashing), nil
				}
				return podList(recovered), nil
			},
			sleep: noSleep(4), threshold: 2,
		}
		if got := watchTerminalImagePull(context.Background(), deps, "ns"); got != nil {
			t.Fatalf("expected no abort after the state recovered, got %v", got)
		}
	})

	t.Run("a climbing restart count keeps the streak", func(t *testing.T) {
		t.Parallel()
		calls := 0
		deps := imagePullWatchDeps{
			list: func(context.Context, string) (*corev1.PodList, error) {
				calls++
				return podList(crashingPod("camunda-zeebe-0", "zeebe", int32(8+calls), backoffMessage, false)), nil
			},
			sleep: noSleep(10), threshold: 2,
		}
		if got := watchTerminalImagePull(context.Background(), deps, "ns"); got == nil {
			t.Fatal("a restart count that climbs between polls must not reset the streak")
		}
		if calls != 2 {
			t.Errorf("expected to abort on the 2nd confirmation, got %d polls", calls)
		}
	})

	t.Run("an unschedulable pod that gets scheduled does not abort", func(t *testing.T) {
		t.Parallel()
		calls := 0
		deps := imagePullWatchDeps{
			list: func(context.Context, string) (*corev1.PodList, error) {
				calls++
				if calls == 1 {
					return podList(scheduledPod("camunda-zeebe-0", corev1.ConditionFalse,
						corev1.PodReasonUnschedulable, unschedulableMessage)), nil
				}
				return podList(scheduledPod("camunda-zeebe-0", corev1.ConditionTrue, "", "")), nil
			},
			sleep: noSleep(4), threshold: 2,
		}
		if got := watchTerminalImagePull(context.Background(), deps, "ns"); got != nil {
			t.Fatalf("a pod scheduled on the next poll must not abort, got %v", got)
		}
	})
}

// TestUpgradeInstall_AbortsOnUnschedulablePod asserts the wait ends early and
// the error names the scheduling problem rather than the killed process.
func TestUpgradeInstall_AbortsOnUnschedulablePod(t *testing.T) {
	stuck := scheduledPod("camunda-zeebe-0", corev1.ConditionFalse,
		corev1.PodReasonUnschedulable, unschedulableMessage)

	origLister := newPodLister
	newPodLister = func(string, string) (podLister, error) {
		return fakeLister{pods: podList(stuck)}, nil
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
	if !strings.Contains(helmErr.Reason, "cannot be scheduled") {
		t.Errorf("reason should name the real cause, got %q", helmErr.Reason)
	}

	var podErr *PodFailure
	if !errors.As(err, &podErr) {
		t.Fatalf("expected the cause to be a *PodFailure, got %v", helmErr.Cause)
	}
	if !strings.Contains(err.Error(), "camunda-zeebe-0") {
		t.Errorf("error should name the pod, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "Insufficient cpu") {
		t.Errorf("error should name the scheduler detail, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "signal: killed") {
		t.Errorf("the killed-process error must not leak to the user, got %q", err.Error())
	}
}
