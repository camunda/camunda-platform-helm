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

// Package phase drives the Camunda Hub 8.9 to 8.10 database migration.
//
// The migration is not backward compatible, so the chart exposes
// camundaHub.upgrade.phase and the operator sequences it:
//
//	normal -> quiesce -> [backup gate] -> migrate -> normal
//
// quiesce scales both Hub Deployments to zero so nothing writes to the database.
// migrate runs a single REST API pod that performs the startup migration; the
// Hub Services pin their selector to camunda.io/upgrade-phase=normal, so that pod
// migrates without ever serving traffic. normal restores replicas.
//
// The decision is a pure function so the sequence can be tested without a
// cluster, a chart, or Helm.
package phase

import "operator/api/v1alpha1"

// Act is what the controller should do for the current step.
type Act string

const (
	// ActApply performs a Helm upgrade that moves the release to Step.Target.
	ActApply Act = "Apply"
	// ActWait means the last applied phase has not converged yet.
	ActWait Act = "Wait"
	// ActBlocked means a human has to act before the sequence continues.
	ActBlocked Act = "Blocked"
	// ActDone means no phased sequence is in progress.
	ActDone Act = "Done"
)

// Reasons reported on the Migrating condition.
const (
	ReasonNotRequested     = "PhasedUpgradeNotRequested"
	ReasonChartUnsupported = "ChartDoesNotSupportPhases"
	ReasonQuiescing        = "Quiescing"
	ReasonAwaitingBackup   = "AwaitingBackupConfirmation"
	ReasonMigrating        = "Migrating"
	ReasonRestoring        = "RestoringService"
	ReasonComplete         = "MigrationComplete"
)

// State is everything the decision depends on.
type State struct {
	// Requested is spec.upgrade.phased.
	Requested bool
	// ChartSupportsPhases is whether the resolved chart declares
	// camundaHub.upgrade.phase.
	ChartSupportsPhases bool
	// PendingRevisionChange is whether the release is not yet at the desired
	// chart version and values. A sequence only starts when there is something to
	// upgrade to.
	PendingRevisionChange bool
	// Current is status.phase: the phase last applied, empty before the sequence
	// starts. Persisting it is what lets the sequence resume after a restart.
	Current v1alpha1.UpgradePhase
	// Converged is whether the running workloads match Current.
	Converged bool
	// BackupVerified is spec.upgrade.backupVerified, the human attestation that a
	// Hub database backup exists.
	BackupVerified bool
}

// Step is the decision.
type Step struct {
	Act    Act
	Target v1alpha1.UpgradePhase
	Reason string
	// Message explains a Blocked or Wait step to the operator.
	Message string
	// AllowRollback is false once the database schema may have moved. Rolling a
	// migrated database back under older code is worse than stopping, so this is
	// derived here rather than left to configuration.
	AllowRollback bool
}

// Next returns the step to take.
func Next(s State) Step {
	if !s.Requested {
		return Step{Act: ActDone, Target: v1alpha1.PhaseNormal, Reason: ReasonNotRequested, AllowRollback: true}
	}

	if !s.ChartSupportsPhases {
		return Step{
			Act: ActDone, Target: v1alpha1.PhaseNormal, Reason: ReasonChartUnsupported,
			AllowRollback: true,
			Message: "the resolved chart does not declare camundaHub.upgrade.phase, so the upgrade " +
				"is applied in one step; phased upgrades need a chart that supports them",
		}
	}

	switch s.Current {
	case "", v1alpha1.PhaseNormal:
		// A finished sequence leaves Current at normal. Only a pending change
		// starts a new one, so completing a sequence does not immediately restart it.
		if !s.PendingRevisionChange {
			return Step{Act: ActDone, Target: v1alpha1.PhaseNormal, Reason: ReasonComplete, AllowRollback: true}
		}
		return Step{
			Act: ActApply, Target: v1alpha1.PhaseQuiesce, Reason: ReasonQuiescing,
			AllowRollback: true,
			Message:       "scaling Camunda Hub to zero so nothing writes to the database during migration",
		}

	case v1alpha1.PhaseQuiesce:
		if !s.Converged {
			return Step{
				Act: ActWait, Target: v1alpha1.PhaseQuiesce, Reason: ReasonQuiescing,
				AllowRollback: true,
				Message:       "waiting for all Camunda Hub pods to stop",
			}
		}
		if !s.BackupVerified {
			return Step{
				Act: ActBlocked, Target: v1alpha1.PhaseQuiesce, Reason: ReasonAwaitingBackup,
				AllowRollback: true,
				Message: "Camunda Hub is stopped and the database is ready to back up. The migration " +
					"is not reversible, so it will not start until spec.upgrade.backupVerified is set " +
					"to true to confirm a verified backup exists",
			}
		}
		return Step{
			Act: ActApply, Target: v1alpha1.PhaseMigrate, Reason: ReasonMigrating,
			// From here the schema may change, so a failure must stop rather than
			// roll back into a mismatch between old code and a migrated database.
			AllowRollback: false,
			Message:       "running the database migration on a single pod that serves no traffic",
		}

	case v1alpha1.PhaseMigrate:
		if !s.Converged {
			return Step{
				Act: ActWait, Target: v1alpha1.PhaseMigrate, Reason: ReasonMigrating,
				AllowRollback: false,
				Message:       "waiting for the migration pod to become ready",
			}
		}
		return Step{
			Act: ActApply, Target: v1alpha1.PhaseNormal, Reason: ReasonRestoring,
			AllowRollback: false,
			Message:       "migration finished; restoring Camunda Hub replicas and traffic",
		}

	default:
		return Step{
			Act: ActBlocked, Target: v1alpha1.PhaseNormal, Reason: "UnknownPhase",
			AllowRollback: false,
			Message:       "status.phase holds an unrecognised value; clear it to resume",
		}
	}
}

// Complete reports whether a sequence that reached normal has finished, which is
// when the controller clears status.phase.
func Complete(s State, step Step) bool {
	return s.Current == v1alpha1.PhaseNormal && s.Converged && step.Act == ActDone
}
