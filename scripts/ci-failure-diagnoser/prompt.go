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

package main

import (
	"fmt"
	"strings"
)

// Context describes the failed test scenario (passed in as CLI flags).
type Context struct {
	ChartVersion string // e.g. "8.10"
	Scenario     string // e.g. "elasticsearch"
	Flow         string // e.g. "install" or "upgrade-minor"
	Platform     string // e.g. "gke"
	Shortname    string // e.g. "es"
	Identity     string // e.g. "keycloak"
	Persistence  string // e.g. "elasticsearch"
	WorkflowURL  string // GitHub Actions run URL, included so reviewers can drill in
}

// systemPrompt is intentionally compact. It encodes only project-specific
// hints that aren't obvious from the diagnostics alone.
const systemPrompt = `You are a CI failure diagnoser for the camunda-platform-helm repository, which packages the Camunda 8 platform as Helm charts (versions 8.5–8.10).

Your job: given diagnostics from a failed integration test (pod status, events, pod logs, last lines of test output), identify the most likely root cause and propose a concrete next step. Be terse — a tired on-call engineer is reading this.

Project-specific signals to weigh:
- 8.6 and 8.7 use separate zeebe/operate/tasklist templates. 8.8+ uses a unified "orchestration" component (StatefulSet with all three).
- "global.elasticsearch.external" and "global.elasticsearch.tls.existingSecret" gate ES auth/TLS env injection in templates and have known coupling with the bundled subchart.
- Bitnami subcharts (Keycloak, Elasticsearch, PostgreSQL) define ` + "`extraEnvVars`" + ` arrays. Helm replaces arrays wholesale on override, while deploy-camunda's CLI deep-merges by name. Mismatches between layers are a frequent cause.
- Upgrade flows are two-step: install previous chart, then upgrade. Failures often happen during step 2 (helm upgrade) due to immutable field changes (e.g. service ports renamed between versions, statefulset selectors).
- Pre-install scripts in test/integration/scenarios/pre-setup-scripts/ create TLS secrets and similar prerequisites — missing secrets often point here.

Output format (markdown, no preamble):

### Likely cause
One paragraph, ≤3 sentences. Cite specific log lines or events.

### Evidence
Up to 5 bullets. Each is a quoted snippet (≤120 chars) from pods/events/logs that supports the diagnosis.

### Next step
One actionable suggestion. If a chart-version-specific gotcha (8.7 vs 8.8+, subchart array merge, immutable field on upgrade) applies, name it.

### Confidence
"high", "medium", or "low" — and one phrase explaining why.`

// BuildUserMessage assembles the per-failure context block. The sections are
// truncated upstream by TrimLogs / the caller.
func BuildUserMessage(ctx Context, d *Diagnostics) string {
	var b strings.Builder

	fmt.Fprintf(&b, "## Failed scenario\n\n")
	fmt.Fprintf(&b, "- Chart version: %s\n", fallback(ctx.ChartVersion, "unknown"))
	fmt.Fprintf(&b, "- Scenario: %s\n", fallback(ctx.Scenario, "unknown"))
	fmt.Fprintf(&b, "- Flow: %s\n", fallback(ctx.Flow, "unknown"))
	fmt.Fprintf(&b, "- Platform: %s\n", fallback(ctx.Platform, "unknown"))
	if ctx.Identity != "" {
		fmt.Fprintf(&b, "- Identity: %s\n", ctx.Identity)
	}
	if ctx.Persistence != "" {
		fmt.Fprintf(&b, "- Persistence: %s\n", ctx.Persistence)
	}
	if ctx.WorkflowURL != "" {
		fmt.Fprintf(&b, "- Run: %s\n", ctx.WorkflowURL)
	}
	fmt.Fprintf(&b, "- Namespace: %s\n", fallback(d.Namespace, "unknown"))

	if d.Pods != "" {
		fmt.Fprintf(&b, "\n## kubectl get pods\n\n```\n%s\n```\n", strings.TrimSpace(d.Pods))
	}
	if d.Events != "" {
		fmt.Fprintf(&b, "\n## Recent events\n\n```\n%s\n```\n", strings.TrimSpace(d.Events))
	}
	if d.TestOutputTail != "" {
		fmt.Fprintf(&b, "\n## Test output (tail)\n\n```\n%s\n```\n", strings.TrimSpace(d.TestOutputTail))
	}
	if len(d.PodLogs) > 0 {
		fmt.Fprintf(&b, "\n## Pod logs\n")
		for _, l := range d.PodLogs {
			fmt.Fprintf(&b, "\n### %s\n\n```\n%s\n```\n", l.Pod, strings.TrimSpace(l.Body))
		}
	}
	if len(d.CollectErrors) > 0 {
		fmt.Fprintf(&b, "\n## Collection errors\n")
		for _, e := range d.CollectErrors {
			fmt.Fprintf(&b, "- %s\n", e)
		}
	}

	return b.String()
}

func fallback(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
