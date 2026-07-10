// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gate

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
)

const (
	LaneHuman    = "human"
	LaneRenovate = "renovate"

	RenovateAuthor  = "renovate[bot]"
	DistroCIAuthor  = "distro-ci[bot]"
	changedFilesCap = 3000
)

var chartTestExclude = regexp.MustCompile(`^charts/[^/]+/test/`)

type Config struct {
	Author                     string
	EventActor                 string
	PRNumber                   int
	AllowlistPath              string
	ProtectedPathsPath         string
	RenovateProtectedPathsPath string
}

type PRMeta struct {
	ChangedFiles int
}

type PRFile struct {
	Filename         string
	PreviousFilename string
}

type Decision struct {
	Allowed  bool
	Lane     string
	Warnings []string
	Notices  []string
}

type Inputs struct {
	Lane              string
	Author            string
	EventActor        string
	Allowlist         []string
	ProtectedPatterns []string
	PRMeta            *PRMeta
	PRMetaErr         error
	PRFiles           []PRFile
	PRFilesErr        error
}

func isTrustedRenovateActor(actor string) bool {
	return actor == RenovateAuthor || actor == DistroCIAuthor
}

func Decide(in Inputs) Decision {
	lane := in.Lane
	if lane == "" {
		if in.Author == RenovateAuthor {
			lane = LaneRenovate
		} else {
			lane = LaneHuman
		}
	}

	if lane == LaneHuman {
		if !containsExact(in.Allowlist, in.Author) {
			return Decision{Allowed: false, Lane: LaneHuman}
		}
	}

	if lane == LaneRenovate && !isTrustedRenovateActor(in.EventActor) {
		return Decision{
			Allowed: false,
			Lane:    LaneRenovate,
			Warnings: []string{
				fmt.Sprintf("event actor %s is not a trusted renovate-lane pusher; requiring human review.", in.EventActor),
			},
		}
	}

	if len(in.ProtectedPatterns) == 0 {
		return Decision{
			Allowed:  false,
			Lane:     lane,
			Warnings: []string{"protected-paths list missing/empty; requiring human review."},
		}
	}

	if in.PRMetaErr != nil {
		return Decision{
			Allowed:  false,
			Lane:     lane,
			Warnings: []string{"Could not read PR metadata; requiring human review."},
		}
	}

	if in.PRMeta.ChangedFiles >= changedFilesCap {
		return Decision{
			Allowed: false,
			Lane:    lane,
			Warnings: []string{
				fmt.Sprintf("PR changes %d files (>=3000 API cap); requiring human review.", in.PRMeta.ChangedFiles),
			},
		}
	}

	if in.PRFilesErr != nil {
		return Decision{
			Allowed:  false,
			Lane:     lane,
			Warnings: []string{"Could not list PR files; requiring human review."},
		}
	}

	matched, err := matchesProtected(in.PRFiles, in.ProtectedPatterns)
	if err != nil {
		return Decision{
			Allowed:  false,
			Lane:     lane,
			Warnings: []string{"protected-path check errored; requiring human review."},
		}
	}
	if matched {
		return Decision{
			Allowed: false,
			Lane:    lane,
			Notices: []string{"PR touches a protected path; human review is required."},
		}
	}

	return Decision{Allowed: true, Lane: lane}
}

func matchesProtected(files []PRFile, patterns []string) (bool, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return false, err
		}
		compiled = append(compiled, re)
	}

	for _, path := range collectPaths(files) {
		if chartTestExclude.MatchString(path) {
			continue
		}
		for _, re := range compiled {
			if re.MatchString(path) {
				return true, nil
			}
		}
	}
	return false, nil
}

func collectPaths(files []PRFile) []string {
	var paths []string
	for _, f := range files {
		if f.Filename != "" {
			paths = append(paths, f.Filename)
		}
		if f.PreviousFilename != "" {
			paths = append(paths, f.PreviousFilename)
		}
	}
	return paths
}

type Client interface {
	GetPullRequest(pr int) (PRMeta, error)
	ListPullRequestFiles(pr int) ([]PRFile, error)
}

func Run(cfg Config, client Client, stdout io.Writer) error {
	lane := LaneHuman
	if cfg.Author == RenovateAuthor {
		lane = LaneRenovate
	}

	allowlist, err := parseListFile(cfg.AllowlistPath)
	if err != nil {
		return fmt.Errorf("read allowlist %s: %w", cfg.AllowlistPath, err)
	}

	protectedPath := cfg.ProtectedPathsPath
	if lane == LaneRenovate {
		protectedPath = cfg.RenovateProtectedPathsPath
	}

	protected, err := parseListFile(protectedPath)
	if err != nil {
		return fmt.Errorf("read protected paths %s: %w", protectedPath, err)
	}

	in := Inputs{
		Lane:              lane,
		Author:            cfg.Author,
		EventActor:        cfg.EventActor,
		Allowlist:         allowlist,
		ProtectedPatterns: protected,
	}

	proceed := (lane == LaneRenovate && isTrustedRenovateActor(cfg.EventActor)) || containsExact(allowlist, cfg.Author)
	if proceed && len(protected) > 0 {
		meta, metaErr := client.GetPullRequest(cfg.PRNumber)
		in.PRMeta = &meta
		in.PRMetaErr = metaErr

		if metaErr == nil && meta.ChangedFiles < changedFilesCap {
			files, filesErr := client.ListPullRequestFiles(cfg.PRNumber)
			in.PRFiles = files
			in.PRFilesErr = filesErr
		}
	}

	dec := Decide(in)
	return emit(dec, cfg.Author, stdout)
}

func emit(dec Decision, author string, stdout io.Writer) error {
	for _, msg := range dec.Warnings {
		fmt.Fprintf(stdout, "::warning::%s\n", msg)
	}
	for _, msg := range dec.Notices {
		fmt.Fprintf(stdout, "::notice::%s\n", msg)
	}

	allowedStr := strconv.FormatBool(dec.Allowed)
	fmt.Fprintf(stdout, "::notice::author=%s lane=%s allowlisted=%s\n", author, dec.Lane, allowedStr)

	return writeGitHubOutput(dec.Allowed, dec.Lane)
}

func writeGitHubOutput(allowed bool, lane string) error {
	path := os.Getenv("GITHUB_OUTPUT")
	allowedStr := strconv.FormatBool(allowed)
	lines := fmt.Sprintf("allowed=%s\nlane=%s\n", allowedStr, lane)
	if path == "" {
		_, err := fmt.Fprint(os.Stdout, lines)
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open GITHUB_OUTPUT: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(lines); err != nil {
		return fmt.Errorf("write GITHUB_OUTPUT: %w", err)
	}
	return nil
}
