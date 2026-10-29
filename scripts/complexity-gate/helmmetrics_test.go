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
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelmMeasurements(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, text, unit string
		branches, depth  int
	}{
		{"root", `{{if .x}}x{{else}}y{{end}}`, "root:test.tpl", 1, 1},
		{"trimmed", `{{- if .x -}}x{{- end -}} {{- range .xs -}}x{{- end -}}`, "root:test.tpl", 2, 1},
		{"nested", `{{if .x}}{{range .xs}}{{with .x}}x{{end}}{{end}}{{end}}`, "root:test.tpl", 3, 3},
		{"alternatives", `{{if .x}}x{{else if .y}}y{{else if .z}}z{{end}}`, "root:test.tpl", 3, 1},
		{"with alternatives", `{{with .x}}x{{else with .y}}y{{end}}`, "root:test.tpl", 2, 1},
		{"nested alternative body", `{{if .x}}x{{else if .y}}{{if .z}}z{{end}}{{end}}`, "root:test.tpl", 3, 2},
		{"explicit alternative", "{{if .x}}x{{else}}\n {{if .y}}y{{end}}\n{{end}}", "root:test.tpl", 2, 1},
		{"else body with text", `{{if .x}}x{{else}}fallback{{if .y}}y{{end}}{{end}}`, "root:test.tpl", 2, 2},
		{"else body with action", `{{if .x}}x{{else}}{{printf "fallback"}}{{if .y}}y{{end}}{{end}}`, "root:test.tpl", 2, 2},
		{"range empty branch", `{{range .xs}}x{{else}}{{range .ys}}y{{end}}{{end}}`, "root:test.tpl", 2, 2},
		{"comments and strings", "{{- /* {{if .ignored}} */ -}}{{printf \"{{if .ignored}}\"}}{{printf `{{range .ignored}}`}}", "root:test.tpl", 0, 0},
		{"definition", `{{define "x"}}{{if .x}}{{include "unknown" .}}{{end}}{{end}}`, "helper:x", 1, 1},
		{"definition is not root", `{{define "x"}}{{if .x}}x{{end}}{{end}}{{include "x" .}}`, "root:test.tpl", 0, 0},
		{"block", `{{if .x}}{{block "b" .}}{{range .xs}}{{with .x}}x{{end}}{{end}}{{end}}{{end}}`, "helper:b", 2, 2},
		{"block is independent", `{{if .x}}{{block "b" .}}{{range .xs}}x{{end}}{{end}}{{end}}`, "root:test.tpl", 1, 1},
		{"unindented", "{{if .x}}\n{{range .xs}}\nx\n{{end}}\n{{end}}", "root:test.tpl", 2, 2},
		{"yaml indentation", "data:\n  value: |\n    {{if .x}}\n      {{range .xs}}x{{end}}\n    {{end}}", "root:test.tpl", 2, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := make(map[string]helmMeasurement)
			require.NoError(t, addHelmMeasurements(got, "test.tpl", test.text))
			require.Contains(t, got, test.unit)
			assert.Equal(t, test.branches, got[test.unit].branches)
			assert.Equal(t, test.depth, got[test.unit].depth)
			assert.Positive(t, got[test.unit].line)
		})
	}
}

func TestHelmParseErrors(t *testing.T) {
	t.Parallel()
	got := make(map[string]helmMeasurement)
	require.ErrorContains(t, addHelmMeasurements(got, "bad.tpl", "{{if .x}}"), "bad.tpl")
	require.NoError(t, addHelmMeasurements(got, "first.tpl", `{{define "shared"}}x{{end}}`))
	require.ErrorContains(t, addHelmMeasurements(got, "second.tpl", `{{define "shared"}}y{{end}}`), "ambiguous template")
}

func TestHelmReportWithoutGit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	chart := "charts/camunda-platform-test"
	writeFile(t, root, chart+"/Chart.yaml", "apiVersion: v2\n")
	writeFile(t, root, chart+"/templates/_helpers.tpl", `{{define "nested"}}{{if .x}}{{range .xs}}x{{end}}{{end}}{{end}}`)
	writeFile(t, root, chart+"/templates/resource.yml", `{{if .x}}kind: ConfigMap{{end}}`)
	writeFile(t, root, chart+"/templates/NOTES.txt", `{{with .x}}x{{end}}`)
	writeFile(t, root, chart+"/charts/dependency/templates/ignored.yaml", "{{invalid")
	var out bytes.Buffer
	require.NoError(t, reportHelmCharts(root, []string{chart}, &out))
	assert.Contains(t, out.String(), "nested branches=2 depth=2")
	assert.Contains(t, out.String(), "resource.yml:1: <root> branches=1 depth=1")
	assert.Contains(t, out.String(), "NOTES.txt:1: <root> branches=1 depth=1")
	assert.NotContains(t, out.String(), "ignored.yaml")
}

func TestHelmReportRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	root := t.TempDir()
	require.ErrorContains(t, reportHelmCharts(root, nil, &out), "at least one chart")
	require.ErrorContains(t, reportHelmCharts(root, []string{"../outside"}, &out), "outside the repository")
	require.ErrorContains(t, reportHelmCharts(root, []string{"charts/missing"}, &out), "Chart.yaml")
	valid := "charts/camunda-platform-test"
	unsupported := "charts/dependency"
	writeFile(t, root, valid+"/Chart.yaml", "apiVersion: v2\n")
	writeFile(t, root, valid+"/templates/configmap.yaml", "kind: ConfigMap\n")
	writeFile(t, root, unsupported+"/Chart.yaml", "apiVersion: v2\n")
	for _, charts := range [][]string{{unsupported}, {valid, unsupported}, {unsupported, valid}} {
		require.ErrorContains(t, reportHelmCharts(root, charts, &out), "unsupported chart path")
	}
	writeFile(t, root, "charts/camunda-platform-empty/Chart.yaml", "apiVersion: v2\n")
	require.ErrorContains(t, reportHelmCharts(root, []string{"charts/camunda-platform-empty"}, &out), "candidate")
	assert.Empty(t, out.String())
}
