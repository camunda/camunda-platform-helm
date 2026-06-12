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

// Package harbortag resolves and parses the Camunda Helm release tags: rolling-
// tag resolution against an artifact's tag list, format validation, and
// version/sha/major extraction. It is a pure transform of the tag list + tag
// string; the Harbor API calls live in the caller.
//
// Tag forms:
//
//	dev:          {version}-dev-{sha}        e.g. 13.4.0-dev-abc1234, 14.0.0-alpha2-dev-abc1234
//	dev rolling:  {major}-dev-latest         e.g. 13-dev-latest
//	rc:           {version}-rc               e.g. 13.4.0-rc, 14.0.0-alpha2-rc
//	rc rolling:   {major}-rc-latest          e.g. 13-rc-latest
package harbortag

import (
	"fmt"
	"regexp"
)

// Kind selects the tag family (dev or rc).
type Kind string

const (
	Dev Kind = "dev"
	RC  Kind = "rc"
)

var (
	devRollingRe = regexp.MustCompile(`^[0-9]+-dev-latest$`)
	rcRollingRe  = regexp.MustCompile(`^[0-9]+-rc-latest$`)
	// Concrete tag patterns.
	devTagRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)?-dev-[a-f0-9]+$`)
	rcTagRe  = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)?-rc$`)
)

// IsRolling reports whether inputTag is a rolling tag ({major}-{kind}-latest)
// that must be resolved to a concrete tag from the artifact's tag list.
func IsRolling(inputTag string, kind Kind) bool {
	switch kind {
	case Dev:
		return devRollingRe.MatchString(inputTag)
	case RC:
		return rcRollingRe.MatchString(inputTag)
	}
	return false
}

// ResolveConcrete returns the first concrete {kind} tag in tags (order matters:
// the first match wins). Used to resolve a rolling tag against the list of tags
// Harbor reports for the rolling artifact.
func ResolveConcrete(tags []string, kind Kind) (string, error) {
	re := concreteRe(kind)
	for _, t := range tags {
		if re.MatchString(t) {
			return t, nil
		}
	}
	return "", fmt.Errorf("no concrete %s tag found in artifact tags", kind)
}

func concreteRe(kind Kind) *regexp.Regexp {
	if kind == RC {
		return rcTagRe
	}
	return devTagRe
}

// DevTag is the parsed form of a concrete dev tag.
type DevTag struct {
	ResolvedTag string // {version}-dev-{sha}
	Version     string // {version}
	SHA         string // {sha} (may be short; the workflow expands it via the GitHub API)
	ChartMajor  string // major of {version}
	RCTag       string // {version}-rc
	RCLatestTag string // {major}-rc-latest
}

// ParseDevTag validates a concrete dev tag and extracts its parts.
func ParseDevTag(tag string) (DevTag, error) {
	if !devTagRe.MatchString(tag) {
		return DevTag{}, fmt.Errorf("invalid dev tag format %q: expected {version}-dev-{sha} (e.g. 13.4.0-dev-abc1234)", tag)
	}
	version, sha, ok := cut(tag, "-dev-") // version before, sha after the LAST "-dev-"
	if !ok {
		return DevTag{}, fmt.Errorf("invalid dev tag %q: missing -dev-", tag)
	}
	major := majorOf(version)
	return DevTag{
		ResolvedTag: tag,
		Version:     version,
		SHA:         sha,
		ChartMajor:  major,
		RCTag:       version + "-rc",
		RCLatestTag: major + "-rc-latest",
	}, nil
}

// RcTag is the parsed form of a concrete rc tag.
type RcTag struct {
	ResolvedTag string // {version}-rc
	Version     string // {version}
	ChartMajor  string // major of {version}
}

// ParseRcTag validates a concrete rc tag and extracts its parts.
func ParseRcTag(tag string) (RcTag, error) {
	if !rcTagRe.MatchString(tag) {
		return RcTag{}, fmt.Errorf("invalid rc tag format %q: expected {version}-rc (e.g. 13.4.0-rc or 14.0.0-alpha2-rc)", tag)
	}
	version := tag[:len(tag)-len("-rc")]
	return RcTag{ResolvedTag: tag, Version: version, ChartMajor: majorOf(version)}, nil
}

// FindDevTag returns the first dev tag in tags (used by Public to recover the
// source dev tag — and thus commit SHA — that an RC artifact was built from).
// Returns "" when none is present.
func FindDevTag(tags []string) string {
	for _, t := range tags {
		if devTagRe.MatchString(t) {
			return t
		}
	}
	return ""
}

// CommitSHAFromDevTag extracts the {sha} from a dev tag (everything after the
// last "-dev-"). Returns "" if tag is not a dev tag.
func CommitSHAFromDevTag(tag string) string {
	if !devTagRe.MatchString(tag) {
		return ""
	}
	_, sha, _ := cut(tag, "-dev-")
	return sha
}

// cut splits s at the LAST occurrence of sep.
func cut(s, sep string) (before, after string, found bool) {
	idx := -1
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			idx = i
		}
	}
	if idx < 0 {
		return s, "", false
	}
	return s[:idx], s[idx+len(sep):], true
}

func majorOf(version string) string {
	for i := 0; i < len(version); i++ {
		if version[i] == '.' {
			return version[:i]
		}
	}
	return version
}
