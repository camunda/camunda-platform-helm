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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseListFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "list.txt")
	require.NoError(t, os.WriteFile(path, []byte("# comment\n\nalice\n  bob  \n# trailing\n"), 0o644))

	lines, err := parseListFile(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"alice", "bob"}, lines)
}

func TestParseListFile_missing(t *testing.T) {
	lines, err := parseListFile(filepath.Join(t.TempDir(), "missing.txt"))
	require.NoError(t, err)
	assert.Nil(t, lines)
}

func TestContainsExact(t *testing.T) {
	lines := []string{"eamonnmoloney", "alice-camunda"}
	assert.True(t, containsExact(lines, "eamonnmoloney"))
	assert.False(t, containsExact(lines, "eamonn"))
	assert.False(t, containsExact(lines, "renovate[bot]"))
}
