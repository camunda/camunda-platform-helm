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

// license-check verifies that every tracked source file carries an Apache 2.0
// license header. It complements `addlicense -check`, which only detects an
// absent header and accepts any license text that is present — a proprietary
// or GPL header passes it unnoticed.
//
// Usage:
//
//	license-check [--repo-root <path>] [--ext .go,.sh]
//
// Exits 1 and lists every offending file, one per line, grouped by reason.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// headerScanLines bounds how far into a file a header may start. Shell files
// carry a shebang and sometimes a `set -euo pipefail` block above the comment.
const headerScanLines = 40

// apacheMarkers identify an Apache 2.0 header. Either form is accepted: the
// full text block that addlicense writes, or an SPDX identifier.
var apacheMarkers = []string{
	"Apache License, Version 2.0",
	"SPDX-License-Identifier: Apache-2.0",
}

// copyrightMarkers identify the presence of some license header, whatever it
// licenses under.
var copyrightMarkers = []string{
	"Copyright",
	"SPDX-License-Identifier",
}

// negationMarkers disqualify a comment that names Apache only to deny it.
// The standard Apache block says "you may not use this file except in
// compliance with the License", so it does not collide with these.
var negationMarkers = []string{
	"not licensed under",
	"is not licensed",
}

// commentPrefixes maps an extension to the line prefixes that start a comment,
// so markers are only ever read out of comments and never out of code.
var commentPrefixes = map[string][]string{
	".go":   {"//", "/*", "*"},
	".sh":   {"#"},
	".yaml": {"#"},
	".yml":  {"#"},
	".tpl":  {"{{/*", "#", "*"},
}

var defaultCommentPrefixes = []string{"#", "//"}

type violation struct {
	path   string
	reason string
}

const (
	reasonMissing   = "no license header"
	reasonNonApache = "license header is not Apache 2.0"
)

func main() {
	repoRoot := flag.String("repo-root", ".", "path to the camunda-platform-helm repo root")
	extList := flag.String("ext", ".go,.sh", "comma-separated file extensions to verify")
	flag.Parse()

	if err := run(*repoRoot, *extList, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func run(repoRoot, extList string, out io.Writer) error {
	exts := parseExts(extList)
	if len(exts) == 0 {
		return fmt.Errorf("no extensions given via --ext")
	}

	files, err := trackedFiles(repoRoot, exts)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no tracked %s files found under %s - refusing to report success",
			extList, repoRoot)
	}

	var violations []violation
	for _, rel := range files {
		head, err := readHead(filepath.Join(repoRoot, rel))
		if err != nil {
			return err
		}
		comments := commentText(head, filepath.Ext(rel))
		switch {
		case containsAny(comments, apacheMarkers) && !containsAny(comments, negationMarkers):
			// Correctly licensed.
		case containsAny(comments, copyrightMarkers):
			violations = append(violations, violation{rel, reasonNonApache})
		default:
			violations = append(violations, violation{rel, reasonMissing})
		}
	}

	if len(violations) > 0 {
		return reportViolations(violations, len(files), out)
	}

	fmt.Fprintf(out, "license-check: %d files verified, all Apache 2.0\n", len(files))
	return nil
}

func reportViolations(violations []violation, checked int, out io.Writer) error {
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].reason != violations[j].reason {
			return violations[i].reason < violations[j].reason
		}
		return violations[i].path < violations[j].path
	})

	byReason := map[string][]string{}
	for _, v := range violations {
		byReason[v.reason] = append(byReason[v.reason], v.path)
	}
	for _, reason := range []string{reasonMissing, reasonNonApache} {
		paths := byReason[reason]
		if len(paths) == 0 {
			continue
		}
		fmt.Fprintf(out, "\n%s (%d):\n", reason, len(paths))
		for _, p := range paths {
			fmt.Fprintf(out, "  %s\n", p)
		}
	}
	fmt.Fprintf(out, "\nchecked %d files, %d violation(s)\n", checked, len(violations))
	return fmt.Errorf("license header verification failed")
}

func parseExts(extList string) []string {
	var exts []string
	for _, e := range strings.Split(extList, ",") {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		exts = append(exts, e)
	}
	return exts
}

// trackedFiles lists git-tracked files matching exts. Using git rather than a
// filesystem walk keeps gitignored and vendored trees out of scope.
func trackedFiles(repoRoot string, exts []string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files in %s: %v: %s", repoRoot, err, stderr.String())
	}

	var files []string
	for _, rel := range strings.Split(string(stdout), "\x00") {
		if rel == "" {
			continue
		}
		for _, ext := range exts {
			if filepath.Ext(rel) == ext {
				files = append(files, rel)
				break
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

func readHead(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var b strings.Builder
	scanner := bufio.NewScanner(f)
	for i := 0; i < headerScanLines && scanner.Scan(); i++ {
		b.WriteString(scanner.Text())
		b.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return b.String(), nil
}

// containsAny matches case-insensitively: SPDX identifiers are case-insensitive
// by spec, and vendored Bitnami charts spell theirs "APACHE-2.0".
// commentText keeps only the comment lines of head, dropping code. A shebang
// counts as a comment line and is harmless.
func commentText(head, ext string) string {
	prefixes, ok := commentPrefixes[ext]
	if !ok {
		prefixes = defaultCommentPrefixes
	}

	var b strings.Builder
	for _, line := range strings.Split(head, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, p := range prefixes {
			if strings.HasPrefix(trimmed, p) {
				b.WriteString(trimmed)
				b.WriteByte('\n')
				break
			}
		}
	}
	return b.String()
}

func containsAny(haystack string, needles []string) bool {
	lower := strings.ToLower(haystack)
	for _, n := range needles {
		if strings.Contains(lower, strings.ToLower(n)) {
			return true
		}
	}
	return false
}
