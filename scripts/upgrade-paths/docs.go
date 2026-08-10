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
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DocsCoverage records whether each discovered key is named in the published
// upgrade guide for this transition.
type DocsCoverage struct {
	// Checked is false when no docs root was supplied or no guide was found,
	// which must be rendered differently from "checked and undocumented".
	Checked bool `json:"checked"`
	// Guides are the guide files consulted, relative to the docs root.
	Guides []string `json:"guides,omitempty"`
	// Documented maps a values key to whether the guide names it.
	Documented map[string]bool `json:"documented,omitempty"`
}

// IsDocumented reports coverage for a key, defaulting to true when unchecked so
// callers never present an unchecked key as a documentation gap.
func (d DocsCoverage) IsDocumented(key string) bool {
	if !d.Checked {
		return true
	}
	return d.Documented[key]
}

// UndocumentedCount returns how many of the given keys the guide omits.
func (d DocsCoverage) UndocumentedCount(keys []string) int {
	if !d.Checked {
		return 0
	}
	n := 0
	for _, k := range keys {
		if !d.Documented[k] {
			n++
		}
	}
	return n
}

// versionSlug converts an app version to the guide filename form: 8.9 -> 890,
// 8.10 -> 8100.
func versionSlug(version string) string {
	return strings.ReplaceAll(version, ".", "") + "0"
}

// findGuides locates every upgrade guide for a transition under docsRoot.
//
// Guides are not stored at a single predictable path — the same transition can
// appear under docs/ and under one or more versioned_docs/version-*/ trees — so
// match on filename and take the union.
func findGuides(docsRoot, from, to string) ([]string, error) {
	want := versionSlug(from) + "-to-" + versionSlug(to) + ".md"
	var out []string

	err := filepath.WalkDir(docsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", ".git", "build", ".docusaurus":
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() == want {
			rel, relErr := filepath.Rel(docsRoot, path)
			if relErr != nil {
				rel = path
			}
			out = append(out, rel)
		}
		return nil
	})
	return out, err
}

// CheckDocsCoverage tests each key for a literal mention in any guide.
//
// A literal substring match is intentional: the guide is prose and tables, and
// a key that is not written out verbatim is not actionable for a customer
// searching for it.
func CheckDocsCoverage(docsRoot, from, to string, keys []string) (DocsCoverage, error) {
	var cov DocsCoverage
	if docsRoot == "" || len(keys) == 0 {
		return cov, nil
	}
	if _, err := os.Stat(docsRoot); err != nil {
		return cov, nil
	}

	guides, err := findGuides(docsRoot, from, to)
	if err != nil {
		return cov, err
	}
	if len(guides) == 0 {
		return cov, nil
	}

	var corpus strings.Builder
	for _, g := range guides {
		b, err := os.ReadFile(filepath.Join(docsRoot, g))
		if err != nil {
			continue
		}
		corpus.Write(b)
		corpus.WriteByte('\n')
	}
	text := corpus.String()

	cov.Checked = true
	cov.Guides = guides
	cov.Documented = make(map[string]bool, len(keys))
	for _, k := range keys {
		cov.Documented[k] = strings.Contains(text, k)
	}
	return cov, nil
}

// defaultDocsRoot resolves a sibling camunda-docs checkout next to repoRoot.
func defaultDocsRoot(repoRoot string) string {
	candidate := filepath.Join(filepath.Dir(repoRoot), "camunda-docs")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}
