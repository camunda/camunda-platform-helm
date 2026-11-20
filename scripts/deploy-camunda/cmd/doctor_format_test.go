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
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"scripts/deploy-camunda/format"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var updateDoctorGolden = flag.Bool("update-doctor-golden", false, "regenerate the doctor JSON golden file")

var fakeSecret = strings.Repeat("sentinel-value-", 3)

func sampleDoctorReport() *deploy.Report {
	return &deploy.Report{Checks: []deploy.Check{
		{Name: "config file", Status: deploy.StatusWarn, Detail: "no config file found (relying on flags/env)", Remediation: "run `deploy-camunda config init` to scaffold one"},
		{Name: "kube context", Status: deploy.StatusOK, Detail: "example-context (reachability not checked)"},
		{Name: "docker creds (Harbor)", Status: deploy.StatusFail, Detail: "missing — required by --ensure-docker-* (pods would ImagePullBackOff)", Remediation: "set HARBOR_USERNAME/HARBOR_PASSWORD"},
		{Name: "scenario placeholders", Status: deploy.StatusFail, Detail: "2 unset", Remediation: "set them in .env", Missing: []string{"ZEBRA_TOKEN", "ALPHA_SECRET"}},
	}}
}

func TestWriteDoctorReportTextMatchesRender(t *testing.T) {
	report := sampleDoctorReport()
	var want bytes.Buffer
	report.Render(&want)

	var got bytes.Buffer
	require.NoError(t, writeDoctorReport(&got, report, format.OutputText))
	assert.Equal(t, want.Bytes(), got.Bytes())
}

func TestDoctorFormatFlagDefaultsToText(t *testing.T) {
	f := newDoctorCommand().Flags().Lookup("format")
	require.NotNil(t, f)
	assert.Equal(t, format.OutputText, f.DefValue)
}

func TestWriteDoctorReportJSONGolden(t *testing.T) {
	var got bytes.Buffer
	require.NoError(t, writeDoctorReport(&got, sampleDoctorReport(), format.OutputJSON))

	golden := filepath.Join("testdata", "doctor_result.golden.json")
	if *updateDoctorGolden {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(golden, got.Bytes(), 0o600))
	}
	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	assert.Equal(t, string(want), got.String())

	var res format.Result
	require.NoError(t, json.Unmarshal(got.Bytes(), &res))
	assert.Equal(t, "deploy-camunda/v1", res.APIVersion)
	assert.Equal(t, "Result", res.Kind)
	assert.Equal(t, format.StatusFail, res.Status)
	assert.Empty(t, res.Slots)
	require.Len(t, res.Errors, 2)
	assert.Equal(t, "docker-creds-harbor", res.Errors[0].Code)
}

func TestDoctorResultEmptyReportIsOK(t *testing.T) {
	var got bytes.Buffer
	require.NoError(t, writeDoctorReport(&got, &deploy.Report{}, format.OutputJSON))
	assert.Contains(t, got.String(), `"status": "ok"`)
	assert.Contains(t, got.String(), `"slots": []`)
	assert.Contains(t, got.String(), `"errors": []`)
}

func TestDoctorJSONDoesNotLeakSecrets(t *testing.T) {
	t.Setenv("HARBOR_USERNAME", "user")
	t.Setenv("HARBOR_PASSWORD", fakeSecret)
	t.Setenv("DOCKERHUB_PASSWORD", fakeSecret)
	t.Setenv("MAPPED_SECRET", fakeSecret)
	var flags config.RuntimeFlags
	flags.Test.KubeContext = "example-context"
	flags.Docker.DockerPassword = fakeSecret
	flags.Docker.DockerHubPassword = fakeSecret
	flags.Docker.EnsureDockerRegistry = true
	flags.Docker.EnsureDockerHub = true
	flags.Secrets.VaultSecretMapping = "MAPPED_SECRET;"
	report := deploy.Preflight(t.Context(), &flags, deploy.PreflightOptions{SkipKubeReachability: true})

	var got bytes.Buffer
	require.NoError(t, writeDoctorReport(&got, report, format.OutputJSON))
	assert.NotContains(t, got.String(), fakeSecret)
	assert.False(t, strings.Contains(strings.ToLower(got.String()), "password\":"))
}

func TestDoctorCheckCode(t *testing.T) {
	cases := map[string]string{
		"docker creds (Docker Hub)": "docker-creds-docker-hub",
		"kube context":              "kube-context",
		"config file":               "config-file",
	}
	for in, want := range cases {
		assert.Equal(t, want, doctorCheckCode(in), in)
	}
}

func TestValidateOutput(t *testing.T) {
	assert.NoError(t, format.ValidateOutput("text"))
	assert.NoError(t, format.ValidateOutput("json"))
	assert.Error(t, format.ValidateOutput("yaml"))
}
