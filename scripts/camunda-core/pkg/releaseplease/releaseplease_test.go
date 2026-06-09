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

package releaseplease

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseTag(t *testing.T) {
	if got := ReleaseTag("8.7", "12.10.0"); got != "camunda-platform-8.7-12.10.0" {
		t.Errorf("ReleaseTag=%q want camunda-platform-8.7-12.10.0", got)
	}
	// Pre-release versions are tagged too (minor prefix retained).
	if got := ReleaseTag("8.10", "15.0.0-alpha2"); got != "camunda-platform-8.10-15.0.0-alpha2" {
		t.Errorf("ReleaseTag prerelease=%q want camunda-platform-8.10-15.0.0-alpha2", got)
	}
}

func TestComputePrereleaseBump(t *testing.T) {
	// alpha still listed → increment the prerelease number.
	r := Compute("8.10.0-alpha2", true, "", "charts/camunda-platform-8.10", "abc1234")
	if !r.IsPrerelease || !r.Computed {
		t.Fatalf("expected prerelease+computed, got %+v", r)
	}
	if r.ReleaseVersion != "8.10.0-alpha3" {
		t.Errorf("ReleaseVersion=%q want 8.10.0-alpha3", r.ReleaseVersion)
	}
	if r.DevTag != "8.10.0-alpha3-dev-abc1234" {
		t.Errorf("DevTag=%q", r.DevTag)
	}
	if r.ChartMajor != "8" {
		t.Errorf("ChartMajor=%q want 8", r.ChartMajor)
	}
}

func TestComputeAlphaToStable(t *testing.T) {
	// prerelease suffix present but no longer alpha → strip to stable.
	r := Compute("8.10.0-alpha5", false, "", "charts/camunda-platform-8.10", "deadbee")
	if r.IsPrerelease {
		t.Error("alpha→stable must not be prerelease")
	}
	if !r.Computed || r.ReleaseVersion != "8.10.0" {
		t.Errorf("ReleaseVersion=%q computed=%v want 8.10.0/true", r.ReleaseVersion, r.Computed)
	}
	if r.DevTag != "8.10.0-dev-deadbee" {
		t.Errorf("DevTag=%q", r.DevTag)
	}
}

func TestComputeStableFromTraceFirstLine(t *testing.T) {
	trace := strings.Join([]string{
		"some log preamble",
		"+  version: 8.8.5",
		"+  other: noise",
	}, "\n")
	r := Compute("8.8.4", false, trace, "charts/camunda-platform-8.8", "f00")
	if !r.Computed || r.ReleaseVersion != "8.8.5" {
		t.Errorf("ReleaseVersion=%q computed=%v want 8.8.5/true", r.ReleaseVersion, r.Computed)
	}
	if r.DevTag != "8.8.5-dev-f00" {
		t.Errorf("DevTag=%q", r.DevTag)
	}
}

func TestComputeStableFromTraceFallback(t *testing.T) {
	// No "+...version:" line → fall back to the manifest "<dir>": "<v>" entry.
	trace := `dry run output ... "charts/camunda-platform-8.9": "8.9.7" ...`
	r := Compute("8.9.6", false, trace, "charts/camunda-platform-8.9", "abc")
	if r.ReleaseVersion != "8.9.7" {
		t.Errorf("ReleaseVersion=%q want 8.9.7", r.ReleaseVersion)
	}
}

func TestComputeStableNoVersionFallsBackToCurrent(t *testing.T) {
	r := Compute("8.8.4", false, "nothing useful here", "charts/camunda-platform-8.8", "sha9")
	if r.Computed {
		t.Error("should not be computed when trace has no version")
	}
	if r.ReleaseVersion != "8.8.4" || r.DevTag != "8.8.4-dev-sha9" {
		t.Errorf("fallback to current failed: %+v", r)
	}
}

func TestScrapeTraceVersionGreedyStrip(t *testing.T) {
	// Greedy strip up to the last "version:" on the line.
	got := ScrapeTraceVersion("+ chore: release version: 9.9.9", "charts/x")
	if got != "9.9.9" {
		t.Errorf("ScrapeTraceVersion=%q want 9.9.9", got)
	}
}

func TestStillAlpha(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "chart-versions.yaml")
	if err := os.WriteFile(p, []byte("camundaVersions:\n  alpha:\n    - \"8.10\"\n  supportStandard:\n    - \"8.9\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	yes, err := StillAlpha(p, "8.10")
	if err != nil || !yes {
		t.Errorf("8.10 should be alpha: %v %v", yes, err)
	}
	no, err := StillAlpha(p, "8.9")
	if err != nil || no {
		t.Errorf("8.9 should not be alpha: %v %v", no, err)
	}
}
