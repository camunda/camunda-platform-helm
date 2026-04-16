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
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestWeekWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 15, 11, 30, 0, 0, time.UTC)

	start, end := weekWindow(now, 0)
	if got, want := start.Format("2006-01-02"), "2026-04-13"; got != want {
		t.Fatalf("weekWindow start mismatch: got %s, want %s", got, want)
	}
	if got, want := end.Format("2006-01-02"), "2026-04-19"; got != want {
		t.Fatalf("weekWindow end mismatch: got %s, want %s", got, want)
	}

	startPrev, endPrev := weekWindow(now, 1)
	if got, want := startPrev.Format("2006-01-02"), "2026-04-06"; got != want {
		t.Fatalf("weekWindow previous start mismatch: got %s, want %s", got, want)
	}
	if got, want := endPrev.Format("2006-01-02"), "2026-04-12"; got != want {
		t.Fatalf("weekWindow previous end mismatch: got %s, want %s", got, want)
	}
}

func TestBuildPrompt(t *testing.T) {
	t.Parallel()

	cfg := config{
		MedicHandle:     "@distro-medic",
		SupportChannels: "#ask-self-managed,#inc-*",
		AlertChannel:    "#team-distribution-alerts",
	}

	weekStart := time.Date(2026, time.April, 13, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, time.April, 19, 0, 0, 0, 0, time.UTC)

	prompt := buildPrompt(cfg, weekStart, weekEnd)

	mustContain := []string{
		"Distro - Medic Report Guidelines",
		"Report period: 2026-04-13 to 2026-04-19 (W16)",
		"Determine who was medic for this report period",
		"Slack @distro-medic user group membership/activity",
		"Current medic: <name>",
		"#ask-self-managed,#inc-*",
		"#team-distribution-alerts",
		"Respond ONLY with the Slack message content. No wrapping, no explanation.",
	}

	for _, expected := range mustContain {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt missing expected content: %q", expected)
		}
	}
}

func TestBuildFailureMessage(t *testing.T) {
	t.Parallel()

	weekStart := time.Date(2026, time.April, 13, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, time.April, 19, 0, 0, 0, 0, time.UTC)
	err := fmt.Errorf("glean timeout")

	msg := buildFailureMessage(weekStart, weekEnd, err)

	mustContain := []string{
		"Weekly distro-medic report generation failed",
		"Period: 2026-04-13 to 2026-04-19",
		"Error: `glean timeout`",
	}

	for _, expected := range mustContain {
		if !strings.Contains(msg, expected) {
			t.Fatalf("failure message missing expected content: %q", expected)
		}
	}
}
