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

package e2erun

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type storedResult struct {
	ID       string     `json:"id"`
	Blocking bool       `json:"blocking"`
	Category string     `json:"category"`
	Error    string     `json:"error,omitempty"`
	Duration string     `json:"duration"`
	Stats    *TestStats `json:"stats,omitempty"`
	Warnings []string   `json:"warnings,omitempty"`
}

func resultPath(artifactsDir string, index int) string {
	return filepath.Join(artifactsDir, "results", fmt.Sprintf("%d.json", index))
}

// SaveResult records the outcome of leg index so a later Report call can
// aggregate legs that ran in separate workflow steps.
func SaveResult(artifactsDir string, index int, lr LegResult) error {
	stored := storedResult{
		ID: lr.Leg.ID, Blocking: lr.Leg.Blocking, Category: lr.Category,
		Duration: lr.Duration.String(), Stats: lr.Stats, Warnings: lr.Warnings,
	}
	if lr.Err != nil {
		stored.Error = lr.Err.Error()
	}
	data, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	path := resultPath(artifactsDir, index)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadResults returns the outcome of every planned leg. A leg without a
// recorded result counts as failed: not-run normally, cancelled when the job
// was cancelled (jobStatus is the workflow's job.status).
func LoadResults(artifactsDir string, legs []Leg, jobStatus string) Result {
	result := Result{}
	for i, leg := range legs {
		lr := LegResult{Leg: leg}
		data, err := os.ReadFile(resultPath(artifactsDir, i))
		var stored storedResult
		switch {
		case err != nil && jobStatus == "cancelled":
			lr.Err, lr.Category = errors.New("the job was cancelled before this leg recorded a result"), CategoryCancelled
		case err != nil:
			lr.Err, lr.Category = fmt.Errorf("no result recorded; the leg step was skipped or errored before recording, see the E2E - 🧪 Run Playwright leg %d 🧪 step", i+1), CategoryNotRun
		case json.Unmarshal(data, &stored) != nil || stored.ID != leg.ID:
			lr.Err, lr.Category = fmt.Errorf("result file does not match the planned leg"), CategoryNotRun
		default:
			lr.Category, lr.Stats, lr.Warnings = stored.Category, stored.Stats, stored.Warnings
			if stored.Error != "" {
				lr.Err = errors.New(stored.Error)
			}
			lr.Duration, _ = time.ParseDuration(stored.Duration)
		}
		result.Legs = append(result.Legs, lr)
	}
	return result
}

// MappedEnvNames returns the environment variable names a
// hashicorp/vault-action secrets list exports: the alias after "|" when
// present, the key otherwise.
func MappedEnvNames(mapping string) []string {
	names := []string{}
	for _, entry := range strings.FieldsFunc(mapping, func(r rune) bool { return r == ';' || r == '\n' }) {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		name := ""
		if _, alias, ok := strings.Cut(entry, "|"); ok {
			name = strings.TrimSpace(alias)
		} else if fields := strings.Fields(entry); len(fields) > 1 {
			name = fields[len(fields)-1]
		}
		if name != "" {
			names = append(names, name, strings.ToUpper(name))
		}
	}
	return names
}
