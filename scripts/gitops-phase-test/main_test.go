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

package main

import (
	"testing"
	"time"
)

func TestFluxReference(t *testing.T) {
	tests := []struct {
		name     string
		revision string
		refName  string
		want     map[string]any
	}{
		{
			name:     "commit",
			revision: "0123456789abcdef",
			want:     map[string]any{"commit": "0123456789abcdef"},
		},
		{
			name:     "merge queue ref",
			revision: "0123456789abcdef",
			refName:  "refs/heads/gh-readonly-queue/main/pr-6788-example",
			want:     map[string]any{"name": "refs/heads/gh-readonly-queue/main/pr-6788-example"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := fluxReference(test.revision, test.refName)
			if len(got) != 1 {
				t.Fatalf("fluxReference() returned %v", got)
			}
			for key, wantValue := range test.want {
				if got[key] != wantValue {
					t.Fatalf("fluxReference() = %v, want %v", got, test.want)
				}
			}
		})
	}
}

func TestControllerConvergedForNonFlux(t *testing.T) {
	for _, controller := range []string{"helm", "argocd"} {
		if err := controllerConverged(config{controller: controller}); err != nil {
			t.Fatalf("controllerConverged(%q): %v", controller, err)
		}
	}
}

func TestHelmReleaseConverged(t *testing.T) {
	ready := helmRelease{}
	ready.Metadata.Generation = 4
	ready.Status.ObservedGeneration = 4
	ready.Status.Conditions = append(ready.Status.Conditions, struct {
		Type    string `json:"type"`
		Status  string `json:"status"`
		Reason  string `json:"reason"`
		Message string `json:"message"`
	}{Type: "Ready", Status: "True"})

	if err := helmReleaseConverged(ready); err != nil {
		t.Fatalf("ready HelmRelease: %v", err)
	}

	stale := ready
	stale.Status.ObservedGeneration = 3
	if err := helmReleaseConverged(stale); err == nil {
		t.Fatal("expected stale generation to fail")
	}

	notReady := ready
	notReady.Status.Conditions[0].Status = "False"
	if err := helmReleaseConverged(notReady); err == nil {
		t.Fatal("expected non-ready condition to fail")
	}
}

func TestPhaseTimeout(t *testing.T) {
	for controller, want := range map[string]time.Duration{
		"helm":   5 * time.Minute,
		"argocd": 5 * time.Minute,
		"flux":   10 * time.Minute,
	} {
		if got := phaseTimeout(controller); got != want {
			t.Fatalf("phaseTimeout(%q) = %s, want %s", controller, got, want)
		}
	}
}
