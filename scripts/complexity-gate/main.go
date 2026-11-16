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
	"os"
	"os/exec"
	"strings"
)

func main() {
	repoRoot := flag.String("repo-root", ".", "path to the camunda-platform-helm repo root")
	base := flag.String("base", "origin/main", "git ref; the change is the diff from its merge base with HEAD to the working tree")
	all := flag.Bool("all", false, "check every workflow run block, changed or not")
	flag.Parse()

	if err := run(*repoRoot, *base, *all, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
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
