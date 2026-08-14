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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const apacheGoHeader = `// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
`

const proprietaryShHeader = `#!/bin/bash
# Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
# under one or more contributor license agreements. Licensed under a proprietary license.
`

// newRepo creates a git repo containing the given files and returns its path.
func newRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()

	for rel, content := range files {
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}

	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	return root
}

func TestPassesWhenEveryFileIsApache(t *testing.T) {
	root := newRepo(t, map[string]string{
		"scripts/tool/main.go": apacheGoHeader + "\npackage main\n",
		"scripts/setup.sh":     "#!/bin/bash\n# Licensed under the Apache License, Version 2.0\n",
	})

	var out bytes.Buffer
	require.NoError(t, run(root, ".go,.sh", &out))
	assert.Contains(t, out.String(), "2 files verified")
}

func TestFailsOnMissingHeader(t *testing.T) {
	root := newRepo(t, map[string]string{
		"scripts/tool/main.go":  apacheGoHeader + "\npackage main\n",
		"scripts/tool/types.go": "package main\n",
	})

	var out bytes.Buffer
	err := run(root, ".go", &out)

	require.Error(t, err)
	assert.Contains(t, out.String(), reasonMissing)
	assert.Contains(t, out.String(), "scripts/tool/types.go")
	assert.NotContains(t, out.String(), "scripts/tool/main.go")
}

// The gap this tool exists to close: addlicense -check accepts any header that
// is present, so a proprietary or GPL header passes it silently.
func TestFailsOnNonApacheHeader(t *testing.T) {
	for name, content := range map[string]string{
		"proprietary": proprietaryShHeader,
		"gpl":         "#!/bin/bash\n# Copyright 2026 Someone\n# Licensed under the GNU General Public License v3.0.\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := newRepo(t, map[string]string{"scripts/thing.sh": content})

			var out bytes.Buffer
			err := run(root, ".sh", &out)

			require.Error(t, err)
			assert.Contains(t, out.String(), reasonNonApache)
			assert.Contains(t, out.String(), "scripts/thing.sh")
		})
	}
}

func TestAcceptsSpdxIdentifier(t *testing.T) {
	// SPDX identifiers are case-insensitive by spec; vendored Bitnami charts
	// spell theirs "APACHE-2.0".
	for name, header := range map[string]string{
		"canonical": "// SPDX-License-Identifier: Apache-2.0\n",
		"uppercase": "// SPDX-License-Identifier: APACHE-2.0\n",
		"lowercase": "// spdx-license-identifier: apache-2.0\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := newRepo(t, map[string]string{"scripts/tool.go": header + "\npackage main\n"})

			var out bytes.Buffer
			assert.NoError(t, run(root, ".go", &out))
		})
	}
}

// A header below a shebang and a set -euo pipefail block must still be found.
func TestFindsHeaderBelowShebang(t *testing.T) {
	root := newRepo(t, map[string]string{
		"scripts/thing.sh": "#!/bin/bash\n\nset -euo pipefail\n\n" +
			"# Copyright 2026 Camunda Services GmbH\n" +
			"# Licensed under the Apache License, Version 2.0\n",
	})

	var out bytes.Buffer
	assert.NoError(t, run(root, ".sh", &out))
}

// A header pushed past the scan window must not count, otherwise an unrelated
// mention of Apache deep in a file would satisfy the check.
func TestIgnoresHeaderBeyondScanWindow(t *testing.T) {
	root := newRepo(t, map[string]string{
		"scripts/thing.sh": "#!/bin/bash\n" + strings.Repeat("echo padding\n", headerScanLines+5) +
			"# Licensed under the Apache License, Version 2.0\n",
	})

	var out bytes.Buffer
	err := run(root, ".sh", &out)

	require.Error(t, err)
	assert.Contains(t, out.String(), reasonMissing)
}

func TestReportsBothReasonsSeparately(t *testing.T) {
	root := newRepo(t, map[string]string{
		"scripts/missing.go": "package main\n",
		"scripts/wrong.sh":   proprietaryShHeader,
	})

	var out bytes.Buffer
	err := run(root, ".go,.sh", &out)

	require.Error(t, err)
	assert.Contains(t, out.String(), reasonMissing)
	assert.Contains(t, out.String(), reasonNonApache)
	assert.Contains(t, out.String(), "2 violation(s)")
}

func TestIgnoresUntrackedAndOtherExtensions(t *testing.T) {
	root := newRepo(t, map[string]string{
		"scripts/tool.go": apacheGoHeader + "\npackage main\n",
		"README.md":       "no header here\n",
	})
	// Untracked files are out of scope even with a matching extension.
	require.NoError(t, os.WriteFile(filepath.Join(root, "scratch.go"), []byte("package main\n"), 0o644))

	var out bytes.Buffer
	require.NoError(t, run(root, ".go", &out))
	assert.Contains(t, out.String(), "1 files verified")
}

// An empty file set means the scope is wrong; reporting success would repeat
// the very bug this tool guards against.
func TestFailsWhenNoFilesMatch(t *testing.T) {
	root := newRepo(t, map[string]string{"README.md": "hello\n"})

	var out bytes.Buffer
	err := run(root, ".go", &out)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing to report success")
}

func TestParseExtsNormalisesLeadingDot(t *testing.T) {
	assert.Equal(t, []string{".go", ".sh"}, parseExts("go, .sh"))
	assert.Nil(t, parseExts(" , "))
}
