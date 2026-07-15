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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// GitOps abstracts the git operations chart-version resolution needs, so the
// resolution logic is unit-testable without a repository.
type GitOps interface {
	// CurrentBranch returns the checked-out branch name.
	CurrentBranch() (string, error)
	// FetchMain fetches origin main into the local main ref.
	FetchMain() error
	// MainChartYaml returns the content of charts/<chartDir>/Chart.yaml on
	// main, and whether the file exists there.
	MainChartYaml(chartDir string) (content []byte, exists bool, err error)
}

// ExecGit is the GitOps implementation backed by the git CLI, run in Dir.
type ExecGit struct {
	Dir string
}

func (g ExecGit) run(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.Dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func (g ExecGit) CurrentBranch() (string, error) {
	out, err := g.run("branch", "--show-current")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (g ExecGit) FetchMain() error {
	_, err := g.run("fetch", "origin", "main:main", "--no-tags")
	return err
}

func (g ExecGit) MainChartYaml(chartDir string) ([]byte, bool, error) {
	path := fmt.Sprintf("charts/%s/Chart.yaml", chartDir)
	lsOut, err := g.run("ls-tree", "main", "--", path)
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(string(lsOut)) == "" {
		return nil, false, nil
	}
	content, err := g.run("show", "main:"+path)
	if err != nil {
		return nil, false, err
	}
	return content, true, nil
}

// ResolveChartVersionInput selects the chart version that upgrade flows
// install before upgrading to the PR branch.
type ResolveChartVersionInput struct {
	SetupFlow           string
	ChartDir            string
	ChartUpgradeVersion string
	// RepoRoot is the directory chart paths are resolved against. Empty means ".".
	RepoRoot string
	// Sleep replaces time.Sleep between fetch retries; nil means time.Sleep.
	Sleep func(time.Duration)
}

// ResolveChartVersion reproduces the "Set workflow vars - Chart version" shell
// step. It returns the resolved version and true for upgrade flows, and
// ("", false, nil) for flows that do not resolve a version.
func ResolveChartVersion(in ResolveChartVersionInput, git GitOps) (string, bool, error) {
	switch in.SetupFlow {
	case "upgrade-patch", "upgrade-minor", "modular-upgrade-minor":
	default:
		return "", false, nil
	}
	if in.ChartUpgradeVersion != "" {
		return in.ChartUpgradeVersion, true, nil
	}

	branch, err := git.CurrentBranch()
	if err != nil {
		return "", false, err
	}
	if branch != "main" {
		if err := fetchMainWithRetry(git, in.Sleep); err != nil {
			return "", false, err
		}
	}

	content, exists, err := git.MainChartYaml(in.ChartDir)
	if err != nil {
		return "", false, err
	}
	if !exists {
		// Fallback needed only when a new chart directory is added.
		repoRoot := in.RepoRoot
		if repoRoot == "" {
			repoRoot = "."
		}
		content, err = os.ReadFile(filepath.Join(repoRoot, "charts", in.ChartDir, "Chart.yaml"))
		if err != nil {
			return "", false, err
		}
	}
	var chart struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(content, &chart); err != nil {
		return "", false, fmt.Errorf("parse Chart.yaml for %s: %w", in.ChartDir, err)
	}
	return chart.Version, true, nil
}

// fetchMainWithRetry retries transient fetch failures (GitHub occasionally
// returns 500s under load) up to 3 times with 10s/20s backoff.
func fetchMainWithRetry(git GitOps, sleep func(time.Duration)) error {
	if sleep == nil {
		sleep = time.Sleep
	}
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		if err = git.FetchMain(); err == nil {
			return nil
		}
		if attempt < 3 {
			sleep(time.Duration(attempt) * 10 * time.Second)
		}
	}
	return fmt.Errorf("all git fetch attempts failed: %w", err)
}

var chartVersionLine = regexp.MustCompile(`(?m)^version:.*$`)

// StampChartVersion sets the top-level version field of
// charts/<chartDir>/Chart.yaml to the CI snapshot version derived from the
// chart directory name (camunda-platform-8.10 → 0.0.0-ci-snapshot-8.10),
// touching only the version line so the rest of the file stays byte-identical.
func StampChartVersion(repoRoot, chartDir string) error {
	if repoRoot == "" {
		repoRoot = "."
	}
	version := strings.ReplaceAll(chartDir, "camunda-platform", "0.0.0-ci-snapshot")
	path := filepath.Join(repoRoot, "charts", chartDir, "Chart.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	matches := chartVersionLine.FindAll(raw, -1)
	if len(matches) != 1 {
		return fmt.Errorf("%s: expected exactly one top-level version line, found %d", path, len(matches))
	}
	updated := chartVersionLine.ReplaceAll(raw, []byte("version: "+version))
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
