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
	"strings"
	"testing"
)

func TestBuildUserMessage_IncludesContextAndSections(t *testing.T) {
	ctx := Context{
		ChartVersion: "8.10",
		Scenario:     "elasticsearch",
		Flow:         "install",
		Platform:     "gke",
		Identity:     "keycloak",
		Persistence:  "elasticsearch",
		WorkflowURL:  "https://example.com/run/1",
	}
	d := &Diagnostics{
		Namespace:      "ns-bar",
		Pods:           "zeebe-0 0/1 CrashLoopBackOff",
		Events:         "Warning Failed image pull",
		TestOutputTail: "venom: scenario failed",
		PodLogs:        []PodLog{{Pod: "zeebe-0", Body: "panic: boom"}},
		CollectErrors:  []string{"warn: foo"},
	}

	out := BuildUserMessage(ctx, d)

	for _, want := range []string{
		"Chart version: 8.10",
		"Scenario: elasticsearch",
		"Flow: install",
		"Platform: gke",
		"Identity: keycloak",
		"Persistence: elasticsearch",
		"Run: https://example.com/run/1",
		"Namespace: ns-bar",
		"## kubectl get pods",
		"CrashLoopBackOff",
		"## Recent events",
		"Failed image pull",
		"## Test output (tail)",
		"venom: scenario failed",
		"## Pod logs",
		"### zeebe-0",
		"panic: boom",
		"## Collection errors",
		"warn: foo",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in user message:\n%s", want, out)
		}
	}
}

func TestBuildUserMessage_FillsUnknownContext(t *testing.T) {
	out := BuildUserMessage(Context{}, &Diagnostics{Namespace: ""})
	for _, want := range []string{
		"Chart version: unknown",
		"Scenario: unknown",
		"Flow: unknown",
		"Platform: unknown",
		"Namespace: unknown",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing fallback %q:\n%s", want, out)
		}
	}
}

func TestBuildUserMessage_OmitsEmptySections(t *testing.T) {
	out := BuildUserMessage(Context{ChartVersion: "8.8"}, &Diagnostics{Namespace: "ns"})
	for _, unwanted := range []string{
		"## kubectl get pods",
		"## Recent events",
		"## Test output (tail)",
		"## Pod logs",
		"## Collection errors",
	} {
		if strings.Contains(out, unwanted) {
			t.Errorf("unexpected section %q in:\n%s", unwanted, out)
		}
	}
}

func TestSystemPrompt_MentionsKeyConventions(t *testing.T) {
	for _, want := range []string{
		"orchestration",
		"global.elasticsearch.external",
		"Bitnami",
		"Upgrade flows",
	} {
		if !strings.Contains(systemPrompt, want) {
			t.Errorf("system prompt missing key marker %q", want)
		}
	}
}
