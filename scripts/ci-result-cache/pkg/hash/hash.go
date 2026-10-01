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

// Package hash computes content hashes for CI result caching.
package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WorkflowFiles are the specific CI files that affect integration test behavior.
// Changes to these files should invalidate all cached results.
var WorkflowFiles = []string{
	".github/workflows/test-chart-version.yaml",
	".github/workflows/test-chart-version-template.yaml",
	".github/workflows/test-integration-template.yaml",
	".github/workflows/test-integration-runner.yaml",
	".github/actions/generate-chart-matrix/action.yaml",
	".github/actions/playwright-e2e-tests/action.yaml",
}

func Compute(repoRoot string, chartVersions []string, e2eSuiteVersion string) (string, error) {
	versions := sortedUnique(chartVersions)
	if len(versions) == 0 {
		return "", fmt.Errorf("at least one chart version is required")
	}
	if e2eSuiteVersion == "" {
		return "", fmt.Errorf("an e2e test suite version is required")
	}

	h := sha256.New()

	// Hash all paths to include — order matters for determinism.
	var paths []string
	for _, version := range versions {
		paths = append(paths, filepath.Join("charts", fmt.Sprintf("camunda-platform-%s", version)))
	}
	paths = append(paths,
		filepath.Join("scripts", "deploy-camunda"),
		filepath.Join("scripts", "camunda-core"),
		filepath.Join("scripts", "run-e2e-tests.sh"),
		filepath.Join("scripts", "render-e2e-env.sh"),
		filepath.Join("scripts", "base_playwright_script.sh"),
	)

	for _, relPath := range paths {
		absPath := filepath.Join(repoRoot, relPath)
		info, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("stat %s: %w", relPath, err)
		}
		if info.IsDir() {
			if err := hashDir(h, repoRoot, absPath); err != nil {
				return "", fmt.Errorf("hashing directory %s: %w", relPath, err)
			}
		} else {
			if err := hashFile(h, repoRoot, absPath); err != nil {
				return "", fmt.Errorf("hashing file %s: %w", relPath, err)
			}
		}
	}

	// Hash specific workflow files.
	for _, relPath := range WorkflowFiles {
		absPath := filepath.Join(repoRoot, relPath)
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			continue
		}
		if err := hashFile(h, repoRoot, absPath); err != nil {
			return "", fmt.Errorf("hashing workflow file %s: %w", relPath, err)
		}
	}

	fmt.Fprintf(h, "e2e-test-suite:%s\n", e2eSuiteVersion)

	return hex.EncodeToString(h.Sum(nil)), nil
}

func ChartVersionsFromCSV(csv string) []string {
	var versions []string
	for _, version := range strings.Split(csv, ",") {
		if version = strings.TrimSpace(version); version != "" {
			versions = append(versions, version)
		}
	}
	return versions
}

func sortedUnique(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// hashDir walks a directory and hashes all regular files within it.
// Files are processed in sorted order for determinism.
func hashDir(h io.Writer, repoRoot, dirPath string) error {
	var files []string
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip hidden directories (e.g., .git).
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	sort.Strings(files)
	for _, f := range files {
		if err := hashFile(h, repoRoot, f); err != nil {
			return err
		}
	}
	return nil
}

// hashFile writes a file's relative path and contents into the hash.
// The path is included so that renaming a file changes the hash.
func hashFile(h io.Writer, repoRoot, filePath string) error {
	relPath, err := filepath.Rel(repoRoot, filePath)
	if err != nil {
		return fmt.Errorf("computing relative path: %w", err)
	}

	// Write the relative path as a separator/identifier.
	fmt.Fprintf(h, "file:%s\n", relPath)

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", relPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("reading %s: %w", relPath, err)
	}

	return nil
}
