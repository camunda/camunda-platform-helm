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

// Package v1alpha1 contains the CamundaHub API.
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TopologyModeHub is forced into the rendered values for every CamundaHub release.
const TopologyModeHub = "hub"

// UpgradePhase mirrors the chart's camundaHub.upgrade.phase enum.
type UpgradePhase string

const (
	PhaseNormal  UpgradePhase = "normal"
	PhaseQuiesce UpgradePhase = "quiesce"
	PhaseMigrate UpgradePhase = "migrate"
)

// ChartSource locates the camunda-platform chart to render.
type ChartSource struct {
	// Repository is an OCI reference such as oci://ghcr.io/camunda/team-distribution/camunda-platform.
	// +kubebuilder:validation:MinLength=1
	Repository string `json:"repository"`
	// Name is the chart name within the repository.
	// +kubebuilder:default=camunda-platform
	Name string `json:"name,omitempty"`
	// Version is the chart SemVer, for example 15.0.0.
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version"`
	// Digest pins the chart by content digest. Required for reproducible
	// disconnected installs; when set it takes precedence over Version.
	// +optional
	Digest string `json:"digest,omitempty"`
	// PullSecretRef names a Secret of type kubernetes.io/dockerconfigjson used to
	// authenticate against a private registry.
	// +optional
	PullSecretRef *corev1.LocalObjectReference `json:"pullSecretRef,omitempty"`
	// PlainHTTP contacts the registry over HTTP instead of HTTPS, for a private
	// registry that does not serve TLS.
	// +optional
	PlainHTTP bool `json:"plainHTTP,omitempty"`
}

// ReleaseSpec describes the Helm release the operator owns.
type ReleaseSpec struct {
	// Name of the Helm release. Defaults to the CamundaHub object name.
	// +optional
	Name string `json:"name,omitempty"`
	// TargetNamespace defaults to the CamundaHub object's namespace.
	// +optional
	TargetNamespace string `json:"targetNamespace,omitempty"`
	// CreateNamespace creates TargetNamespace when it does not exist.
	// +optional
	CreateNamespace bool `json:"createNamespace,omitempty"`
	// Timeout for each Helm operation.
	//
	// A pointer so that omitempty actually omits it: metav1.Duration is a struct,
	// which always serialises (as "0s"), and the API server only applies a default
	// to a field that is absent.
	// +kubebuilder:default="15m"
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
	// Atomic rolls back a failed install or upgrade.
	// +kubebuilder:default=true
	// +optional
	Atomic *bool `json:"atomic,omitempty"`
}

// ValuesSource references chart values held outside the CamundaHub object.
// Sources are merged in declaration order, and Spec.Values is merged last.
type ValuesSource struct {
	// +kubebuilder:validation:Enum=ConfigMap;Secret
	Kind string `json:"kind"`
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Key selects a single YAML document within the source. Defaults to values.yaml.
	// +optional
	Key string `json:"key,omitempty"`
	// Optional tolerates a missing source instead of failing the reconcile.
	// +optional
	Optional bool `json:"optional,omitempty"`
}

// ApprovalMode controls whether the operator upgrades without human action.
// +kubebuilder:validation:Enum=Automatic;Manual
type ApprovalMode string

const (
	ApprovalAutomatic ApprovalMode = "Automatic"
	ApprovalManual    ApprovalMode = "Manual"
)

// UpgradeSpec governs how the operator moves between chart revisions.
type UpgradeSpec struct {
	// Approval gates upgrades that change the chart version. Values-only changes
	// always apply, because withholding them would leave the release diverged from
	// its own spec.
	// +kubebuilder:default=Manual
	// +optional
	Approval ApprovalMode `json:"approval,omitempty"`
	// ApprovedVersion authorises one specific chart version under Manual approval.
	// +optional
	ApprovedVersion string `json:"approvedVersion,omitempty"`
	// Phased drives the chart's camundaHub.upgrade.phase state machine for the
	// non-backward-compatible 8.9 to 8.10 Hub database migration. Ignored, with a
	// warning, when the resolved chart does not declare that key.
	// +optional
	Phased bool `json:"phased,omitempty"`
	// BackupVerified is the human attestation that a Hub database backup exists.
	// A phased upgrade will not leave the quiesce phase without it.
	// +optional
	BackupVerified bool `json:"backupVerified,omitempty"`
}

// DriftMode controls reaction to divergence between rendered and live state.
// +kubebuilder:validation:Enum=Detect;Correct;Off
type DriftMode string

const (
	DriftDetect  DriftMode = "Detect"
	DriftCorrect DriftMode = "Correct"
	DriftOff     DriftMode = "Off"
)

// AdoptionSpec controls takeover of a pre-existing Helm release.
type AdoptionSpec struct {
	// AdoptExisting allows the operator to assume ownership of a Helm release it
	// did not create, in place and without reinstalling. This is the migration
	// path for customers already running the chart via the Helm CLI.
	// +optional
	AdoptExisting bool `json:"adoptExisting,omitempty"`
}

// DeletionPolicy decides what happens to the Helm release when the CR is deleted.
// +kubebuilder:validation:Enum=Retain;Delete
type DeletionPolicy string

const (
	// DeletionRetain leaves the Helm release, its workloads and its data in place.
	DeletionRetain DeletionPolicy = "Retain"
	// DeletionDelete uninstalls the Helm release. It never deletes PersistentVolumeClaims,
	// Secrets, or externally provisioned Identity and Keycloak records.
	DeletionDelete DeletionPolicy = "Delete"
)

// CamundaHubSpec is the desired state of a Camunda Hub release.
type CamundaHubSpec struct {
	Chart   ChartSource `json:"chart"`
	Release ReleaseSpec `json:"release,omitempty"`

	// Values are chart values passed through verbatim. They are validated against
	// the resolved chart's own values.schema.json rather than duplicated as typed
	// fields, so this API does not drift against the chart's values contract.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Values *apiextensionsv1.JSON `json:"values,omitempty"`

	// +optional
	ValuesFrom []ValuesSource `json:"valuesFrom,omitempty"`

	// +optional
	Upgrade UpgradeSpec `json:"upgrade,omitempty"`
	// +optional
	Adoption AdoptionSpec `json:"adoption,omitempty"`
	// +kubebuilder:default=Detect
	// +optional
	Drift DriftMode `json:"drift,omitempty"`
	// Paused stops all writes so an operator can take the release over with the
	// Helm CLI without the controller fighting them.
	// +optional
	Paused bool `json:"paused,omitempty"`
	// +kubebuilder:default=Retain
	// +optional
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// HelmReleaseStatus reports the Helm release backing this object.
type HelmReleaseStatus struct {
	Name         string `json:"name,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
	Revision     int    `json:"revision,omitempty"`
	ChartVersion string `json:"chartVersion,omitempty"`
	AppVersion   string `json:"appVersion,omitempty"`
	Status       string `json:"status,omitempty"`
}

// CamundaHubStatus is the observed state of a Camunda Hub release.
type CamundaHubStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// LastAppliedRevision is chartVersion@valuesHash, the identity of what was last applied.
	// +optional
	LastAppliedRevision string `json:"lastAppliedRevision,omitempty"`
	// ManifestDigest is a SHA-256 over the rendered manifest, used for drift detection.
	// +optional
	ManifestDigest string `json:"manifestDigest,omitempty"`
	// +optional
	HelmRelease HelmReleaseStatus `json:"helmRelease,omitempty"`
	// Phase is the current camundaHub.upgrade.phase during a phased upgrade.
	// +optional
	Phase UpgradePhase `json:"phase,omitempty"`
	// +optional
	LastFailure string `json:"lastFailure,omitempty"`
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Condition types reported on CamundaHub.
const (
	ConditionReady           = "Ready"
	ConditionChartResolved   = "ChartResolved"
	ConditionValuesValid     = "ValuesValid"
	ConditionReleaseDeployed = "ReleaseDeployed"
	ConditionDrifted         = "Drifted"
	ConditionMigrating       = "Migrating"
	ConditionCleanupRequired = "CleanupRequired"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=chub
// +kubebuilder:printcolumn:name="Chart",type=string,JSONPath=`.status.helmRelease.chartVersion`
// +kubebuilder:printcolumn:name="Release",type=string,JSONPath=`.status.helmRelease.status`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CamundaHub manages one Camunda Hub release: Management Identity plus Camunda Hub,
// rendered from the camunda-platform chart with global.topology.mode set to hub.
type CamundaHub struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CamundaHubSpec   `json:"spec,omitempty"`
	Status CamundaHubStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CamundaHubList contains a list of CamundaHub.
type CamundaHubList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CamundaHub `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CamundaHub{}, &CamundaHubList{})
}
