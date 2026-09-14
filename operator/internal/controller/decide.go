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

import "fmt"

// Action is what a single reconcile decided to do.
type Action string

const (
	// ActionNoop means the live release already matches the desired revision.
	ActionNoop Action = "Noop"
	// ActionInstall creates a release that does not exist yet.
	ActionInstall Action = "Install"
	// ActionUpgrade moves an owned release to the desired revision.
	ActionUpgrade Action = "Upgrade"
	// ActionAdopt records ownership of a pre-existing release without writing to
	// the cluster. Any convergence happens on the next reconcile as an ordinary
	// upgrade, so adopting an already-correct release causes no pod churn.
	ActionAdopt Action = "Adopt"
	// ActionBlocked means the operator refuses to proceed and needs a human.
	ActionBlocked Action = "Blocked"
)

// Block reasons, surfaced verbatim as condition reasons.
const (
	ReasonAdoptionRequired   = "AdoptionRequired"
	ReasonForeignOwner       = "ForeignOwner"
	ReasonReleasePending     = "ReleasePending"
	ReasonNotCamundaPlatform = "NotCamundaPlatform"
	ReasonFailedFirstInstall = "FailedFirstInstall"
	ReasonPaused             = "Paused"
)

// ReleaseState is the operator's view of a Helm release, reduced to the fields the
// decision depends on. Keeping it free of Helm types makes the decision a pure
// function that is testable without a cluster or a chart.
type ReleaseState struct {
	// Exists is false when no release by that name is stored in the namespace.
	Exists bool
	// Status is the Helm release status, for example "deployed", "failed",
	// "pending-upgrade".
	Status string
	// ChartName is the chart the release was installed from.
	ChartName string
	// Owner is the value of the operator's ownership label, empty when the
	// release was not created by an operator.
	Owner string
	// Revision is the current Helm revision.
	Revision int
}

// Desired is what the CamundaHub asks for.
type Desired struct {
	// Revision identifies the desired chart version and values, as
	// chartVersion@valuesChecksum.
	Revision string
	// Owner is this object's ownership identity, namespace/name.
	Owner string
	// AdoptExisting permits taking over a release the operator did not create.
	AdoptExisting bool
	// Paused stops all writes.
	Paused bool
	// DriftDetected is true when the rendered manifest no longer matches what was
	// last applied.
	DriftDetected bool
	// CorrectDrift upgrades to repair detected drift rather than only reporting it.
	CorrectDrift bool
}

// Observed is the operator's own record of what it last applied.
type Observed struct {
	// Revision is status.lastAppliedRevision, empty before the first apply.
	Revision string
	// Adopted is true when this object's status already records the live release.
	//
	// The ownership label is only written by an install or upgrade, and adoption
	// performs neither, so a freshly adopted release carries no label until it is
	// next upgraded. Without this the operator would re-adopt on every pass and
	// never converge.
	Adopted bool
}

// Decision is the outcome of Decide.
type Decision struct {
	Action Action
	Reason string
	// Message explains a Blocked decision in terms the user can act on.
	Message string
}

// expectedChartName is the only chart a CamundaHub will manage. Adopting anything
// else would put the operator in charge of a release it cannot reason about.
const expectedChartName = "camunda-platform"

// Decide chooses the action for one reconcile.
//
// The ordering matters: pause beats everything, a release the operator does not
// own is never touched without explicit consent, and a release mid-operation is
// left alone rather than clobbered.
func Decide(rel ReleaseState, desired Desired, observed Observed) Decision {
	if desired.Paused {
		return Decision{
			Action: ActionNoop, Reason: ReasonPaused,
			Message: "spec.paused is set; the operator is not writing to this release",
		}
	}

	if !rel.Exists {
		return Decision{Action: ActionInstall, Reason: "ReleaseAbsent"}
	}

	if isPending(rel.Status) {
		return Decision{
			Action: ActionBlocked, Reason: ReasonReleasePending,
			Message: fmt.Sprintf(
				"release is %q, meaning another Helm operation is in progress or was interrupted; "+
					"resolve it with helm rollback or helm uninstall before the operator continues",
				rel.Status),
		}
	}

	if rel.Owner == "" && !observed.Adopted {
		if !desired.AdoptExisting {
			return Decision{
				Action: ActionBlocked, Reason: ReasonAdoptionRequired,
				Message: "a Helm release with this name already exists and was not created by the " +
					"operator; set spec.adoption.adoptExisting to true to take it over in place",
			}
		}
		if rel.ChartName != "" && rel.ChartName != expectedChartName {
			return Decision{
				Action: ActionBlocked, Reason: ReasonNotCamundaPlatform,
				Message: fmt.Sprintf(
					"refusing to adopt release installed from chart %q; a CamundaHub manages only %q",
					rel.ChartName, expectedChartName),
			}
		}
		return Decision{Action: ActionAdopt, Reason: "AdoptingExistingRelease"}
	}

	if rel.Owner != "" && rel.Owner != desired.Owner {
		return Decision{
			Action: ActionBlocked, Reason: ReasonForeignOwner,
			Message: fmt.Sprintf(
				"release is owned by %q, not this CamundaHub (%q); two objects must not manage one release",
				rel.Owner, desired.Owner),
		}
	}

	if rel.Status == "failed" {
		// Helm cannot upgrade a release that never reached "deployed", and it
		// refuses to install over one that still exists. A first install that
		// failed is therefore a dead end that only an uninstall clears, and the
		// operator says so rather than surfacing Helm's confusing
		// "has no deployed releases".
		if rel.Revision <= 1 {
			return Decision{
				Action: ActionBlocked, Reason: ReasonFailedFirstInstall,
				Message: "the initial install failed and Helm has no deployed revision to upgrade " +
					"from; uninstall the release with `helm uninstall`, after which the operator " +
					"will install it again",
			}
		}
		return Decision{Action: ActionUpgrade, Reason: "RetryingFailedRelease"}
	}

	if observed.Revision != desired.Revision {
		return Decision{Action: ActionUpgrade, Reason: "RevisionChanged"}
	}

	if desired.DriftDetected {
		if desired.CorrectDrift {
			return Decision{Action: ActionUpgrade, Reason: "CorrectingDrift"}
		}
		return Decision{
			Action: ActionNoop, Reason: "DriftDetected",
			Message: "live resources no longer match the last applied manifest; " +
				"set spec.drift to Correct to have the operator repair it",
		}
	}

	return Decision{Action: ActionNoop, Reason: "UpToDate"}
}

func isPending(status string) bool {
	switch status {
	case "pending-install", "pending-upgrade", "pending-rollback", "uninstalling":
		return true
	default:
		return false
	}
}
