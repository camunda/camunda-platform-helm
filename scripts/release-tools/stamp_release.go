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
	"os"
	"path/filepath"
	"strings"

	"scripts/camunda-core/pkg/executil"
	"scripts/camunda-core/pkg/releasenotes"
	"scripts/camunda-core/pkg/releaseplease"
	"scripts/camunda-core/pkg/versionmatrix"
)

// runStampRelease writes the published GitHub release's date onto the chart
// version's version-matrix.json entry. Run by the public-release pipeline
// after the release exists, on the still-open release-please branch — the one
// write of release_date, so the matrix always reflects the artifact's real
// publish date (never a promotion-time guess). Idempotent: a re-run with an
// unchanged date is a no-op.
func runStampRelease(args []string) error {
	fs := flag.NewFlagSet("stamp-release", flag.ContinueOnError)
	var app, chartVersion, matrixFile string
	fs.StringVar(&app, "app", "", "Camunda minor (e.g. 8.8)")
	fs.StringVar(&chartVersion, "chart-version", "", "released chart version (e.g. 13.4.0)")
	fs.StringVar(&matrixFile, "matrix-file", "", "path to version-matrix.json (default version-matrix/camunda-<app>/version-matrix.json)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if app == "" || chartVersion == "" {
		return fmt.Errorf("--app and --chart-version are required")
	}
	if matrixFile == "" {
		matrixFile = filepath.Join("version-matrix", "camunda-"+app, "version-matrix.json")
	}

	tag := releaseplease.ReleaseTag(app, chartVersion)
	out, err := executil.RunCommandCapture(context.Background(), "gh",
		[]string{"release", "view", tag, "--json", "publishedAt", "--jq", ".publishedAt"}, nil, "")
	if err != nil {
		return fmt.Errorf("GitHub release %s not found — stamp-release must run after the release is published: %w", tag, err)
	}
	published := strings.TrimSpace(string(out))
	if len(published) < 10 {
		return fmt.Errorf("GitHub release %s has no usable publishedAt (%q)", tag, published)
	}
	date := published[:10]

	existing, err := os.ReadFile(matrixFile)
	if err != nil {
		return fmt.Errorf("read %s: %w", matrixFile, err)
	}
	entry, ok, err := versionmatrix.FindEntry(existing, chartVersion)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no entry for chart version %s in %s — promotion must write the entry before stamping", chartVersion, matrixFile)
	}
	if entry.ReleaseDate == date && entry.ReleaseTag == tag {
		fmt.Fprintf(os.Stderr, "entry %s already stamped with %s — no-op\n", chartVersion, date)
		return nil
	}
	// Release facts are write-once: a differing existing stamp means historical
	// drift and must be investigated, not silently rewritten (repairs go
	// through backfill-matrix explicitly).
	if entry.ReleaseDate != "" && entry.ReleaseDate != date {
		return fmt.Errorf("entry %s already stamped with %s; refusing to overwrite with %s", chartVersion, entry.ReleaseDate, date)
	}
	entry.ReleaseDate = date
	entry.ReleaseTag = tag
	if entry.HelmCLI == "" {
		if ann := helmCLIAnnotationAtRef(app, tag, chartVersion); ann != "" {
			entry.HelmCLI = ann
		} else if pin := helmPinAtTag(app, chartVersion); pin != "" {
			entry.HelmCLI = releasenotes.HelmCLIVersion(app, pin)
		}
	}

	updated, err := versionmatrix.UpsertEntry(existing, entry)
	if err != nil {
		return err
	}
	if err := os.WriteFile(matrixFile, updated, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", matrixFile, err)
	}
	fmt.Fprintf(os.Stderr, "stamped %s %s with release date %s (%s)\n", app, chartVersion, date, tag)
	return nil
}
