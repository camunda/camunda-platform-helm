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
	"testing"

	"github.com/stretchr/testify/suite"
)

type DecideTest struct {
	suite.Suite
}

func TestDecide(t *testing.T) {
	suite.Run(t, new(DecideTest))
}

const (
	thisOwner  = "camunda-hub/camunda"
	otherOwner = "other-ns/other"
	rev1       = "15.0.0-alpha4@aaaa"
	rev2       = "15.0.0-alpha4@bbbb"
)

func ownedRelease() ReleaseState {
	return ReleaseState{
		Exists:    true,
		Status:    "deployed",
		ChartName: "camunda-platform",
		Owner:     thisOwner,
		Revision:  1,
	}
}

func desired() Desired {
	return Desired{Revision: rev1, Owner: thisOwner}
}

func (s *DecideTest) TestTable() {
	cases := []struct {
		name     string
		rel      ReleaseState
		desired  Desired
		observed Observed
		want     Action
		reason   string
	}{
		{
			name:    "absent release installs",
			rel:     ReleaseState{},
			desired: desired(),
			want:    ActionInstall,
		},
		{
			name:     "owned and current is a no-op",
			rel:      ownedRelease(),
			desired:  desired(),
			observed: Observed{Revision: rev1},
			want:     ActionNoop,
			reason:   "UpToDate",
		},
		{
			name:     "changed revision upgrades",
			rel:      ownedRelease(),
			desired:  Desired{Revision: rev2, Owner: thisOwner},
			observed: Observed{Revision: rev1},
			want:     ActionUpgrade,
			reason:   "RevisionChanged",
		},
		{
			name: "failed upgrade is retried",
			rel: func() ReleaseState {
				r := ownedRelease()
				r.Status = "failed"
				r.Revision = 4
				return r
			}(),
			desired:  desired(),
			observed: Observed{Revision: rev1},
			want:     ActionUpgrade,
			reason:   "RetryingFailedRelease",
		},
		{
			// Helm cannot upgrade a release that never reached "deployed", nor
			// install over one that still exists, so this state needs a human.
			name: "failed first install is a dead end",
			rel: func() ReleaseState {
				r := ownedRelease()
				r.Status = "failed"
				r.Revision = 1
				return r
			}(),
			desired:  desired(),
			observed: Observed{Revision: rev1},
			want:     ActionBlocked,
			reason:   ReasonFailedFirstInstall,
		},
		{
			// An adopted release carries no ownership label until its next
			// upgrade writes one; status is what keeps it owned in between.
			name:     "release recorded in status stays owned without a label",
			rel:      func() ReleaseState { r := ownedRelease(); r.Owner = ""; return r }(),
			desired:  desired(),
			observed: Observed{Revision: rev1, Adopted: true},
			want:     ActionNoop,
			reason:   "UpToDate",
		},
		{
			name:     "unowned release without consent is blocked",
			rel:      func() ReleaseState { r := ownedRelease(); r.Owner = ""; return r }(),
			desired:  desired(),
			observed: Observed{},
			want:     ActionBlocked,
			reason:   ReasonAdoptionRequired,
		},
		{
			name:     "unowned release with consent is adopted",
			rel:      func() ReleaseState { r := ownedRelease(); r.Owner = ""; return r }(),
			desired:  Desired{Revision: rev1, Owner: thisOwner, AdoptExisting: true},
			observed: Observed{},
			want:     ActionAdopt,
		},
		{
			name: "adoption refuses a foreign chart",
			rel: func() ReleaseState {
				r := ownedRelease()
				r.Owner = ""
				r.ChartName = "some-other-chart"
				return r
			}(),
			desired:  Desired{Revision: rev1, Owner: thisOwner, AdoptExisting: true},
			observed: Observed{},
			want:     ActionBlocked,
			reason:   ReasonNotCamundaPlatform,
		},
		{
			name:     "release owned by another object is never touched",
			rel:      func() ReleaseState { r := ownedRelease(); r.Owner = otherOwner; return r }(),
			desired:  desired(),
			observed: Observed{Revision: rev1},
			want:     ActionBlocked,
			reason:   ReasonForeignOwner,
		},
		{
			name:     "pending release is left alone",
			rel:      func() ReleaseState { r := ownedRelease(); r.Status = "pending-upgrade"; return r }(),
			desired:  desired(),
			observed: Observed{Revision: rev1},
			want:     ActionBlocked,
			reason:   ReasonReleasePending,
		},
		{
			name:     "paused beats a pending revision change",
			rel:      ownedRelease(),
			desired:  Desired{Revision: rev2, Owner: thisOwner, Paused: true},
			observed: Observed{Revision: rev1},
			want:     ActionNoop,
			reason:   ReasonPaused,
		},
		{
			name:     "detected drift is reported but not corrected by default",
			rel:      ownedRelease(),
			desired:  Desired{Revision: rev1, Owner: thisOwner, DriftDetected: true},
			observed: Observed{Revision: rev1},
			want:     ActionNoop,
			reason:   "DriftDetected",
		},
		{
			name:     "drift is corrected when asked",
			rel:      ownedRelease(),
			desired:  Desired{Revision: rev1, Owner: thisOwner, DriftDetected: true, CorrectDrift: true},
			observed: Observed{Revision: rev1},
			want:     ActionUpgrade,
			reason:   "CorrectingDrift",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got := Decide(tc.rel, tc.desired, tc.observed)
			s.Equal(tc.want, got.Action)
			if tc.reason != "" {
				s.Equal(tc.reason, got.Reason)
			}
		})
	}
}

// TestAdoptionOfMatchingReleaseCausesNoWrite is the migration guarantee for
// existing Helm customers, expressed as a decision: adopting records ownership,
// and the follow-up reconcile finds nothing to do. No upgrade, so no pod churn.
func (s *DecideTest) TestAdoptionOfMatchingReleaseCausesNoWrite() {
	rel := ownedRelease()
	rel.Owner = ""

	first := Decide(rel, Desired{Revision: rev1, Owner: thisOwner, AdoptExisting: true}, Observed{})
	s.Require().Equal(ActionAdopt, first.Action)

	// Adoption records the release in status; no Helm write happened, so the
	// release still carries no ownership label.
	second := Decide(rel, Desired{Revision: rev1, Owner: thisOwner, AdoptExisting: true},
		Observed{Revision: rev1, Adopted: true})

	s.Equal(ActionNoop, second.Action)
	s.Equal("UpToDate", second.Reason)
}

// TestBlockedDecisionsExplainThemselves keeps the operator debuggable: every
// refusal has to tell the user what to do about it.
func (s *DecideTest) TestBlockedDecisionsExplainThemselves() {
	blocked := []ReleaseState{
		func() ReleaseState { r := ownedRelease(); r.Owner = ""; return r }(),
		func() ReleaseState { r := ownedRelease(); r.Owner = otherOwner; return r }(),
		func() ReleaseState { r := ownedRelease(); r.Status = "pending-install"; return r }(),
	}

	for _, rel := range blocked {
		got := Decide(rel, desired(), Observed{Revision: rev1})
		s.Require().Equal(ActionBlocked, got.Action)
		s.NotEmpty(got.Reason)
		s.NotEmpty(got.Message, "a blocked decision must say how to unblock it")
	}
}
