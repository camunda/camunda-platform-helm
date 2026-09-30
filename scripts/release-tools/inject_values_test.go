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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildOverridesClassic_ZeebeLinksToGateway(t *testing.T) {
	t.Setenv("ZEEBE_IMAGE_TAG", "8.7.99")
	t.Setenv("ZEEBE_GATEWAY_IMAGE_TAG", "")

	o := buildOverridesClassic()

	if o.Zeebe == nil {
		t.Fatal("expected Zeebe override to be set")
	}
	if o.ZeebeGateway == nil {
		t.Fatal("expected ZeebeGateway override to be set when ZEEBE_IMAGE_TAG is provided")
	}
	if o.ZeebeGateway.Image.Tag != "8.7.99" {
		t.Errorf("expected ZeebeGateway tag 8.7.99, got %s", o.ZeebeGateway.Image.Tag)
	}
}

func TestBuildOverridesClassic_ExplicitGatewayOverridesLink(t *testing.T) {
	t.Setenv("ZEEBE_IMAGE_TAG", "8.7.99")
	t.Setenv("ZEEBE_GATEWAY_IMAGE_TAG", "8.7.50")

	o := buildOverridesClassic()

	if o.ZeebeGateway == nil {
		t.Fatal("expected ZeebeGateway override to be set")
	}
	if o.ZeebeGateway.Image.Tag != "8.7.50" {
		t.Errorf("expected explicit gateway tag 8.7.50, got %s", o.ZeebeGateway.Image.Tag)
	}
}

func TestBuildOverridesClassic_NoZeebeNoGateway(t *testing.T) {
	t.Setenv("ZEEBE_IMAGE_TAG", "")
	t.Setenv("ZEEBE_GATEWAY_IMAGE_TAG", "")

	o := buildOverridesClassic()

	if o.Zeebe != nil {
		t.Error("expected Zeebe override to be nil")
	}
	if o.ZeebeGateway != nil {
		t.Error("expected ZeebeGateway override to be nil")
	}
}

func TestRunInjectValuesUpdatesValuesLatest(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "charts", "camunda-platform-8.10")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		t.Fatal(err)
	}
	values := "orchestration:\n  image:\n    tag: 8.10.0-alpha5\nconnectors:\n  image:\n    tag: 8.10.0-alpha5\n"
	latest := "orchestration:\n  image:\n    repository: camunda/camunda\n    tag: SNAPSHOT\n"
	for name, body := range map[string]string{"values.yaml": values, "values-latest.yaml": latest} {
		if err := os.WriteFile(filepath.Join(chartDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	t.Setenv("CHART_VERSION", "8.10")
	t.Setenv("ORCHESTRATION_IMAGE_TAG", "8.10.0")
	t.Setenv("CONNECTORS_IMAGE_TAG", "8.10.1")
	t.Setenv("CONSOLE_IMAGE_TAG", "not included")

	if err := runInjectValues(nil); err != nil {
		t.Fatalf("runInjectValues: %v", err)
	}
	gotValues, _ := os.ReadFile(filepath.Join(chartDir, "values.yaml"))
	if !strings.Contains(string(gotValues), "tag: 8.10.0\n") || !strings.Contains(string(gotValues), "tag: 8.10.1\n") {
		t.Errorf("values.yaml not updated:\n%s", gotValues)
	}
	gotLatest, _ := os.ReadFile(filepath.Join(chartDir, "values-latest.yaml"))
	if !strings.Contains(string(gotLatest), "tag: 8.10.0\n") || strings.Contains(string(gotLatest), "SNAPSHOT") {
		t.Errorf("values-latest.yaml not updated:\n%s", gotLatest)
	}
}

func TestRunImageOverridesRejectsGATagForAlphaMinor(t *testing.T) {
	dir := t.TempDir()
	cv := filepath.Join(dir, "chart-versions.yaml")
	body := "chartAutomation: {routineVersions: [\"8.10\", \"8.9\"]}\ncamundaSupportLifecycle:\n  \"8.10\": {}\n  \"8.9\": {released: \"2026-04-14\"}\n"
	if err := os.WriteFile(cv, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_ENV", filepath.Join(dir, "env"))
	out := filepath.Join(dir, "overrides.yaml")

	err := runImageOverrides([]string{"--chart-version", "8.10", "--chart-versions-file", cv, "--orchestration", "8.10.0", "--out", out})
	if err == nil || !strings.Contains(err.Error(), "no released date for 8.10") {
		t.Errorf("GA tag for an alpha minor must fail, got %v", err)
	}
	if err := runImageOverrides([]string{"--chart-version", "8.10", "--chart-versions-file", cv, "--orchestration", "8.10.0-alpha6", "--out", out}); err != nil {
		t.Errorf("alpha tag for an alpha minor: %v", err)
	}
	if err := runImageOverrides([]string{"--chart-version", "8.9", "--chart-versions-file", cv, "--orchestration", "8.9.23", "--out", out}); err != nil {
		t.Errorf("GA tag for a released minor: %v", err)
	}
}
