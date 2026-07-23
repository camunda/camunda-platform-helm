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
	"errors"
	"fmt"
	"io"
)

const RenovateApprover = "github-actions[bot]"

type ApproveClient interface {
	GetPullRequestHeadSHA(pr int) (string, error)
	ListReviews(pr int) ([]Review, error)
	CreateReview(pr int, commitID, event, body string) error
	DismissReview(pr int, reviewID int64, message string) error
}

func resolve(lane string) (approver, body string, err error) {
	switch lane {
	case LaneHuman:
		return DistroCIAuthor, "Auto-approved: author is on .github/auto-approve-allowlist.txt.", nil
	case LaneRenovate:
		return RenovateApprover, "Auto-approved: Renovate PR (re-approved on every push).", nil
	}
	return "", "", fmt.Errorf("unknown lane: %s", lane)
}

func Apply(lane string, prNumber int, vettedSHA string, client ApproveClient, stdout io.Writer) error {
	current, err := client.GetPullRequestHeadSHA(prNumber)
	if err != nil {
		return err
	}
	if current != vettedSHA {
		fmt.Fprintf(stdout, "::notice::head advanced from %s to %s; skipping approval (a newer run will evaluate it).\n", vettedSHA, current)
		return nil
	}

	approver, body, err := resolve(lane)
	if err != nil {
		return err
	}

	reviews, err := client.ListReviews(prNumber)
	if err != nil {
		return err
	}
	for _, r := range reviews {
		if r.State == "APPROVED" && r.UserLogin == approver && r.CommitID == vettedSHA {
			fmt.Fprintf(stdout, "::notice::Already approved by %s for %s; skipping.\n", approver, vettedSHA)
			return nil
		}
	}

	return client.CreateReview(prNumber, vettedSHA, "APPROVE", body)
}

func Dismiss(prNumber int, client ApproveClient, stdout io.Writer) error {
	botLogins := map[string]bool{
		"github-actions[bot]":   true,
		DistroCIAuthor:          true,
		"renovate-approve[bot]": true,
	}

	reviews, err := client.ListReviews(prNumber)
	if err != nil {
		return err
	}

	var errs []error
	for _, r := range reviews {
		if r.State == "APPROVED" && botLogins[r.UserLogin] {
			if err := client.DismissReview(prNumber, r.ID, "Head is no longer auto-approvable; dismissing stale bot approval."); err != nil {
				errs = append(errs, err)
				continue
			}
			fmt.Fprintf(stdout, "::notice::dismissed stale bot approval %d\n", r.ID)
		}
	}
	return errors.Join(errs...)
}
