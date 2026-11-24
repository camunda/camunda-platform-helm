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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

var ghaExpression = regexp.MustCompile(`(?s)\$\{\{.*?\}\}`)

type runBlock struct {
	path      string
	step      string
	script    string
	firstLine int
}

func run(repoRoot string, out io.Writer) error {
	paths, err := git(repoRoot, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", ".github/actions")
	if err != nil {
		return err
	}
	blocks, err := runBlocksIn(repoRoot, strings.Split(paths, "\x00"))
	if err != nil {
		return err
	}

	checked := 0
	var findings []string
	for _, b := range blocks {
		checked++
		cmd := exec.Command("shellcheck", "--shell=bash", "--format=gcc", "--exclude=SC2043,SC2050,SC2157,SC2194", "-")
		cmd.Stdin = strings.NewReader(maskExpressions(b.script))
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

func runBlocksIn(repoRoot string, paths []string) ([]runBlock, error) {
	var blocks []runBlock
	for _, path := range paths {
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(repoRoot, path))
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

func parseRunBlocks(src []byte) ([]runBlock, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, err
	}
	var blocks []runBlock
	collectRunBlocks(&doc, "", &blocks)
	return blocks, nil
}

func collectRunBlocks(n *yaml.Node, path string, blocks *[]runBlock) {
	if n.Kind == yaml.DocumentNode {
		for _, c := range n.Content {
			collectRunBlocks(c, path, blocks)
		}
		return
	}
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := n.Content[i], n.Content[i+1]
		collect := collectRunBlocks
		if key.Value == "steps" && value.Kind == yaml.SequenceNode {
			collect = collectSteps
		}
		collect(value, joinStep(path, key.Value), blocks)
	}
}

func collectSteps(steps *yaml.Node, path string, blocks *[]runBlock) {
	for i, s := range steps.Content {
		run := field(s, "run")
		if run == nil || run.Kind != yaml.ScalarNode {
			continue
		}
		b := runBlock{step: joinStep(path, stepName(s, i)), script: run.Value, firstLine: run.Line}
		if run.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
			b.firstLine++
		}
		*blocks = append(*blocks, b)
	}
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

func maskExpressions(script string) string {
	return ghaExpression.ReplaceAllStringFunc(script, func(e string) string {
		parts := strings.Split(e, "\n")
		for i, p := range parts {
			parts[i] = strings.Repeat("_", len(p))
		}
		return strings.Join(parts, "\\\n")
	})
}
