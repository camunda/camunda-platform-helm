// Copyright 2025 Camunda Services GmbH
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
	"testing"
	"time"
)

type fakeClient struct {
	t                   testing.TB
	findRunQueue        []findRunResp
	runURL              string
	runURLErr           error
	attemptsQueue       []int
	statusByAttempt     map[int][]statusResp
	conclusionByAttempt map[int]string
	conclusionErr       map[int]error
	rerunQueue          []error
	rerunCalls          int
}

type findRunResp struct {
	id  string
	err error
}

type statusResp struct {
	status string
	err    error
}

func (f *fakeClient) FindRun(_, _, _ string) (string, error) {
	if len(f.findRunQueue) == 0 {
		f.t.Fatalf("findRunQueue exhausted")
	}
	r := f.findRunQueue[0]
	f.findRunQueue = f.findRunQueue[1:]
	return r.id, r.err
}
func (f *fakeClient) RunURL(string) (string, error) {
	return f.runURL, f.runURLErr
}
func (f *fakeClient) RunAttempt(string) (int, error) {
	if len(f.attemptsQueue) == 0 {
		f.t.Fatalf("attemptsQueue exhausted")
	}
	v := f.attemptsQueue[0]
	f.attemptsQueue = f.attemptsQueue[1:]
	return v, nil
}
func (f *fakeClient) AttemptStatus(_ string, attempt int) (string, error) {
	q := f.statusByAttempt[attempt]
	if len(q) == 0 {
		f.t.Fatalf("statusByAttempt[%d] exhausted", attempt)
	}
	r := q[0]
	f.statusByAttempt[attempt] = q[1:]
	return r.status, r.err
}
func (f *fakeClient) AttemptConclusion(_ string, attempt int) (string, error) {
	if err, ok := f.conclusionErr[attempt]; ok {
		return "", err
	}
	return f.conclusionByAttempt[attempt], nil
}
func (f *fakeClient) RerunFailed(string) error {
	f.rerunCalls++
	if len(f.rerunQueue) == 0 {
		return nil
	}
	e := f.rerunQueue[0]
	f.rerunQueue = f.rerunQueue[1:]
	return e
}

func statusList(statuses ...string) []statusResp {
	out := make([]statusResp, len(statuses))
	for i, s := range statuses {
		out[i] = statusResp{status: s}
	}
	return out
}

func newTestGate(client ghClient) *Gate {
	return &Gate{
		Client:               client,
		Workflow:             "test-chart-version.yaml",
		DiscoveryTries:       3,
		DiscoveryInterval:    time.Nanosecond,
		PollInterval:         time.Nanosecond,
		MaxConsecutiveErrors: 5,
		RegistrationTries:    5,
		RegistrationInterval: time.Nanosecond,
		RerunTries:           3,
		RerunBackoff:         time.Nanosecond,
		Sleep:                func(time.Duration) {},
		Logf:                 func(string, ...any) {},
	}
}

func TestResolveSHA(t *testing.T) {
	cases := []struct {
		name      string
		event     string
		prHead    string
		mgHead    string
		want      string
		wantError bool
	}{
		{"pull_request", "pull_request", "abc", "", "abc", false},
		{"pull_request_target", "pull_request_target", "abc", "", "abc", false},
		{"merge_group_uses_head", "merge_group", "", "def", "def", false},
		{"merge_group_ignores_pr_head", "merge_group", "abc", "def", "def", false},
		{"unknown_event", "schedule", "abc", "def", "", true},
		{"empty_pr_head", "pull_request", "", "", "", true},
		{"empty_mg_head", "merge_group", "abc", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveSHA(tc.event, tc.prHead, tc.mgHead)
			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestDiscover_EventualVisibility(t *testing.T) {
	c := &fakeClient{
		t: t,
		findRunQueue: []findRunResp{
			{id: ""}, {id: ""}, {id: "12345"},
		},
		runURL: "https://github.com/x/y/actions/runs/12345",
	}
	g := newTestGate(c)
	runID, runURL, err := g.Discover("sha", "pull_request")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runID != "12345" {
		t.Fatalf("runID=%q", runID)
	}
	if runURL == "" {
		t.Fatalf("expected runURL, got empty")
	}
}

func TestDiscover_TimesOut(t *testing.T) {
	c := &fakeClient{
		t:            t,
		findRunQueue: []findRunResp{{id: ""}, {id: ""}, {id: ""}},
	}
	g := newTestGate(c)
	if _, _, err := g.Discover("sha", "pull_request"); err == nil {
		t.Fatalf("expected timeout error")
	}
}

func TestDiscover_URLFallback(t *testing.T) {
	c := &fakeClient{
		t:            t,
		findRunQueue: []findRunResp{{id: "12345"}},
		runURLErr:    errors.New("api error"),
	}
	g := newTestGate(c)
	runID, runURL, err := g.Discover("sha", "pull_request")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runID != "12345" {
		t.Fatalf("runID=%q", runID)
	}
	if runURL != "" {
		t.Fatalf("expected empty URL on error fallback")
	}
}

func TestWaitForCompletion_AcceptsImmediateCompleted(t *testing.T) {
	c := &fakeClient{
		t: t,
		statusByAttempt: map[int][]statusResp{
			2: statusList("completed"),
		},
	}
	g := newTestGate(c)
	if err := g.WaitForCompletion("r", 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForCompletion_PollsUntilCompleted(t *testing.T) {
	c := &fakeClient{
		t: t,
		statusByAttempt: map[int][]statusResp{
			1: statusList("queued", "in_progress", "in_progress", "completed"),
		},
	}
	g := newTestGate(c)
	if err := g.WaitForCompletion("r", 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForCompletion_BailsAfterConsecutiveErrors(t *testing.T) {
	apiErr := errors.New("api down")
	c := &fakeClient{
		t: t,
		statusByAttempt: map[int][]statusResp{
			1: {
				{err: apiErr}, {err: apiErr}, {err: apiErr},
				{err: apiErr}, {err: apiErr},
			},
		},
	}
	g := newTestGate(c)
	err := g.WaitForCompletion("r", 1)
	if err == nil {
		t.Fatalf("expected bail-out error")
	}
	if !errors.Is(err, apiErr) {
		t.Fatalf("expected wrapped api error, got %v", err)
	}
}

func TestWaitForCompletion_RecoversFromTransientErrors(t *testing.T) {
	apiErr := errors.New("flake")
	c := &fakeClient{
		t: t,
		statusByAttempt: map[int][]statusResp{
			1: {
				{err: apiErr},
				{err: apiErr},
				{status: "in_progress"},
				{status: "completed"},
			},
		},
	}
	g := newTestGate(c)
	if err := g.WaitForCompletion("r", 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRerunWithBackoff_SucceedsAfterTransientFailure(t *testing.T) {
	c := &fakeClient{
		t:          t,
		rerunQueue: []error{errors.New("422"), errors.New("422"), nil},
	}
	g := newTestGate(c)
	if err := g.RerunWithBackoff("r"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.rerunCalls != 3 {
		t.Fatalf("expected 3 rerun calls, got %d", c.rerunCalls)
	}
}

func TestRerunWithBackoff_ExhaustsRetries(t *testing.T) {
	c := &fakeClient{
		t: t,
		rerunQueue: []error{
			errors.New("422"), errors.New("422"), errors.New("422"),
		},
	}
	g := newTestGate(c)
	if err := g.RerunWithBackoff("r"); err == nil {
		t.Fatalf("expected error after exhausting retries")
	}
}

func TestWaitForAttemptRegistered_AdvancesEventually(t *testing.T) {
	c := &fakeClient{t: t, attemptsQueue: []int{1, 1, 2}}
	g := newTestGate(c)
	if err := g.WaitForAttemptRegistered("r", 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForAttemptRegistered_TimesOut(t *testing.T) {
	c := &fakeClient{t: t, attemptsQueue: []int{1, 1, 1, 1, 1}}
	g := newTestGate(c)
	if err := g.WaitForAttemptRegistered("r", 2); err == nil {
		t.Fatalf("expected timeout error")
	}
}

func TestRun_AttemptOneSuccess(t *testing.T) {
	c := &fakeClient{
		t:                   t,
		findRunQueue:        []findRunResp{{id: "100"}},
		runURL:              "url",
		attemptsQueue:       []int{1},
		statusByAttempt:     map[int][]statusResp{1: statusList("completed")},
		conclusionByAttempt: map[int]string{1: "success"},
	}
	g := newTestGate(c)
	if err := g.Run("pull_request", "sha", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.rerunCalls != 0 {
		t.Fatalf("expected no rerun, got %d", c.rerunCalls)
	}
}

func TestRun_RetriesOnceAndPasses(t *testing.T) {
	c := &fakeClient{
		t:             t,
		findRunQueue:  []findRunResp{{id: "100"}},
		runURL:        "url",
		attemptsQueue: []int{1, 2},
		statusByAttempt: map[int][]statusResp{
			1: statusList("completed"),
			2: statusList("completed"),
		},
		conclusionByAttempt: map[int]string{1: "failure", 2: "success"},
	}
	g := newTestGate(c)
	if err := g.Run("pull_request", "sha", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.rerunCalls != 1 {
		t.Fatalf("expected 1 rerun, got %d", c.rerunCalls)
	}
}

func TestRun_RetriesOnceAndFails(t *testing.T) {
	c := &fakeClient{
		t:             t,
		findRunQueue:  []findRunResp{{id: "100"}},
		runURL:        "url",
		attemptsQueue: []int{1, 2},
		statusByAttempt: map[int][]statusResp{
			1: statusList("completed"),
			2: statusList("completed"),
		},
		conclusionByAttempt: map[int]string{1: "failure", 2: "failure"},
	}
	g := newTestGate(c)
	if err := g.Run("pull_request", "sha", ""); err == nil {
		t.Fatalf("expected gate failure")
	}
	if c.rerunCalls != 1 {
		t.Fatalf("expected exactly 1 rerun, got %d", c.rerunCalls)
	}
}

func TestRun_CancelledIsNotRetryable(t *testing.T) {
	c := &fakeClient{
		t:                   t,
		findRunQueue:        []findRunResp{{id: "100"}},
		runURL:              "url",
		attemptsQueue:       []int{1},
		statusByAttempt:     map[int][]statusResp{1: statusList("completed")},
		conclusionByAttempt: map[int]string{1: "cancelled"},
	}
	g := newTestGate(c)
	err := g.Run("pull_request", "sha", "")
	if err == nil {
		t.Fatalf("expected error for cancelled conclusion")
	}
	if !errors.Is(err, ErrNotRetryable) {
		t.Fatalf("expected ErrNotRetryable, got %v", err)
	}
	if c.rerunCalls != 0 {
		t.Fatalf("expected no rerun for cancelled, got %d", c.rerunCalls)
	}
}

func TestRun_HumanRerunStartingAtAttempt2(t *testing.T) {
	c := &fakeClient{
		t:             t,
		findRunQueue:  []findRunResp{{id: "100"}},
		runURL:        "url",
		attemptsQueue: []int{2, 3},
		statusByAttempt: map[int][]statusResp{
			2: statusList("completed"),
			3: statusList("completed"),
		},
		conclusionByAttempt: map[int]string{2: "failure", 3: "success"},
	}
	g := newTestGate(c)
	if err := g.Run("pull_request", "sha", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.rerunCalls != 1 {
		t.Fatalf("expected 1 rerun, got %d", c.rerunCalls)
	}
}

func TestRun_DiscoveryFailurePropagates(t *testing.T) {
	c := &fakeClient{
		t:            t,
		findRunQueue: []findRunResp{{id: ""}, {id: ""}, {id: ""}},
	}
	g := newTestGate(c)
	if err := g.Run("pull_request", "sha", ""); err == nil {
		t.Fatalf("expected discovery error")
	}
}

func TestRun_MergeGroupUsesHeadSHA(t *testing.T) {
	c := &fakeClient{
		t:                   t,
		findRunQueue:        []findRunResp{{id: "200"}},
		runURL:              "url",
		attemptsQueue:       []int{1},
		statusByAttempt:     map[int][]statusResp{1: statusList("completed")},
		conclusionByAttempt: map[int]string{1: "success"},
	}
	g := newTestGate(c)
	if err := g.Run("merge_group", "", "head_sha"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_FastAttempt2_NoSeenRunningGuard(t *testing.T) {
	// Attempt 2 finishes so quickly that the first poll already shows
	// `completed` — must NOT hang waiting for a phantom "in_progress"
	// observation. Regression guard for the seenRunning guard that was
	// removed because registration upstream already guarantees freshness.
	c := &fakeClient{
		t:             t,
		findRunQueue:  []findRunResp{{id: "100"}},
		runURL:        "url",
		attemptsQueue: []int{1, 2},
		statusByAttempt: map[int][]statusResp{
			1: statusList("completed"),
			2: statusList("completed"),
		},
		conclusionByAttempt: map[int]string{1: "failure", 2: "success"},
	}
	g := newTestGate(c)
	if err := g.Run("pull_request", "sha", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
