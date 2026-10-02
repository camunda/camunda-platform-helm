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
	"time"

	corev1 "k8s.io/api/core/v1"
)

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
	// evidenceAt is the autoscaler verdict time for Unschedulable; zero otherwise.
	evidenceAt time.Time
}

func (f *PodFailure) Error() string {
	subject := fmt.Sprintf("pod %q", f.Pod)
	if f.Container != "" {
		subject = fmt.Sprintf("container %q in pod %q", f.Container, f.Pod)
	}
	return fmt.Sprintf("%s will not become ready (%s): %s", subject, f.Reason, f.Message)
}

func (f *PodFailure) streakKey() string {
	return f.Pod + "/" + f.Container + "/" + f.Reason
}

func (f *PodFailure) evidenceTime() time.Time {
	return f.evidenceAt
}

func (f *PodFailure) abortReason() string {
	if reason, ok := podFailureAbortReasons[f.Reason]; ok {
		return reason
	}
	return "helm upgrade --install aborted early: terminal pod state"
}

var podFailureAbortReasons = map[string]string{
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

// Cluster autoscaler event reasons recorded against a pending pod.
const (
	reasonTriggeredScaleUp  = "TriggeredScaleUp"
	reasonNotTriggerScaleUp = "NotTriggerScaleUp"
)

// autoscalerVerdict is the most recent cluster autoscaler decision for a pod.
type autoscalerVerdict struct {
	uid     string
	reason  string
	message string
	at      time.Time
}

// autoscalerVerdicts keys the latest TriggeredScaleUp or NotTriggerScaleUp
// event by pod name. On a timestamp tie TriggeredScaleUp wins.
func autoscalerVerdicts(events *corev1.EventList) map[string]autoscalerVerdict {
	verdicts := map[string]autoscalerVerdict{}
	if events == nil {
		return verdicts
	}
	for _, event := range events.Items {
		if event.InvolvedObject.Kind != "Pod" {
			continue
		}
		if event.Reason != reasonTriggeredScaleUp && event.Reason != reasonNotTriggerScaleUp {
			continue
		}
		at := eventTime(event)
		prev, seen := verdicts[event.InvolvedObject.Name]
		if seen && (at.Before(prev.at) || (at.Equal(prev.at) && event.Reason == reasonNotTriggerScaleUp)) {
			continue
		}
		verdicts[event.InvolvedObject.Name] = autoscalerVerdict{
			uid:     string(event.InvolvedObject.UID),
			reason:  event.Reason,
			message: strings.TrimSpace(event.Message),
			at:      at,
		}
	}
	return verdicts
}

func eventTime(event corev1.Event) time.Time {
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	return event.FirstTimestamp.Time
}

// terminalUnschedulable reports an Unschedulable pod only when the cluster
// autoscaler's latest verdict for it is NotTriggerScaleUp. A pending scale-up,
// or a cluster without an autoscaler, never yields a failure.
func terminalUnschedulable(pod *corev1.Pod, verdicts map[string]autoscalerVerdict) (*PodFailure, bool) {
	verdict, ok := verdicts[pod.Name]
	if !ok || verdict.reason != reasonNotTriggerScaleUp {
		return nil, false
	}
	if verdict.uid != "" && pod.UID != "" && verdict.uid != string(pod.UID) {
		return nil, false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type != corev1.PodScheduled || condition.Status != corev1.ConditionFalse {
			continue
		}
		if condition.Reason != corev1.PodReasonUnschedulable {
			continue
		}
		message := strings.TrimSpace(condition.Message)
		if verdict.message != "" {
			message += "; cluster autoscaler: " + verdict.message
		}
		return &PodFailure{
			Pod:        pod.Name,
			Reason:     condition.Reason,
			Message:    message,
			evidenceAt: verdict.at,
		}, true
	}
	return nil, false
}

// terminalPodStateFailure runs the detectors in a fixed order so a pod with more
// than one problem always reports the same one, keeping the streak key stable
// across polls.
func terminalPodStateFailure(pod *corev1.Pod, verdicts map[string]autoscalerVerdict) (terminalPodFailure, bool) {
	if failure, ok := terminalImagePullFailure(pod); ok {
		return failure, true
	}
	if failure, ok := terminalConfigError(pod); ok {
		return failure, true
	}
	if failure, ok := terminalUnschedulable(pod, verdicts); ok {
		return failure, true
	}
	return nil, false
}

// firstTerminalFailure returns the terminal failure of the alphabetically first
// affected pod, keeping the streak key stable across polls.
func firstTerminalFailure(pods *corev1.PodList, verdicts map[string]autoscalerVerdict) terminalPodFailure {
	var chosen terminalPodFailure
	var chosenPod string
	for i := range pods.Items {
		pod := &pods.Items[i]
		failure, ok := terminalPodStateFailure(pod, verdicts)
		if !ok {
			continue
		}
		if chosen == nil || pod.Name < chosenPod {
			chosen, chosenPod = failure, pod.Name
		}
	}
	return chosen
}
