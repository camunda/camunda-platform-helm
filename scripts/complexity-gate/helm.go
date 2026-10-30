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
	"io"
	"strings"
)

const (
	maxHelmBranches = 30
	maxHelmDepth    = 4
)

func helmHelp(out io.Writer) {
	fmt.Fprintf(out, `
Helm budgets: branches <= %d; control depth <= %d.
Count if/range/with actions; chained alternatives share a level.
An else arm with only one same-kind branch is a chain, including explicit else/end.
Other else bodies retain nesting.
Definitions and blocks start independent bodies; called templates are not expanded.
New or edited parsed bodies must satisfy both budgets, even when counts decrease.
Untouched legacy debt passes. Comments and expression formatting preserve eligibility.
Parse/Git errors fail. Candidate input includes staged, unstaged and new template files.
Only charts/camunda-platform-*/templates is checked; dependency charts are excluded.
Normal Helm checks print only violations and a summary; -report-helm prints an inventory.
`, maxHelmBranches, maxHelmDepth)
}

func checkHelm(repoRoot, base string, charts []string, out io.Writer) error {
	root, selected, err := helmInputs(repoRoot, charts)
	if err != nil {
		return err
	}
	baseSHA, err := git(root, "merge-base", base, "HEAD")
	if err != nil {
		return err
	}
	baseSHA = strings.TrimSpace(baseSHA)
	checked, failures := 0, 0
	for _, chart := range selected {
		before, err := helmBaseline(root, baseSHA, chart)
		if err != nil {
			return err
		}
		after, err := helmCandidate(root, chart)
		if err != nil {
			return err
		}
		failures += compareHelm(before, after, out)
		checked += len(after)
	}
	if failures != 0 {
		return fmt.Errorf("%d over-budget metric(s) in new or edited bodies; refactor cohesive responsibilities, preserve behavior, and rerun the same validation command", failures)
	}
	fmt.Fprintf(out, "helm-template-check: %d templates in %d chart(s); no over-budget new or edited bodies (base %s)\n",
		checked, len(selected), baseSHA)
	return nil
}

func compareHelm(before, after map[string]helmMeasurement, out io.Writer) int {
	failures := 0
	for _, name := range helmNames(after) {
		current := after[name]
		previous, exists := before[name]
		changed := !exists || current.body != previous.body
		for _, metric := range []struct {
			name                  string
			before, after, budget int
		}{
			{"branches", previous.branches, current.branches, maxHelmBranches},
			{"depth", previous.depth, current.depth, maxHelmDepth},
		} {
			if !helmExceedsBudget(metric.after, metric.budget, changed) {
				continue
			}
			old := "new"
			if exists {
				old = fmt.Sprint(metric.before)
			}
			fmt.Fprintf(out, "%s:%d: %s: %s %s -> %d; budget %d\n",
				current.file, current.line, current.name, metric.name, old, metric.after, metric.budget)
			failures++
		}
	}
	return failures
}

func helmExceedsBudget(value, budget int, changed bool) bool {
	return changed && value > budget
}

func helmBaseline(root, base, chart string) (map[string]helmMeasurement, error) {
	files, err := git(root, "ls-tree", "-r", "-z", "--name-only", base, "--", chart+"/templates")
	if err != nil {
		return nil, err
	}
	result := make(map[string]helmMeasurement)
	for file := range strings.SplitSeq(files, "\x00") {
		if !helmTemplateFile(file) {
			continue
		}
		text, err := git(root, "show", base+":"+file)
		if err != nil {
			return nil, err
		}
		if err := addHelmMeasurements(result, file, text); err != nil {
			return nil, fmt.Errorf("baseline %s: %w", base, err)
		}
	}
	return result, nil
}
