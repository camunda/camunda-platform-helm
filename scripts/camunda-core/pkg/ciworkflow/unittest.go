// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ciworkflow

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type UnitTestInput struct {
	ChartDir    string
	Package     string
	Run         string
	HelmVersion string
	RepoRoot    string
}

type goTestEvent struct {
	Action string
	Test   string
	Output string
}

func RunUnitTest(ctx context.Context, in UnitTestInput, stdout, stderr io.Writer) error {
	pattern, err := compileRunPattern(in.Run)
	if err != nil {
		return err
	}
	if in.HelmVersion != "" {
		if err := requireHelmVersion(ctx, in.HelmVersion, stdout); err != nil {
			return err
		}
	}

	repoRoot := in.RepoRoot
	if repoRoot == "" {
		repoRoot = "."
	}
	cmd := exec.CommandContext(ctx, "go", "test", "-json", "-count=1", "-run", in.Run, "./"+in.Package)
	cmd.Dir = filepath.Join(repoRoot, "charts", in.ChartDir, "test", "unit")
	cmd.Stderr = stderr
	events, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start go test: %w", err)
	}
	matched, readErr := passedMatches(events, stdout, pattern)
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("go test ./%s -run %q: %w", in.Package, in.Run, err)
	}
	if readErr != nil {
		return readErr
	}
	if len(matched) == 0 {
		return fmt.Errorf("go test ./%s -run %q passed without running a matching test", in.Package, in.Run)
	}
	fmt.Fprintf(stdout, "%d passed test(s) match -run %q:\n", len(matched), in.Run)
	for _, name := range matched {
		fmt.Fprintf(stdout, "  %s\n", name)
	}
	return nil
}

func requireHelmVersion(ctx context.Context, want string, w io.Writer) error {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return fmt.Errorf("find helm: %w", err)
	}
	out, err := exec.CommandContext(ctx, helmPath, "version", "--template", "{{.Version}}").Output()
	if err != nil {
		return fmt.Errorf("%s version: %w", helmPath, err)
	}
	got := strings.TrimSpace(string(out))
	if got != "v"+want {
		return fmt.Errorf("helm on PATH is %s (%s), want v%s", got, helmPath, want)
	}
	fmt.Fprintf(w, "helm %s (%s)\n", got, helmPath)
	return nil
}

func passedMatches(r io.Reader, w io.Writer, pattern [][]*regexp.Regexp) ([]string, error) {
	var matched []string
	var writeErr error
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			out := line
			var ev goTestEvent
			if json.Unmarshal(line, &ev) == nil {
				out = []byte(ev.Output)
				if ev.Action == "pass" && ev.Test != "" && matchesRunPattern(ev.Test, pattern) {
					matched = append(matched, ev.Test)
				}
			}
			if _, werr := w.Write(out); werr != nil && writeErr == nil {
				writeErr = werr
			}
		}
		if errors.Is(err, io.EOF) {
			return matched, writeErr
		}
		if err != nil {
			return matched, err
		}
	}
}

func matchesRunPattern(name string, pattern [][]*regexp.Regexp) bool {
	parts := strings.Split(name, "/")
	for _, elems := range pattern {
		if len(parts) < len(elems) {
			continue
		}
		matched := true
		for i, re := range elems {
			if !re.MatchString(parts[i]) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func compileRunPattern(run string) ([][]*regexp.Regexp, error) {
	var pattern [][]*regexp.Regexp
	for _, elems := range splitRunPattern(run) {
		compiled := make([]*regexp.Regexp, 0, len(elems))
		for _, elem := range elems {
			re, err := regexp.Compile(elem)
			if err != nil {
				return nil, fmt.Errorf("invalid -run pattern %q: %w", run, err)
			}
			compiled = append(compiled, re)
		}
		pattern = append(pattern, compiled)
	}
	return pattern, nil
}

func splitRunPattern(s string) [][]string {
	var alternatives [][]string
	var elems []string
	brackets, parens := 0, 0
	for i := 0; i < len(s); {
		switch s[i] {
		case '[':
			brackets++
		case ']':
			if brackets > 0 {
				brackets--
			}
		case '(':
			if brackets == 0 {
				parens++
			}
		case ')':
			if brackets == 0 {
				parens--
			}
		case '\\':
			i++
		case '/', '|':
			if brackets == 0 && parens == 0 {
				elems = append(elems, s[:i])
				if s[i] == '|' {
					alternatives = append(alternatives, elems)
					elems = nil
				}
				s = s[i+1:]
				i = 0
				continue
			}
		}
		i++
	}
	return append(alternatives, append(elems, s))
}
