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
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
	"mvdan.cc/sh/v3/syntax"
)

const (
	maxRunCyclomatic = 10
	maxRunLines      = 100
)

var (
	workflowScopes = []string{".github/workflows", ".github/actions"}
	ghaExpression  = regexp.MustCompile(`(?s)\$\{\{.*?\}\}`)
)

type runBlock struct {
	step       string
	script     string
	line       int
	cyclomatic int
	lines      int
}

func checkRunBlocks(repoRoot, base string, all bool, out io.Writer) error {
	mergeBase, files, err := workflowFiles(repoRoot, base, all)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)

	checked := 0
	var violations []string
	for _, path := range paths {
		v, n, err := checkWorkflow(repoRoot, path, mergeBase, files[path])
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		checked += n
		violations = append(violations, v...)
	}

	limits := fmt.Sprintf("cyclomatic %d, %d lines", maxRunCyclomatic, maxRunLines)
	scope := "above"
	if !all {
		scope = "added or grown above"
	}
	if len(violations) == 0 {
		fmt.Fprintf(out, "complexity-gate: %d run block(s) checked, none %s %s\n", checked, scope, limits)
		return nil
	}
	for _, v := range violations {
		fmt.Fprintln(out, v)
	}
	return fmt.Errorf("%d of %d checked run block(s) %s %s; split the step or move its logic into a tool the job already runs",
		len(violations), checked, scope, limits)
}

func workflowFiles(repoRoot, base string, all bool) (string, map[string]string, error) {
	listArgs := []string{"ls-files", "-z"}
	mergeBase := ""
	files := map[string]string{}
	if !all {
		out, err := git(repoRoot, "merge-base", base, "HEAD")
		if err != nil {
			return "", nil, err
		}
		mergeBase = strings.TrimSpace(out)
		diff, err := git(repoRoot, append([]string{"diff", "--name-status", "-z", "-M", mergeBase, "--"}, workflowScopes...)...)
		if err != nil {
			return "", nil, err
		}
		fields := strings.Split(strings.TrimSuffix(diff, "\x00"), "\x00")
		for i := 0; i+1 < len(fields); i += 2 {
			switch fields[i][0] {
			case 'R':
				files[fields[i+2]] = fields[i+1]
				i++
			case 'A', 'D':
				files[fields[i+1]] = ""
			default:
				files[fields[i+1]] = fields[i+1]
			}
		}
		listArgs = append(listArgs, "--others", "--exclude-standard")
	}
	listed, err := git(repoRoot, append(append(listArgs, "--"), workflowScopes...)...)
	if err != nil {
		return "", nil, err
	}
	for path := range strings.SplitSeq(listed, "\x00") {
		files[path] = ""
	}
	return mergeBase, files, nil
}

func checkWorkflow(repoRoot, path, mergeBase, basePath string) ([]string, int, error) {
	src, err := os.ReadFile(filepath.Join(repoRoot, path))
	if os.IsNotExist(err) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	blocks, err := parseRunBlocks(src)
	if err != nil {
		return nil, 0, err
	}
	var baseBlocks []runBlock
	if basePath != "" {
		baseSrc, err := git(repoRoot, "show", mergeBase+":"+basePath)
		if err != nil {
			return nil, 0, err
		}
		if baseBlocks, err = parseRunBlocks([]byte(baseSrc)); err != nil {
			return nil, 0, fmt.Errorf("%s on %s: %w", basePath, mergeBase, err)
		}
	}
	return runBlockViolations(path, blocks, baseBlocks), len(blocks), nil
}

func runBlockViolations(path string, blocks, baseBlocks []runBlock) []string {
	headSteps := map[string]bool{}
	for _, b := range blocks {
		headSteps[b.step] = true
	}
	before := map[string]runBlock{}
	unchanged := map[string]bool{}
	renamed := map[string]bool{}
	for _, b := range baseBlocks {
		unchanged[b.step+"\x00"+b.script] = true
		if !headSteps[b.step] {
			renamed[b.script] = true
		}
		if prev, ok := before[b.step]; ok {
			b.cyclomatic, b.lines = min(b.cyclomatic, prev.cyclomatic), min(b.lines, prev.lines)
		}
		before[b.step] = b
	}

	var violations []string
	for _, b := range blocks {
		prev, known := before[b.step]
		over := b.overLimits()
		switch {
		case over == "", unchanged[b.step+"\x00"+b.script], !known && renamed[b.script]:
			continue
		case known && b.cyclomatic <= prev.cyclomatic && b.lines <= prev.lines:
			continue
		case known:
			over += fmt.Sprintf(", up from cyclomatic %d, %d lines", prev.cyclomatic, prev.lines)
		}
		violations = append(violations, fmt.Sprintf("%s:%d: %s: %s", path, b.line, b.step, over))
	}
	return violations
}

func parseRunBlocks(src []byte) ([]runBlock, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, err
	}
	var blocks []runBlock
	if err := collectRunBlocks(&doc, "", strings.Split(string(src), "\n"), &blocks); err != nil {
		return nil, err
	}
	return blocks, nil
}

func collectRunBlocks(n *yaml.Node, path string, source []string, blocks *[]runBlock) error {
	if n.Kind == yaml.DocumentNode {
		for _, c := range n.Content {
			if err := collectRunBlocks(c, path, source, blocks); err != nil {
				return err
			}
		}
		return nil
	}
	if n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := n.Content[i], n.Content[i+1]
		collect := collectRunBlocks
		if key.Value == "steps" && value.Kind == yaml.SequenceNode {
			collect = collectSteps
		}
		if err := collect(value, joinStep(path, key.Value), source, blocks); err != nil {
			return err
		}
	}
	return nil
}

func collectSteps(steps *yaml.Node, path string, source []string, blocks *[]runBlock) error {
	for i, s := range steps.Content {
		run := field(s, "run")
		if run == nil || run.Kind != yaml.ScalarNode {
			continue
		}
		b, err := measureRun(run)
		if err != nil {
			return err
		}
		if run.Style&yaml.FoldedStyle != 0 {
			b.lines = foldedLines(source[run.Line:], s.Column)
		}
		b.step = joinStep(path, stepName(s, i))
		*blocks = append(*blocks, b)
	}
	return nil
}

func foldedLines(source []string, stepColumn int) int {
	lines := 0
	for _, line := range source {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line)-len(strings.TrimLeft(line, " ")) < stepColumn {
			break
		}
		lines++
	}
	return lines
}

func field(n *yaml.Node, name string) *yaml.Node {
	if n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == name {
			return n.Content[i+1]
		}
	}
	return nil
}

func stepName(n *yaml.Node, index int) string {
	for _, name := range []string{"name", "id"} {
		if v := field(n, name); v != nil && v.Kind == yaml.ScalarNode {
			return v.Value
		}
	}
	return strconv.Itoa(index)
}

func joinStep(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "/" + child
}

func measureRun(n *yaml.Node) (runBlock, error) {
	script := ghaExpression.ReplaceAllString(n.Value, "GHA_EXPRESSION")
	file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil {
		return runBlock{}, fmt.Errorf("run block at line %d: %w", n.Line, err)
	}
	b := runBlock{script: n.Value, line: n.Line, cyclomatic: bashCyclomatic(file)}
	for line := range strings.Lines(n.Value) {
		if strings.TrimSpace(line) != "" {
			b.lines++
		}
	}
	return b, nil
}

func bashCyclomatic(file *syntax.File) int {
	complexity := 1
	syntax.Walk(file, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.IfClause:
			if len(n.Cond) > 0 {
				complexity++
			}
		case *syntax.CaseItem, *syntax.ForClause, *syntax.WhileClause:
			complexity++
		case *syntax.BinaryCmd:
			if n.Op == syntax.AndStmt || n.Op == syntax.OrStmt {
				complexity++
			}
		case *syntax.BinaryTest:
			if n.Op == syntax.AndTest || n.Op == syntax.OrTest {
				complexity++
			}
		}
		return true
	})
	return complexity
}

func (b runBlock) overLimits() string {
	var over []string
	if b.cyclomatic > maxRunCyclomatic {
		over = append(over, fmt.Sprintf("cyclomatic %d > %d", b.cyclomatic, maxRunCyclomatic))
	}
	if b.lines > maxRunLines {
		over = append(over, fmt.Sprintf("%d lines > %d", b.lines, maxRunLines))
	}
	return strings.Join(over, ", ")
}
