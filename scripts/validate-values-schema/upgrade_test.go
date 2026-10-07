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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpgradeReportsCoverageWhenKeyIsRejected(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, key, template, allowlist, class string
		failures                              int
	}{
		{"uncovered", "lost.key", "", "", "UNCOVERED", 1},
		{"deprecated descendant", "old.child", `{{ include "camundaPlatform.keyDeprecated" (dict "condition" true "oldName" "old" "migration" "new") }}`, "", "deprecated", 0},
		{"removed descendant", "old.child", `{{ include "camundaPlatform.keyRemoved" (dict "condition" true "oldName" "old") }}`, "", "removed", 0},
		{"renamed descendant", "old.child", `{{ include "camundaPlatform.keyRenamed" (dict "condition" true "oldName" "old" "newName" "new") }}`, "", "removed", 0},
		{"reassigned root warning", "old", `{{- $warningMessage = printf "%s %s" "[camunda][warning]" "DEPRECATION: \"old\" is deprecated." -}}`, "", "deprecated", 0},
		{"allowlisted exact key", "old.key", "", `package deprecation; var allowlist = map[string]string{"old.key": "exception"}`, "allowlisted", 0},
		{"allowlist does not cover descendants", "old.key.child", "", `package deprecation; var allowlist = map[string]string{"old.key": "exception"}`, "UNCOVERED", 1},
		{"commented registration", "old", `{{/* {{ include "camundaPlatform.keyRemoved" (dict "condition" true "oldName" "old") }} */}}`, "", "UNCOVERED", 1},
		{"unrelated oldName", "old", `{{ dict "oldName" "old" }}`, "", "UNCOVERED", 1},
		{"prefix boundary", "older.child", `{{ include "camundaPlatform.keyRemoved" (dict "condition" true "oldName" "old") }}`, "", "UNCOVERED", 1},
		{"bespoke wildcard warning", "console.image", `{{- $warningMessage := printf "%s %s" "[camunda][warning]" "DEPRECATION: console.* configuration keys have no effect." -}}`, "", "deprecated", 0},
		{"console enabled requires its own warning", "console.enabled", `{{- $warningMessage := printf "%s %s" "[camunda][warning]" "DEPRECATION: console.* configuration keys have no effect." -}}`, "", "UNCOVERED", 1},
		{"bespoke quoted wildcard root", "global.identity.auth.console", `{{- $warningMessage := printf "%s %s" "[camunda][warning]" "DEPRECATION: \"global.identity.auth.console.*\" is no longer used." -}}`, "", "deprecated", 0},
		{"commented warning", "console.image", `{{/* {{- $warningMessage := printf "%s %s" "[camunda][warning]" "DEPRECATION: console.* ignored." -}} */}}`, "", "UNCOVERED", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			coverage, err := parseUpgradeCoverage(tc.template, tc.allowlist)
			require.NoError(t, err)
			var output bytes.Buffer

			failures := coverage.report(&output, "previous/values.yaml", []string{tc.key})

			require.Equal(t, tc.failures, failures)
			require.Contains(t, output.String(), tc.class+": "+tc.key)
			if tc.failures == 0 {
				require.Contains(t, output.String(), "INFO")
				require.NotContains(t, output.String(), "::error::")
			} else {
				require.Contains(t, output.String(), "::error::")
			}
		})
	}
}

func TestUpgradeRejectsMalformedAllowlist(t *testing.T) {
	t.Parallel()
	_, err := parseUpgradeCoverage("", "not Go source")
	require.Error(t, err)
}

func TestUpgradeReportsRemovedLeavesWhenUnknownParentHasCoverage(t *testing.T) {
	t.Parallel()
	coverage, err := parseUpgradeCoverage(`{{ include "camundaPlatform.keyRemoved" (dict "condition" true "oldName" "global.secrets.name") }}`, "")
	require.NoError(t, err)
	schema := strictify(mustJSON(t, `{"type":"object","properties":{"global":{"type":"object","properties":{}}}}`))
	values := mustYAMLMap(t, `{"global":{"secrets":{"name":"old","uncovered":true}}}`)

	keys := coverage.findUnknownKeys(schema, values)

	require.Equal(t, []string{"global.secrets.name", "global.secrets.uncovered"}, keys)
}

func TestUpgradeExpandsUnknownParentsWhenCoverageIsNested(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, template, allowlist, values string
		want                              []string
	}{
		{"deep registration", `{{ include "camundaPlatform.keyRemoved" (dict "condition" true "oldName" "parent.sub.field") }}`, "", `{"parent":{"sub":{"field":true,"lost":true}}}`, []string{"parent.sub.field", "parent.sub.lost"}},
		{"allowlisted leaf", "", `package deprecation; var allowlist = map[string]string{"parent.sub.field":"exception"}`, `{"parent":{"sub":{"field":true}}}`, []string{"parent.sub.field"}},
		{"wildcard preserves enabled exception", `{{- $warningMessage := printf "%s %s" "[camunda][warning]" "DEPRECATION: console.* ignored." -}}`, "", `{"console":{"enabled":true,"image":"old"}}`, []string{"console.enabled", "console.image"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			coverage, err := parseUpgradeCoverage(tc.template, tc.allowlist)
			require.NoError(t, err)
			schema := strictify(mustJSON(t, `{"type":"object","properties":{}}`))

			keys := coverage.findUnknownKeys(schema, mustYAMLMap(t, tc.values))

			require.Equal(t, tc.want, keys)
		})
	}
}

func TestPreviousMinorChartWhenMinorHasTwoDigits(t *testing.T) {
	t.Parallel()
	previous, err := previousMinorChart("charts/camunda-platform-8.10")
	require.NoError(t, err)
	require.Equal(t, "charts/camunda-platform-8.9", previous)
}

func TestPreviousMinorChartRejectsInvalidVersion(t *testing.T) {
	t.Parallel()
	_, err := previousMinorChart("charts/camunda-platform-alpha")
	require.Error(t, err)
}
