// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gate

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type createCall struct {
	commitID string
	event    string
	body     string
}

type fakeApproveClient struct {
	headSHA    string
	headErr    error
	reviews    []Review
	revErr     error
	dismissErr error

	createCalls  []createCall
	dismissedIDs []int64
}

func (f *fakeApproveClient) GetPullRequestHeadSHA(pr int) (string, error) {
	return f.headSHA, f.headErr
}

func (f *fakeApproveClient) ListReviews(pr int) ([]Review, error) {
	return f.reviews, f.revErr
}

func (f *fakeApproveClient) CreateReview(pr int, commitID, event, body string) error {
	f.createCalls = append(f.createCalls, createCall{commitID: commitID, event: event, body: body})
	return nil
}

func (f *fakeApproveClient) DismissReview(pr int, reviewID int64, message string) error {
	if f.dismissErr != nil {
		return f.dismissErr
	}
	f.dismissedIDs = append(f.dismissedIDs, reviewID)
	return nil
}

func TestApply_headAdvanced(t *testing.T) {
	client := &fakeApproveClient{headSHA: "newsha"}
	var buf bytes.Buffer

	err := Apply(LaneHuman, 1, "oldsha", client, &buf)
	require.NoError(t, err)
	assert.Empty(t, client.createCalls)
	assert.Contains(t, buf.String(), "head advanced")
}

func TestApply_headFetchError(t *testing.T) {
	client := &fakeApproveClient{headErr: errors.New("api down")}
	var buf bytes.Buffer

	err := Apply(LaneHuman, 1, "sha1", client, &buf)
	require.Error(t, err)
	assert.Empty(t, client.createCalls)
}

func TestApply_alreadyApproved(t *testing.T) {
	client := &fakeApproveClient{
		headSHA: "sha1",
		reviews: []Review{
			{ID: 1, UserLogin: DistroCIAuthor, CommitID: "sha1", State: "APPROVED"},
		},
	}
	var buf bytes.Buffer

	err := Apply(LaneHuman, 1, "sha1", client, &buf)
	require.NoError(t, err)
	assert.Empty(t, client.createCalls)
	assert.Contains(t, buf.String(), "Already approved")
}

func TestApply_happyHuman(t *testing.T) {
	client := &fakeApproveClient{headSHA: "sha1"}
	var buf bytes.Buffer

	err := Apply(LaneHuman, 1, "sha1", client, &buf)
	require.NoError(t, err)
	require.Len(t, client.createCalls, 1)
	assert.Equal(t, createCall{
		commitID: "sha1",
		event:    "APPROVE",
		body:     "Auto-approved: author is on .github/auto-approve-allowlist.txt.",
	}, client.createCalls[0])
}

func TestApply_happyRenovate(t *testing.T) {
	client := &fakeApproveClient{headSHA: "sha1"}
	var buf bytes.Buffer

	err := Apply(LaneRenovate, 1, "sha1", client, &buf)
	require.NoError(t, err)
	require.Len(t, client.createCalls, 1)
	assert.Equal(t, createCall{
		commitID: "sha1",
		event:    "APPROVE",
		body:     "Auto-approved: Renovate PR (re-approved on every push).",
	}, client.createCalls[0])
}

func TestApply_dedupSkipsOnlyExactMatch(t *testing.T) {
	tests := []struct {
		name    string
		reviews []Review
	}{
		{
			name: "different commit id",
			reviews: []Review{
				{ID: 1, UserLogin: RenovateApprover, CommitID: "othersha", State: "APPROVED"},
			},
		},
		{
			name: "different login",
			reviews: []Review{
				{ID: 1, UserLogin: "someone-else[bot]", CommitID: "sha1", State: "APPROVED"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeApproveClient{headSHA: "sha1", reviews: tt.reviews}
			var buf bytes.Buffer

			err := Apply(LaneRenovate, 1, "sha1", client, &buf)
			require.NoError(t, err)
			require.Len(t, client.createCalls, 1)
		})
	}
}

func TestApply_unknownLane(t *testing.T) {
	client := &fakeApproveClient{headSHA: "sha1"}
	var buf bytes.Buffer

	err := Apply("bogus", 1, "sha1", client, &buf)
	require.Error(t, err)
	assert.Empty(t, client.createCalls)
}

func TestDismiss_dismissesOnlyApprovedBots(t *testing.T) {
	client := &fakeApproveClient{
		reviews: []Review{
			{ID: 1, UserLogin: "github-actions[bot]", State: "APPROVED"},
			{ID: 2, UserLogin: DistroCIAuthor, State: "APPROVED"},
			{ID: 3, UserLogin: "renovate-approve[bot]", State: "APPROVED"},
			{ID: 4, UserLogin: "eamonnmoloney", State: "APPROVED"},
			{ID: 5, UserLogin: "github-actions[bot]", State: "COMMENTED"},
		},
	}
	var buf bytes.Buffer

	err := Dismiss(1, client, &buf)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{1, 2, 3}, client.dismissedIDs)
}

func TestDismiss_listReviewsError(t *testing.T) {
	client := &fakeApproveClient{revErr: errors.New("api down")}
	var buf bytes.Buffer

	err := Dismiss(1, client, &buf)
	require.Error(t, err)
	assert.Empty(t, client.dismissedIDs)
}
