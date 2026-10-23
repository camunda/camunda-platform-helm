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
	ghaExpression  = regexp.MustCompile(`\$\{\{.*?\}\}`)
)

type runBlock struct {
	step       string
	line       int
	cyclomatic int
	lines      int
}

func checkRunBlocks(repoRoot string, all bool, out io.Writer) error {
	if !all {
		return nil
	}
	tracked, err := git(repoRoot, append([]string{"ls-files", "-z", "--"}, workflowScopes...)...)
	if err != nil {
		return err
	}

	checked := 0
	var violations []string
	for path := range strings.SplitSeq(tracked, "\x00") {
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			continue
		}
		v, n, err := checkWorkflow(repoRoot, path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		checked += n
		violations = append(violations, v...)
	}

	limits := fmt.Sprintf("cyclomatic %d, %d lines", maxRunCyclomatic, maxRunLines)
	if len(violations) == 0 {
		fmt.Fprintf(out, "complexity-gate: %d run block(s) checked, none above %s\n", checked, limits)
		return nil
	}
	for _, v := range violations {
		fmt.Fprintln(out, v)
	}
	return fmt.Errorf("%d of %d checked run block(s) above %s; split the step or move its logic into a tool the job already runs",
		len(violations), checked, limits)
}

func checkWorkflow(repoRoot, path string) ([]string, int, error) {
	src, err := os.ReadFile(filepath.Join(repoRoot, path))
	if err != nil {
		return nil, 0, err
	}
	blocks, err := parseRunBlocks(src)
	if err != nil {
		return nil, 0, err
	}
	var violations []string
	for _, b := range blocks {
		if over := b.overLimits(); over != "" {
			violations = append(violations, fmt.Sprintf("%s:%d: %s: %s", path, b.line, b.step, over))
		}
	}
	return violations, len(blocks), nil
}

func parseRunBlocks(src []byte) ([]runBlock, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, err
	}
	var blocks []runBlock
	if err := collectRunBlocks(&doc, "", &blocks); err != nil {
		return nil, err
	}
	return blocks, nil
}

func collectRunBlocks(n *yaml.Node, step string, blocks *[]runBlock) error {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			if err := collectRunBlocks(c, step, blocks); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for i, c := range n.Content {
			if err := collectRunBlocks(c, joinStep(step, stepName(c, i)), blocks); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, value := n.Content[i], n.Content[i+1]
			if key.Value == "run" && value.Kind == yaml.ScalarNode {
				b, err := measureRun(value)
				if err != nil {
					return err
				}
				b.step = step
				*blocks = append(*blocks, b)
				continue
			}
			if err := collectRunBlocks(value, joinStep(step, key.Value), blocks); err != nil {
				return err
			}
		}
	}
	return nil
}

func stepName(n *yaml.Node, index int) string {
	if n.Kind == yaml.MappingNode {
		for _, field := range []string{"name", "id"} {
			for i := 0; i+1 < len(n.Content); i += 2 {
				if n.Content[i].Value == field && n.Content[i+1].Kind == yaml.ScalarNode {
					return n.Content[i+1].Value
				}
			}
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
	b := runBlock{line: n.Line, cyclomatic: bashCyclomatic(file)}
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
