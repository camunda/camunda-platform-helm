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

// Command release-tools provides the data-transformation subcommands the Camunda
// chart release pipelines use; each subcommand wraps a tested package in
// scripts/camunda-core/pkg. It is a lean stdlib-flag dispatcher (no cobra):
//
//	release-tools <subcommand> [flags]
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch sub := os.Args[1]; sub {
	case "chart-images":
		err = runChartImages(os.Args[2:])
	case "update-matrix":
		err = runUpdateMatrix(os.Args[2:])
	case "resolve-tag":
		err = runResolveTag(os.Args[2:])
	case "harbor-tag":
		err = runHarborTag(os.Args[2:])
	case "component-image-versions":
		err = runComponentImageVersions(os.Args[2:])
	case "image-overrides":
		err = runImageOverrides(os.Args[2:])
	case "chart-metadata":
		err = runChartMetadata(os.Args[2:])
	case "release-version":
		err = runReleaseVersion(os.Args[2:])
	case "release-notes":
		err = runReleaseNotes(os.Args[2:])
	case "inject-values":
		err = runInjectValues(os.Args[2:])
	case "version-matrix":
		err = runVersionMatrix(os.Args[2:])
	case "retag-release":
		err = runRetagRelease(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "release-tools: unknown subcommand %q\n", sub)
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "release-tools: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `release-tools <subcommand> [flags]

Subcommands:
  chart-images    Derive the chart's declared image set from values.yaml (one ref per line)
  update-matrix   Update a version-matrix.json entry from the chart's recorded camunda.io/chart-images annotation
  resolve-tag     Resolve a rolling Harbor tag to concrete, validate, and emit its parts to $GITHUB_OUTPUT
  harbor-tag      Idempotent Harbor artifact tag operations (digest|add|delete|ensure)
  component-image-versions  Build the human-readable component-image-versions annotation block
  image-overrides Collect *-image-tag override inputs into the imageOverrides annotation + HAS_IMAGE_OVERRIDES
  chart-metadata  Read a pulled artifact's Chart.yaml and emit its metadata to $GITHUB_OUTPUT
  release-version Compute the dev-build release version + dev tag from release-please trace → $GITHUB_ENV
  release-notes   Generate RELEASE-NOTES.md + Chart.yaml release annotations (--main / --footer)
  inject-values   Override component image tags in a chart's values.yaml from *_IMAGE_TAG env
  version-matrix  Render version-matrix/camunda-<app>/README.md (--readme <app>) or version-matrix/README.md (--index)
  retag-release   Move each chart's release Git tag to the release-please merge commit (--repo --sha)
`)
}
