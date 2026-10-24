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
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	path       string
	step       string
	script     string
	line       int
	firstLine  int
	cyclomatic int
	lines      int
}

func checkRunBlocks(repoRoot, base string, all bool, out io.Writer) error {
	head, before, err := changedRunBlocks(repoRoot, base, all)
	if err != nil {
		return err
	}
	violations := runBlockViolations(head, before)

	limits := fmt.Sprintf("cyclomatic %d, %d lines", maxRunCyclomatic, maxRunLines)
	scope := "above"
	if !all {
		scope = "added or grown above"
	}
	var complexityErr error
	if len(violations) == 0 {
		fmt.Fprintf(out, "complexity-gate: %d run block(s) checked, none %s %s\n", len(head), scope, limits)
	} else {
		for _, v := range violations {
			fmt.Fprintln(out, v)
		}
		complexityErr = fmt.Errorf("%d of %d checked run block(s) %s %s; split the step or delete code (see .github/instructions/github-actions.instructions.md)",
			len(violations), len(head), scope, limits)
	}
	return errors.Join(complexityErr, shellcheckActions(head, out))
}

func shellcheckActions(blocks []runBlock, out io.Writer) error {
	checked := 0
	var findings []string
	for _, b := range blocks {
		if !strings.HasPrefix(b.path, ".github/actions/") {
			continue
		}
		checked++
		cmd := exec.Command("shellcheck", "--shell=bash", "--format=gcc", "-")
		cmd.Stdin = strings.NewReader(ghaExpression.ReplaceAllLiteralString(b.script, "${GHA_EXPRESSION}"))
		report, err := cmd.Output()
		var exitErr *exec.ExitError
		if err != nil && (!errors.As(err, &exitErr) || exitErr.ExitCode() != 1) {
			return fmt.Errorf("shellcheck %s: %w", b.path, err)
		}
		for line := range strings.Lines(string(report)) {
			fields := strings.SplitN(strings.TrimSpace(line), ":", 4)
			if len(fields) < 4 {
				continue
			}
			n, _ := strconv.Atoi(fields[1])
			findings = append(findings, fmt.Sprintf("%s:%d: %s:%s", b.path, b.firstLine+n-1, b.step, fields[3]))
		}
	}
	if len(findings) == 0 {
		fmt.Fprintf(out, "complexity-gate: %d composite action run block(s) checked, no shellcheck finding\n", checked)
		return nil
	}
	for _, f := range findings {
		fmt.Fprintln(out, f)
	}
	return fmt.Errorf("shellcheck reports %d finding(s) in %d checked composite action run block(s)", len(findings), checked)
}

func changedRunBlocks(repoRoot, base string, all bool) (head, before []runBlock, err error) {
	readHead := func(path string) ([]byte, error) { return os.ReadFile(filepath.Join(repoRoot, path)) }
	if all {
		tracked, err := git(repoRoot, append([]string{"ls-files", "-z", "--cached", "--others", "--exclude-standard", "--"}, workflowScopes...)...)
		if err != nil {
			return nil, nil, err
		}
		head, err = runBlocksIn(strings.Split(tracked, "\x00"), readHead)
		return head, nil, err
	}

	mergeBase, err := git(repoRoot, "merge-base", base, "HEAD")
	if err != nil {
		return nil, nil, err
	}
	mergeBase = strings.TrimSpace(mergeBase)
	diff, err := git(repoRoot, append([]string{"diff", "--name-status", "-z", "--no-renames", mergeBase, "--"}, workflowScopes...)...)
	if err != nil {
		return nil, nil, err
	}
	untracked, err := git(repoRoot, append([]string{"ls-files", "-z", "--others", "--exclude-standard", "--"}, workflowScopes...)...)
	if err != nil {
		return nil, nil, err
	}
	var modified, added, deleted []string
	fields := strings.Split(strings.TrimSuffix(diff, "\x00"), "\x00")
	for i := 0; i+1 < len(fields); i += 2 {
		switch fields[i][0] {
		case 'A':
			added = append(added, fields[i+1])
		case 'D':
			deleted = append(deleted, fields[i+1])
		default:
			modified = append(modified, fields[i+1])
		}
	}
	added = append(added, strings.Split(untracked, "\x00")...)

	if head, err = runBlocksIn(append(modified, added...), readHead); err != nil {
		return nil, nil, err
	}
	before, err = runBlocksIn(append(modified, deleted...), func(path string) ([]byte, error) {
		src, err := git(repoRoot, "show", mergeBase+":"+path)
		return []byte(src), err
	})
	if err != nil {
		return nil, nil, fmt.Errorf("on %s: %w", mergeBase, err)
	}
	return head, before, nil
}

func runBlocksIn(paths []string, read func(string) ([]byte, error)) ([]runBlock, error) {
	var blocks []runBlock
	for _, path := range paths {
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			continue
		}
		src, err := read(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		fileBlocks, err := parseRunBlocks(src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for i := range fileBlocks {
			fileBlocks[i].path = path
		}
		blocks = append(blocks, fileBlocks...)
	}
	return blocks, nil
}

func runBlockViolations(head, before []runBlock) []string {
	headSteps := map[string]bool{}
	for _, b := range head {
		headSteps[b.step] = true
	}
	unchanged := map[string]int{}
	renamed := map[string]int{}
	slots := map[string][]runBlock{}
	for _, b := range before {
		unchanged[b.step+"\x00"+b.script]++
		if headSteps[b.step] {
			slots[b.step] = append(slots[b.step], b)
		} else {
			renamed[b.script]++
		}
	}
	for _, blocks := range slots {
		slices.SortFunc(blocks, func(a, b runBlock) int {
			return cmp.Or(cmp.Compare(a.cyclomatic, b.cyclomatic), cmp.Compare(a.lines, b.lines))
		})
	}

	var changed []runBlock
	for _, b := range head {
		key := b.step + "\x00" + b.script
		if unchanged[key] == 0 {
			changed = append(changed, b)
			continue
		}
		unchanged[key]--
		i := slices.IndexFunc(slots[b.step], func(s runBlock) bool { return s.script == b.script })
		slots[b.step] = slices.Delete(slots[b.step], i, i+1)
	}

	var violations []string
	for _, b := range changed {
		over := b.overLimits()
		if over == "" {
			continue
		}
		if renamed[b.script] > 0 {
			renamed[b.script]--
			continue
		}
		if i := slices.IndexFunc(slots[b.step], func(s runBlock) bool {
			return b.cyclomatic <= s.cyclomatic && b.lines <= s.lines
		}); i >= 0 {
			slots[b.step] = slices.Delete(slots[b.step], i, i+1)
			continue
		}
		if prev, ok := smallest(slots[b.step]); ok {
			over += fmt.Sprintf(", up from cyclomatic %d, %d lines", prev.cyclomatic, prev.lines)
		}
		violations = append(violations, fmt.Sprintf("%s:%d: %s: %s", b.path, b.line, b.step, over))
	}
	return violations
}

func smallest(blocks []runBlock) (runBlock, bool) {
	if len(blocks) == 0 {
		return runBlock{}, false
	}
	s := blocks[0]
	for _, b := range blocks[1:] {
		s.cyclomatic, s.lines = min(s.cyclomatic, b.cyclomatic), min(s.lines, b.lines)
	}
	return s, true
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
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		if len(line)-len(strings.TrimLeft(line, " ")) < stepColumn {
			break
		}
		if !strings.HasPrefix(text, "#") {
			lines++
		}
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
	file, err := syntax.NewParser(syntax.KeepComments(true)).Parse(strings.NewReader(script), "")
	if err != nil {
		return runBlock{}, fmt.Errorf("run block at line %d: %w", n.Line, err)
	}
	commentLines := map[uint]bool{}
	syntax.Walk(file, func(node syntax.Node) bool {
		if c, ok := node.(*syntax.Comment); ok {
			commentLines[c.Pos().Line()] = true
		}
		return true
	})
	b := runBlock{script: n.Value, line: n.Line, firstLine: n.Line, cyclomatic: bashCyclomatic(file)}
	if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		b.firstLine++
	}
	var number uint
	for line := range strings.Lines(script) {
		number++
		text := strings.TrimSpace(line)
		if text != "" && (!commentLines[number] || !strings.HasPrefix(text, "#")) {
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
		case *syntax.BinaryArithm:
			if n.Op == syntax.AndArit || n.Op == syntax.OrArit {
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
