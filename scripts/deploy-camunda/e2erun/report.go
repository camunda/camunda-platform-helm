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
	"fmt"
	"os"
	"strings"
)

const maxListedTests = 20

// TestStats summarises a Playwright JSON report. Only test titles are kept:
// error messages can carry URLs and credentials, and the job summary and
// annotations are not secret-masked the way step logs are.
type TestStats struct {
	Passed       int      `json:"passed"`
	Failed       int      `json:"failed"`
	Flaky        int      `json:"flaky"`
	Skipped      int      `json:"skipped"`
	GlobalErrors int      `json:"global_errors"`
	FailedTests  []string `json:"failed_tests,omitempty"`
	FlakyTests   []string `json:"flaky_tests,omitempty"`
}

// Executed is the number of tests that ran.
func (s *TestStats) Executed() int {
	if s == nil {
		return 0
	}
	return s.Passed + s.Failed + s.Flaky
}

func (s *TestStats) String() string {
	if s == nil {
		return "no results"
	}
	return fmt.Sprintf("%d passed, %d failed, %d flaky, %d skipped", s.Passed, s.Failed, s.Flaky, s.Skipped)
}

type pwReport struct {
	Suites []pwSuite  `json:"suites"`
	Errors []struct{} `json:"errors"`
	Stats  struct {
		Expected   int `json:"expected"`
		Unexpected int `json:"unexpected"`
		Flaky      int `json:"flaky"`
		Skipped    int `json:"skipped"`
	} `json:"stats"`
}

type pwSuite struct {
	Title  string    `json:"title"`
	Specs  []pwSpec  `json:"specs"`
	Suites []pwSuite `json:"suites"`
}

type pwSpec struct {
	Title string `json:"title"`
	File  string `json:"file"`
	Line  int    `json:"line"`
	Tests []struct {
		ProjectName string `json:"projectName"`
		Status      string `json:"status"`
	} `json:"tests"`
}

// ReadTestStats parses the Playwright JSON reporter output at path.
func ReadTestStats(path string) (*TestStats, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var report pwReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	stats := &TestStats{
		Passed:       report.Stats.Expected,
		Failed:       report.Stats.Unexpected,
		Flaky:        report.Stats.Flaky,
		Skipped:      report.Stats.Skipped,
		GlobalErrors: len(report.Errors),
	}
	for _, suite := range report.Suites {
		walkSuite(suite, nil, stats)
	}
	return stats, nil
}

func walkSuite(suite pwSuite, describe []string, stats *TestStats) {
	for _, spec := range suite.Specs {
		for _, test := range spec.Tests {
			title := specTitle(spec, describe, test.ProjectName)
			switch test.Status {
			case "unexpected":
				stats.FailedTests = append(stats.FailedTests, title)
			case "flaky":
				stats.FlakyTests = append(stats.FlakyTests, title)
			}
		}
	}
	for _, child := range suite.Suites {
		walkSuite(child, append(append([]string{}, describe...), child.Title), stats)
	}
}

func specTitle(spec pwSpec, describe []string, project string) string {
	parts := append(append([]string{}, describe...), spec.Title)
	title := fmt.Sprintf("%s:%d › %s", spec.File, spec.Line, strings.Join(parts, " › "))
	if project != "" {
		title += " [" + project + "]"
	}
	return title
}

// Summary renders the Markdown step summary of all legs.
func Summary(r Result) string {
	var b strings.Builder
	b.WriteString("## E2E legs\n\n")
	b.WriteString("| # | Leg | Namespace | Blocking | Result | Tests | Duration |\n|---|---|---|---|---|---|---|\n")
	for i, l := range r.Legs {
		result := l.Category
		if l.Failed() {
			result = "❌ " + result
		} else if len(l.Warnings) > 0 {
			result = "⚠️ " + result
		} else {
			result = "✅ " + result
		}
		fmt.Fprintf(&b, "| %d | `%s` | `%s` | %t | %s | %s | %s |\n", i+1, l.Leg.ID, l.Leg.Namespace, l.Leg.Blocking, result, l.Stats, l.Duration)
	}

	for i, l := range r.Legs {
		if !l.Failed() && len(l.Warnings) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n### Leg %d `%s`\n\n", i+1, l.Leg.ID)
		if l.Failed() {
			fmt.Fprintf(&b, "**%s**: %s\n\n", l.Category, l.Err)
			fmt.Fprintf(&b, "Full output is in the `E2E - 🧪 Run Playwright leg %d 🧪` step log.\n", i+1)
		}
		if l.Stats != nil {
			writeList(&b, "Failed tests", l.Stats.FailedTests)
			writeList(&b, "Flaky tests", l.Stats.FlakyTests)
		}
		writeList(&b, "Warnings", l.Warnings)
	}

	if r.BlockingFailed() || r.NonBlockingFailed() {
		b.WriteString("\n**Artifacts:** `e2e-html-report-*` (merged report), `playwright-traces-*` (traces and screenshots), " +
			"`playwright-results-json-*` (per-leg JSON results), `diagnostics-e2e-*` (namespace state at the time of each failed leg).\n")
	}
	return b.String()
}

func writeList(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s:\n\n", title)
	for i, item := range items {
		if i == maxListedTests {
			fmt.Fprintf(b, "- ... and %d more\n", len(items)-maxListedTests)
			break
		}
		fmt.Fprintf(b, "- `%s`\n", item)
	}
}

// Annotations returns GitHub workflow commands that surface each failed leg
// and each warning on the run page. Blocking failures are errors; everything
// else is a warning.
func Annotations(r Result) []string {
	out := []string{}
	for _, l := range r.Legs {
		title := "E2E leg " + l.Leg.ID
		if l.Failed() {
			level := "warning"
			if l.Leg.Blocking {
				level = "error"
			}
			out = append(out, workflowCommand(level, title+" "+l.Category, l.Err.Error()))
		}
		for _, w := range l.Warnings {
			out = append(out, workflowCommand("warning", title, w))
		}
	}
	return out
}

func workflowCommand(level, title, message string) string {
	escape := strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	property := strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
	return fmt.Sprintf("::%s title=%s::%s", level, property.Replace(title), escape.Replace(message))
}
