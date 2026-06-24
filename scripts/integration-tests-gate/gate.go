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
	"fmt"
	"time"
)

type ghClient interface {
	FindRun(workflow, sha, event string) (string, error)
	RunURL(runID string) (string, error)
	RunAttempt(runID string) (int, error)
	AttemptStatus(runID string, attempt int) (string, error)
	AttemptConclusion(runID string, attempt int) (string, error)
	RerunFailed(runID string) error
}

type Gate struct {
	Client   ghClient
	Workflow string

	DiscoveryTries    int
	DiscoveryInterval time.Duration

	PollInterval time.Duration

	RegistrationTries    int
	RegistrationInterval time.Duration

	RerunTries   int
	RerunBackoff time.Duration

	Sleep func(time.Duration)
	Log   func(format string, args ...any)
}

func ResolveSHA(event, prHeadSHA, mgHeadSHA string) (string, error) {
	switch event {
	case "pull_request", "pull_request_target":
		if prHeadSHA == "" {
			return "", fmt.Errorf("missing pull_request head sha")
		}
		return prHeadSHA, nil
	case "merge_group":
		if mgHeadSHA == "" {
			return "", fmt.Errorf("missing merge_group head sha")
		}
		return mgHeadSHA, nil
	default:
		return "", fmt.Errorf("unsupported event: %s", event)
	}
}

func (g *Gate) Discover(sha, event string) (runID, runURL string, err error) {
	for i := 0; i < g.DiscoveryTries; i++ {
		runID, err = g.Client.FindRun(g.Workflow, sha, event)
		if err == nil && runID != "" {
			break
		}
		g.Log("matrix run not yet visible (%d/%d)", i+1, g.DiscoveryTries)
		g.Sleep(g.DiscoveryInterval)
	}
	if runID == "" {
		return "", "", fmt.Errorf("no matrix run for %s @ %s", g.Workflow, sha)
	}
	runURL, urlErr := g.Client.RunURL(runID)
	if urlErr != nil || runURL == "" {
		runURL = ""
	}
	return runID, runURL, nil
}

// WaitForCompletion polls until the attempt's status is "completed".
// It requires at least one observation of a non-completed status before
// believing a "completed" reading, to guard against the API returning stale
// top-level state for an attempt that has not actually started yet.
func (g *Gate) WaitForCompletion(runID string, attempt int) error {
	seenRunning := false
	for {
		status, err := g.Client.AttemptStatus(runID, attempt)
		if err != nil {
			g.Log("attempt %d status read error: %v", attempt, err)
			g.Sleep(g.PollInterval)
			continue
		}
		switch status {
		case "completed":
			if seenRunning {
				return nil
			}
			g.Log("attempt %d completed-but-unconfirmed; awaiting start", attempt)
		case "queued", "in_progress", "waiting", "requested", "pending":
			seenRunning = true
			g.Log("attempt %d status=%s", attempt, status)
		case "":
			g.Log("attempt %d not visible yet", attempt)
		default:
			g.Log("attempt %d status=%s", attempt, status)
		}
		g.Sleep(g.PollInterval)
	}
}

func (g *Gate) RerunWithBackoff(runID string) error {
	var last error
	for i := 0; i < g.RerunTries; i++ {
		if err := g.Client.RerunFailed(runID); err == nil {
			return nil
		} else {
			last = err
			g.Log("rerun --failed try %d/%d failed: %v", i+1, g.RerunTries, err)
			g.Sleep(g.RerunBackoff)
		}
	}
	if last == nil {
		last = errors.New("rerun --failed exhausted retries")
	}
	return last
}

func (g *Gate) WaitForAttemptRegistered(runID string, want int) error {
	for i := 0; i < g.RegistrationTries; i++ {
		got, err := g.Client.RunAttempt(runID)
		if err == nil && got >= want {
			return nil
		}
		g.Sleep(g.RegistrationInterval)
	}
	return fmt.Errorf("attempt %d was not registered", want)
}

// ErrNotRetryable is returned when an attempt finished in a state that
// `gh run rerun --failed` cannot recover (cancelled / timed_out / etc).
var ErrNotRetryable = errors.New("not retryable")

func (g *Gate) Run(event, prHeadSHA, mgHeadSHA string) error {
	sha, err := ResolveSHA(event, prHeadSHA, mgHeadSHA)
	if err != nil {
		return err
	}
	g.Log("gating event=%s sha=%s", event, sha)

	runID, runURL, err := g.Discover(sha, event)
	if err != nil {
		return err
	}
	if runURL == "" {
		runURL = "(url unknown)"
	}
	g.Log("matrix run %s: %s", runID, runURL)

	current, err := g.Client.RunAttempt(runID)
	if err != nil || current < 1 {
		return fmt.Errorf("could not read run_attempt for %s: %v", runID, err)
	}

	if err := g.watchAndDecide(runID, runURL, current); err != nil {
		if !errors.Is(err, errNeedsRetry) {
			return err
		}
	} else {
		return nil
	}

	g.Log("triggering retry of failed jobs on %s", runURL)
	if err := g.RerunWithBackoff(runID); err != nil {
		return err
	}

	next := current + 1
	if err := g.WaitForAttemptRegistered(runID, next); err != nil {
		return err
	}

	g.Log("watching attempt %d: %s/attempts/%d", next, runURL, next)
	if err := g.WaitForCompletion(runID, next); err != nil {
		return err
	}
	final, err := g.Client.AttemptConclusion(runID, next)
	if err != nil {
		return err
	}
	g.Log("attempt %d conclusion: %s", next, final)
	if final == "success" {
		return nil
	}
	return fmt.Errorf("attempt %d conclusion: %s", next, final)
}

var errNeedsRetry = errors.New("retry needed")

func (g *Gate) watchAndDecide(runID, runURL string, attempt int) error {
	g.Log("watching attempt %d: %s", attempt, runURL)
	if err := g.WaitForCompletion(runID, attempt); err != nil {
		return err
	}
	conclusion, err := g.Client.AttemptConclusion(runID, attempt)
	if err != nil {
		return err
	}
	g.Log("attempt %d conclusion: %s", attempt, conclusion)
	switch conclusion {
	case "success":
		return nil
	case "failure":
		return errNeedsRetry
	default:
		return fmt.Errorf("%w: %s", ErrNotRetryable, conclusion)
	}
}
