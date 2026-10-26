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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func complexFunc(name string, branches int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "func %s(x int) int {\n", name)
	for i := range branches {
		fmt.Fprintf(&b, "\tif x == %d {\n\t\treturn %d\n\t}\n", i, i)
	}
	b.WriteString("\treturn 0\n}\n")
	return b.String()
}

func longFunc(name string, lines int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "func %s() int {\n\tx := 0\n", name)
	for range lines - 4 {
		b.WriteString("\tx++\n")
	}
	b.WriteString("\treturn x\n}\n")
	return b.String()
}

func gitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func newRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		writeFile(t, root, rel, content)
	}
	gitRun(t, root, "init", "-q", "-b", "main")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "base")
	return root
}

const pkgHeader = "package tool\n\n"

func TestRun(t *testing.T) {
	baseFile := pkgHeader + complexFunc("Big", 40) + "\n" + complexFunc("Small", 2)

	cases := []struct {
		name       string
		gitConfig  map[string]string
		edit       map[string]string
		wantErr    bool
		wantOut    []string
		notWantOut []string
	}{
		{
			name:    "no change",
			wantOut: []string{"0 function(s) checked"},
		},
		{
			name:       "change below the limits passes even when the file holds an over-limit function",
			edit:       map[string]string{"scripts/tool/a.go": pkgHeader + complexFunc("Big", 40) + "\n" + complexFunc("Small", 3)},
			wantOut:    []string{"1 function(s) checked, none above"},
			notWantOut: []string{"Big"},
		},
		{
			name:    "change inside an over-limit function fails",
			edit:    map[string]string{"scripts/tool/a.go": pkgHeader + complexFunc("Big", 41) + "\n" + complexFunc("Small", 2)},
			wantErr: true,
			wantOut: []string{"scripts/tool/a.go:3: Big: cognitive 41 > 30, cyclomatic 42 > 30"},
		},
		{
			name:    "deleting lines inside an over-limit function fails",
			edit:    map[string]string{"scripts/tool/a.go": pkgHeader + complexFunc("Big", 39) + "\n" + complexFunc("Small", 2)},
			wantErr: true,
			wantOut: []string{"Big: cognitive 39 > 30"},
		},
		{
			name:    "new over-length function fails",
			edit:    map[string]string{"scripts/tool/b.go": pkgHeader + longFunc("Long", 251)},
			wantErr: true,
			wantOut: []string{"scripts/tool/b.go:3: Long: 251 lines > 250"},
		},
		{
			name:    "function at the length limit passes",
			edit:    map[string]string{"scripts/tool/b.go": pkgHeader + longFunc("Long", 250)},
			wantOut: []string{"1 function(s) checked, none above"},
		},
		{
			name:    "over-limit method is named with its receiver",
			edit:    map[string]string{"scripts/tool/c.go": pkgHeader + "type T struct{}\n\n" + strings.Replace(complexFunc("Method", 31), "func ", "func (t *T) ", 1)},
			wantErr: true,
			wantOut: []string{"(*T).Method: cognitive 31 > 30"},
		},
		{
			name: "over-limit function literal in a package variable is named with the variable",
			edit: map[string]string{
				"scripts/tool/d.go": pkgHeader + strings.Replace(complexFunc("", 31), "func (", "var seam = func(", 1) +
					"\nvar hooks = map[string]func(int) int{\"big\": " + strings.TrimSuffix(strings.Replace(complexFunc("", 32), "func (", "func(", 1), "\n") + "}\n",
			},
			wantErr: true,
			wantOut: []string{"scripts/tool/d.go:3: seam: cognitive 31 > 30", "hooks: cognitive 32 > 30"},
		},
		{
			name: "test files and files outside scripts are not checked",
			edit: map[string]string{
				"scripts/tool/a_test.go": pkgHeader + complexFunc("TestBig", 40),
				"other/a.go":             pkgHeader + complexFunc("Other", 40),
			},
			wantOut: []string{"0 function(s) checked"},
		},
		{
			name:      "diff prefixes set in git config do not change the parsed paths",
			gitConfig: map[string]string{"diff.mnemonicPrefix": "true", "diff.srcPrefix": "s/", "diff.dstPrefix": "d/"},
			edit:      map[string]string{"scripts/tool/a.go": pkgHeader + complexFunc("Big", 40) + "\n" + complexFunc("Small", 3)},
			wantOut:   []string{"1 function(s) checked, none above"},
		},
		{
			name:       "edits on both sides of an unchanged over-limit function do not select it under diff.interHunkContext",
			gitConfig:  map[string]string{"diff.interHunkContext": "1000"},
			edit:       map[string]string{"scripts/tool/a.go": pkgHeader + "var x = 1\n\n" + complexFunc("Big", 40) + "\n" + complexFunc("Small", 3)},
			wantOut:    []string{"1 function(s) checked, none above"},
			notWantOut: []string{"Big"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newRepo(t, map[string]string{"scripts/tool/a.go": baseFile})
			for key, value := range tc.gitConfig {
				gitRun(t, root, "config", key, value)
			}
			for rel, content := range tc.edit {
				writeFile(t, root, rel, content)
			}

			var out bytes.Buffer
			err := run(root, "main", false, &out)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			for _, want := range tc.wantOut {
				assert.Contains(t, out.String(), want)
			}
			for _, notWant := range tc.notWantOut {
				assert.NotContains(t, out.String(), notWant)
			}
		})
	}
}

func TestRunComparesAgainstMergeBase(t *testing.T) {
	root := newRepo(t, map[string]string{"scripts/tool/a.go": pkgHeader + complexFunc("Big", 40)})
	gitRun(t, root, "switch", "-q", "-c", "feature")
	writeFile(t, root, "scripts/tool/b.go", pkgHeader+complexFunc("Small", 2))
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "feature")
	gitRun(t, root, "switch", "-q", "main")
	writeFile(t, root, "scripts/tool/a.go", pkgHeader+complexFunc("Big", 41))
	gitRun(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-am", "main moves on")
	gitRun(t, root, "switch", "-q", "feature")

	var out bytes.Buffer
	require.NoError(t, run(root, "main", false, &out))
	assert.Contains(t, out.String(), "1 function(s) checked, none above")
}

func TestRunAllReportsUnchangedFunctions(t *testing.T) {
	root := newRepo(t, map[string]string{"scripts/tool/a.go": pkgHeader + complexFunc("Big", 40) + "\n" + complexFunc("Small", 2)})

	var out bytes.Buffer
	err := run(root, "main", true, &out)

	require.Error(t, err)
	assert.Contains(t, out.String(), "Big: cognitive 40 > 30")
	assert.Contains(t, err.Error(), "1 of 2 checked function(s)")
}

func TestRunReadsChangedFileNamesThatGitQuotes(t *testing.T) {
	root := newRepo(t, map[string]string{
		"scripts/tool/a b.go": pkgHeader + complexFunc("Spaced", 31),
		"scripts/tool/ö.go":   pkgHeader + complexFunc("Umlaut", 31),
	})
	writeFile(t, root, "scripts/tool/a b.go", pkgHeader+complexFunc("Spaced", 32))
	writeFile(t, root, "scripts/tool/ö.go", pkgHeader+complexFunc("Umlaut", 32))
	writeFile(t, root, "scripts/tool/ä b.go", pkgHeader+complexFunc("Untracked", 31))

	var out bytes.Buffer
	require.Error(t, run(root, "main", false, &out))
	assert.Contains(t, out.String(), "scripts/tool/a b.go:3: Spaced: cognitive 32 > 30")
	assert.Contains(t, out.String(), "scripts/tool/ö.go:3: Umlaut: cognitive 32 > 30")
	assert.Contains(t, out.String(), "scripts/tool/ä b.go:3: Untracked: cognitive 31 > 30")
}

func TestRunAllReadsFileNamesThatGitQuotes(t *testing.T) {
	root := newRepo(t, map[string]string{
		"scripts/tool/a b.go": pkgHeader + complexFunc("Big", 31),
		"scripts/tool/ä.go":   pkgHeader + complexFunc("Bigger", 32),
	})

	var out bytes.Buffer
	require.Error(t, run(root, "main", true, &out))
	assert.Contains(t, out.String(), "scripts/tool/a b.go:3: Big: cognitive 31 > 30")
	assert.Contains(t, out.String(), "scripts/tool/ä.go:3: Bigger: cognitive 32 > 30")
}

func TestTouches(t *testing.T) {
	cases := []struct {
		name  string
		hunks []hunk
		want  bool
	}{
		{"added line before the function", []hunk{{start: 9, count: 1}}, false},
		{"added lines overlap the first line", []hunk{{start: 8, count: 3}}, true},
		{"added line on the closing brace", []hunk{{start: 20, count: 1}}, true},
		{"added line after the function", []hunk{{start: 21, count: 2}}, false},
		{"deletion after the signature", []hunk{{start: 10, count: 0}}, true},
		{"deletion after the closing brace", []hunk{{start: 20, count: 0}}, false},
		{"deletion before the function", []hunk{{start: 9, count: 0}}, false},
		{"whole file", wholeFile, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, touches(tc.hunks, 10, 20))
		})
	}
}

func TestParseDiff(t *testing.T) {
	diff := `diff --git a/scripts/a.go b/scripts/a.go
index 1111111..2222222 100644
--- a/scripts/a.go
+++ b/scripts/a.go
@@ -3 +3 @@ func A() {
-	x := 1
+	x := 2
@@ -10,2 +9,0 @@ func A() {
-++ not a header
-	y := 1
diff --git a/scripts/old.go b/scripts/new.go
similarity index 90%
rename from scripts/old.go
rename to scripts/new.go
--- a/scripts/old.go
+++ b/scripts/new.go
@@ -1,0 +2,3 @@
+++ also not a header
+b
+c
diff --git a/scripts/gone.go b/scripts/gone.go
deleted file mode 100644
--- a/scripts/gone.go
+++ /dev/null
@@ -1,2 +0,0 @@
-package gone
-
`
	files, err := parseDiff(diff)
	require.NoError(t, err)
	assert.Equal(t, map[string][]hunk{
		"scripts/a.go":   {{start: 3, count: 1}, {start: 9, count: 0}},
		"scripts/new.go": {{start: 2, count: 3}},
		"/dev/null":      {{start: 0, count: 0}},
	}, files)
}

func TestParseDiffDecodesQuotedPaths(t *testing.T) {
	diff := "diff --git \"a/scripts/\\303\\244.go\" \"b/scripts/\\303\\244.go\"\n" +
		"--- \"a/scripts/\\303\\244.go\"\n" +
		"+++ \"b/scripts/\\303\\244.go\"\n" +
		"@@ -1 +1 @@\n" +
		"diff --git a/scripts/a b.go b/scripts/a b.go\n" +
		"--- a/scripts/a b.go\t\n" +
		"+++ b/scripts/a b.go\t\n" +
		"@@ -2 +2,2 @@\n"
	files, err := parseDiff(diff)
	require.NoError(t, err)
	assert.Equal(t, map[string][]hunk{
		"scripts/ä.go":   {{start: 1, count: 1}},
		"scripts/a b.go": {{start: 2, count: 2}},
	}, files)
}

func TestParseDiffRejectsMalformedHunk(t *testing.T) {
	_, err := parseDiff("diff --git a/x b/x\n+++ b/x\n@@ -1 +x,2 @@\n")
	require.Error(t, err)
}
