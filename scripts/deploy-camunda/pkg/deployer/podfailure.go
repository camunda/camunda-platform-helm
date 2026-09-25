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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// crashLoopRestartThreshold is how many restarts a container must have before a
// CrashLoopBackOff counts as terminal. A var rather than a const so tests drive
// it without waiting for real restarts.
var crashLoopRestartThreshold int32 = 3

// terminalPodFailure is a pod state the guard treats as unrecoverable.
type terminalPodFailure interface {
	error
	// streakKey is a stable identity so repeat observations can be counted.
	streakKey() string
	// abortReason replaces the generic Helm failure reason.
	abortReason() string
}

// PodFailure is a terminal pod state other than an unresolvable image.
type PodFailure struct {
	Pod string
	// Container is empty for pod-level failures such as Unschedulable.
	Container string
	// Reason is the kubelet waiting reason or the pod condition reason.
	Reason string
	// Message is the kubelet or scheduler detail naming what is missing.
	Message string
	// RestartCount is set for CrashLoopBackOff only.
	RestartCount int32
}

func (f *PodFailure) Error() string {
	subject := fmt.Sprintf("pod %q", f.Pod)
	if f.Container != "" {
		subject = fmt.Sprintf("container %q in pod %q", f.Container, f.Pod)
	}
	if f.RestartCount > 0 {
		return fmt.Sprintf("%s has restarted %d times and will not become ready (%s): %s",
			subject, f.RestartCount, f.Reason, f.Message)
	}
	return fmt.Sprintf("%s will not become ready (%s): %s", subject, f.Reason, f.Message)
}

// streakKey omits RestartCount, which climbs between polls while the failure
// stays the same.
func (f *PodFailure) streakKey() string {
	return f.Pod + "/" + f.Container + "/" + f.Reason
}

func (f *PodFailure) abortReason() string {
	if reason, ok := podFailureAbortReasons[f.Reason]; ok {
		return reason
	}
	return "helm upgrade --install aborted early: terminal pod state"
}

var podFailureAbortReasons = map[string]string{
	"CrashLoopBackOff":            "helm upgrade --install aborted early: container crash loop",
	"CreateContainerConfigError":  "helm upgrade --install aborted early: unresolvable container configuration",
	"InvalidImageName":            "helm upgrade --install aborted early: unresolvable container configuration",
	corev1.PodReasonUnschedulable: "helm upgrade --install aborted early: pod cannot be scheduled",
}

// terminalConfigReasons are kubelet waiting reasons that cannot clear without a
// change to the release: a missing Secret or ConfigMap key, or an image
// reference the kubelet cannot parse.
var terminalConfigReasons = map[string]bool{
	"CreateContainerConfigError": true,
	"InvalidImageName":           true,
}

// podContainerStatuses returns init container statuses followed by app container
// statuses.
func podContainerStatuses(pod *corev1.Pod) []corev1.ContainerStatus {
	statuses := make([]corev1.ContainerStatus, 0,
		len(pod.Status.InitContainerStatuses)+len(pod.Status.ContainerStatuses))
	statuses = append(statuses, pod.Status.InitContainerStatuses...)
	return append(statuses, pod.Status.ContainerStatuses...)
}

// terminalCrashLoop reports a container that keeps dying. Restarts below
// crashLoopRestartThreshold are left alone: a container that crashes once or
// twice while a dependency comes up still reaches ready.
func terminalCrashLoop(pod *corev1.Pod) (*PodFailure, bool) {
	for _, cs := range podContainerStatuses(pod) {
		waiting := cs.State.Waiting
		if waiting == nil || waiting.Reason != "CrashLoopBackOff" {
			continue
		}
		if cs.RestartCount < crashLoopRestartThreshold {
			continue
		}
		return &PodFailure{
			Pod:          pod.Name,
			Container:    cs.Name,
			Reason:       waiting.Reason,
			Message:      crashLoopMessage(waiting.Message, cs.LastTerminationState.Terminated),
			RestartCount: cs.RestartCount,
		}, true
	}
	return nil, false
}

// crashLoopMessage prefers the kubelet backoff message and falls back to the
// last exit code, which is the only datum that says why the container died.
func crashLoopMessage(waiting string, last *corev1.ContainerStateTerminated) string {
	if message := strings.TrimSpace(waiting); message != "" {
		return message
	}
	if last == nil {
		return ""
	}
	return fmt.Sprintf("last termination: exit code %d (%s)", last.ExitCode, last.Reason)
}

// terminalConfigError reports a container the kubelet cannot start because the
// release references something that is not there.
func terminalConfigError(pod *corev1.Pod) (*PodFailure, bool) {
	for _, cs := range podContainerStatuses(pod) {
		waiting := cs.State.Waiting
		if waiting == nil || !terminalConfigReasons[waiting.Reason] {
			continue
		}
		return &PodFailure{
			Pod:       pod.Name,
			Container: cs.Name,
			Reason:    waiting.Reason,
			Message:   strings.TrimSpace(waiting.Message),
		}, true
	}
	return nil, false
}

// terminalUnschedulable reports a pod the scheduler has rejected. Other
// PodScheduled failures, such as SchedulerError, are transient and ignored.
func terminalUnschedulable(pod *corev1.Pod) (*PodFailure, bool) {
	for _, condition := range pod.Status.Conditions {
		if condition.Type != corev1.PodScheduled || condition.Status != corev1.ConditionFalse {
			continue
		}
		if condition.Reason != corev1.PodReasonUnschedulable {
			continue
		}
		return &PodFailure{
			Pod:     pod.Name,
			Reason:  condition.Reason,
			Message: strings.TrimSpace(condition.Message),
		}, true
	}
	return nil, false
}

// terminalPodStateFailure runs the detectors in a fixed order so a pod with more
// than one problem always reports the same one, keeping the streak key stable
// across polls.
func terminalPodStateFailure(pod *corev1.Pod) (terminalPodFailure, bool) {
	if failure, ok := terminalImagePullFailure(pod); ok {
		return failure, true
	}
	if failure, ok := terminalConfigError(pod); ok {
		return failure, true
	}
	if failure, ok := terminalCrashLoop(pod); ok {
		return failure, true
	}
	if failure, ok := terminalUnschedulable(pod); ok {
		return failure, true
	}
	return nil, false
}

// firstTerminalFailure returns the terminal failure of the alphabetically first
// affected pod, keeping the streak key stable across polls.
func firstTerminalFailure(pods *corev1.PodList) terminalPodFailure {
	var chosen terminalPodFailure
	var chosenPod string
	for i := range pods.Items {
		pod := &pods.Items[i]
		failure, ok := terminalPodStateFailure(pod)
		if !ok {
			continue
		}
		if chosen == nil || pod.Name < chosenPod {
			chosen, chosenPod = failure, pod.Name
		}
	}
	return chosen
}
