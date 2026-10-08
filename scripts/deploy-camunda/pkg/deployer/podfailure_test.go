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
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"scripts/deploy-camunda/pkg/types"
)

// Verbatim kubelet and scheduler messages.
const (
	backoffMessage       = "back-off 5m0s restarting failed container=migration pod=camunda-optimize-0_ns(9f2)"
	missingKeyMessage    = `couldn't find key smtp-password in Secret ns/camunda-credentials`
	invalidImageMessage  = `Failed to apply default image tag "reg/camunda/zeebe:8.8:latest": couldn't parse image reference`
	unschedulableMessage = "0/6 nodes are available: 6 Insufficient cpu. preemption: 0/6 nodes are available."
	// Verbatim cluster-autoscaler messages.
	scaleUpMessage   = "Pod triggered scale-up: [{https://www.googleapis.com/compute/v1/projects/p/zones/z/instanceGroups/grp 26->27 (max: 150)}]"
	noScaleUpMessage = "pod didn't trigger scale-up: 3 node(s) didn't match Pod's node affinity/selector"
)

var autoscalerEpoch = time.Date(2026, 9, 30, 13, 28, 0, 0, time.UTC)

// autoscalerEvent is a cluster-autoscaler event recorded against a pod,
// offset seconds after autoscalerEpoch.
func autoscalerEvent(pod, reason, message string, offset int) corev1.Event {
	return corev1.Event{
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: pod},
		Reason:         reason,
		Message:        message,
		Source:         corev1.EventSource{Component: "cluster-autoscaler"},
		LastTimestamp:  metav1.NewTime(autoscalerEpoch.Add(time.Duration(offset) * time.Second)),
	}
}

func eventList(events ...corev1.Event) *corev1.EventList {
	return &corev1.EventList{Items: events}
}

func noScaleUp(pod string) map[string]autoscalerVerdict {
	return autoscalerVerdicts(eventList(autoscalerEvent(pod, reasonNotTriggerScaleUp, noScaleUpMessage, 0)))
}

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

	unschedulable := scheduledPod("camunda-zeebe-0", corev1.ConditionFalse, corev1.PodReasonUnschedulable, unschedulableMessage)
	withUID := unschedulable
	withUID.UID = "new-uid"

	tests := []struct {
		name    string
		pod     corev1.Pod
		events  *corev1.EventList
		wantHit bool
	}{
		{
			name:    "the autoscaler declining a scale-up is terminal",
			pod:     unschedulable,
			events:  eventList(autoscalerEvent("camunda-zeebe-0", reasonNotTriggerScaleUp, noScaleUpMessage, 0)),
			wantHit: true,
		},
		{
			name:    "a pod that triggered a scale-up is waiting for a node, not terminal",
			pod:     unschedulable,
			events:  eventList(autoscalerEvent("camunda-zeebe-0", reasonTriggeredScaleUp, scaleUpMessage, 0)),
			wantHit: false,
		},
		{
			name:    "without autoscaler events an unschedulable pod is never terminal",
			pod:     unschedulable,
			events:  eventList(),
			wantHit: false,
		},
		{
			name: "a later scale-up supersedes an earlier refusal",
			pod:  unschedulable,
			events: eventList(
				autoscalerEvent("camunda-zeebe-0", reasonNotTriggerScaleUp, noScaleUpMessage, 0),
				autoscalerEvent("camunda-zeebe-0", reasonTriggeredScaleUp, scaleUpMessage, 10)),
			wantHit: false,
		},
		{
			name: "a later refusal supersedes an earlier scale-up",
			pod:  unschedulable,
			events: eventList(
				autoscalerEvent("camunda-zeebe-0", reasonNotTriggerScaleUp, noScaleUpMessage, 10),
				autoscalerEvent("camunda-zeebe-0", reasonTriggeredScaleUp, scaleUpMessage, 0)),
			wantHit: true,
		},
		{
			name: "a refusal recorded for a previous pod with the same name is ignored",
			pod:  withUID,
			events: func() *corev1.EventList {
				e := autoscalerEvent("camunda-zeebe-0", reasonNotTriggerScaleUp, noScaleUpMessage, 0)
				e.InvolvedObject.UID = "old-uid"
				return eventList(e)
			}(),
			wantHit: false,
		},
		{
			name:    "a refusal for another pod does not apply",
			pod:     unschedulable,
			events:  eventList(autoscalerEvent("camunda-zeebe-1", reasonNotTriggerScaleUp, noScaleUpMessage, 0)),
			wantHit: false,
		},
		{
			name:    "a scheduler error is not terminal",
			pod:     scheduledPod("camunda-zeebe-0", corev1.ConditionFalse, "SchedulerError", "error getting node"),
			events:  eventList(autoscalerEvent("camunda-zeebe-0", reasonNotTriggerScaleUp, noScaleUpMessage, 0)),
			wantHit: false,
		},
		{
			name:    "a scheduled pod is not terminal",
			pod:     scheduledPod("camunda-zeebe-0", corev1.ConditionTrue, "", ""),
			events:  eventList(autoscalerEvent("camunda-zeebe-0", reasonNotTriggerScaleUp, noScaleUpMessage, 0)),
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := terminalUnschedulable(&tt.pod, autoscalerVerdicts(tt.events))
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
			for _, want := range []string{"Insufficient cpu", "didn't trigger scale-up"} {
				if !strings.Contains(got.Message, want) {
					t.Errorf("message must carry %q, got %q", want, got.Message)
				}
			}
		})
	}
}

// TestPodFailureErrorNamesEverything asserts the operator learns what failed
// without running a second command.
func TestPodFailureErrorNamesEverything(t *testing.T) {
	t.Parallel()

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
	if len(seen) != 3 {
		t.Errorf("expected 3 distinct abort reasons, got %d: %v", len(seen), seen)
	}
}

// TestStreakKey asserts repeat observations of the same problem are countable.
func TestStreakKey(t *testing.T) {
	t.Parallel()

	pull := &ImagePullFailure{Pod: "p", Container: "c", Image: "reg/i:1"}
	if got, want := pull.streakKey(), "p/c/reg/i:1"; got != want {
		t.Errorf("image pull streak key = %q, want %q", got, want)
	}

	early := &PodFailure{Pod: "p", Reason: corev1.PodReasonUnschedulable, Message: "a", evidenceAt: autoscalerEpoch}
	later := &PodFailure{Pod: "p", Reason: corev1.PodReasonUnschedulable, Message: "b", evidenceAt: autoscalerEpoch.Add(time.Minute)}
	if early.streakKey() != later.streakKey() {
		t.Errorf("advancing evidence must not break the streak: %q vs %q",
			early.streakKey(), later.streakKey())
	}

	config := &PodFailure{Pod: "p", Container: "c", Reason: "CreateContainerConfigError"}
	invalid := &PodFailure{Pod: "p", Container: "c", Reason: "InvalidImageName"}
	if config.streakKey() == invalid.streakKey() {
		t.Errorf("different failure kinds on one container must not share a key: %q", config.streakKey())
	}

	otherPod := &PodFailure{Pod: "q", Reason: corev1.PodReasonUnschedulable}
	if early.streakKey() == otherPod.streakKey() {
		t.Errorf("different pods must not share a key: %q", otherPod.streakKey())
	}
}

// TestTerminalPodStateFailureOrder pins the detector order so a pod with more
// than one problem always reports the same one.
func TestTerminalPodStateFailureOrder(t *testing.T) {
	t.Parallel()

	t.Run("an unpullable image outranks a config error", func(t *testing.T) {
		t.Parallel()
		pod := withContainer(
			waitingPod("camunda-zeebe-0", "zeebe", "reg/zeebe:1", "CreateContainerConfigError", missingKeyMessage, false),
			waitingStatus("exporter", "ImagePullBackOff", childManifest404))
		got, ok := terminalPodStateFailure(&pod, nil)
		if !ok {
			t.Fatal("expected a failure, got none")
		}
		var pull *ImagePullFailure
		if !errors.As(got, &pull) {
			t.Fatalf("expected an *ImagePullFailure, got %T (%v)", got, got)
		}
	})

	t.Run("a config error outranks an unschedulable pod", func(t *testing.T) {
		t.Parallel()
		pod := withContainer(
			scheduledPod("camunda-zeebe-0", corev1.ConditionFalse, corev1.PodReasonUnschedulable, unschedulableMessage),
			waitingStatus("exporter", "CreateContainerConfigError", missingKeyMessage))
		got, ok := terminalPodStateFailure(&pod, noScaleUp("camunda-zeebe-0"))
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

	t.Run("a crash-looping init container is not terminal", func(t *testing.T) {
		t.Parallel()
		pod := waitingPod("camunda-optimize-0", "migration", "reg/optimize:8.6", "CrashLoopBackOff", backoffMessage, true)
		pod.Status.InitContainerStatuses[0].RestartCount = 9
		if got, ok := terminalPodStateFailure(&pod, nil); ok {
			t.Fatalf("expected no failure, got %v", got)
		}
	})

	t.Run("a healthy pod reports nothing", func(t *testing.T) {
		t.Parallel()
		pod := healthyPod("camunda-zeebe-0")
		if got, ok := terminalPodStateFailure(&pod, nil); ok {
			t.Fatalf("expected no failure, got %v", got)
		}
	})
}

// TestFirstTerminalFailureIsDeterministic asserts the streak key stays stable
// when several pods fail in different ways.
func TestFirstTerminalFailureIsDeterministic(t *testing.T) {
	t.Parallel()

	unschedulable := scheduledPod("camunda-zeebe-2", corev1.ConditionFalse, corev1.PodReasonUnschedulable, unschedulableMessage)
	configError := waitingPod("camunda-connectors-0", "connectors", "reg/connectors:1", "CreateContainerConfigError", missingKeyMessage, false)
	invalidImage := waitingPod("camunda-operate-0", "operate", "reg/operate:1", "InvalidImageName", invalidImageMessage, false)

	orders := map[string]*corev1.PodList{
		"as listed":  podList(unschedulable, configError, invalidImage),
		"reversed":   podList(invalidImage, configError, unschedulable),
		"interposed": podList(configError, unschedulable, invalidImage),
	}
	for name, pods := range orders {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := firstTerminalFailure(pods, noScaleUp("camunda-zeebe-2"))
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
		if got := firstTerminalFailure(podList(healthyPod("a"), healthyPod("b")), nil); got != nil {
			t.Fatalf("expected nil, got %v (%T)", got, got)
		}
	})
}

// TestWatchTerminalPodFailure drives the existing poll loop over the new failure
// kinds and asserts the consecutive-observation streak is honoured.
func TestWatchTerminalPodFailure(t *testing.T) {
	t.Parallel()

	broken := waitingPod("camunda-zeebe-0", "zeebe", "reg/zeebe:1", "CreateContainerConfigError", missingKeyMessage, false)
	recovered := healthyPod("camunda-zeebe-0")

	t.Run("two consecutive observations abort", func(t *testing.T) {
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
					return podList(broken), nil
				}
				return podList(recovered), nil
			},
			sleep: noSleep(4), threshold: 2,
		}
		if got := watchTerminalImagePull(context.Background(), deps, "ns"); got != nil {
			t.Fatalf("expected no abort after the state recovered, got %v", got)
		}
	})

	t.Run("a crash-looping init container never aborts", func(t *testing.T) {
		t.Parallel()
		pod := waitingPod("camunda-optimize-0", "migration", "reg/optimize:8.6", "CrashLoopBackOff", backoffMessage, true)
		pod.Status.InitContainerStatuses[0].RestartCount = 9
		deps := imagePullWatchDeps{
			list: func(context.Context, string) (*corev1.PodList, error) {
				return podList(pod), nil
			},
			sleep: noSleep(10), threshold: 2,
		}
		if got := watchTerminalImagePull(context.Background(), deps, "ns"); got != nil {
			t.Fatalf("a crash loop must not abort, got %v", got)
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
			events: func(context.Context, string) (*corev1.EventList, error) {
				return eventList(autoscalerEvent("camunda-zeebe-0", reasonNotTriggerScaleUp, noScaleUpMessage, 0)), nil
			},
			sleep: noSleep(4), threshold: 2,
		}
		if got := watchTerminalImagePull(context.Background(), deps, "ns"); got != nil {
			t.Fatalf("a pod scheduled on the next poll must not abort, got %v", got)
		}
	})

	// Regression: 8.10 mns2 on GKE aborted while the autoscaler was adding the
	// node that scheduled the pod seconds later.
	t.Run("an unschedulable pod awaiting a scale-up never aborts", func(t *testing.T) {
		t.Parallel()
		deps := imagePullWatchDeps{
			list: func(context.Context, string) (*corev1.PodList, error) {
				return podList(scheduledPod("integration-connectors-0", corev1.ConditionFalse,
					corev1.PodReasonUnschedulable, unschedulableMessage)), nil
			},
			events: func(context.Context, string) (*corev1.EventList, error) {
				return eventList(autoscalerEvent("integration-connectors-0", reasonTriggeredScaleUp, scaleUpMessage, 0)), nil
			},
			sleep: noSleep(10), threshold: 2,
		}
		if got := watchTerminalImagePull(context.Background(), deps, "ns"); got != nil {
			t.Fatalf("a pending scale-up must not abort, got %v", got)
		}
	})

	t.Run("repeated autoscaler refusals abort", func(t *testing.T) {
		t.Parallel()
		polls := 0
		deps := imagePullWatchDeps{
			list: func(context.Context, string) (*corev1.PodList, error) {
				polls++
				return podList(scheduledPod("integration-connectors-0", corev1.ConditionFalse,
					corev1.PodReasonUnschedulable, unschedulableMessage)), nil
			},
			events: func(context.Context, string) (*corev1.EventList, error) {
				return eventList(autoscalerEvent("integration-connectors-0", reasonNotTriggerScaleUp, noScaleUpMessage, 10*polls)), nil
			},
			sleep: noSleep(10), threshold: 2,
		}
		if got := watchTerminalImagePull(context.Background(), deps, "ns"); got == nil {
			t.Fatal("an autoscaler refusal must abort")
		}
		if polls != 2 {
			t.Errorf("expected to abort on the 2nd confirmation, got %d polls", polls)
		}
	})

	t.Run("a single stale autoscaler refusal never aborts", func(t *testing.T) {
		t.Parallel()
		deps := imagePullWatchDeps{
			list: func(context.Context, string) (*corev1.PodList, error) {
				return podList(scheduledPod("integration-connectors-0", corev1.ConditionFalse,
					corev1.PodReasonUnschedulable, unschedulableMessage)), nil
			},
			events: func(context.Context, string) (*corev1.EventList, error) {
				return eventList(autoscalerEvent("integration-connectors-0", reasonNotTriggerScaleUp, noScaleUpMessage, 0)), nil
			},
			sleep: noSleep(20), threshold: 2,
		}
		if got := watchTerminalImagePull(context.Background(), deps, "ns"); got != nil {
			t.Fatalf("one refusal seen on every poll must not abort, got %v", got)
		}
	})

	t.Run("a stale refusal between fresh ones keeps the streak", func(t *testing.T) {
		t.Parallel()
		offsets := []int{0, 0, 0, 30}
		polls := 0
		deps := imagePullWatchDeps{
			list: func(context.Context, string) (*corev1.PodList, error) {
				return podList(scheduledPod("integration-connectors-0", corev1.ConditionFalse,
					corev1.PodReasonUnschedulable, unschedulableMessage)), nil
			},
			events: func(context.Context, string) (*corev1.EventList, error) {
				offset := offsets[min(polls, len(offsets)-1)]
				polls++
				return eventList(autoscalerEvent("integration-connectors-0", reasonNotTriggerScaleUp, noScaleUpMessage, offset)), nil
			},
			sleep: noSleep(10), threshold: 2,
		}
		if got := watchTerminalImagePull(context.Background(), deps, "ns"); got == nil {
			t.Fatal("a refusal that advances after stale polls must abort")
		}
		if polls != 4 {
			t.Errorf("expected to abort on the 4th poll, got %d polls", polls)
		}
	})

	t.Run("an events list failure never makes a pod unschedulable-terminal", func(t *testing.T) {
		t.Parallel()
		deps := imagePullWatchDeps{
			list: func(context.Context, string) (*corev1.PodList, error) {
				return podList(scheduledPod("integration-connectors-0", corev1.ConditionFalse,
					corev1.PodReasonUnschedulable, unschedulableMessage)), nil
			},
			events: func(context.Context, string) (*corev1.EventList, error) {
				return nil, errors.New("events is forbidden")
			},
			sleep: noSleep(10), threshold: 2,
		}
		if got := watchTerminalImagePull(context.Background(), deps, "ns"); got != nil {
			t.Fatalf("without autoscaler evidence the guard must keep waiting, got %v", got)
		}
	})
}

// TestUpgradeInstall_AbortsOnUnschedulablePod asserts the wait ends early and
// the error names the scheduling problem rather than the killed process.
func TestUpgradeInstall_AbortsOnUnschedulablePod(t *testing.T) {
	stuck := scheduledPod("camunda-zeebe-0", corev1.ConditionFalse,
		corev1.PodReasonUnschedulable, unschedulableMessage)
	stuck.Labels = map[string]string{"app.kubernetes.io/instance": "integration"}

	origLister := newPodLister
	newPodLister = func(string, string) (podLister, error) {
		var scans atomic.Int32
		return fakeLister{
			pods: podList(stuck),
			eventsFn: func() *corev1.EventList {
				offset := int(scans.Add(1)) * 10
				return eventList(autoscalerEvent("camunda-zeebe-0", reasonNotTriggerScaleUp, noScaleUpMessage, offset))
			},
		}, nil
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
	if !strings.Contains(err.Error(), "didn't trigger scale-up") {
		t.Errorf("error should name the autoscaler refusal, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "signal: killed") {
		t.Errorf("the killed-process error must not leak to the user, got %q", err.Error())
	}
}
