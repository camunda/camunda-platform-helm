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
	cases := []struct {
		name       string
		files      map[string]string
		wantErr    bool
		wantOut    []string
		notWantOut []string
	}{
		{
			name:    "functions within the limits pass",
			files:   map[string]string{"scripts/tool/a.go": pkgHeader + complexFunc("Small", 29) + "\n" + longFunc("Long", 250)},
			wantOut: []string{"2 function(s) checked, none above cognitive 30, cyclomatic 30, 250 lines"},
		},
		{
			name:       "over-complex function fails",
			files:      map[string]string{"scripts/tool/a.go": pkgHeader + complexFunc("Big", 40) + "\n" + complexFunc("Small", 2)},
			wantErr:    true,
			wantOut:    []string{"scripts/tool/a.go:3: Big: cognitive 40 > 30, cyclomatic 41 > 30"},
			notWantOut: []string{"Small"},
		},
		{
			name:    "over-length function fails",
			files:   map[string]string{"scripts/tool/a.go": pkgHeader + longFunc("Long", 251)},
			wantErr: true,
			wantOut: []string{"scripts/tool/a.go:3: Long: 251 lines > 250"},
		},
		{
			name:    "over-limit method is named with its receiver",
			files:   map[string]string{"scripts/tool/a.go": pkgHeader + "type T struct{}\n\n" + strings.Replace(complexFunc("Method", 31), "func ", "func (t *T) ", 1)},
			wantErr: true,
			wantOut: []string{"(*T).Method: cognitive 31 > 30"},
		},
		{
			name: "over-limit function literal in a package variable is named with the variable",
			files: map[string]string{
				"scripts/tool/a.go": pkgHeader + strings.Replace(complexFunc("", 31), "func (", "var seam = func(", 1) +
					"\nvar hooks = map[string]func(int) int{\"big\": " + strings.TrimSuffix(strings.Replace(complexFunc("", 32), "func (", "func(", 1), "\n") + "}\n",
			},
			wantErr: true,
			wantOut: []string{"scripts/tool/a.go:3: seam: cognitive 31 > 30", "hooks: cognitive 32 > 30"},
		},
		{
			name:    "file names that git quotes are read",
			files:   map[string]string{"scripts/tool/a b.go": pkgHeader + complexFunc("Big", 31), "scripts/tool/ä.go": pkgHeader + complexFunc("Bigger", 32)},
			wantErr: true,
			wantOut: []string{"scripts/tool/a b.go:3: Big: cognitive 31 > 30", "scripts/tool/ä.go:3: Bigger: cognitive 32 > 30"},
		},
		{
			name: "test files and files outside scripts are not checked",
			files: map[string]string{
				"scripts/tool/a.go":      pkgHeader + complexFunc("Small", 2),
				"scripts/tool/a_test.go": pkgHeader + complexFunc("TestBig", 40),
				"other/a.go":             pkgHeader + complexFunc("Other", 40),
			},
			wantOut: []string{"1 function(s) checked, none above"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newRepo(t, tc.files)

			var out bytes.Buffer
			err := run(root, &out)

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
