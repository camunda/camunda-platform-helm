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
	"fmt"
	"sort"
	"strings"
)

// Report is the aggregate of every path run for one transition.
type Report struct {
	Transition string       `json:"transition"`
	From       string       `json:"from"`
	To         string       `json:"to"`
	Stage      string       `json:"stage"`
	Results    []PathResult `json:"results"`
}

// ExitCode returns 1 for UNREMEDIATED or STALE_DELTA outcomes, 0 otherwise.
// REMEDIATED is not a failure.
func (r Report) ExitCode() int {
	for _, res := range r.Results {
		if res.Outcome == OutcomeUnremediated || res.Outcome == OutcomeStaleDelta {
			return 1
		}
	}
	return 0
}

// Counts summarises outcomes for the header line.
func (r Report) Counts() map[Outcome]int {
	m := map[Outcome]int{}
	for _, res := range r.Results {
		m[res.Outcome]++
	}
	return m
}

// Markdown renders the human-facing report.
func (r Report) Markdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Upgrade path report — %s → %s\n\n", r.From, r.To)
	fmt.Fprintf(&b, "Stage: `%s`\n\n", r.Stage)

	c := r.Counts()
	fmt.Fprintf(&b, "**%d clean · %d remediated · %d unremediated · %d stale**\n\n",
		c[OutcomeClean], c[OutcomeRemediated], c[OutcomeUnremediated], c[OutcomeStaleDelta])

	if c[OutcomeClean] == len(r.Results) && len(r.Results) > 0 {
		b.WriteString("> All paths upgrade with no customer action required.\n\n")
	}

	b.WriteString("## Summary\n\n")
	b.WriteString("| Path | Run A (naïve) | Run B (remediated) | Outcome | Customer action |\n")
	b.WriteString("|---|---|---|---|---|\n")

	results := append([]PathResult{}, r.Results...)
	sort.Slice(results, func(i, j int) bool { return results[i].Path < results[j].Path })

	for _, res := range results {
		action := "none"
		switch res.Outcome {
		case OutcomeRemediated:
			action = "values change required"
			if res.HasRemedy {
				action = "values change **+ out-of-band procedure**"
			}
		case OutcomeUnremediated:
			action = "**unknown — no remedy**"
		case OutcomeStaleDelta:
			action = "_fixture bug_"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n",
			res.Path, mark(res.RunA.Succeeded), mark(res.RunB.Succeeded), res.Outcome, action)
	}
	b.WriteString("\n")

	for _, res := range results {
		d := res.Discovery
		if d == nil || len(d.Changes) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## Required values changes — `%s`\n\n", res.Path)
		if d.Truncated {
			b.WriteString("> ⚠️ Discovery hit its round limit — this list is **incomplete**.\n\n")
		}
		cov := res.DocsCoverage
		b.WriteString("| Key | Change | Action | Documented |\n|---|---|---|---|\n")
		for _, c := range d.Changes {
			action := "delete; migrate to an external service if applicable"
			kind := "removed"
			if c.Kind == "renamed" {
				kind = "renamed"
				action = fmt.Sprintf("move value to `%s`", c.New)
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", c.Old, kind, action, docMark(cov, c.Old))
		}
		b.WriteString("\n")

		if cov.Checked {
			keys := make([]string, 0, len(d.Changes))
			for _, c := range d.Changes {
				keys = append(keys, c.Old)
			}
			if n := cov.UndocumentedCount(keys); n > 0 {
				fmt.Fprintf(&b, "> ⚠️ **%d of %d changes are not named in the published upgrade guide** "+
					"(%s). A customer following the guide alone will not learn about them.\n\n",
					n, len(keys), strings.Join(cov.Guides, ", "))
			} else {
				fmt.Fprintf(&b, "All changes are named in the published upgrade guide (%s).\n\n",
					strings.Join(cov.Guides, ", "))
			}
		} else {
			b.WriteString("_Doc coverage not checked: no upgrade guide found for this transition._\n\n")
		}
		if len(res.ScaffoldingKeys) > 0 {
			fmt.Fprintf(&b, "> Harness-only, **not** customer steps: %s. These exist in the CI "+
				"scenario's values, not in the chart, and must be excluded from the upgrade guide.\n\n",
				"`"+strings.Join(res.ScaffoldingKeys, "`, `")+"`")
		}
		if d.Final.Succeeded {
			b.WriteString("After these changes the chart renders. Note that a clean render proves only " +
				"that values are accepted — not that data survives the upgrade.\n\n")
		} else if d.Residual != "" {
			fmt.Fprintf(&b, "**Not resolved by values changes alone:** %s\n\n", d.Residual)
		}
	}

	var findings []Finding
	for _, res := range results {
		findings = append(findings, res.Findings...)
	}
	if len(findings) == 0 {
		return b.String()
	}

	sort.SliceStable(findings, func(i, j int) bool {
		return severityRank(findings[i].Severity) < severityRank(findings[j].Severity)
	})

	b.WriteString("## Findings\n\n")
	for _, f := range findings {
		fmt.Fprintf(&b, "### `%s` — %s\n\n", f.Path, f.Title)
		fmt.Fprintf(&b, "- **Class:** %s · **Severity:** %s · **Fingerprint:** `%s`\n",
			f.Class, f.Severity, f.Fingerprint)
		if f.Evidence.Source != "" {
			fmt.Fprintf(&b, "- **Raised by:** `%s`\n", f.Evidence.Source)
		}
		switch {
		case f.Remedy.Kind == "none":
			b.WriteString("- **Remedy:** none known — escalate\n")
		case f.Remedy.OutOfBand:
			fmt.Fprintf(&b, "- **Remedy:** values delta **plus an out-of-band procedure** (`%s`)\n", f.Remedy.Diff)
		case f.Remedy.Available:
			fmt.Fprintf(&b, "- **Remedy:** values delta (`%s`)\n", f.Remedy.Diff)
		default:
			b.WriteString("- **Remedy:** fix the fixture\n")
		}
		if f.Evidence.Error != "" {
			b.WriteString("\n<details><summary>Error the customer sees</summary>\n\n```\n")
			b.WriteString(f.Evidence.Error)
			b.WriteString("\n```\n\n</details>\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

// docMark renders a coverage cell, distinguishing unchecked from undocumented.
func docMark(cov DocsCoverage, key string) string {
	if !cov.Checked {
		return "—"
	}
	if cov.Documented[key] {
		return "yes"
	}
	return "**NO**"
}

func mark(ok bool) string {
	if ok {
		return "✅"
	}
	return "❌"
}

func severityRank(s string) int {
	switch s {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
}
