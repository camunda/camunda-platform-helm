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

package main

import (
	"errors"
	"fmt"
	"io"
)

type triager struct {
	github  *githubClient
	assess  func(string, issue) (assessment, error)
	slack   *slackClient
	summary io.Writer
	project int
}

func (t *triager) run(number int) error {
	i, err := t.github.getIssue(number)
	if err != nil {
		return err
	}
	internal, err := t.github.internal(i)
	if err != nil {
		return err
	}
	if i.State != "open" {
		return nil
	}
	if !internal && !i.has(slackNotified) {
		if err := t.github.ensureLabel(slackRetry); err != nil {
			return err
		}
		if err := t.github.addLabels(number, slackRetry); err != nil {
			return err
		}
	}
	status, assessmentErr := t.process(i, internal)
	fmt.Fprintf(t.summary, "\n### Issue #%d\n\nInternal: %t\n\n%s\n", number, internal, status)
	if internal {
		return assessmentErr
	}
	if assessmentErr != nil {
		status = "Manual attention required: assessment or routing failed. See the Actions run."
	}
	return errors.Join(assessmentErr, t.slack.notify(t.github, i, status))
}

func (t *triager) process(i issue, internal bool) (string, error) {
	if !i.assessable() {
		if i.has("triage:manual") {
			return "Manual triage required.", nil
		}
		return "Already assessed; see the issue labels and project priority.", nil
	}
	a, err := t.assess(t.github.repo, i)
	if err != nil {
		if !internal {
			if labelErr := t.github.addLabels(i.Number, "triage:manual"); labelErr != nil {
				return "Assessment failed.", errors.Join(err, labelErr)
			}
			if commentErr := t.github.comment(i.Number, "❌ **Manual triage required**\n\nThe AI triage assistant failed to produce a valid assessment. The `triage:manual` label suppresses further automated assessments."); commentErr != nil {
				return "Assessment failed.", errors.Join(err, commentErr)
			}
		}
		return "Manual review needed: " + err.Error(), err
	}
	fmt.Fprintf(t.summary, "\n#### Suggested assessment for #%d\n\nSeverity: %s; likelihood: %s; confidence: %.2f\n\n%s\n", i.Number, a.Severity, a.Likelihood, a.Confidence, a.Rationale)
	if a.Confidence < 0.7 || a.Severity == "unknown" {
		return "Manual assessment needed: AI confidence is below 0.7 or severity is unknown.", nil
	}
	fresh, err := t.github.getIssue(i.Number)
	if err != nil {
		return "Could not refresh issue.", err
	}
	if !fresh.assessable() {
		return "Assessment skipped: issue state or triage labels changed.", nil
	}
	if !internal {
		if err := t.github.comment(i.Number, assessmentComment(a, fresh.User.Login)); err != nil {
			return "Could not publish assessment.", err
		}
		if a.Kind == "feature-request" && !fresh.has("kind/internal") {
			if err := t.github.addLabels(i.Number, "kind/feature-request", "triage:manual"); err != nil {
				return "PM routing failed.", err
			}
			err := t.github.request("POST", t.github.issuePath(i.Number)+"/assignees", map[string]any{"assignees": []string{"mlowe-camunda"}}, nil)
			return "Feature request routed to PM for manual triage.", err
		}
	}
	if err := t.github.addLabels(i.Number, assessmentLabels(fresh, a, internal)...); err != nil {
		return "Could not apply suggested priority.", err
	}
	if err := t.github.backlog(i.Number, t.project, internal); err != nil {
		return "Could not route issue to Backlog.", err
	}
	if err := t.github.addLabels(i.Number, "TRIAGE:COMPLETED"); err != nil {
		return "Could not mark assessment complete.", err
	}
	return fmt.Sprintf("AI assessment complete: severity %s, likelihood %s. See the issue for priority and details.", a.Severity, a.Likelihood), nil
}
