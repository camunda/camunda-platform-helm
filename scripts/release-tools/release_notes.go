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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"scripts/camunda-core/pkg/chartmeta"
	"scripts/camunda-core/pkg/executil"
	"scripts/camunda-core/pkg/releasenotes"
	"scripts/camunda-core/pkg/releaseplease"
	"scripts/camunda-core/pkg/versionmatrix"
)

const cliffConfigFile = ".github/config/cliff.toml"

// runReleaseNotes generates RELEASE-NOTES.md and the release Chart.yaml
// annotations. git/git-cliff/yq run as exec calls; the Chart.yaml writes go
// through yq to preserve formatting and comments.
//
//	release-tools release-notes --main   <chart-dir>
//	release-tools release-notes --footer <chart-dir>
//
// Run from the repository root (paths like .tool-versions and cliff.toml are
// repo-root-relative).
func runReleaseNotes(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: release-notes --main|--footer <chart-dir>")
	}
	mode, chartDir := args[0], args[1]
	if chartDir == "" {
		return fmt.Errorf("chart dir is required")
	}
	ctx := context.Background()
	switch mode {
	case "--main":
		return releaseNotesMain(ctx, chartDir)
	case "--footer":
		return releaseNotesFooter(ctx, chartDir)
	default:
		return fmt.Errorf("unknown mode %q (want --main or --footer)", mode)
	}
}

// capture runs a command and returns its trimmed stdout.
func capture(ctx context.Context, name string, args ...string) (string, error) {
	out, err := executil.RunCommandCapture(ctx, name, args, nil, "")
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func releaseNotesMain(ctx context.Context, chartDir string) error {
	// Fetch main unless already on it (git-cliff diffs from the last release tag).
	if branch, _ := capture(ctx, "git", "branch", "--show-current"); branch != "main" {
		if err := executil.RunCommand(ctx, "git", []string{"fetch", "origin", "main:main"}, nil, ""); err != nil {
			return err
		}
	}

	chartFile := filepath.Join(chartDir, "Chart.yaml")

	// Latest released version from main (not the release PR, which may bump it).
	latestShow, err := executil.RunCommandCapture(ctx, "git", []string{"show", "main:" + chartDir + "/Chart.yaml"}, nil, "")
	if err != nil {
		return fmt.Errorf("git show main Chart.yaml: %w", err)
	}
	latestID, err := releasenotes.ParseChartIdentity(latestShow)
	if err != nil {
		return err
	}
	latestChartName := filepath.Base(chartDir)
	latestTagHash, err := capture(ctx, "git", "show-ref", "--hash", latestChartName+"-"+latestID.Version)
	if err != nil {
		return err
	}

	chartBytes, err := os.ReadFile(chartFile)
	if err != nil {
		return err
	}
	id, err := releasenotes.ParseChartIdentity(chartBytes)
	if err != nil {
		return err
	}
	appVersion := releasenotes.AppMinor(id.AppVersion)
	chartTag := releaseplease.ReleaseTag(appVersion, id.Version)

	// Early exit if the tag already exists (exact match, not substring).
	if existing, _ := capture(ctx, "git", "tag", "-l", chartTag); existing != "" {
		fmt.Printf("[WARN] The tag %s already exists, nothing to do...\n", chartTag)
		return nil
	}

	// camunda.io/helmCLIVersion annotation (Helm v3→v4 clamp).
	pin, err := helmPin()
	if err != nil {
		return err
	}
	helmCLIVersion := releasenotes.HelmCLIVersion(appVersion, pin)
	if err := executil.RunCommand(ctx, "yq",
		[]string{"-i", `.annotations."camunda.io/helmCLIVersion" = env(helm_cli_version)`, chartFile},
		[]string{"helm_cli_version=" + helmCLIVersion}, ""); err != nil {
		return fmt.Errorf("set helmCLIVersion: %w", err)
	}

	// RELEASE-NOTES.md via git-cliff. Exclude paths not shipped to end users.
	releaseNotesPath := filepath.Join(chartDir, "RELEASE-NOTES.md")
	if err := executil.RunCommand(ctx, "git-cliff", []string{
		latestTagHash + "..",
		"--tag-pattern", "camunda-platform-" + appVersion + ".*",
		"--config", cliffConfigFile,
		"--output", releaseNotesPath,
		"--include-path", chartDir + "/**",
		"--exclude-path", chartDir + "/test/**",
		"--exclude-path", chartDir + "/go.mod",
		"--exclude-path", chartDir + "/go.sum",
		"--tag", chartTag,
	}, nil, ""); err != nil {
		return fmt.Errorf("git-cliff: %w", err)
	}

	// artifacthub.io/changes annotation from the RELEASE-NOTES.md sections.
	cliffTOML, err := os.ReadFile(cliffConfigFile)
	if err != nil {
		return err
	}
	notesMD, err := os.ReadFile(releaseNotesPath)
	if err != nil {
		return err
	}
	block, _ := releasenotes.ArtifactHubChanges(string(notesMD), releasenotes.CliffGroups(string(cliffTOML)))

	// Seed an empty literal block (yq can't create one directly), then merge.
	if err := executil.RunCommand(ctx, "yq",
		[]string{"-i", `.annotations."artifacthub.io/changes" = "placeholder" | .annotations."artifacthub.io/changes" style="literal"`, chartFile},
		nil, ""); err != nil {
		return fmt.Errorf("seed changes annotation: %w", err)
	}
	tmpFile, err := os.CreateTemp("", "changes-for-artifacthub-*.yaml")
	if err != nil {
		return err
	}
	tmp := tmpFile.Name()
	defer os.Remove(tmp)
	if _, err := tmpFile.Write([]byte(block)); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	merged, err := executil.RunCommandCapture(ctx, "yq",
		[]string{"eval-all", ". as $item ireduce ({}; . * $item )", chartFile, tmp}, nil, "")
	if err != nil {
		return fmt.Errorf("merge changes annotation: %w", err)
	}
	if err := os.WriteFile(chartFile, merged, 0o644); err != nil {
		return err
	}
	return nil
}

func releaseNotesFooter(_ context.Context, chartDir string) error {
	chartFile := filepath.Join(chartDir, "Chart.yaml")
	chartBytes, err := os.ReadFile(chartFile)
	if err != nil {
		return err
	}
	id, err := releasenotes.ParseChartIdentity(chartBytes)
	if err != nil {
		return err
	}
	appVersion := releasenotes.AppStripLastSegment(id.AppVersion)
	chartReleaseName := id.Name + "-" + id.Version

	images, err := chartmeta.ImageSet(chartDir)
	if err != nil {
		return fmt.Errorf("derive image set: %w", err)
	}
	enterpriseImgs, err := enterpriseImageSet(chartDir)
	if err != nil {
		return err
	}

	pin, err := helmPin()
	if err != nil {
		return err
	}
	helmCLIVersions := versionmatrix.SplitHelmCLI(releasenotes.HelmCLIVersion(appVersion, pin))

	entry := versionmatrix.ChartEntry{
		ChartVersion:          id.Version,
		ChartImages:           images,
		ChartEnterpriseImages: enterpriseImgs,
	}
	section := versionmatrix.ReleaseSection(appVersion, entry, helmCLIVersions, false)

	githubRepo := os.Getenv("GITHUB_REPOSITORY")
	footer := "### Release Info\n\n" + section +
		"\n### Verification\n\n" +
		"For quick verification of the Helm chart integrity using [Cosign](https://docs.sigstore.dev/signing/quickstart/):\n\n" +
		"```shell\n" +
		"cosign verify-blob " + chartReleaseName + ".tgz \\\n" +
		"  --bundle \"" + chartReleaseName + "-cosign-bundle.json\" \\\n" +
		"  --certificate-identity-regexp \"https://github.com/" + githubRepo + "\" \\\n" +
		"  --certificate-oidc-issuer \"https://token.actions.githubusercontent.com\"\n" +
		"```\n\n" +
		"For detailed verification instructions, check the steps in the `" + chartReleaseName + "-cosign-verify.sh` file.\n"

	notesPath := filepath.Join(chartDir, "RELEASE-NOTES.md")
	existing, err := os.ReadFile(notesPath)
	if err != nil {
		return err
	}
	// Cut from the first footer marker so re-runs replace rather than stack.
	body := string(existing)
	if i := strings.Index(body, "### Release Info"); i >= 0 {
		body = body[:i]
	}
	if err := os.WriteFile(notesPath, []byte(body+footer), 0o644); err != nil {
		return err
	}
	fmt.Print(footer)
	return nil
}

// helmPin returns the helm version pinned in the repo-root .tool-versions.
func helmPin() (string, error) {
	data, err := os.ReadFile(".tool-versions")
	if err != nil {
		return "", err
	}
	pin := parseHelmPin(string(data))
	if pin == "" {
		return "", fmt.Errorf("no helm pin found in .tool-versions")
	}
	return pin, nil
}
