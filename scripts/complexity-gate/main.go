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
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/fzipp/gocyclo"
	"github.com/uudashr/gocognit"
)

const (
	maxCognitive  = 30
	maxCyclomatic = 30
	maxLines      = 250
	scope         = "scripts"
)

type hunk struct {
	start, count int
}

var wholeFile = []hunk{{start: 1, count: math.MaxInt32}}

func main() {
	repoRoot := flag.String("repo-root", ".", "path to the camunda-platform-helm repo root")
	base := flag.String("base", "origin/main", "git ref; the change is the diff from its merge base with HEAD to the working tree")
	all := flag.Bool("all", false, "check every function under scripts/ and every workflow run block, changed or not")
	flag.Parse()

	if err := run(*repoRoot, *base, *all, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func run(repoRoot, base string, all bool, out io.Writer) error {
	return errors.Join(checkFuncs(repoRoot, base, all, out), checkRunBlocks(repoRoot, base, all, out))
}

func checkFuncs(repoRoot, base string, all bool, out io.Writer) error {
	var files map[string][]hunk
	var err error
	if all {
		files, err = trackedFiles(repoRoot)
	} else {
		files, err = changedFiles(repoRoot, base)
	}
	if err != nil {
		return err
	}

	paths := make([]string, 0, len(files))
	for path := range files {
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)

	checked := 0
	var violations []string
	for _, path := range paths {
		v, n, err := checkFile(repoRoot, path, files[path])
		if err != nil {
			return err
		}
		checked += n
		violations = append(violations, v...)
	}

	limits := fmt.Sprintf("cognitive %d, cyclomatic %d, %d lines", maxCognitive, maxCyclomatic, maxLines)
	if len(violations) == 0 {
		fmt.Fprintf(out, "complexity-gate: %d function(s) checked, none above %s\n", checked, limits)
		return nil
	}
	for _, v := range violations {
		fmt.Fprintln(out, v)
	}
	return fmt.Errorf("%d of %d checked function(s) above %s; split them", len(violations), checked, limits)
}

func changedFiles(repoRoot, base string) (map[string][]hunk, error) {
	mergeBase, err := git(repoRoot, "merge-base", base, "HEAD")
	if err != nil {
		return nil, err
	}
	diff, err := git(repoRoot, "diff", "--unified=0", "--no-color", "--no-ext-diff", "-M", strings.TrimSpace(mergeBase), "--", scope)
	if err != nil {
		return nil, err
	}
	files, err := parseDiff(diff)
	if err != nil {
		return nil, err
	}
	untracked, err := git(repoRoot, "ls-files", "-z", "--others", "--exclude-standard", "--", scope)
	if err != nil {
		return nil, err
	}
	for path := range strings.SplitSeq(untracked, "\x00") {
		files[path] = wholeFile
	}
	return files, nil
}

func trackedFiles(repoRoot string) (map[string][]hunk, error) {
	tracked, err := git(repoRoot, "ls-files", "-z", "--", scope)
	if err != nil {
		return nil, err
	}
	files := map[string][]hunk{}
	for path := range strings.SplitSeq(tracked, "\x00") {
		files[path] = wholeFile
	}
	return files, nil
}

func parseDiff(diff string) (map[string][]hunk, error) {
	files := map[string][]hunk{}
	path := ""
	inHeader := false
	for line := range strings.Lines(diff) {
		line = strings.TrimRight(line, "\n")
		switch {
		case strings.HasPrefix(line, "diff --git "):
			path, inHeader = "", true
		case inHeader && strings.HasPrefix(line, "+++ "):
			p, err := diffPath(strings.TrimPrefix(line, "+++ "))
			if err != nil {
				return nil, err
			}
			path = p
		case strings.HasPrefix(line, "@@ "):
			inHeader = false
			h, err := parseHunk(line)
			if err != nil {
				return nil, err
			}
			files[path] = append(files[path], h)
		}
	}
	return files, nil
}

func diffPath(name string) (string, error) {
	name = strings.TrimSuffix(name, "\t")
	if strings.HasPrefix(name, `"`) {
		unquoted, err := strconv.Unquote(name)
		if err != nil {
			return "", fmt.Errorf("diff path %q: %w", name, err)
		}
		name = unquoted
	}
	return strings.TrimPrefix(name, "b/"), nil
}

func parseHunk(line string) (hunk, error) {
	fields := strings.Fields(line)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "+") {
		return hunk{}, fmt.Errorf("unexpected hunk header %q", line)
	}
	startText, countText, hasCount := strings.Cut(fields[2][1:], ",")
	start, err := strconv.Atoi(startText)
	if err != nil {
		return hunk{}, fmt.Errorf("hunk header %q: %w", line, err)
	}
	count := 1
	if hasCount {
		if count, err = strconv.Atoi(countText); err != nil {
			return hunk{}, fmt.Errorf("hunk header %q: %w", line, err)
		}
	}
	return hunk{start: start, count: count}, nil
}

func touches(hunks []hunk, from, to int) bool {
	for _, h := range hunks {
		if h.count == 0 {
			if h.start >= from && h.start < to {
				return true
			}
			continue
		}
		if h.start <= to && h.start+h.count-1 >= from {
			return true
		}
	}
	return false
}

func checkFile(repoRoot, path string, hunks []hunk) ([]string, int, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(repoRoot, path), nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, 0, err
	}
	checked := 0
	var violations []string
	for _, fn := range topLevelFuncs(file) {
		from, to := fset.Position(fn.Pos()).Line, fset.Position(fn.End()).Line
		if !touches(hunks, from, to) {
			continue
		}
		checked++
		if over := overLimits(fn, to-from+1); over != "" {
			violations = append(violations, fmt.Sprintf("%s:%d: %s: %s", path, from, funcName(fn), over))
		}
	}
	return violations, checked, nil
}

func topLevelFuncs(file *ast.File) []*ast.FuncDecl {
	var fns []*ast.FuncDecl
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Body != nil {
				fns = append(fns, d)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, value := range vs.Values {
					name := vs.Names[min(i, len(vs.Names)-1)]
					ast.Inspect(value, func(n ast.Node) bool {
						lit, ok := n.(*ast.FuncLit)
						if ok {
							fns = append(fns, &ast.FuncDecl{Name: name, Type: lit.Type, Body: lit.Body})
						}
						return !ok
					})
				}
			}
		}
	}
	return fns
}

func overLimits(fn *ast.FuncDecl, lines int) string {
	var over []string
	if c := gocognit.Complexity(fn); c > maxCognitive {
		over = append(over, fmt.Sprintf("cognitive %d > %d", c, maxCognitive))
	}
	if c := gocyclo.Complexity(fn); c > maxCyclomatic {
		over = append(over, fmt.Sprintf("cyclomatic %d > %d", c, maxCyclomatic))
	}
	if lines > maxLines {
		over = append(over, fmt.Sprintf("%d lines > %d", lines, maxLines))
	}
	return strings.Join(over, ", ")
}

func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return fmt.Sprintf("(%s).%s", types.ExprString(fn.Recv.List[0].Type), fn.Name.Name)
}

func git(repoRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return string(out), nil
}
