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
	"flag"
	"fmt"
	"strings"

	"scripts/camunda-core/pkg/chartmeta"
	"scripts/camunda-core/pkg/releaseplease"
	"scripts/camunda-core/pkg/versionmatrix"
)

// historicalReleaseSources maps release tags whose target is not the
// package's build commit to that commit.
var historicalReleaseSources = map[string]string{
	"camunda-platform-8.7-12.13.2":        "619f62b2dad46299c6207bdf43cb0657efd056df",
	"camunda-platform-8.7-12.13.5":        "8aab0c74e89fc1586cdde2cd4ea4ecdfcb9301f8",
	"camunda-platform-8.7-12.13.6":        "49983f4df58573453c7a589162a73461f45f21be",
	"camunda-platform-8.7-12.14.0":        "7f7bd9a4d4b49cac7a3141b6b69f7bcc59ba8b28",
	"camunda-platform-8.8-13.12.3":        "30731dc9b9b560268518b1cd4aa167b5be7af27c",
	"camunda-platform-8.8-13.12.4":        "7a6f547c8e9dc9c3c8b041b4ba5395893ce8aeee",
	"camunda-platform-8.8-13.12.5":        "6638138359e25c035d8fbe9213f709d40fce5edd",
	"camunda-platform-8.8-13.12.6":        "41e27dca3181908a4e4530049c837fdfbaacbbdb",
	"camunda-platform-8.8-13.12.7":        "9a38e5aae94c40f2170236de8f7c05b60d67aea2",
	"camunda-platform-8.8-13.12.8":        "8bbcad939fd567801196ae71e03d56250cdf2a10",
	"camunda-platform-8.8-13.14.0":        "9ad9c5302c85bc8d4690bc9c959f47f735d4c864",
	"camunda-platform-8.8-13.14.1":        "5713eb88aaaa65df4ee2e3bfa75ef505bc36a024",
	"camunda-platform-8.8-13.14.2":        "f64ca6d7c5e06ec1952744b579bd7031cd55171d",
	"camunda-platform-8.9-14.8.1":         "619f62b2dad46299c6207bdf43cb0657efd056df",
	"camunda-platform-8.9-14.8.2":         "46e080702e140cecf581113906f8e58d69b36686",
	"camunda-platform-8.9-14.8.4":         "9a38e5aae94c40f2170236de8f7c05b60d67aea2",
	"camunda-platform-8.9-14.8.5":         "8bbcad939fd567801196ae71e03d56250cdf2a10",
	"camunda-platform-8.9-14.9.0":         "051b2eb965e5b4f109bbd428f9aa53e4408bc858",
	"camunda-platform-8.9-14.10.0":        "6a4bae2f0db1a47d8183272b94610e35cceb3a27",
	"camunda-platform-8.9-14.11.0":        "9ad9c5302c85bc8d4690bc9c959f47f735d4c864",
	"camunda-platform-8.9-14.11.1":        "5713eb88aaaa65df4ee2e3bfa75ef505bc36a024",
	"camunda-platform-8.10-15.0.0-alpha4": "619f62b2dad46299c6207bdf43cb0657efd056df",
	"camunda-platform-8.10-15.0.0-alpha5": "d8dfa78d08efd336f79741d4b043e823333101ba",
	"camunda-platform-8.10-15.0.0":        "e4254baad6035e3d531751e4f39a5f859c776ed7",
	"camunda-platform-8.10-15.2.0":        "4005f793c7423ff4fad3336bc040cd14a5cc6ea0",
}

// runReleaseHistory prints the source commit of the release that precedes the
// chart version in --chart-yaml: the highest release tag on the chart's line
// below that version, or its recorded build commit from
// historicalReleaseSources.
//
//	release-history --chart-yaml <Chart.yaml>
func runReleaseHistory(args []string) error {
	fs := flag.NewFlagSet("release-history", flag.ContinueOnError)
	chart := fs.String("chart-yaml", "", "Chart.yaml of the release being built")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *chart == "" || len(fs.Args()) != 0 {
		return fmt.Errorf("release-history requires --chart-yaml")
	}
	meta, err := chartmeta.ReadPackageMetadata(*chart, "")
	if err != nil {
		return err
	}
	if meta.Version == "" || meta.AppVersion == "" {
		return fmt.Errorf("Chart.yaml must provide version and appVersion")
	}
	prefix := releaseplease.ReleaseTag(meta.AppVersion, "")
	tags, err := capture(context.Background(), "git", "tag", "--list", prefix+"*")
	if err != nil {
		return err
	}
	previous := ""
	for _, tag := range strings.Fields(tags) {
		version := strings.TrimPrefix(tag, prefix)
		if versionmatrix.CompareChartVersionsFull(version, meta.Version) >= 0 {
			continue
		}
		if previous == "" || versionmatrix.CompareChartVersionsFull(version, strings.TrimPrefix(previous, prefix)) > 0 {
			previous = tag
		}
	}
	if previous == "" {
		return fmt.Errorf("no release tag precedes %s%s", prefix, meta.Version)
	}
	ref := "refs/tags/" + previous
	if recorded, ok := historicalReleaseSources[previous]; ok {
		ref = recorded
	}
	fmt.Println(ref)
	return nil
}
