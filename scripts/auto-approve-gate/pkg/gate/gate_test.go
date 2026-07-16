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

package gate

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var defaultProtected = []string{
	`^\.github/workflows/`,
	`^\.github/actions/`,
	`^\.github/CODEOWNERS$`,
	`^\.github/auto-approve-allowlist\.txt$`,
	`^\.github/auto-approve-protected-paths\.txt$`,
	`^charts/.+/values\.yaml$`,
	`^charts/.+/values\.schema(\.extra)?\.json$`,
	`^charts/.+/constraints\.tpl$`,
	`^scripts/auto-approve-gate/`,
}

var defaultRenovateProtected = []string{
	`^\.github/auto-approve-`,
	`^\.github/workflows/repo-auto-approve\.yaml$`,
	`^\.github/CODEOWNERS$`,
	`^charts/.+/values\.schema(\.extra)?\.json$`,
	`^charts/.+/constraints\.tpl$`,
	`^scripts/auto-approve-gate/`,
}

func TestDecide_renovateLane(t *testing.T) {
	dec := Decide(Inputs{
		Lane:              LaneRenovate,
		Author:            RenovateAuthor,
		EventActor:        RenovateAuthor,
		ProtectedPatterns: defaultRenovateProtected,
		PRMeta:            &PRMeta{ChangedFiles: 3},
		PRFiles: []PRFile{
			{Filename: ".github/workflows/chart-chores.yaml"},
			{Filename: "charts/camunda-platform-8.10/values.yaml"},
			{Filename: "go.mod"},
		},
	})
	assert.True(t, dec.Allowed)
	assert.Equal(t, LaneRenovate, dec.Lane)
}

func TestDecide_imposterRenovate(t *testing.T) {
	dec := Decide(Inputs{
		Author:    "renovate",
		Allowlist: []string{"eamonnmoloney"},
	})
	assert.False(t, dec.Allowed)
	assert.Equal(t, LaneHuman, dec.Lane)
}

func TestDecide_renovateUntrustedActor(t *testing.T) {
	dec := Decide(Inputs{
		Lane:              LaneRenovate,
		Author:            RenovateAuthor,
		EventActor:        "mallory",
		ProtectedPatterns: defaultRenovateProtected,
		PRMeta:            &PRMeta{ChangedFiles: 1},
		PRFiles:           []PRFile{{Filename: "go.mod"}},
	})
	assert.False(t, dec.Allowed)
	assert.Equal(t, LaneRenovate, dec.Lane)
	assert.Equal(t, []string{"event actor mallory is not a trusted renovate-lane pusher; requiring human review."}, dec.Warnings)
}

func TestDecide_renovateTrustedDistroCIActor(t *testing.T) {
	dec := Decide(Inputs{
		Lane:              LaneRenovate,
		Author:            RenovateAuthor,
		EventActor:        DistroCIAuthor,
		ProtectedPatterns: defaultRenovateProtected,
		PRMeta:            &PRMeta{ChangedFiles: 1},
		PRFiles:           []PRFile{{Filename: "go.mod"}},
	})
	assert.True(t, dec.Allowed)
	assert.Equal(t, LaneRenovate, dec.Lane)
}

func TestDecide_renovateBlockedValuesSchema(t *testing.T) {
	dec := Decide(Inputs{
		Lane:              LaneRenovate,
		Author:            RenovateAuthor,
		EventActor:        RenovateAuthor,
		ProtectedPatterns: defaultRenovateProtected,
		PRMeta:            &PRMeta{ChangedFiles: 1},
		PRFiles:           []PRFile{{Filename: "charts/camunda-platform-8.10/values.schema.json"}},
	})
	assert.False(t, dec.Allowed)
	assert.Equal(t, LaneRenovate, dec.Lane)
	assert.Equal(t, []string{"PR touches a protected path; human review is required."}, dec.Notices)
}

func TestDecide_humanLaneRequiresTrustedPusher(t *testing.T) {
	dec := Decide(Inputs{
		Author:            "eamonnmoloney",
		EventActor:        "mallory",
		Allowlist:         []string{"eamonnmoloney"},
		ProtectedPatterns: defaultProtected,
		PRMeta:            &PRMeta{ChangedFiles: 1},
		PRFiles:           []PRFile{{Filename: "README.md"}},
	})
	assert.False(t, dec.Allowed)
	assert.Equal(t, LaneHuman, dec.Lane)

	decTrusted := Decide(Inputs{
		Author:            "eamonnmoloney",
		EventActor:        "eamonnmoloney",
		Allowlist:         []string{"eamonnmoloney"},
		ProtectedPatterns: defaultProtected,
		PRMeta:            &PRMeta{ChangedFiles: 1},
		PRFiles:           []PRFile{{Filename: "README.md"}},
	})
	assert.True(t, decTrusted.Allowed)
	assert.Equal(t, LaneHuman, decTrusted.Lane)
}

func TestDecide_table(t *testing.T) {
	metaOK := &PRMeta{ChangedFiles: 7}

	tests := []struct {
		name     string
		in       Inputs
		allowed  bool
		lane     string
		warnings []string
		notices  []string
	}{
		{
			name: "human allowlisted docs only",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFiles:           []PRFile{{Filename: "README.md"}, {Filename: "docs/foo.md"}},
			},
			allowed: true,
			lane:    LaneHuman,
		},
		{
			name: "human not allowlisted",
			in: Inputs{
				Author:            "mallory",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFiles:           []PRFile{{Filename: "README.md"}},
			},
			allowed: false,
			lane:    LaneHuman,
		},
		{
			name: "blocked values.yaml",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFiles:           []PRFile{{Filename: "charts/camunda-platform-8.10/values.yaml"}},
			},
			allowed: false,
			lane:    LaneHuman,
			notices: []string{"PR touches a protected path; human review is required."},
		},
		{
			name: "blocked values.schema.json",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFiles:           []PRFile{{Filename: "charts/camunda-platform-8.10/values.schema.json"}},
			},
			allowed: false,
			lane:    LaneHuman,
			notices: []string{"PR touches a protected path; human review is required."},
		},
		{
			name: "blocked values.schema.extra.json",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFiles:           []PRFile{{Filename: "charts/camunda-platform-8.10/values.schema.extra.json"}},
			},
			allowed: false,
			lane:    LaneHuman,
			notices: []string{"PR touches a protected path; human review is required."},
		},
		{
			name: "blocked constraints.tpl",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFiles:           []PRFile{{Filename: "charts/camunda-platform-8.10/templates/constraints.tpl"}},
			},
			allowed: false,
			lane:    LaneHuman,
			notices: []string{"PR touches a protected path; human review is required."},
		},
		{
			name: "blocked workflow",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFiles:           []PRFile{{Filename: ".github/workflows/foo.yaml"}},
			},
			allowed: false,
			lane:    LaneHuman,
			notices: []string{"PR touches a protected path; human review is required."},
		},
		{
			name: "blocked CODEOWNERS",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFiles:           []PRFile{{Filename: ".github/CODEOWNERS"}},
			},
			allowed: false,
			lane:    LaneHuman,
			notices: []string{"PR touches a protected path; human review is required."},
		},
		{
			name: "blocked allowlist file",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFiles:           []PRFile{{Filename: ".github/auto-approve-allowlist.txt"}},
			},
			allowed: false,
			lane:    LaneHuman,
			notices: []string{"PR touches a protected path; human review is required."},
		},
		{
			name: "blocked protected paths file",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFiles:           []PRFile{{Filename: ".github/auto-approve-protected-paths.txt"}},
			},
			allowed: false,
			lane:    LaneHuman,
			notices: []string{"PR touches a protected path; human review is required."},
		},
		{
			name: "blocked auto-approve gate implementation",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFiles:           []PRFile{{Filename: "scripts/auto-approve-gate/pkg/gate/gate.go"}},
			},
			allowed: false,
			lane:    LaneHuman,
			notices: []string{"PR touches a protected path; human review is required."},
		},
		{
			name: "rename evasion CODEOWNERS",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFiles:           []PRFile{{Filename: "owners.txt", PreviousFilename: ".github/CODEOWNERS"}},
			},
			allowed: false,
			lane:    LaneHuman,
			notices: []string{"PR touches a protected path; human review is required."},
		},
		{
			name: "chart test fixtures allowed",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFiles: []PRFile{
					{Filename: "charts/camunda-platform-8.10/test/unit/orchestration/testdata/values.yaml"},
				},
			},
			allowed: true,
			lane:    LaneHuman,
		},
		{
			name: "docs github workflows not anchored blocked path",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFiles:           []PRFile{{Filename: "docs/.github/workflows/x"}},
			},
			allowed: true,
			lane:    LaneHuman,
		},
		{
			name: "CODEOWNERS.bak not anchored",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFiles:           []PRFile{{Filename: "CODEOWNERS.bak"}},
			},
			allowed: true,
			lane:    LaneHuman,
		},
		{
			name: "fail closed empty protected patterns",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: nil,
				PRMeta:            metaOK,
			},
			allowed:  false,
			lane:     LaneHuman,
			warnings: []string{"protected-paths list missing/empty; requiring human review."},
		},
		{
			name: "fail closed invalid regex",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: []string{"["},
				PRMeta:            metaOK,
				PRFiles:           []PRFile{{Filename: "README.md"}},
			},
			allowed:  false,
			lane:     LaneHuman,
			warnings: []string{"protected-path check errored; requiring human review."},
		},
		{
			name: "fail closed changed_files cap",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            &PRMeta{ChangedFiles: 3000},
			},
			allowed:  false,
			lane:     LaneHuman,
			warnings: []string{"PR changes 3000 files (>=3000 API cap); requiring human review."},
		},
		{
			name: "fail closed PR meta API error",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMetaErr:         errors.New("api down"),
			},
			allowed:  false,
			lane:     LaneHuman,
			warnings: []string{"Could not read PR metadata; requiring human review."},
		},
		{
			name: "fail closed PR files API error",
			in: Inputs{
				Author:            "eamonnmoloney",
				EventActor:        "eamonnmoloney",
				Allowlist:         []string{"eamonnmoloney"},
				ProtectedPatterns: defaultProtected,
				PRMeta:            metaOK,
				PRFilesErr:        errors.New("api down"),
			},
			allowed:  false,
			lane:     LaneHuman,
			warnings: []string{"Could not list PR files; requiring human review."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := Decide(tt.in)
			assert.Equal(t, tt.allowed, dec.Allowed, "allowed")
			assert.Equal(t, tt.lane, dec.Lane, "lane")
			assert.Equal(t, tt.warnings, dec.Warnings, "warnings")
			assert.Equal(t, tt.notices, dec.Notices, "notices")
		})
	}
}

type fakeClient struct {
	t              testing.TB
	meta           PRMeta
	metaErr        error
	files          []PRFile
	filesErr       error
	metaCalls      int
	filesCalls     int
	filesPageCalls int
}

func (f *fakeClient) GetPullRequest(int) (PRMeta, error) {
	f.metaCalls++
	return f.meta, f.metaErr
}

func (f *fakeClient) ListPullRequestFiles(int) ([]PRFile, error) {
	f.filesCalls++
	return f.files, f.filesErr
}

func TestRun_renovateLane(t *testing.T) {
	writeRenovateProtected := func(t *testing.T, dir string) {
		t.Helper()
		writeList(t, filepath.Join(dir, "renovate-protected.txt"), strings.Join(defaultRenovateProtected, "\n")+"\n")
	}

	tests := []struct {
		name        string
		eventActor  string
		files       []PRFile
		filesErr    error
		protected   string
		wantAllowed bool
		wantNotice  string
		wantWarning string
		metaCalls   int
		filesCalls  int
	}{
		{
			name:       "pin and tag bumps allowed",
			eventActor: RenovateAuthor,
			files: []PRFile{
				{Filename: ".github/workflows/chart-chores.yaml"},
				{Filename: "charts/camunda-platform-8.10/values.yaml"},
				{Filename: "go.mod"},
			},
			wantAllowed: true,
			metaCalls:   1,
			filesCalls:  1,
		},
		{
			name:        "distro-ci actor proceeds",
			eventActor:  DistroCIAuthor,
			files:       []PRFile{{Filename: "go.mod"}},
			wantAllowed: true,
			metaCalls:   1,
			filesCalls:  1,
		},
		{
			name:        "untrusted actor blocked",
			eventActor:  "mallory",
			files:       []PRFile{{Filename: "go.mod"}},
			wantAllowed: false,
			wantWarning: "event actor mallory is not a trusted renovate-lane pusher",
			metaCalls:   0,
			filesCalls:  0,
		},
		{
			name:        "blocked values.schema.json",
			eventActor:  RenovateAuthor,
			files:       []PRFile{{Filename: "charts/camunda-platform-8.10/values.schema.json"}},
			wantAllowed: false,
			wantNotice:  "PR touches a protected path",
			metaCalls:   1,
			filesCalls:  1,
		},
		{
			name:        "blocked allowlist",
			eventActor:  RenovateAuthor,
			files:       []PRFile{{Filename: ".github/auto-approve-allowlist.txt"}},
			wantAllowed: false,
			wantNotice:  "PR touches a protected path",
			metaCalls:   1,
			filesCalls:  1,
		},
		{
			name:        "blocked renovate protected list self-protection",
			eventActor:  RenovateAuthor,
			files:       []PRFile{{Filename: ".github/auto-approve-protected-paths-renovate.txt"}},
			wantAllowed: false,
			wantNotice:  "PR touches a protected path",
			metaCalls:   1,
			filesCalls:  1,
		},
		{
			name:        "blocked repo-auto-approve workflow",
			eventActor:  RenovateAuthor,
			files:       []PRFile{{Filename: ".github/workflows/repo-auto-approve.yaml"}},
			wantAllowed: false,
			wantNotice:  "PR touches a protected path",
			metaCalls:   1,
			filesCalls:  1,
		},
		{
			name:        "blocked CODEOWNERS",
			eventActor:  RenovateAuthor,
			files:       []PRFile{{Filename: ".github/CODEOWNERS"}},
			wantAllowed: false,
			wantNotice:  "PR touches a protected path",
			metaCalls:   1,
			filesCalls:  1,
		},
		{
			name:        "blocked auto-approve gate go.mod",
			eventActor:  RenovateAuthor,
			files:       []PRFile{{Filename: "scripts/auto-approve-gate/go.mod"}},
			wantAllowed: false,
			wantNotice:  "PR touches a protected path",
			metaCalls:   1,
			filesCalls:  1,
		},
		{
			name:       "blocked rename evasion",
			eventActor: RenovateAuthor,
			files: []PRFile{
				{Filename: "safe.txt", PreviousFilename: ".github/auto-approve-allowlist.txt"},
			},
			wantAllowed: false,
			wantNotice:  "PR touches a protected path",
			metaCalls:   1,
			filesCalls:  1,
		},
		{
			name:        "fail closed files API error",
			eventActor:  RenovateAuthor,
			filesErr:    errors.New("api down"),
			wantAllowed: false,
			wantWarning: "Could not list PR files",
			metaCalls:   1,
			filesCalls:  1,
		},
		{
			name:        "fail closed missing renovate list",
			eventActor:  RenovateAuthor,
			protected:   "missing",
			wantAllowed: false,
			wantWarning: "protected-paths list missing/empty",
			metaCalls:   0,
			filesCalls:  0,
		},
		{
			name:        "fail closed empty renovate list",
			eventActor:  RenovateAuthor,
			protected:   "empty",
			wantAllowed: false,
			wantWarning: "protected-paths list missing/empty",
			metaCalls:   0,
			filesCalls:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			protectedPath := filepath.Join(dir, "renovate-protected.txt")
			switch tt.protected {
			case "missing":
				protectedPath = filepath.Join(dir, "missing-renovate-protected.txt")
			case "empty":
				writeList(t, protectedPath, "# empty\n")
			default:
				writeRenovateProtected(t, dir)
			}

			client := &fakeClient{
				t:        t,
				meta:     PRMeta{ChangedFiles: len(tt.files)},
				files:    tt.files,
				filesErr: tt.filesErr,
			}
			var buf bytes.Buffer
			t.Setenv("GITHUB_OUTPUT", filepath.Join(t.TempDir(), "out"))

			eventActor := tt.eventActor
			if eventActor == "" {
				eventActor = RenovateAuthor
			}
			err := Run(Config{
				Author:                     RenovateAuthor,
				EventActor:                 eventActor,
				PRNumber:                   1,
				AllowlistPath:              filepath.Join(t.TempDir(), "missing"),
				ProtectedPathsPath:         filepath.Join(t.TempDir(), "missing"),
				RenovateProtectedPathsPath: protectedPath,
			}, client, &buf)
			require.NoError(t, err)
			assert.Equal(t, tt.metaCalls, client.metaCalls)
			assert.Equal(t, tt.filesCalls, client.filesCalls)
			assert.Contains(t, buf.String(), "lane=renovate")
			if tt.wantAllowed {
				assert.Contains(t, buf.String(), "allowlisted=true")
			} else {
				assert.Contains(t, buf.String(), "allowlisted=false")
			}
			if tt.wantNotice != "" {
				assert.Contains(t, buf.String(), tt.wantNotice)
			}
			if tt.wantWarning != "" {
				assert.Contains(t, buf.String(), tt.wantWarning)
			}
		})
	}
}

func TestRun_paginationBothPagesScanned(t *testing.T) {
	dir := t.TempDir()
	writeList(t, filepath.Join(dir, "allowlist.txt"), "eamonnmoloney\n")
	writeList(t, filepath.Join(dir, "protected.txt"), `^charts/.+/values\.yaml$`+"\n")

	client := &fakeClient{
		t:    t,
		meta: PRMeta{ChangedFiles: 150},
		files: []PRFile{
			{Filename: "README.md"},
			{Filename: "charts/camunda-platform-8.10/values.yaml"},
		},
	}

	var buf bytes.Buffer
	outPath := filepath.Join(t.TempDir(), "gh-out")
	t.Setenv("GITHUB_OUTPUT", outPath)

	err := Run(Config{
		Author:             "eamonnmoloney",
		EventActor:         "eamonnmoloney",
		PRNumber:           42,
		AllowlistPath:      filepath.Join(dir, "allowlist.txt"),
		ProtectedPathsPath: filepath.Join(dir, "protected.txt"),
	}, client, &buf)
	require.NoError(t, err)
	assert.Equal(t, 1, client.metaCalls)
	assert.Equal(t, 1, client.filesCalls)
	assert.Contains(t, buf.String(), "PR touches a protected path")
}

func TestRun_integrationFromFiles(t *testing.T) {
	dir := t.TempDir()
	writeList(t, filepath.Join(dir, "allowlist.txt"), "# comment\neamonnmoloney\n")
	writeList(t, filepath.Join(dir, "protected.txt"), strings.Join(defaultProtected, "\n")+"\n")

	client := &fakeClient{
		t:    t,
		meta: PRMeta{ChangedFiles: 3},
		files: []PRFile{
			{Filename: "charts/camunda-platform-8.10/test/unit/foo_test.go"},
			{Filename: "SKILLS.md"},
		},
	}

	var buf bytes.Buffer
	outPath := filepath.Join(t.TempDir(), "gh-out")
	t.Setenv("GITHUB_OUTPUT", outPath)

	err := Run(Config{
		Author:             "eamonnmoloney",
		EventActor:         "eamonnmoloney",
		PRNumber:           6525,
		AllowlistPath:      filepath.Join(dir, "allowlist.txt"),
		ProtectedPathsPath: filepath.Join(dir, "protected.txt"),
	}, client, &buf)
	require.NoError(t, err)

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "allowed=true")
	assert.Contains(t, string(data), "lane=human")
}

func writeList(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestEmit_writesGitHubOutput(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "out")
	t.Setenv("GITHUB_OUTPUT", outPath)

	var buf bytes.Buffer
	err := emit(Decision{Allowed: true, Lane: LaneHuman}, "eamonnmoloney", &buf)
	require.NoError(t, err)

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, "allowed=true\nlane=human\n", string(data))
	assert.Contains(t, buf.String(), "::notice::author=eamonnmoloney")
}

func TestWriteGitHubOutput_stdoutFallback(t *testing.T) {
	t.Setenv("GITHUB_OUTPUT", "")
	// capture stdout
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	err = writeGitHubOutput(false, LaneRenovate)
	w.Close()
	require.NoError(t, err)
	assert.Equal(t, "allowed=false\nlane=renovate\n", <-done)
}

func TestMatchesProtected_paginationFake(t *testing.T) {
	// Simulate two pages merged: first page benign, second page protected.
	files := []PRFile{
		{Filename: "docs/readme.md"},
		{Filename: "charts/camunda-platform-8.9/values.yaml"},
	}
	matched, err := matchesProtected(files, []string{`^charts/.+/values\.yaml$`})
	require.NoError(t, err)
	assert.True(t, matched)
}

func TestRun_notAllowlistedSkipsAPI(t *testing.T) {
	dir := t.TempDir()
	writeList(t, filepath.Join(dir, "allowlist.txt"), "eamonnmoloney\n")
	writeList(t, filepath.Join(dir, "protected.txt"), `^foo$`+"\n")

	client := &fakeClient{t: t, meta: PRMeta{ChangedFiles: 1}}
	var buf bytes.Buffer
	t.Setenv("GITHUB_OUTPUT", filepath.Join(t.TempDir(), "out"))

	err := Run(Config{
		Author:             "mallory",
		EventActor:         "mallory",
		PRNumber:           1,
		AllowlistPath:      filepath.Join(dir, "allowlist.txt"),
		ProtectedPathsPath: filepath.Join(dir, "protected.txt"),
	}, client, &buf)
	require.NoError(t, err)
	assert.Equal(t, 0, client.metaCalls)
	assert.Equal(t, 0, client.filesCalls)
}

func TestRun_untrustedPusherSkipsAPI(t *testing.T) {
	dir := t.TempDir()
	writeList(t, filepath.Join(dir, "allowlist.txt"), "eamonnmoloney\n")
	writeList(t, filepath.Join(dir, "protected.txt"), `^foo$`+"\n")

	client := &fakeClient{t: t, meta: PRMeta{ChangedFiles: 1}}
	var buf bytes.Buffer
	t.Setenv("GITHUB_OUTPUT", filepath.Join(t.TempDir(), "out"))

	err := Run(Config{
		Author:             "eamonnmoloney",
		EventActor:         "mallory",
		PRNumber:           1,
		AllowlistPath:      filepath.Join(dir, "allowlist.txt"),
		ProtectedPathsPath: filepath.Join(dir, "protected.txt"),
	}, client, &buf)
	require.NoError(t, err)
	assert.Equal(t, 0, client.metaCalls)
	assert.Equal(t, 0, client.filesCalls)
	assert.Contains(t, buf.String(), "allowlisted=false")
}

func TestCollectPaths(t *testing.T) {
	paths := collectPaths([]PRFile{
		{Filename: "a", PreviousFilename: "b"},
		{Filename: "", PreviousFilename: "c"},
	})
	assert.Equal(t, []string{"a", "b", "c"}, paths)
}

func TestRun_missingProtectedFileFailClosed(t *testing.T) {
	dir := t.TempDir()
	writeList(t, filepath.Join(dir, "allowlist.txt"), "eamonnmoloney\n")

	client := &fakeClient{t: t, meta: PRMeta{ChangedFiles: 1}, files: []PRFile{{Filename: "README.md"}}}
	var buf bytes.Buffer
	t.Setenv("GITHUB_OUTPUT", filepath.Join(t.TempDir(), "out"))

	err := Run(Config{
		Author:             "eamonnmoloney",
		EventActor:         "eamonnmoloney",
		PRNumber:           1,
		AllowlistPath:      filepath.Join(dir, "allowlist.txt"),
		ProtectedPathsPath: filepath.Join(dir, "missing-protected.txt"),
	}, client, &buf)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "protected-paths list missing/empty")
	assert.Equal(t, 0, client.metaCalls)
}
