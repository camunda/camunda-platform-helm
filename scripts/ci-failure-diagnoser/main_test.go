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

func TestAssembleReport_IncludesContextAndFooter(t *testing.T) {
	out := assembleReport(Context{
		ChartVersion: "8.10",
		Scenario:     "opensearch",
		Flow:         "upgrade-minor",
		Platform:     "eks",
	}, "### Likely cause\nfoo")

	for _, want := range []string{
		":robot_face: CI failure diagnosis",
		"`opensearch`",
		"`upgrade-minor`",
		"`eks`",
		"`8.10`",
		"### Likely cause",
		"machine-generated diagnosis",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in report:\n%s", want, out)
		}
	}
}

func TestAssembleReport_NoContextStillRenders(t *testing.T) {
	out := assembleReport(Context{}, "body")
	if !strings.Contains(out, "CI failure diagnosis") {
		t.Errorf("missing header: %q", out)
	}
	if !strings.Contains(out, "body") {
		t.Errorf("missing body: %q", out)
	}
}
