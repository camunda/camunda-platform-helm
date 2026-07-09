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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveListPath_fromRepoRoot(t *testing.T) {
	dir := t.TempDir()
	allowlist := filepath.Join(dir, ".github", "auto-approve-allowlist.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(allowlist), 0o755))
	require.NoError(t, os.WriteFile(allowlist, []byte("alice\n"), 0o644))

	moduleDir := filepath.Join(dir, "scripts", "auto-approve-gate")
	require.NoError(t, os.MkdirAll(moduleDir, 0o755))
	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(moduleDir))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	got := resolveListPath("AUTO_APPROVE_ALLOWLIST", ".github/auto-approve-allowlist.txt")
	assert.Equal(t, filepath.Join("..", "..", ".github", "auto-approve-allowlist.txt"), got)
}
