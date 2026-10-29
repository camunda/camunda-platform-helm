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
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template/parse"
)

type helmMeasurement struct {
	file, name      string
	line            int
	branches, depth int
}

func reportHelmCharts(repoRoot string, charts []string, out io.Writer) error {
	root, selected, err := helmInputs(repoRoot, charts)
	if err != nil {
		return err
	}
	checked := 0
	for _, chart := range selected {
		bodies, err := helmCandidate(root, chart)
		if err != nil {
			return err
		}
		for _, name := range helmNames(bodies) {
			body := bodies[name]
			fmt.Fprintf(out, "%s:%d: %s branches=%d depth=%d\n",
				body.file, body.line, body.name, body.branches, body.depth)
			checked++
		}
	}
	fmt.Fprintf(out, "helm-template-check: %d bodies in %d Camunda chart(s); dependency charts excluded\n", checked, len(selected))
	return nil
}

func helmInputs(repoRoot string, charts []string) (string, []string, error) {
	if len(charts) == 0 {
		return "", nil, fmt.Errorf("at least one chart path is required")
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", nil, err
	}
	var selected []string
	for _, chart := range charts {
		absolute := chart
		if !filepath.IsAbs(absolute) {
			absolute = filepath.Join(root, chart)
		}
		relative, err := filepath.Rel(root, absolute)
		if err != nil {
			return "", nil, err
		}
		relative = filepath.ToSlash(relative)
		if relative == ".." || strings.HasPrefix(relative, "../") {
			return "", nil, fmt.Errorf("chart %q is outside the repository", chart)
		}
		if _, err := os.Stat(filepath.Join(absolute, "Chart.yaml")); err != nil {
			return "", nil, fmt.Errorf("chart %q: %w", chart, err)
		}
		parts := strings.Split(relative, "/")
		if len(parts) == 2 && parts[0] == "charts" && strings.HasPrefix(parts[1], "camunda-platform-") {
			selected = append(selected, relative)
		}
	}
	return root, selected, nil
}

func helmCandidate(root, chart string) (map[string]helmMeasurement, error) {
	result := make(map[string]helmMeasurement)
	err := filepath.WalkDir(filepath.Join(root, chart, "templates"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !helmTemplateFile(path) {
			return nil
		}
		file, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return addHelmMeasurements(result, filepath.ToSlash(file), string(text))
	})
	if err != nil {
		return nil, fmt.Errorf("candidate %s: %w", chart, err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("candidate %s has no template source files", chart)
	}
	return result, nil
}

func helmTemplateFile(path string) bool {
	switch filepath.Ext(path) {
	case ".yaml", ".yml", ".tpl", ".txt":
		return true
	default:
		return false
	}
}

func helmNames(bodies map[string]helmMeasurement) []string {
	names := make([]string, 0, len(bodies))
	for name := range bodies {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func addHelmMeasurements(result map[string]helmMeasurement, file, text string) error {
	trees := make(map[string]*parse.Tree)
	tree := parse.New(file)
	tree.Mode = parse.SkipFuncCheck
	if _, err := tree.Parse(text, "{{", "}}", trees); err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	for name, tree := range trees {
		id, display := "helper:"+name, name
		if name == file {
			id, display = "root:"+file, "<root>"
		}
		if previous, ok := result[id]; ok {
			return fmt.Errorf("ambiguous template %q in %s and %s", name, previous.file, file)
		}
		current := helmMeasurement{
			file: file,
			name: display,
			line: 1 + strings.Count(text[:int(tree.Root.Position())], "\n"),
		}
		countHelm(tree.Root, 0, &current)
		result[id] = current
	}
	return nil
}

func countHelm(list *parse.ListNode, depth int, result *helmMeasurement) {
	if list == nil {
		return
	}
	for _, node := range list.Nodes {
		var branch *parse.BranchNode
		switch node := node.(type) {
		case *parse.IfNode:
			branch = &node.BranchNode
		case *parse.RangeNode:
			branch = &node.BranchNode
		case *parse.WithNode:
			branch = &node.BranchNode
		default:
			continue
		}
		result.branches++
		result.depth = max(result.depth, depth+1)
		countHelm(branch.List, depth+1, result)
		alternativeDepth := depth + 1
		if (node.Type() == parse.NodeIf || node.Type() == parse.NodeWith) && helmAlternative(branch.ElseList, node.Type()) {
			alternativeDepth = depth
		}
		countHelm(branch.ElseList, alternativeDepth, result)
	}
}

func helmAlternative(list *parse.ListNode, kind parse.NodeType) bool {
	if list == nil {
		return false
	}
	found := false
	for _, node := range list.Nodes {
		if text, ok := node.(*parse.TextNode); ok && strings.TrimSpace(string(text.Text)) == "" {
			continue
		}
		if found || node.Type() != kind {
			return false
		}
		found = true
	}
	return found
}
