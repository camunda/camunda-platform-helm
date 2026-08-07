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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const removedKeyStderr = `Error: execution error at (camunda-platform/templates/common/constraints.tpl:1243:3):
[camunda][error] The Helm values file key "global.ingress.host" has been removed. For more details, please check Camunda Helm chart documentation. https://docs.camunda.io/docs/self-managed/deployment/helm/upgrade/

Use --debug flag to render out invalid YAML`

const renamedKeyStderr = `Error: execution error at (camunda-platform/templates/common/constraints.tpl:761:3):
[camunda][error] The Helm values file key changed from "identity.firstUser.existingSecret" to "identity.firstUser.secret.existingSecret". For more details, please check Camunda Helm chart documentation.`

func TestSignature(t *testing.T) {
	tests := []struct {
		name       string
		stderr     string
		wantTitle  string
		wantSource string
	}{
		{
			name:       "camunda error line preferred over helm error line",
			stderr:     removedKeyStderr,
			wantTitle:  `The Helm values file key "global.ingress.host" has been removed. For more details, please check Camunda Helm chart documentation. https://docs.camunda.io/docs/self-managed/deployment/helm/upgrade/`,
			wantSource: "camunda-platform/templates/common/constraints.tpl:1243:3",
		},
		{
			name:       "falls back to helm error when no camunda marker",
			stderr:     "Error: could not find chart\n",
			wantTitle:  "could not find chart",
			wantSource: "",
		},
		{
			name:      "empty stderr yields empty title",
			stderr:    "",
			wantTitle: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Signature(tt.stderr)
			assert.Equal(t, tt.wantTitle, got.Title)
			assert.Equal(t, tt.wantSource, got.Source)
		})
	}
}

func TestSignatureIsPathIndependent(t *testing.T) {
	a := Signature(removedKeyStderr)
	b := Signature(removedKeyStderr + "\n  in /home/runner/work/checkout/values.yaml")
	assert.Equal(t, a.Title, b.Title, "signature must not vary with local paths")
}

func TestParseKeyChange(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   KeyChange
		wantOK bool
	}{
		{
			name:   "removed",
			stderr: removedKeyStderr,
			want:   KeyChange{Old: "global.ingress.host", Kind: "removed"},
			wantOK: true,
		},
		{
			name:   "renamed",
			stderr: renamedKeyStderr,
			want:   KeyChange{Old: "identity.firstUser.existingSecret", New: "identity.firstUser.secret.existingSecret", Kind: "renamed"},
			wantOK: true,
		},
		{
			name:   "unrelated failure is not a key change",
			stderr: "Error: template: parse error at line 3",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseKeyChange(tt.stderr)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestClassify(t *testing.T) {
	ok := RenderResult{Succeeded: true}
	bad := RenderResult{Succeeded: false}

	tests := []struct {
		name string
		a, b RenderResult
		want Outcome
	}{
		{"both pass", ok, ok, OutcomeClean},
		{"delta fixes it", bad, ok, OutcomeRemediated},
		{"nothing fixes it", bad, bad, OutcomeUnremediated},
		{"delta breaks a working baseline", ok, bad, OutcomeStaleDelta},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Classify(tt.a, tt.b))
		})
	}
}

func TestReportExitCode(t *testing.T) {
	tests := []struct {
		name     string
		outcomes []Outcome
		want     int
	}{
		{"all clean", []Outcome{OutcomeClean, OutcomeClean}, 0},
		{"remediated does not fail CI", []Outcome{OutcomeClean, OutcomeRemediated}, 0},
		{"unremediated fails", []Outcome{OutcomeClean, OutcomeUnremediated}, 1},
		{"stale delta fails", []Outcome{OutcomeStaleDelta}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r Report
			for _, o := range tt.outcomes {
				r.Results = append(r.Results, PathResult{Outcome: o})
			}
			assert.Equal(t, tt.want, r.ExitCode())
		})
	}
}

// fakeRenderer fails with a scripted stderr per call, then succeeds.
type fakeRenderer struct {
	responses []string
	calls     int
}

func (f *fakeRenderer) Template(_ context.Context, _ string, _ []string) (string, string, error) {
	i := f.calls
	f.calls++
	if i < len(f.responses) {
		return "", f.responses[i], fmt.Errorf("exit status 1")
	}
	return "rendered: true", "", nil
}

func (f *fakeRenderer) DependencyBuild(context.Context, string) error { return nil }

func testTransition(t *testing.T, values string) Transition {
	t.Helper()
	p := filepath.Join(t.TempDir(), "baseline.yaml")
	require.NoError(t, os.WriteFile(p, []byte(values), 0o644))
	return Transition{
		From:           "8.9",
		To:             "8.10",
		Archetype:      Archetype{Name: "test"},
		BaselineLayers: []string{p},
	}
}

func TestDiscoverEnumeratesEveryGuard(t *testing.T) {
	tr := testTransition(t, "global:\n  ingress:\n    host: example.com\nidentityKeycloak:\n  enabled: true\n")

	f := &fakeRenderer{responses: []string{
		removedKeyStderr,
		`[camunda][error] The Helm values file key "identityKeycloak" has been removed.`,
	}}

	got, err := Discover(context.Background(), f, tr, t.TempDir(), t.TempDir())
	require.NoError(t, err)

	require.Len(t, got.Changes, 2, "one render alone would report only the first guard")
	assert.Equal(t, "global.ingress.host", got.Changes[0].Old)
	assert.Equal(t, "identityKeycloak", got.Changes[1].Old)
	assert.True(t, got.Final.Succeeded)
	assert.Empty(t, got.Residual)
	assert.False(t, got.Truncated)
}

func TestDiscoverStopsOnNonKeyFailure(t *testing.T) {
	tr := testTransition(t, "global:\n  ingress:\n    host: example.com\n")

	f := &fakeRenderer{responses: []string{
		removedKeyStderr,
		"Error: YAML parse error on camunda-platform/templates/x.yaml",
	}}

	got, err := Discover(context.Background(), f, tr, t.TempDir(), t.TempDir())
	require.NoError(t, err)

	assert.Len(t, got.Changes, 1)
	assert.False(t, got.Final.Succeeded)
	assert.Contains(t, got.Residual, "YAML parse error")
}

func TestDiscoverReportsGuardNamingAbsentKey(t *testing.T) {
	tr := testTransition(t, "unrelated: true\n")

	f := &fakeRenderer{responses: []string{removedKeyStderr, removedKeyStderr}}

	got, err := Discover(context.Background(), f, tr, t.TempDir(), t.TempDir())
	require.NoError(t, err)

	assert.Len(t, got.Changes, 1)
	assert.Contains(t, got.Residual, "absent from the merged values")
}

func TestFingerprintIsStableAndDistinct(t *testing.T) {
	a := Fingerprint("classic-bundled", "key X removed")
	assert.Equal(t, a, Fingerprint("classic-bundled", "key X removed"))
	assert.NotEqual(t, a, Fingerprint("other-path", "key X removed"))
	assert.NotEqual(t, a, Fingerprint("classic-bundled", "key Y removed"))
}

func TestSlug(t *testing.T) {
	tests := []struct{ in, want string }{
		{`The Helm values file key "global.ingress.host" has been removed.`, "the-helm-values-file-key-globalingresshost-has-been-removed"},
		{"", "unknown"},
		{"multiple   spaces", "multiple-spaces"},
		{"punctuation!@#$only", "punctuationonly"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, slug(tt.in))
	}
}

func TestSlugTruncatesLongTitles(t *testing.T) {
	got := slug(strings.Repeat("verylongword ", 20))
	assert.LessOrEqual(t, len(got), 60)
	assert.NotEmpty(t, got)
	assert.False(t, strings.HasSuffix(got, "-"), "truncation must not leave a trailing separator")
}

func TestFileHasContent(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.yaml")
	comment := filepath.Join(dir, "comment.yaml")
	real := filepath.Join(dir, "real.yaml")
	require.NoError(t, os.WriteFile(empty, []byte("\n  \n\t\n"), 0o644))
	require.NoError(t, os.WriteFile(comment, []byte("# just a comment\n"), 0o644))
	require.NoError(t, os.WriteFile(real, []byte("key: value\n"), 0o644))

	assert.False(t, fileHasContent(empty), "whitespace-only counts as no delta")
	assert.True(t, fileHasContent(comment), "comments count as content")
	assert.True(t, fileHasContent(real))
	assert.False(t, fileHasContent(filepath.Join(dir, "missing.yaml")))
}

func TestVersionSlug(t *testing.T) {
	assert.Equal(t, "880", versionSlug("8.8"))
	assert.Equal(t, "890", versionSlug("8.9"))
	assert.Equal(t, "8100", versionSlug("8.10"))
}

func writeGuide(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
}

func TestCheckDocsCoverage(t *testing.T) {
	root := t.TempDir()
	writeGuide(t, root, "docs/self-managed/upgrade/helm/890-to-8100.md",
		"Remove `identityKeycloak` before upgrading.\n")
	writeGuide(t, root, "versioned_docs/version-8.9/self-managed/upgrade/helm/890-to-8100.md",
		"Also remove `elasticsearch`.\n")
	writeGuide(t, root, "node_modules/junk/890-to-8100.md", "`shouldBeIgnored`\n")

	cov, err := CheckDocsCoverage(root, "8.9", "8.10",
		[]string{"identityKeycloak", "elasticsearch", "webModelerPostgresql", "shouldBeIgnored"})
	require.NoError(t, err)

	require.True(t, cov.Checked)
	assert.Len(t, cov.Guides, 2, "node_modules must be skipped")
	assert.True(t, cov.Documented["identityKeycloak"])
	assert.True(t, cov.Documented["elasticsearch"], "guides are unioned across trees")
	assert.False(t, cov.Documented["webModelerPostgresql"])
	assert.False(t, cov.Documented["shouldBeIgnored"], "excluded dirs must not count as coverage")
	assert.Equal(t, 2, cov.UndocumentedCount(
		[]string{"identityKeycloak", "elasticsearch", "webModelerPostgresql", "shouldBeIgnored"}))
}

func TestCheckDocsCoverageUncheckedCases(t *testing.T) {
	tests := []struct {
		name string
		root func(t *testing.T) string
	}{
		{"no docs root", func(*testing.T) string { return "" }},
		{"missing docs root", func(*testing.T) string { return "/nonexistent/docs" }},
		{"no guide for transition", func(t *testing.T) string { return t.TempDir() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cov, err := CheckDocsCoverage(tt.root(t), "8.9", "8.10", []string{"someKey"})
			require.NoError(t, err)
			assert.False(t, cov.Checked)
			assert.True(t, cov.IsDocumented("someKey"),
				"unchecked keys must not be reported as documentation gaps")
			assert.Equal(t, 0, cov.UndocumentedCount([]string{"someKey"}))
		})
	}
}

func TestDocMark(t *testing.T) {
	unchecked := DocsCoverage{}
	checked := DocsCoverage{Checked: true, Documented: map[string]bool{"a": true, "b": false}}

	assert.Equal(t, "—", docMark(unchecked, "a"))
	assert.Equal(t, "yes", docMark(checked, "a"))
	assert.Equal(t, "**NO**", docMark(checked, "b"))
}

func writeCoverage(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "test", "upgrade-paths")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "coverage.yaml"), []byte(body), 0o644))
	return root
}

func TestLoadCoverage(t *testing.T) {
	root := writeCoverage(t, `
categories:
  - id: a
    title: Covered thing
    status: covered
    stage: render
    detail: does the thing
  - id: b
    title: Missing thing
    status: uncovered
    stage: none
    detail: does not do the thing
`)
	c, err := LoadCoverage(root)
	require.NoError(t, err)
	require.Len(t, c.Categories, 2)
	assert.Equal(t, map[string]int{"covered": 1, "uncovered": 1}, c.Counts())

	gaps := c.Gaps()
	require.Len(t, gaps, 1)
	assert.Equal(t, "b", gaps[0].ID, "only non-covered categories are gaps")
}

func TestLoadCoverageMissingFileIsNotAnError(t *testing.T) {
	c, err := LoadCoverage(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, c.Categories)
}

func TestLoadCoverageRejectsBadManifests(t *testing.T) {
	tests := []struct {
		name, body, wantErr string
	}{
		{"unknown status", "categories:\n  - id: a\n    title: T\n    status: maybe\n", "unknown status"},
		{"duplicate id", "categories:\n  - id: a\n    title: T\n    status: covered\n  - id: a\n    title: U\n    status: covered\n", "duplicate id"},
		{"missing title", "categories:\n  - id: a\n    status: covered\n", "id and title are required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadCoverage(writeCoverage(t, tt.body))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestReportCoverageSectionNamesGaps(t *testing.T) {
	r := Report{
		From: "8.9", To: "8.10", Stage: "render",
		Results: []PathResult{{Path: "p", Outcome: OutcomeClean}},
		Coverage: Coverage{Categories: []CoverageCategory{
			{ID: "a", Title: "Checked", Status: StatusCovered},
			{ID: "b", Title: "Data survival", Status: StatusUncovered, Detail: "no seeding\n  at all"},
		}},
	}
	md := r.Markdown()

	assert.Contains(t, md, "## Coverage")
	assert.Contains(t, md, "Data survival", "gaps are named")
	assert.NotContains(t, md, "| Checked |", "covered categories are not listed as gaps")
	assert.Contains(t, md, "no seeding at all", "multi-line detail is flattened for the table")
	assert.Contains(t, md, "for the categories this harness checks",
		"an all-clean run is qualified rather than absolute")
}

func TestReportWithoutCoverageOmitsSection(t *testing.T) {
	r := Report{From: "8.9", To: "8.10", Results: []PathResult{{Path: "p", Outcome: OutcomeClean}}}
	assert.NotContains(t, r.Markdown(), "## Coverage")
}
