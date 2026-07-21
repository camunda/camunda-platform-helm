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
	"regexp"
	"strings"

	"scripts/camunda-core/pkg/executil"
	"scripts/camunda-core/pkg/releaseplease"
)

// parseHelmPin extracts the helm version from a .tool-versions file content.
func parseHelmPin(toolVersions string) string {
	for _, line := range strings.Split(toolVersions, "\n") {
		if strings.HasPrefix(line, "helm ") {
			if fields := strings.Fields(line); len(fields) >= 2 {
				return fields[1]
			}
		}
	}
	return ""
}

// helmPinAtTag reads the helm pin from .tool-versions as it existed at the
// release tag of the given chart version. Returns "" when the tag (or the
// file at that tag) does not exist — callers decide the fallback.
func helmPinAtTag(app, chartVersion string) string {
	return helmPinAtRef(releaseplease.ReleaseTag(app, chartVersion))
}

// helmPinAtRef reads the helm pin from .tool-versions at an arbitrary git ref
// (e.g. a legacy camunda-platform-<version> tag). Returns "" when the ref or
// the file does not exist.
func helmPinAtRef(ref string) string {
	out, err := executil.RunCommandCapture(
		context.Background(), "git", []string{"show", ref + ":.tool-versions"}, nil, "")
	if err != nil {
		return ""
	}
	return parseHelmPin(string(out))
}

// helmCLIAnnotationRe extracts the camunda.io/helmCLIVersion annotation value
// from a Chart.yaml body (quoted or bare).
var helmCLIAnnotationRe = regexp.MustCompile(`camunda\.io/helmCLIVersion:\s*"?([^"\n]+)"?`)

// chartVersionRe extracts the top-level version field of a Chart.yaml body.
var chartVersionRe = regexp.MustCompile(`(?m)^version:\s*"?([^"\n]+)"?`)

// helmCLIAnnotationAtRef reads the chart's camunda.io/helmCLIVersion
// annotation as recorded at a git ref — the authoritative supported-CLI list
// stamped at release time. Tries the per-minor chart dir first, then the two
// historical layouts (the alpha-era charts/camunda-platform-alpha dir and the
// legacy single-chart dir). A candidate only counts when its Chart.yaml
// version matches chartVersion — at old tags the sibling dirs hold OTHER
// minors' charts. Returns "" when no matching chart carries the annotation.
func helmCLIAnnotationAtRef(app, ref, chartVersion string) string {
	for _, path := range []string{
		"charts/camunda-platform-" + app + "/Chart.yaml",
		"charts/camunda-platform-alpha/Chart.yaml",
		"charts/camunda-platform/Chart.yaml",
	} {
		out, err := executil.RunCommandCapture(
			context.Background(), "git", []string{"show", ref + ":" + path}, nil, "")
		if err != nil {
			continue
		}
		body := string(out)
		if v := chartVersionRe.FindStringSubmatch(body); v == nil || strings.TrimSpace(v[1]) != chartVersion {
			continue
		}
		if m := helmCLIAnnotationRe.FindStringSubmatch(body); m != nil {
			if v := strings.TrimSpace(m[1]); v != "" {
				return v
			}
		}
	}
	return ""
}
