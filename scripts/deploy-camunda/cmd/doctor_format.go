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
	"fmt"
	"io"
	"strings"

	"scripts/deploy-camunda/deploy"
	"scripts/deploy-camunda/format"
)

func validateDoctorFlags(outputFormat string, fix bool) error {
	if err := format.ValidateOutput(outputFormat); err != nil {
		return err
	}
	if fix && outputFormat == format.OutputJSON {
		return fmt.Errorf("--fix is interactive and cannot be combined with --format json")
	}
	return nil
}

type doctorCheckOutput struct {
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Detail      string   `json:"detail"`
	Remediation string   `json:"remediation,omitempty"`
	Missing     []string `json:"missing,omitempty"`
}

// doctorResult maps a preflight report onto the shared envelope:
// every check goes to outputs.checks, outputs.missingEnv holds the union of
// unset variable names, and each failed check becomes one entry in errors.
func doctorResult(report *deploy.Report) format.Result {
	res := format.NewResult()
	checks := make([]doctorCheckOutput, 0, len(report.Checks))
	for _, c := range report.Checks {
		checks = append(checks, doctorCheckOutput{
			Name:        c.Name,
			Status:      string(c.Status),
			Detail:      c.Detail,
			Remediation: c.Remediation,
			Missing:     c.Missing,
		})
		if c.Status == deploy.StatusFail {
			res.Errors = append(res.Errors, format.Error{
				Code:        doctorCheckCode(c.Name),
				Message:     c.Name + ": " + c.Detail,
				Remediation: c.Remediation,
			})
		}
	}
	res.Outputs["checks"] = checks
	res.Outputs["missingEnv"] = report.MissingEnv()
	if !report.OK() {
		res.Status = format.StatusFail
	}
	return res
}

// doctorCheckCode turns a check name like "docker creds (Harbor)" into "docker-creds-harbor".
func doctorCheckCode(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

func writeDoctorReport(w io.Writer, report *deploy.Report, outputFormat string) error {
	if outputFormat == format.OutputJSON {
		return format.WriteJSON(w, doctorResult(report))
	}
	var buf bytes.Buffer
	report.Render(&buf)
	_, err := io.WriteString(w, buf.String())
	return err
}
