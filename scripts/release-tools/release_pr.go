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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"scripts/camunda-core/pkg/releasenotes"
	"scripts/camunda-core/pkg/releaseplease"
)

type releaseInputCommit struct {
	SHA     string   `json:"sha"`
	Message string   `json:"message"`
	Files   []string `json:"files"`
}

type releasePRInput struct {
	Owner          string               `json:"owner"`
	Repo           string               `json:"repo"`
	ChartDir       string               `json:"chartDir"`
	ConfigFile     string               `json:"configFile"`
	ManifestFile   string               `json:"manifestFile"`
	PreviousTag    string               `json:"previousTag"`
	PreviousSource string               `json:"previousSource"`
	Commits        []releaseInputCommit `json:"commits"`
	ReleaseAs      string               `json:"releaseAs"`
	DryRun         bool                 `json:"dryRun"`
}

const releaseSourcePluginJS = `
function bindArtifactSource(manifest, input, github, ManifestPlugin, TagName) {
  const plugin = new ManifestPlugin(github, "main", manifest.repositoryConfig);
  plugin.preconfigure = async (strategies, commits, releases) => {
    const tag = TagName.parse(input.previousTag);
    if (!tag) throw new Error("Invalid predecessor release tag: " + input.previousTag);
    commits[input.chartDir] = input.commits;
    releases[input.chartDir] = {tag, sha: input.previousSource, notes: ""};
    return strategies;
  };
  manifest.plugins.unshift(plugin);
}
`

const releasePleaseJS = releaseSourcePluginJS + `
(async () => {
  const root = process.argv[1];
  const {GitHub, Manifest} = require(root);
  const {ManifestPlugin} = require(root + "/build/src/plugin");
  const {TagName} = require(root + "/build/src/util/tag-name");
  const input = JSON.parse(require("fs").readFileSync(0, "utf8"));
  const github = await GitHub.create({
    owner: input.owner, repo: input.repo,
    token: process.env.GH_TOKEN || process.env.GITHUB_TOKEN
  });
  const manifest = await Manifest.fromManifest(github, "main", input.configFile,
    input.manifestFile, {}, input.chartDir, input.releaseAs || undefined);
  bindArtifactSource(manifest, input, github, ManifestPlugin, TagName);
  if (input.dryRun) {
    const requests = await manifest.buildPullRequests();
    if (requests.length > 1) throw new Error("Expected one chart's release candidate");
    for (const request of requests) {
      if (request.version) {
        console.log("+ version: " + request.version.toString());
      }
    }
  } else {
    await manifest.createPullRequests();
  }
})().catch(error => { console.error(error.message); process.exitCode = 1; });
`

func runReleasePR(args []string) error {
	fs := flag.NewFlagSet("release-pr", flag.ContinueOnError)
	var chartDir, source, releaseAs, configFile, manifestFile string
	var dryRun bool
	fs.StringVar(&chartDir, "chart-dir", "", "chart to release")
	fs.StringVar(&source, "source", "", "explicit build-base commit")
	fs.StringVar(&releaseAs, "release-as", "", "version of the selected dev artifact")
	fs.StringVar(&configFile, "config-file", ".github/config/release-please/release-please-config.json", "release-please configuration path")
	fs.StringVar(&manifestFile, "manifest-file", ".github/config/release-please/.release-please-manifest.json", "release-please version manifest path")
	fs.BoolVar(&dryRun, "dry-run", false, "calculate the candidate without creating or updating a PR")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if chartDir == "" || source == "" || len(fs.Args()) != 0 {
		return fmt.Errorf("release-pr requires --chart-dir and --source")
	}
	if os.Getenv("GH_TOKEN") == "" && os.Getenv("GITHUB_TOKEN") == "" {
		return fmt.Errorf("release-pr requires GH_TOKEN or GITHUB_TOKEN")
	}
	owner, repo, ok := strings.Cut(os.Getenv("GITHUB_REPOSITORY"), "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return fmt.Errorf("release-pr requires GITHUB_REPOSITORY=owner/repo")
	}
	ctx := context.Background()
	source, err := resolveSourceCommit(ctx, source)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(chartDir, "Chart.yaml"))
	if err != nil {
		return err
	}
	identity, err := checkedChartIdentity(data)
	if err != nil {
		return err
	}
	client, ociRef, err := releaseHarbor()
	if err != nil {
		return err
	}
	beforeTag := ""
	if releaseAs != "" {
		beforeTag = releaseplease.ReleaseTag(releasenotes.AppMinor(identity.AppVersion), releaseAs)
	}
	previous, err := previousArtifactSource(ctx, client, ociRef, identity, beforeTag, source)
	if err != nil {
		return err
	}
	commits, err := releaseInputCommits(ctx, chartDir, previous.Commit, source)
	if err != nil {
		return err
	}
	input := releasePRInput{
		Owner: owner, Repo: repo, ChartDir: filepath.ToSlash(filepath.Clean(chartDir)),
		ConfigFile:   configFile,
		ManifestFile: manifestFile,
		PreviousTag:  previous.Tag, PreviousSource: previous.Commit, Commits: commits,
		ReleaseAs: releaseAs, DryRun: dryRun,
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return err
	}
	moduleRoot, err := capture(ctx, "npm", "root", "-g")
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "node", "-e", releasePleaseJS, filepath.Join(moduleRoot, "release-please"))
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("release-please: %w", err)
	}
	return nil
}

func releaseInputCommits(ctx context.Context, chartDir, previous, current string) ([]releaseInputCommit, error) {
	if err := requireSourceAncestor(ctx, previous, current); err != nil {
		return nil, err
	}
	output, err := capture(ctx, "git", "log", "--format=%H", previous+".."+current, "--", chartDir)
	if err != nil {
		return nil, err
	}
	commits := make([]releaseInputCommit, 0)
	for _, sha := range strings.Fields(output) {
		message, err := capture(ctx, "git", "show", "-s", "--format=%B", sha)
		if err != nil {
			return nil, err
		}
		files, err := capture(ctx, "git", "diff-tree", "--root", "--no-commit-id", "--name-only", "-r", sha)
		if err != nil {
			return nil, err
		}
		commits = append(commits, releaseInputCommit{SHA: sha, Message: message, Files: strings.Split(files, "\n")})
	}
	return commits, nil
}
