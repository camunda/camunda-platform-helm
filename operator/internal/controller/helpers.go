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

package controller

import (
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"operator/api/v1alpha1"
	operatorhelm "operator/internal/helm"
)

const defaultTimeout = 15 * time.Minute

func releaseName(hub *v1alpha1.CamundaHub) string {
	if hub.Spec.Release.Name != "" {
		return hub.Spec.Release.Name
	}
	return hub.Name
}

func releaseNamespace(hub *v1alpha1.CamundaHub) string {
	if hub.Spec.Release.TargetNamespace != "" {
		return hub.Spec.Release.TargetNamespace
	}
	return hub.Namespace
}

// ownerRef is the identity written to the Helm release ownership label.
//
// The UID is used rather than namespace/name because label values may not
// contain "/" and are capped at 63 characters, and because a UID distinguishes a
// recreated CamundaHub from the one it replaced.
func ownerRef(hub *v1alpha1.CamundaHub) string {
	return string(hub.UID)
}

func chartRef(hub *v1alpha1.CamundaHub) operatorhelm.ChartRef {
	return operatorhelm.ChartRef{
		Repository: hub.Spec.Chart.Repository,
		Name:       hub.Spec.Chart.Name,
		Version:    hub.Spec.Chart.Version,
		PlainHTTP:  hub.Spec.Chart.PlainHTTP,
	}
}

func timeout(hub *v1alpha1.CamundaHub) time.Duration {
	if hub.Spec.Release.Timeout != nil && hub.Spec.Release.Timeout.Duration > 0 {
		return hub.Spec.Release.Timeout.Duration
	}
	return defaultTimeout
}

// rollbackOnFailure defaults to true. It is a *bool in the API so an explicit
// false is distinguishable from an unset field.
func rollbackOnFailure(hub *v1alpha1.CamundaHub) bool {
	if hub.Spec.Release.Atomic == nil {
		return true
	}
	return *hub.Spec.Release.Atomic
}

func releaseState(info *operatorhelm.ReleaseInfo) ReleaseState {
	if info == nil {
		return ReleaseState{}
	}
	return ReleaseState{
		Exists:    true,
		Status:    info.Status,
		ChartName: info.ChartName,
		Owner:     info.Owner,
		Revision:  info.Revision,
	}
}

func recordRelease(hub *v1alpha1.CamundaHub, info *operatorhelm.ReleaseInfo) {
	if info == nil {
		return
	}
	hub.Status.HelmRelease = v1alpha1.HelmReleaseStatus{
		Name:         info.Name,
		Namespace:    info.Namespace,
		Revision:     info.Revision,
		ChartVersion: info.ChartVersion,
		AppVersion:   info.AppVersion,
		Status:       info.Status,
	}
}

func setCondition(hub *v1alpha1.CamundaHub, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&hub.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: hub.Generation,
	})
}

func setReady(hub *v1alpha1.CamundaHub, status metav1.ConditionStatus, reason, message string) {
	setCondition(hub, v1alpha1.ConditionReady, status, reason, message)
}

func controllerutilContains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func removeString(items []string, drop string) []string {
	out := items[:0]
	for _, item := range items {
		if item != drop {
			out = append(out, item)
		}
	}
	return out
}

// alreadyRecorded reports whether this object's status already refers to the live
// release, which is how an adopted release stays owned until the ownership label
// is written by its next upgrade.
func alreadyRecorded(hub *v1alpha1.CamundaHub, name, namespace string) bool {
	return hub.Status.HelmRelease.Name == name &&
		hub.Status.HelmRelease.Namespace == namespace &&
		hub.Status.LastAppliedRevision != ""
}

// releaseTargetKey identifies the Helm release a CamundaHub manages. Two objects
// resolving to the same key are in conflict.
func releaseTargetKey(hub *v1alpha1.CamundaHub) string {
	return releaseNamespace(hub) + "/" + releaseName(hub)
}
