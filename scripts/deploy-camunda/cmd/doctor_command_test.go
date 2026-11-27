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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/format"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runDoctorCapturingStdio(t *testing.T, files map[string]string, args ...string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}

	origFlags, origConfig := flags, configFile
	t.Cleanup(func() { flags, configFile = origFlags, origConfig })
	flags, configFile = config.RuntimeFlags{}, ""

	stdoutPath, stderrPath := filepath.Join(dir, "stdout"), filepath.Join(dir, "stderr")
	stdout, err := os.Create(stdoutPath)
	require.NoError(t, err)
	stderr, err := os.Create(stderrPath)
	require.NoError(t, err)
	origStdout, origStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdout, stderr

	root := NewRootCommand()
	root.AddCommand(newDoctorCommand())
	root.SetArgs(append([]string{"doctor"}, args...))
	runErr := root.Execute()

	os.Stdout, os.Stderr = origStdout, origStderr
	require.NoError(t, stdout.Close())
	require.NoError(t, stderr.Close())
	out, err := os.ReadFile(stdoutPath)
	require.NoError(t, err)
	errOut, err := os.ReadFile(stderrPath)
	require.NoError(t, err)
	return string(out), string(errOut), runErr
}

func decodeDoctorEnvelope(t *testing.T, stdout string) format.Result {
	t.Helper()
	var res format.Result
	dec := json.NewDecoder(strings.NewReader(stdout))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(&res), "stdout must be one JSON envelope:\n%s", stdout)
	assert.False(t, dec.More(), "stdout has data after the envelope:\n%s", stdout)
	assert.Equal(t, format.ResultAPIVersion, res.APIVersion)
	assert.Equal(t, format.ResultKind, res.Kind)
	return res
}

func TestDoctorJSONStdoutIsOnlyEnvelopeWithDotEnv(t *testing.T) {
	stdout, stderr, err := runDoctorCapturingStdio(t, map[string]string{".env": "DOCTOR_TEST_VAR=1\n"}, "--format", "json", "--skip-kube-check")
	if err != nil {
		assert.Contains(t, err.Error(), "preflight failed")
	}

	decodeDoctorEnvelope(t, stdout)
	assert.Contains(t, stderr, "Loaded .env file")
}

func TestDoctorJSONSetupErrorsWriteEnvelope(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		args  []string
		code  string
	}{
		{
			name:  "config load",
			files: map[string]string{"bad.yaml": "bad: [\n"},
			args:  []string{"--config", "bad.yaml"},
			code:  "config-load",
		},
		{
			name: "registry credentials",
			args: []string{"--ensure-docker-registry", "--docker-username", "someone"},
			code: "registry-credentials",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"--format", "json", "--skip-kube-check"}, tc.args...)
			stdout, _, err := runDoctorCapturingStdio(t, tc.files, args...)
			require.Error(t, err)

			res := decodeDoctorEnvelope(t, stdout)
			assert.Equal(t, format.StatusFail, res.Status)
			require.Len(t, res.Errors, 1)
			assert.Equal(t, tc.code, res.Errors[0].Code)
			assert.Equal(t, err.Error(), res.Errors[0].Message)
		})
	}
}

func TestDoctorTextSetupErrorWritesNothingToStdout(t *testing.T) {
	stdout, _, err := runDoctorCapturingStdio(t, nil, "--skip-kube-check", "--ensure-docker-registry", "--docker-username", "someone")
	require.Error(t, err)
	assert.Empty(t, stdout)
}
