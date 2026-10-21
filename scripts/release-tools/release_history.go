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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"scripts/camunda-core/pkg/chartmeta"
	"scripts/camunda-core/pkg/harbor"
	"scripts/camunda-core/pkg/harbortag"
	"scripts/camunda-core/pkg/releaseplease"
)

type publishedRelease struct {
	Tag         string    `json:"tag_name"`
	Draft       bool      `json:"draft"`
	PublishedAt time.Time `json:"published_at"`
}

func runReleaseHistory(args []string) error {
	fs := flag.NewFlagSet("release-history", flag.ContinueOnError)
	chart := fs.String("chart-yaml", "", "Chart.yaml for the selected chart")
	releasesFile := fs.String("releases-file", "", "gh api releases --paginate --slurp output")
	beforeVersion := fs.String("before-version", "", "use the predecessor of this version when already published")
	api := fs.String("api", "", "Harbor API base")
	repo := fs.String("repo", "", "Harbor repository path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *chart == "" || *releasesFile == "" || *api == "" || *repo == "" || len(fs.Args()) != 0 {
		return fmt.Errorf("release-history requires --chart-yaml, --releases-file, --api and --repo")
	}
	meta, err := chartmeta.ReadPackageMetadata(*chart, "")
	if err != nil {
		return err
	}
	if meta.Version == "" || meta.AppVersion == "" {
		return fmt.Errorf("Chart.yaml must provide version and appVersion")
	}
	data, err := os.ReadFile(*releasesFile)
	if err != nil {
		return err
	}
	var pages [][]publishedRelease
	if err := json.Unmarshal(data, &pages); err != nil {
		return fmt.Errorf("read published releases: %w", err)
	}
	if len(pages) == 0 {
		return fmt.Errorf("release listing contains no pages")
	}
	var releases []publishedRelease
	for _, page := range pages {
		if page == nil {
			return fmt.Errorf("release listing contains a null page")
		}
		releases = append(releases, page...)
	}
	prefix := releaseplease.ReleaseTag(meta.AppVersion, "")
	var before time.Time
	for _, release := range releases {
		if *beforeVersion != "" && release.Tag == prefix+*beforeVersion && !release.Draft {
			before = release.PublishedAt
		}
	}
	var previous *publishedRelease
	for i := range releases {
		release := &releases[i]
		if release.Draft || !strings.HasPrefix(release.Tag, prefix) || (!before.IsZero() && !release.PublishedAt.Before(before)) {
			continue
		}
		if previous == nil || release.PublishedAt.After(previous.PublishedAt) {
			previous = release
		}
	}
	major, _, _ := strings.Cut(meta.Version, ".")
	tag := prefix + major + ".0.0-alpha0"
	ref := "refs/tags/" + tag
	if previous != nil {
		rcTag := strings.TrimPrefix(previous.Tag, prefix) + "-rc"
		artifact, err := harbor.New(*api, *repo, false).Artifact(rcTag)
		if err != nil {
			return fmt.Errorf("previous release %s: %w", previous.Tag, err)
		}
		tags := make([]string, 0, len(artifact.Tags))
		for _, tag := range artifact.Tags {
			tags = append(tags, tag.Name)
		}
		_, dev, err := harbortag.ResolveRCSource(rcTag, tags, false)
		if err != nil {
			return err
		}
		tag, ref = previous.Tag, dev.SHA
	}
	if err := newGitHubOutput().set("previous_release_tag", tag); err != nil {
		return err
	}
	fmt.Println(ref)
	return nil
}
