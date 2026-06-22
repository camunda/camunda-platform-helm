// playwright-pin resolves the @playwright/test version pinned across every
// chart's test/e2e/package.json, asserts the pins are exact and identical,
// and (optionally) copies the highest-chart-version package.json into a
// build context.
//
// Usage:
//
//	playwright-pin --repo-root <path> [--copy-to <file>]
//
// Output (stdout, one key=value per line — safe for $GITHUB_OUTPUT):
//
//	playwright-version=<exact-semver>
//	playwright-source=<path/to/chosen/package.json>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var exactSemver = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

type pkg struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

type pin struct {
	file    string
	version string
}

func main() {
	repoRoot := flag.String("repo-root", ".", "path to the camunda-platform-helm repo root")
	copyTo := flag.String("copy-to", "", "if set, copies the chosen package.json to this path")
	flag.Parse()

	if err := run(*repoRoot, *copyTo, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func run(repoRoot, copyTo string, out io.Writer) error {
	files, err := filepath.Glob(filepath.Join(repoRoot, "charts", "camunda-platform-*", "test", "e2e", "package.json"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no charts/camunda-platform-*/test/e2e/package.json found under %s", repoRoot)
	}
	sort.Strings(files)

	pins, err := collectPins(files)
	if err != nil {
		return err
	}
	if err := assertAgree(pins); err != nil {
		return err
	}

	src := chooseHighestVersion(files)
	if copyTo != "" {
		if err := copyFile(src, copyTo); err != nil {
			return fmt.Errorf("copy %s -> %s: %w", src, copyTo, err)
		}
	}
	fmt.Fprintf(out, "playwright-version=%s\n", pins[0].version)
	fmt.Fprintf(out, "playwright-source=%s\n", src)
	return nil
}

func collectPins(files []string) ([]pin, error) {
	out := make([]pin, 0, len(files))
	for _, f := range files {
		v, err := readPin(f)
		if err != nil {
			return nil, err
		}
		if v == "" {
			return nil, fmt.Errorf("%s does not declare @playwright/test", f)
		}
		if !exactSemver.MatchString(v) {
			return nil, fmt.Errorf("%s pins @playwright/test as %q; must be an exact version (e.g. 1.61.0)", f, v)
		}
		out = append(out, pin{file: f, version: v})
	}
	return out, nil
}

func readPin(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var p pkg
	if err := json.Unmarshal(b, &p); err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if v, ok := p.DevDependencies["@playwright/test"]; ok && v != "" {
		return v, nil
	}
	if v, ok := p.Dependencies["@playwright/test"]; ok && v != "" {
		return v, nil
	}
	return "", nil
}

func assertAgree(pins []pin) error {
	if len(pins) == 0 {
		return nil
	}
	first := pins[0].version
	var disagree []string
	for _, p := range pins {
		if p.version != first {
			disagree = append(disagree, fmt.Sprintf("  %s=%s", p.file, p.version))
		}
	}
	if len(disagree) > 0 {
		header := fmt.Sprintf("  %s=%s", pins[0].file, pins[0].version)
		return fmt.Errorf("chart versions disagree on @playwright/test pin:\n%s\n%s", header, strings.Join(disagree, "\n"))
	}
	return nil
}

// chooseHighestVersion picks the package.json belonging to the chart directory
// with the highest semver-style numeric suffix (e.g. camunda-platform-8.10 over -8.9).
func chooseHighestVersion(files []string) string {
	sort.SliceStable(files, func(i, j int) bool {
		return less(extractChartVersion(files[i]), extractChartVersion(files[j]))
	})
	return files[len(files)-1]
}

var chartVersionDir = regexp.MustCompile(`(?:^|/)camunda-platform-([0-9]+(?:\.[0-9]+)*)(?:/|$)`)

func extractChartVersion(path string) []int {
	m := chartVersionDir.FindStringSubmatch(filepath.ToSlash(path))
	if m == nil {
		return nil
	}
	return parseSemverParts(m[1])
}

func parseSemverParts(s string) []int {
	out := make([]int, 0, 3)
	for _, seg := range strings.Split(s, ".") {
		n, err := strconv.Atoi(seg)
		if err != nil {
			return out
		}
		out = append(out, n)
	}
	return out
}

func less(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0o644)
}
