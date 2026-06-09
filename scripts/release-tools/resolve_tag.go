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
//	resolve-tag --kind dev|rc --input-tag <tag> [--tags-file <harbor-tags.json|->]
//
// dev: emits resolved_tag, version, chart_major, rc_tag, rc_latest_tag to
//
//	$GITHUB_OUTPUT and prints the (short) commit SHA to stdout. The caller
//	captures stdout to expand the short SHA to a full 40-char SHA (via the
//	GitHub API) and emits the final `sha` output itself.
//
// rc:  emits resolved_tag, version, and (when --tags-file is given) dev_tag +
//
//	commit_sha for traceability — empty when no dev tag is present. commit_sha
//	is the short SHA as-is (no expansion; the commit is not checked out).
func runResolveTag(args []string) error {
	fs := flag.NewFlagSet("resolve-tag", flag.ContinueOnError)
	var (
		kindStr  string
		input    string
		tagsFile string
	)
	fs.StringVar(&kindStr, "kind", "", "tag family: dev or rc")
	fs.StringVar(&input, "input-tag", "", "the input tag (concrete or rolling {major}-{kind}-latest)")
	fs.StringVar(&tagsFile, "tags-file", "", "Harbor artifact tags JSON (path or - for stdin); required to resolve a rolling tag")
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

	concrete := input
	var tags []string
	if tagsFile != "" {
		var err error
		tags, err = readHarborTagNames(tagsFile)
		if err != nil {
			return err
		}
	}
	if harbortag.IsRolling(input, kind) {
		if len(tags) == 0 {
			return fmt.Errorf("rolling tag %q requires --tags-file with the artifact's tags", input)
		}
		var err error
		concrete, err = harbortag.ResolveConcrete(tags, kind)
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
		// `sha` is intentionally NOT emitted here: the workflow expands the
		// short SHA to a full 40-char SHA via the GitHub API and emits `sha`
		// itself. We print the short SHA to stdout so it can capture it.
		for _, kv := range [][2]string{
			{"resolved_tag", d.ResolvedTag}, {"version", d.Version},
			{"chart_major", d.ChartMajor}, {"rc_tag", d.RCTag}, {"rc_latest_tag", d.RCLatestTag},
		} {
			if err := out.set(kv[0], kv[1]); err != nil {
				return err
			}
		}
		fmt.Println(d.SHA)
	case harbortag.RC:
		r, err := harbortag.ParseRcTag(concrete)
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
