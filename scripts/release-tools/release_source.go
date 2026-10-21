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
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"scripts/camunda-core/pkg/harbor"
	"scripts/camunda-core/pkg/harbortag"
	"scripts/camunda-core/pkg/releasenotes"
	"scripts/camunda-core/pkg/releaseplease"
)

var (
	sourceSHARe     = regexp.MustCompile(`^[a-f0-9]{4,40}$`)
	fullSourceSHARe = regexp.MustCompile(`^[a-f0-9]{40}$`)
	artifactSHARe   = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	appMinorRe      = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)
)

type releaseArtifact struct {
	Identity      releasenotes.ChartIdentity
	Tag           string
	DevTag        string
	Commit        string
	Digest        string
	Package       string
	PackageDigest string
}

type publishedRelease struct {
	Tag         string    `json:"tag_name"`
	Draft       bool      `json:"draft"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []struct {
		Name   string `json:"name"`
		Digest string `json:"digest"`
	} `json:"assets"`
}

func runReleaseSource(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("release-source requires prepare, previous, or verify")
	}
	op := args[0]
	fs := flag.NewFlagSet("release-source "+op, flag.ContinueOnError)
	var input, destination, chartDir, source, packageFile string
	var dryRun bool
	switch op {
	case "prepare":
		fs.StringVar(&input, "input-tag", "", "concrete or rolling RC tag")
		fs.StringVar(&destination, "destination", ".cr-release-packages", "empty destination for the validated package")
		fs.BoolVar(&dryRun, "dry-run", false, "accept dry-run RC tags and inspect existing releases without publishing")
	case "previous":
		fs.StringVar(&chartDir, "chart-dir", "", "chart whose previous artifact source is required")
		fs.StringVar(&source, "source", "", "current build-base commit")
	case "verify":
		fs.StringVar(&packageFile, "package", "", "the unchanged package uploaded to GitHub")
		fs.StringVar(&source, "source", "", "expected full build-base commit")
	default:
		return fmt.Errorf("unknown release-source operation %q", op)
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("unexpected release-source arguments: %v", fs.Args())
	}
	if op == "prepare" && input == "" {
		return fmt.Errorf("prepare requires --input-tag")
	}
	if op == "previous" && (chartDir == "" || source == "") {
		return fmt.Errorf("previous requires --chart-dir and --source")
	}
	ctx := context.Background()
	if op == "verify" {
		if packageFile == "" || !fullSourceSHARe.MatchString(source) {
			return fmt.Errorf("verify requires --package and a full --source commit")
		}
		return verifyPublishedSource(ctx, packageFile, source)
	}
	client, ociRef, err := releaseHarbor()
	if err != nil {
		return err
	}
	out := newGitHubOutput()
	if op == "previous" {
		data, err := os.ReadFile(filepath.Join(chartDir, "Chart.yaml"))
		if err != nil {
			return err
		}
		identity, err := checkedChartIdentity(data)
		if err != nil {
			return err
		}
		current, err := resolveSourceCommit(ctx, source)
		if err != nil {
			return err
		}
		previous, err := previousArtifactSource(ctx, client, ociRef, identity, "", current)
		if err != nil {
			return err
		}
		if err := out.set("previous_commit_sha", previous.Commit); err != nil {
			return err
		}
		return out.set("previous_release_tag", previous.Tag)
	}
	current, previous, err := prepareReleaseSource(ctx, client, ociRef, input, destination, dryRun)
	if err != nil {
		return err
	}
	for _, pair := range [][2]string{
		{"resolved_tag", current.Tag}, {"dev_tag", current.DevTag},
		{"commit_sha", current.Commit}, {"previous_commit_sha", previous.Commit},
		{"previous_release_tag", previous.Tag}, {"artifact_digest", current.Digest},
		{"package_digest", current.PackageDigest}, {"package_file", filepath.Base(current.Package)},
		{"version", current.Identity.Version},
	} {
		if err := out.set(pair[0], pair[1]); err != nil {
			return err
		}
	}
	return nil
}

func releaseHarbor() (*harbor.Client, string, error) {
	registry := os.Getenv("HARBOR_REGISTRY_HOST")
	if registry == "" {
		registry = os.Getenv("HARBOR_REGISTRY")
	}
	project := os.Getenv("HARBOR_REGISTRY_PROJECT")
	if project == "" {
		project = os.Getenv("HARBOR_PROJECT")
	}
	if registry == "" || project == "" {
		return nil, "", fmt.Errorf("set HARBOR_REGISTRY/HARBOR_PROJECT or HARBOR_REGISTRY_HOST/HARBOR_REGISTRY_PROJECT")
	}
	api := os.Getenv("HARBOR_API")
	if api == "" {
		api = "https://" + registry + "/api/v2.0"
	}
	repo := os.Getenv("HARBOR_REPO")
	if repo == "" {
		repo = "projects/" + project + "/repositories/camunda-platform"
	}
	return harbor.New(api, repo, false), "oci://" + registry + "/" + project + "/camunda-platform", nil
}

func prepareReleaseSource(ctx context.Context, client *harbor.Client, ociRef, input, destination string, dryRun bool) (releaseArtifact, releaseArtifact, error) {
	var current, previous releaseArtifact
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return current, previous, err
	}
	packages, err := filepath.Glob(filepath.Join(destination, "*.tgz"))
	if err != nil || len(packages) != 0 {
		return current, previous, fmt.Errorf("release destination must contain no existing chart packages: %s (%v)", destination, err)
	}
	stage, err := os.MkdirTemp(destination, ".prepare-")
	if err != nil {
		return current, previous, err
	}
	defer removeReleaseTemp(stage)
	current, err = pullReleaseSource(ctx, client, ociRef, input, stage, dryRun)
	if err != nil {
		return current, previous, err
	}
	tag := releaseplease.ReleaseTag(releasenotes.AppMinor(current.Identity.AppVersion), current.Identity.Version)
	releases, err := publishedReleases(ctx)
	if err != nil {
		return current, previous, err
	}
	if !dryRun {
		for _, release := range releases {
			if release.Tag == tag {
				return current, previous, fmt.Errorf("GitHub release %s already exists; refusing to republish", tag)
			}
		}
		target, err := remoteTagCommit(ctx, tag)
		if err != nil {
			return current, previous, err
		}
		if target != "" && target != current.Commit {
			return current, previous, fmt.Errorf("existing tag %s points to %s, not artifact source %s", tag, target, current.Commit)
		}
	}
	previous, err = previousSourceFromReleases(ctx, client, ociRef, current.Identity, tag, current.Commit, releases)
	if err != nil {
		return current, previous, err
	}
	target := filepath.Join(destination, current.Identity.Name+"-"+current.Identity.Version+".tgz")
	if err := os.Link(current.Package, target); err != nil {
		return current, previous, fmt.Errorf("retain validated package: %w", err)
	}
	current.Package = target
	return current, previous, nil
}

func pullReleaseSource(ctx context.Context, client *harbor.Client, ociRef, input, destination string, dryRun bool) (releaseArtifact, error) {
	var result releaseArtifact
	artifact, err := client.Artifact(input)
	if err != nil {
		return result, fmt.Errorf("resolve Harbor artifact %q: %w", input, err)
	}
	if !artifactSHARe.MatchString(artifact.Digest) {
		return result, fmt.Errorf("invalid OCI digest for %q: %q", input, artifact.Digest)
	}
	tags := make([]string, 0, len(artifact.Tags))
	for _, tag := range artifact.Tags {
		tags = append(tags, tag.Name)
	}
	rc, dev, err := harbortag.ResolveRCSource(input, tags, dryRun)
	if err != nil {
		return result, err
	}
	commit, err := resolveSourceCommit(ctx, dev.SHA)
	if err != nil {
		return result, err
	}
	if err := requireSourceAncestor(ctx, commit, "origin/main"); err != nil {
		return result, err
	}
	root, err := capture(ctx, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return result, err
	}
	if _, err := capture(ctx, "bash", "-c", `source "$1"; shift; harbor_helm_pull "$@"`, "harbor-pull",
		filepath.Join(root, "scripts", "harbor-retry.sh"), ociRef+"@"+artifact.Digest, "--destination", destination); err != nil {
		return result, fmt.Errorf("pull immutable artifact %s: %w", artifact.Digest, err)
	}
	packages, err := filepath.Glob(filepath.Join(destination, "*.tgz"))
	if err != nil || len(packages) != 1 {
		return result, fmt.Errorf("expected one pulled chart package in %s, found %d (%v)", destination, len(packages), err)
	}
	identity, err := packageIdentity(packages[0])
	if err != nil {
		return result, err
	}
	if identity.Version != rc.Version {
		return result, fmt.Errorf("RC version %s differs from packaged version %s", rc.Version, identity.Version)
	}
	app := releasenotes.AppMinor(identity.AppVersion)
	sourceChart, err := capture(ctx, "git", "show", commit+":charts/camunda-platform-"+app+"/Chart.yaml")
	if err != nil {
		return result, fmt.Errorf("chart %s is absent from artifact source %s: %w", app, commit, err)
	}
	sourceIdentity, err := checkedChartIdentity([]byte(sourceChart))
	if err != nil {
		return result, err
	}
	if sourceIdentity.Name != identity.Name || releasenotes.AppMinor(sourceIdentity.AppVersion) != app {
		return result, fmt.Errorf("packaged chart identity does not match source %s", commit)
	}
	digest, err := fileSHA256(packages[0])
	if err != nil {
		return result, err
	}
	result = releaseArtifact{
		Identity: identity, Tag: rc.ResolvedTag, DevTag: dev.ResolvedTag,
		Commit: commit, Digest: artifact.Digest, Package: packages[0], PackageDigest: digest,
	}
	fmt.Fprintf(os.Stderr, "Artifact %s: %s, source %s (%s)\n", input, artifact.Digest, commit, dev.ResolvedTag)
	return result, nil
}

func previousArtifactSource(ctx context.Context, client *harbor.Client, ociRef string, identity releasenotes.ChartIdentity, beforeTag, current string) (releaseArtifact, error) {
	releases, err := publishedReleases(ctx)
	if err != nil {
		return releaseArtifact{}, err
	}
	return previousSourceFromReleases(ctx, client, ociRef, identity, beforeTag, current, releases)
}

func previousSourceFromReleases(ctx context.Context, client *harbor.Client, ociRef string, identity releasenotes.ChartIdentity, beforeTag, current string, releases []publishedRelease) (releaseArtifact, error) {
	app := releasenotes.AppMinor(identity.AppVersion)
	prefix := releaseplease.ReleaseTag(app, "")
	var before time.Time
	for _, release := range releases {
		if release.Tag == beforeTag && !release.Draft {
			before = release.PublishedAt
		}
	}
	var previous *publishedRelease
	conflictingTag := ""
	for i := range releases {
		release := &releases[i]
		if release.Draft || !strings.HasPrefix(release.Tag, prefix) {
			continue
		}
		if release.PublishedAt.IsZero() {
			return releaseArtifact{}, fmt.Errorf("release %s has no publication date", release.Tag)
		}
		if !before.IsZero() && !release.PublishedAt.Before(before) {
			continue
		}
		if previous == nil || release.PublishedAt.After(previous.PublishedAt) {
			previous = release
			conflictingTag = ""
		} else if release.PublishedAt.Equal(previous.PublishedAt) && release.Tag != previous.Tag {
			conflictingTag = release.Tag
		}
	}
	if conflictingTag != "" {
		return releaseArtifact{}, fmt.Errorf("ambiguous predecessor publication order: %s and %s", previous.Tag, conflictingTag)
	}
	if previous == nil {
		major, _, _ := strings.Cut(identity.Version, ".")
		bootstrap := prefix + major + ".0.0-alpha0"
		commit, err := remoteTagCommit(ctx, bootstrap)
		if err != nil {
			return releaseArtifact{}, err
		}
		if commit == "" {
			return releaseArtifact{}, fmt.Errorf("no published predecessor or explicit bootstrap tag %s", bootstrap)
		}
		data, err := capture(ctx, "git", "show", commit+":charts/camunda-platform-"+app+"/Chart.yaml")
		if err != nil {
			return releaseArtifact{}, fmt.Errorf("read bootstrap chart: %w", err)
		}
		initial, err := checkedChartIdentity([]byte(data))
		if err != nil {
			return releaseArtifact{}, err
		}
		if initial.Version != major+".0.0-alpha0" || releasenotes.AppMinor(initial.AppVersion) != app {
			return releaseArtifact{}, fmt.Errorf("bootstrap tag %s does not identify the initial chart", bootstrap)
		}
		if err := requireSourceAncestor(ctx, commit, current); err != nil {
			return releaseArtifact{}, err
		}
		return releaseArtifact{Tag: bootstrap, Commit: commit}, nil
	}
	version := strings.TrimPrefix(previous.Tag, prefix)
	if _, err := harbortag.ParseRcTag(version + "-rc"); err != nil {
		return releaseArtifact{}, err
	}
	dir, err := os.MkdirTemp("", "camunda-release-source-")
	if err != nil {
		return releaseArtifact{}, err
	}
	defer removeReleaseTemp(dir)
	source, err := pullReleaseSource(ctx, client, ociRef, version+"-rc", dir, false)
	if err != nil {
		return releaseArtifact{}, fmt.Errorf("previous release %s has no verified Harbor source: %w", previous.Tag, err)
	}
	if releasenotes.AppMinor(source.Identity.AppVersion) != app {
		return releaseArtifact{}, fmt.Errorf("previous release %s resolves to another chart line", previous.Tag)
	}
	digest, err := publishedPackageDigest(ctx, *previous, dir)
	if err != nil {
		return releaseArtifact{}, err
	}
	if digest != source.PackageDigest {
		return releaseArtifact{}, fmt.Errorf("previous release %s package digest %s differs from Harbor %s; historical provenance requires review", previous.Tag, digest, source.PackageDigest)
	}
	if err := requireSourceAncestor(ctx, source.Commit, current); err != nil {
		return releaseArtifact{}, err
	}
	source.Tag = previous.Tag
	return source, nil
}

func publishedReleases(ctx context.Context) ([]publishedRelease, error) {
	data, err := capture(ctx, "gh", "api", "repos/{owner}/{repo}/releases?per_page=100", "--paginate", "--slurp")
	if err != nil {
		return nil, err
	}
	var pages [][]publishedRelease
	if err := json.Unmarshal([]byte(data), &pages); err != nil {
		return nil, fmt.Errorf("decode GitHub releases: %w", err)
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("GitHub release listing returned no pages")
	}
	var releases []publishedRelease
	for _, page := range pages {
		if page == nil {
			return nil, fmt.Errorf("GitHub release listing returned a null page")
		}
		for _, release := range page {
			if release.Tag == "" || (!release.Draft && release.PublishedAt.IsZero()) {
				return nil, fmt.Errorf("GitHub release listing contains an invalid release record")
			}
		}
		releases = append(releases, page...)
	}
	return releases, nil
}

func publishedPackageDigest(ctx context.Context, release publishedRelease, dir string) (string, error) {
	name, digest := "", ""
	for _, asset := range release.Assets {
		if strings.HasPrefix(asset.Name, "camunda-platform-") && strings.HasSuffix(asset.Name, ".tgz") {
			if name != "" {
				return "", fmt.Errorf("release %s has multiple chart packages", release.Tag)
			}
			name, digest = asset.Name, asset.Digest
		}
	}
	if name == "" {
		return "", fmt.Errorf("release %s has no chart package", release.Tag)
	}
	if digest != "" {
		if !artifactSHARe.MatchString(digest) {
			return "", fmt.Errorf("invalid package digest for release %s: %q", release.Tag, digest)
		}
		return digest, nil
	}
	fmt.Fprintf(os.Stderr, "Release %s has no asset digest; hashing its published package\n", release.Tag)
	path := filepath.Join(dir, "published-package.tgz")
	if _, err := capture(ctx, "gh", "release", "download", release.Tag, "--pattern", name, "--output", path); err != nil {
		return "", err
	}
	return fileSHA256(path)
}

func verifyPublishedSource(ctx context.Context, packageFile, source string) error {
	identity, err := packageIdentity(packageFile)
	if err != nil {
		return err
	}
	tag := releaseplease.ReleaseTag(releasenotes.AppMinor(identity.AppVersion), identity.Version)
	actual, err := remoteTagCommit(ctx, tag)
	if err != nil {
		return err
	}
	if actual != source {
		return fmt.Errorf("published tag %s points to %q, expected artifact source %s", tag, actual, source)
	}
	releases, err := publishedReleases(ctx)
	if err != nil {
		return err
	}
	for _, release := range releases {
		if release.Tag != tag || release.Draft {
			continue
		}
		dir, err := os.MkdirTemp("", "camunda-release-verify-")
		if err != nil {
			return err
		}
		defer removeReleaseTemp(dir)
		published, err := publishedPackageDigest(ctx, release, dir)
		if err != nil {
			return err
		}
		expected, err := fileSHA256(packageFile)
		if err != nil {
			return err
		}
		if published != expected {
			return fmt.Errorf("published package %s has digest %s, expected tested package %s", tag, published, expected)
		}
		return nil
	}
	return fmt.Errorf("published release %s was not found", tag)
}

func resolveSourceCommit(ctx context.Context, source string) (string, error) {
	if !sourceSHARe.MatchString(source) {
		return "", fmt.Errorf("invalid source commit %q", source)
	}
	commit, err := capture(ctx, "git", "rev-parse", "--verify", source+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve source commit %s: %w", source, err)
	}
	if !fullSourceSHARe.MatchString(commit) || !strings.HasPrefix(commit, source) {
		return "", fmt.Errorf("source %q did not resolve to a matching full commit", source)
	}
	return commit, nil
}

func requireSourceAncestor(ctx context.Context, previous, current string) error {
	if _, err := capture(ctx, "git", "merge-base", "--is-ancestor", previous, current); err != nil {
		return fmt.Errorf("source %s is not an ancestor of %s: %w", previous, current, err)
	}
	return nil
}

func remoteTagCommit(ctx context.Context, tag string) (string, error) {
	ref := "refs/tags/" + tag
	output, err := capture(ctx, "git", "ls-remote", "--tags", "origin", ref, ref+"^{}")
	if err != nil {
		return "", err
	}
	commit := ""
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || !fullSourceSHARe.MatchString(fields[0]) {
			return "", fmt.Errorf("invalid remote tag response for %s", tag)
		}
		if fields[1] == ref+"^{}" {
			return fields[0], nil
		}
		if fields[1] == ref {
			commit = fields[0]
		}
	}
	return commit, nil
}

func checkedChartIdentity(data []byte) (releasenotes.ChartIdentity, error) {
	identity, err := releasenotes.ParseChartIdentity(data)
	if err != nil {
		return identity, err
	}
	app := releasenotes.AppMinor(identity.AppVersion)
	if identity.Name != "camunda-platform" || !appMinorRe.MatchString(app) || (identity.AppVersion != app && identity.AppVersion != app+".x") {
		return identity, fmt.Errorf("invalid chart identity: name=%q appVersion=%q", identity.Name, identity.AppVersion)
	}
	if _, err := harbortag.ParseRcTag(identity.Version + "-rc"); err != nil {
		return identity, err
	}
	return identity, nil
}

func packageIdentity(path string) (releasenotes.ChartIdentity, error) {
	var identity releasenotes.ChartIdentity
	f, err := os.Open(path)
	if err != nil {
		return identity, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return identity, err
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	var metadata []byte
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return identity, fmt.Errorf("read chart package %s: %w", path, err)
		}
		if header.Name != "camunda-platform/Chart.yaml" {
			continue
		}
		if metadata != nil {
			return identity, fmt.Errorf("duplicate Chart.yaml in %s", path)
		}
		metadata, err = io.ReadAll(io.LimitReader(reader, 1<<20+1))
		if err != nil {
			return identity, err
		}
		if len(metadata) > 1<<20 {
			return identity, fmt.Errorf("Chart.yaml exceeds 1 MiB in %s", path)
		}
	}
	if metadata == nil {
		return identity, fmt.Errorf("no camunda-platform/Chart.yaml in %s", path)
	}
	return checkedChartIdentity(metadata)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func removeReleaseTemp(path string) {
	if err := os.RemoveAll(path); err != nil {
		fmt.Fprintf(os.Stderr, "remove release temporary directory %s: %v\n", path, err)
	}
}
