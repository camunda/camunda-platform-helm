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

package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"scripts/deploy-camunda/entra"
)

// stubSweep replaces the sweep call for one test.
func stubSweep(t *testing.T, result entra.SweepResult, err error) *entra.SweepOptions {
	t.Helper()
	var seen entra.SweepOptions
	original := sweepLoginApps
	sweepLoginApps = func(_ context.Context, opts entra.SweepOptions) (entra.SweepResult, error) {
		seen = opts
		return result, err
	}
	t.Cleanup(func() { sweepLoginApps = original })
	return &seen
}

func TestEntraSweepCommand_FailedDeletesFailTheCommand(t *testing.T) {
	stubSweep(t, entra.SweepResult{
		Candidates: []entra.SweptApp{{DisplayName: "camunda-oidc-qa-id-qaen-1-8-10-qaen"}},
		Failed:     []entra.SweptApp{{DisplayName: "camunda-oidc-qa-id-qaen-1-8-10-qaen", Error: "status 403"}},
	}, nil)

	cmd := newEntraSweepLoginAppsCommand()
	cmd.SetArgs([]string{"--dry-run=false"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete 1 of 1")
}

func TestEntraSweepCommand_WritesResultAndPassesFlags(t *testing.T) {
	seen := stubSweep(t, entra.SweepResult{DryRun: true, Scanned: 3}, nil)
	output := filepath.Join(t.TempDir(), "sweep.json")

	cmd := newEntraSweepLoginAppsCommand()
	cmd.SetArgs([]string{"--max-age=36h", "--output", output})
	require.NoError(t, cmd.Execute())

	assert.Equal(t, 36*time.Hour, seen.MaxAge)
	assert.True(t, seen.DryRun)

	payload, err := os.ReadFile(output)
	require.NoError(t, err)
	var written entra.SweepResult
	require.NoError(t, json.Unmarshal(payload, &written))
	assert.Equal(t, 3, written.Scanned)
	assert.True(t, written.DryRun)
}

func TestWriteSweepResult_NoOutputFile(t *testing.T) {
	require.NoError(t, writeSweepResult("", entra.SweepResult{}))
	assert.Error(t, writeSweepResult(filepath.Join(t.TempDir(), "missing-dir", "x.json"), entra.SweepResult{}))
}
