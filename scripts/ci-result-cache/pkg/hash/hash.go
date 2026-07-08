// Package hash computes content hashes for CI result caching.
//
// A content hash captures the state of all files that could affect the outcome
// of an integration test scenario. It covers:
//   - The chart directory (charts/camunda-platform-{version}/)
//   - The deploy-camunda scripts (scripts/deploy-camunda/)
//   - The shared core package that deploy-camunda depends on (scripts/camunda-core/)
//   - Specific workflow files used in the integration test path
//
// If any of these files change, the hash changes, invalidating cached results.
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
}

// Compute calculates a SHA-256 content hash for a given chart version.
// It hashes all relevant files that could affect integration test outcomes.
//
// repoRoot is the repository root directory.
// version is the chart version (e.g., "8.9").
func Compute(repoRoot, version string) (string, error) {
	h := sha256.New()

	// Hash all paths to include — order matters for determinism.
	paths := []string{
		filepath.Join("charts", fmt.Sprintf("camunda-platform-%s", version)),
		filepath.Join("scripts", "deploy-camunda"),
		filepath.Join("scripts", "camunda-core"),
	}

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

	return hex.EncodeToString(h.Sum(nil)), nil
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
