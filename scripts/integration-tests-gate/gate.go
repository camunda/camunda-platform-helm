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

	PollInterval         time.Duration
	MaxConsecutiveErrors int

	RegistrationTries    int
	RegistrationInterval time.Duration

	RunAttemptTries   int
	RunAttemptBackoff time.Duration

	RerunTries   int
	RerunBackoff time.Duration

	Sleep func(time.Duration)
	// Logf prints human-readable progress to stderr.
	Logf func(format string, args ...any)
	// Cmdf prints GitHub Actions workflow commands (::group::,
	// ::endgroup::, ::warning::) to stdout.
	Cmdf func(format string, args ...any)
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

// ResolveDispatchOverride remaps the event and SHAs when the gate is
// triggered manually via workflow_dispatch with an OVERRIDE_SHA. If
// overrideSHA is empty the inputs pass through unchanged.
func ResolveDispatchOverride(
	event, prHead, mgHead, overrideSHA, overrideEvent string,
) (string, string, string) {
	if overrideSHA == "" {
		return event, prHead, mgHead
	}
	resolved := overrideEvent
	if resolved == "" {
		resolved = "pull_request"
	}
	return resolved, overrideSHA, overrideSHA
}

func (g *Gate) Discover(sha, event string) (runID, runURL string, err error) {
	for i := 0; i < g.DiscoveryTries; i++ {
		runID, err = g.Client.FindRun(g.Workflow, sha, event)
		if err == nil && runID != "" {
			break
		}
		g.Logf("matrix run not yet visible (%d/%d)", i+1, g.DiscoveryTries)
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

func (g *Gate) RunAttemptWithRetry(runID string) (int, error) {
	var last error
	for i := 0; i < g.RunAttemptTries; i++ {
		n, err := g.Client.RunAttempt(runID)
		if err == nil && n >= 1 {
			return n, nil
		}
		last = err
		g.Logf("run_attempt read failed (%d/%d): %v",
			i+1, g.RunAttemptTries, err)
		g.Sleep(g.RunAttemptBackoff)
	}
	if last == nil {
		last = errors.New("run_attempt < 1")
	}
	return 0, last
}

func (g *Gate) WaitForCompletion(runID string, attempt int) error {
	consecutiveErrors := 0
	for {
		status, err := g.Client.AttemptStatus(runID, attempt)
		if err != nil {
			consecutiveErrors++
			g.Logf("attempt %d status read error (%d/%d): %v",
				attempt, consecutiveErrors, g.MaxConsecutiveErrors, err)
			if consecutiveErrors >= g.MaxConsecutiveErrors {
				return fmt.Errorf("attempt %d: %d consecutive status errors: %w",
					attempt, consecutiveErrors, err)
			}
			g.Sleep(g.PollInterval)
			continue
		}
		consecutiveErrors = 0
		if status == "completed" {
			return nil
		}
		g.Logf("attempt %d status=%q", attempt, status)
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
			g.Logf("rerun --failed try %d/%d failed: %v",
				i+1, g.RerunTries, err)
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

var ErrNotRetryable = errors.New("not retryable")

func (g *Gate) Run(event, prHeadSHA, mgHeadSHA string) error {
	sha, err := ResolveSHA(event, prHeadSHA, mgHeadSHA)
	if err != nil {
		return err
	}
	g.Logf("gating event=%s sha=%s", event, sha)

	g.Cmdf("::group::discover")
	runID, runURL, err := g.Discover(sha, event)
	g.Cmdf("::endgroup::")
	if err != nil {
		return err
	}
	if runURL == "" {
		runURL = "(url unknown)"
	}
	g.Logf("matrix run %s: %s", runID, runURL)

	current, err := g.RunAttemptWithRetry(runID)
	if err != nil {
		return fmt.Errorf("could not read run_attempt for %s: %w", runID, err)
	}

	g.Cmdf("::group::watch attempt %d", current)
	watchErr := g.watchAndDecide(runID, runURL, current)
	g.Cmdf("::endgroup::")
	if watchErr == nil {
		return nil
	}
	if !errors.Is(watchErr, errNeedsRetry) {
		return watchErr
	}

	g.Logf("triggering retry of failed jobs on %s", runURL)
	g.Cmdf("::group::rerun --failed")
	err = g.RerunWithBackoff(runID)
	g.Cmdf("::endgroup::")
	if err != nil {
		return err
	}

	next := current + 1
	g.Cmdf("::group::await attempt %d registration", next)
	err = g.WaitForAttemptRegistered(runID, next)
	g.Cmdf("::endgroup::")
	if err != nil {
		return err
	}

	g.Logf("watching attempt %d: %s/attempts/%d", next, runURL, next)
	g.Cmdf("::group::watch attempt %d", next)
	err = g.WaitForCompletion(runID, next)
	g.Cmdf("::endgroup::")
	if err != nil {
		return err
	}
	final, err := g.Client.AttemptConclusion(runID, next)
	if err != nil {
		return err
	}
	g.Logf("attempt %d conclusion: %s", next, final)
	if final == "success" {
		return nil
	}
	if final == "failure" {
		g.Cmdf("::warning::attempt %d still failure after retry; " +
			"jobs with conclusion=cancelled are not rerun by --failed " +
			"and may need manual intervention")
	}
	return fmt.Errorf("attempt %d conclusion: %s", next, final)
}

var errNeedsRetry = errors.New("retry needed")

func (g *Gate) watchAndDecide(runID, runURL string, attempt int) error {
	g.Logf("watching attempt %d: %s", attempt, runURL)
	if err := g.WaitForCompletion(runID, attempt); err != nil {
		return err
	}
	conclusion, err := g.Client.AttemptConclusion(runID, attempt)
	if err != nil {
		return err
	}
	g.Logf("attempt %d conclusion: %s", attempt, conclusion)
	switch conclusion {
	case "success":
		return nil
	case "failure":
		return errNeedsRetry
	default:
		return fmt.Errorf("%w: %s", ErrNotRetryable, conclusion)
	}
}
