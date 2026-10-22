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

// runResolveTag resolves a rolling dev tag to a concrete one, validates its
// format, and emits the extracted fields to $GITHUB_OUTPUT. The Harbor API GET
// (the artifact's tag list) is passed via --tags-file.
//
//	resolve-tag --kind dev --input-tag <tag> [--tags-file <harbor-tags.json|->] [--dry-run]
//
// Emits resolved_tag, version, chart_major, rc_tag, rc_latest_tag to
// $GITHUB_OUTPUT and prints the (short) commit SHA to stdout. The caller
// captures stdout to expand the short SHA to a full 40-char SHA (via the
// GitHub API) and emits the final `sha` output itself. With --dry-run,
// rc_tag/rc_latest_tag carry the isolated dry-run names
// ({version}-rc-dryrun / {major}-rc-dryrun-latest).
func runResolveTag(args []string) error {
	fs := flag.NewFlagSet("resolve-tag", flag.ContinueOnError)
	var (
		kind     string
		input    string
		tagsFile string
		dryRun   bool
	)
	fs.StringVar(&kind, "kind", "", "tag family: dev")
	fs.StringVar(&input, "input-tag", "", "the input tag (concrete or rolling {major}-dev-latest)")
	fs.StringVar(&tagsFile, "tags-file", "", "Harbor artifact tags JSON (path or - for stdin); required to resolve a rolling tag")
	fs.BoolVar(&dryRun, "dry-run", false, "dry-run tag naming: emit -rc-dryrun names")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if harbortag.Kind(kind) != harbortag.Dev {
		return fmt.Errorf("--kind must be dev")
	}
	if input == "" {
		return fmt.Errorf("--input-tag is required")
	}

	concrete := input
	if harbortag.IsRolling(input, harbortag.Dev) {
		if tagsFile == "" {
			return fmt.Errorf("rolling tag %q requires --tags-file with the artifact's tags", input)
		}
		tags, err := readHarborTagNames(tagsFile)
		if err != nil {
			return err
		}
		concrete, err = harbortag.ResolveConcrete(tags, harbortag.Dev)
		if err != nil {
			return fmt.Errorf("resolve rolling tag %q: %w", input, err)
		}
	}

	d, err := harbortag.ParseDevTag(concrete)
	if err != nil {
		return err
	}
	rcTag, rcLatestTag := d.RCTag, d.RCLatestTag
	if dryRun {
		rcTag, rcLatestTag = d.RCDryRunTag, d.RCDryRunLatestTag
	}
	out := newGitHubOutput()
	for _, kv := range [][2]string{
		{"resolved_tag", d.ResolvedTag}, {"version", d.Version},
		{"chart_major", d.ChartMajor}, {"rc_tag", rcTag}, {"rc_latest_tag", rcLatestTag},
	} {
		if err := out.set(kv[0], kv[1]); err != nil {
			return err
		}
	}
	fmt.Println(d.SHA)
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
