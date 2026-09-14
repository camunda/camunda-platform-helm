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

package phase

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"operator/api/v1alpha1"
)

type MachineTest struct {
	suite.Suite
}

func TestMachine(t *testing.T) {
	suite.Run(t, new(MachineTest))
}

func phased() State {
	return State{Requested: true, ChartSupportsPhases: true, PendingRevisionChange: true}
}

func (s *MachineTest) TestSequence() {
	cases := []struct {
		name   string
		state  State
		act    Act
		target v1alpha1.UpgradePhase
		reason string
	}{
		{
			name:  "not requested does nothing",
			state: State{Requested: false, ChartSupportsPhases: true, PendingRevisionChange: true},
			act:   ActDone, target: v1alpha1.PhaseNormal, reason: ReasonNotRequested,
		},
		{
			name:  "chart without the values key degrades to a plain upgrade",
			state: State{Requested: true, ChartSupportsPhases: false, PendingRevisionChange: true},
			act:   ActDone, target: v1alpha1.PhaseNormal, reason: ReasonChartUnsupported,
		},
		{
			name:  "nothing to upgrade to does not start a sequence",
			state: State{Requested: true, ChartSupportsPhases: true, PendingRevisionChange: false},
			act:   ActDone, target: v1alpha1.PhaseNormal, reason: ReasonComplete,
		},
		{
			name:  "a pending change starts by quiescing",
			state: phased(),
			act:   ActApply, target: v1alpha1.PhaseQuiesce, reason: ReasonQuiescing,
		},
		{
			name: "quiesce waits for pods to stop",
			state: func() State {
				st := phased()
				st.Current = v1alpha1.PhaseQuiesce
				return st
			}(),
			act: ActWait, target: v1alpha1.PhaseQuiesce, reason: ReasonQuiescing,
		},
		{
			name: "quiesced without a backup blocks",
			state: func() State {
				st := phased()
				st.Current, st.Converged = v1alpha1.PhaseQuiesce, true
				return st
			}(),
			act: ActBlocked, target: v1alpha1.PhaseQuiesce, reason: ReasonAwaitingBackup,
		},
		{
			name: "a confirmed backup releases the migration",
			state: func() State {
				st := phased()
				st.Current, st.Converged, st.BackupVerified = v1alpha1.PhaseQuiesce, true, true
				return st
			}(),
			act: ActApply, target: v1alpha1.PhaseMigrate, reason: ReasonMigrating,
		},
		{
			name: "migrate waits for the migration pod",
			state: func() State {
				st := phased()
				st.Current, st.BackupVerified = v1alpha1.PhaseMigrate, true
				return st
			}(),
			act: ActWait, target: v1alpha1.PhaseMigrate, reason: ReasonMigrating,
		},
		{
			name: "a finished migration restores service",
			state: func() State {
				st := phased()
				st.Current, st.Converged, st.BackupVerified = v1alpha1.PhaseMigrate, true, true
				return st
			}(),
			act: ActApply, target: v1alpha1.PhaseNormal, reason: ReasonRestoring,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got := Next(tc.state)
			s.Equal(tc.act, got.Act)
			s.Equal(tc.target, got.Target)
			s.Equal(tc.reason, got.Reason)
		})
	}
}

// TestRollbackIsForbiddenFromMigrateOnwards is the safety property that must not
// regress. Once the schema may have moved, rolling back would leave 8.9 code
// against an 8.10 database, which is worse than stopping and asking for help.
func (s *MachineTest) TestRollbackIsForbiddenFromMigrateOnwards() {
	st := phased()
	st.Current, st.Converged, st.BackupVerified = v1alpha1.PhaseQuiesce, true, true

	toMigrate := Next(st)
	s.Require().Equal(v1alpha1.PhaseMigrate, toMigrate.Target)
	s.False(toMigrate.AllowRollback, "the step that starts the migration must not roll back")

	st.Current = v1alpha1.PhaseMigrate
	s.False(Next(st).AllowRollback, "waiting for the migration must not roll back")

	st.Converged = true
	toNormal := Next(st)
	s.Require().Equal(v1alpha1.PhaseNormal, toNormal.Target)
	s.False(toNormal.AllowRollback,
		"restoring service after a migration must not roll back into the pre-migration chart")
}

// TestRollbackIsAllowedBeforeAnySchemaChange is the other half: quiescing only
// scales workloads down, so a failure there is safe to revert.
func (s *MachineTest) TestRollbackIsAllowedBeforeAnySchemaChange() {
	s.True(Next(phased()).AllowRollback, "quiescing changes no data")

	st := phased()
	st.Current = v1alpha1.PhaseQuiesce
	s.True(Next(st).AllowRollback)
}

// TestResumesMidSequence covers a manager restart: the phase is read back from
// status, so the sequence continues instead of starting again.
func (s *MachineTest) TestResumesMidSequence() {
	restarted := State{
		Requested: true, ChartSupportsPhases: true, PendingRevisionChange: true,
		Current: v1alpha1.PhaseMigrate, Converged: true, BackupVerified: true,
	}

	step := Next(restarted)
	s.Equal(ActApply, step.Act)
	s.Equal(v1alpha1.PhaseNormal, step.Target,
		"a restart mid-migration resumes at the next step, it does not re-quiesce")
}

// TestBlockedAndWaitingStepsExplainThemselves keeps the sequence debuggable from
// kubectl alone.
func (s *MachineTest) TestBlockedAndWaitingStepsExplainThemselves() {
	st := phased()
	st.Current, st.Converged = v1alpha1.PhaseQuiesce, true

	blocked := Next(st)
	s.Require().Equal(ActBlocked, blocked.Act)
	s.Contains(blocked.Message, "backupVerified")
}

func (s *MachineTest) TestUnknownPhaseBlocks() {
	st := phased()
	st.Current = v1alpha1.UpgradePhase("bogus")

	got := Next(st)
	s.Equal(ActBlocked, got.Act)
	s.False(got.AllowRollback)
}
