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
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

func main() {
	repoRoot := flag.String("repo-root", ".", "path to the camunda-platform-helm repo root")
	flag.Parse()

	if err := run(*repoRoot, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func run(repoRoot string, out io.Writer) error {
	tracked, err := git(repoRoot, "ls-files", "--", scope)
	if err != nil {
		return err
	}
	var paths []string
	for path := range strings.Lines(tracked) {
		path = strings.TrimSpace(path)
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)

	checked := 0
	var violations []string
	for _, path := range paths {
		v, n, err := checkFile(repoRoot, path)
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

func checkFile(repoRoot, path string) ([]string, int, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(repoRoot, path), nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, 0, err
	}
	checked := 0
	var violations []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		from, to := fset.Position(fn.Pos()).Line, fset.Position(fn.End()).Line
		checked++
		if over := overLimits(fn, to-from+1); over != "" {
			violations = append(violations, fmt.Sprintf("%s:%d: %s: %s", path, from, funcName(fn), over))
		}
	}
	return violations, checked, nil
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
