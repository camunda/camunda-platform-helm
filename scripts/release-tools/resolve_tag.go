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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"scripts/camunda-core/pkg/harbortag"
)

// runResolveTag resolves a rolling tag to a concrete one, validates its format,
// and emits the extracted fields to $GITHUB_OUTPUT. The Harbor API GET (the
// artifact's tag list) is passed via --tags-file. Because a release artifact's
// tags (dev/rc/rolling) all live on the one underlying artifact, a single tag
// list resolves both the concrete tag and (for rc) the source dev tag.
//
//	resolve-tag --kind dev|rc --input-tag <tag> [--tags-file <harbor-tags.json|->] [--dry-run]
//
// dev: emits resolved_tag, version, chart_major, rc_tag, rc_latest_tag to
//
//	$GITHUB_OUTPUT and prints the (short) commit SHA to stdout. The caller
//	captures stdout to expand the short SHA to a full 40-char SHA (via the
//	GitHub API) and emits the final `sha` output itself. With --dry-run,
//	rc_tag/rc_latest_tag carry the isolated dry-run names
//	({version}-rc-dryrun / {major}-rc-dryrun-latest).
//
// rc:  emits resolved_tag, version, and (when --tags-file is given) dev_tag +
//
//	commit_sha for traceability — empty when no dev tag is present. commit_sha
//	is the short SHA as-is (no expansion; the commit is not checked out).
//	With --dry-run, dry-run rc tags ({version}-rc-dryrun and rolling
//	{major}-rc-dryrun-latest) are accepted in addition to the plain rc forms.
func runResolveTag(args []string) error {
	fs := flag.NewFlagSet("resolve-tag", flag.ContinueOnError)
	var (
		kindStr  string
		input    string
		tagsFile string
		dryRun   bool
	)
	fs.StringVar(&kindStr, "kind", "", "tag family: dev or rc")
	fs.StringVar(&input, "input-tag", "", "the input tag (concrete or rolling {major}-{kind}-latest)")
	fs.StringVar(&tagsFile, "tags-file", "", "Harbor artifact tags JSON (path or - for stdin); required to resolve a rolling tag")
	fs.BoolVar(&dryRun, "dry-run", false, "dry-run tag naming: emit -rc-dryrun names for dev; also accept -rc-dryrun tags for rc")
	if err := fs.Parse(args); err != nil {
		return err
	}
	kind := harbortag.Kind(kindStr)
	if kind != harbortag.Dev && kind != harbortag.RC {
		return fmt.Errorf("--kind must be dev or rc")
	}
	if input == "" {
		return fmt.Errorf("--input-tag is required")
	}

	// In dry-run, an rc input in the isolated dry-run namespace resolves and
	// parses against the -rc-dryrun tag forms.
	resolveKind := kind
	if dryRun && kind == harbortag.RC &&
		(harbortag.IsRolling(input, harbortag.RCDryRun) || harbortag.IsRcDryRunTag(input)) {
		resolveKind = harbortag.RCDryRun
	}

	concrete := input
	var tags []string
	if tagsFile != "" {
		var err error
		tags, err = readHarborTagNames(tagsFile)
		if err != nil {
			return err
		}
	}
	if harbortag.IsRolling(input, resolveKind) {
		if len(tags) == 0 {
			return fmt.Errorf("rolling tag %q requires --tags-file with the artifact's tags", input)
		}
		var err error
		concrete, err = harbortag.ResolveConcrete(tags, resolveKind)
		if err != nil {
			return fmt.Errorf("resolve rolling tag %q: %w", input, err)
		}
	}

	out := newGitHubOutput()
	switch kind {
	case harbortag.Dev:
		d, err := harbortag.ParseDevTag(concrete)
		if err != nil {
			return err
		}
		rcTag, rcLatestTag := d.RCTag, d.RCLatestTag
		if dryRun {
			rcTag, rcLatestTag = d.RCDryRunTag, d.RCDryRunLatestTag
		}
		// `sha` is intentionally NOT emitted here: the workflow expands the
		// short SHA to a full 40-char SHA via the GitHub API and emits `sha`
		// itself. We print the short SHA to stdout so it can capture it.
		for _, kv := range [][2]string{
			{"resolved_tag", d.ResolvedTag}, {"version", d.Version},
			{"chart_major", d.ChartMajor}, {"rc_tag", rcTag}, {"rc_latest_tag", rcLatestTag},
		} {
			if err := out.set(kv[0], kv[1]); err != nil {
				return err
			}
		}
		fmt.Println(d.SHA)
	case harbortag.RC:
		var r harbortag.RcTag
		var err error
		if resolveKind == harbortag.RCDryRun {
			r, err = harbortag.ParseRcDryRunTag(concrete)
		} else {
			r, err = harbortag.ParseRcTag(concrete)
		}
		if err != nil {
			return err
		}
		for _, kv := range [][2]string{
			{"resolved_tag", r.ResolvedTag}, {"version", r.Version},
		} {
			if err := out.set(kv[0], kv[1]); err != nil {
				return err
			}
		}
		// Traceability: find the source dev tag among the artifact's tags and
		// extract its commit SHA. Emitted even when absent (empty values).
		if tagsFile != "" {
			dev := harbortag.FindDevTag(tags)
			if err := out.set("dev_tag", dev); err != nil {
				return err
			}
			if err := out.set("commit_sha", harbortag.CommitSHAFromDevTag(dev)); err != nil {
				return err
			}
		}
	}
	return nil
}

// readHarborTagNames reads a Harbor `/artifacts/{tag}/tags` JSON response (an
// array of {"name": ...} objects) and returns the tag names. Accepts "-" for
// stdin. An empty/whitespace body yields no tags.
func readHarborTagNames(path string) ([]string, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("read tags: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var entries []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse Harbor tags JSON: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Name != "" {
			names = append(names, e.Name)
		}
	}
	return names, nil
}
