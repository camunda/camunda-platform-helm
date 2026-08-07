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

// Command upgrade-paths answers, for a given minor-version transition, "what
// breaks for a customer, and what must they do about it?"
//
// Each upgrade path is run twice against the target chart:
//
//	Run A (naive)      target chart + the SOURCE chart's values, unchanged.
//	                   The customer who upgrades without reading release notes.
//	Run B (remediated) the same, plus the transition's delta.values.yaml.
//	                   The customer who follows the upgrade guide.
//
// The delta starts empty at each minor fork; whatever accumulates in it by GA
// is that path's upgrade guide, kept honest by CI. Comparing the two runs
// yields one of four outcomes (see Classify): clean, remediated, unremediated,
// or a stale fixture.
//
// This command implements the render-only stage: it stops after `helm
// template`, so it needs no cluster and completes in seconds. Values-shaped
// breaks (removed keys, schema violations, the Helm CLI version constraint)
// surface here. Data loss, orphaned resources, and migration duration need the
// cluster stage and are not covered by this command.
//
// Usage:
//
//	upgrade-paths --from 8.9 --to 8.10                 # every archetype
//	upgrade-paths --from 8.8 --to 8.9 --path classic-bundled
//	upgrade-paths --from 8.9 --to 8.10 --json out.json --markdown out.md
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"scripts/camunda-core/pkg/chartvalues"
)

func main() {
	var (
		from     = flag.String("from", "", "source app version, e.g. 8.9 (required)")
		to       = flag.String("to", "", "target app version, e.g. 8.10 (required)")
		pathList = flag.String("path", "", "comma-separated archetypes; default all")
		repoRoot = flag.String("repo-root", "", "chart repo root; default auto-detected")
		discover = flag.Bool("discover", false,
			"after a Run A failure, iterate to find every removed/renamed key rather than only the first")
		docsRoot = flag.String("docs-root", "",
			"camunda-docs checkout; default is a sibling directory. Empty disables doc-coverage checking")
		jsonOut = flag.String("json", "", "write findings JSON to this file")
		mdOut   = flag.String("markdown", "", "write markdown report to this file")
		timeout = flag.Duration("timeout", 5*time.Minute, "overall timeout")
		quiet   = flag.Bool("quiet", false, "suppress the markdown report on stdout")
	)
	flag.Parse()

	if *from == "" || *to == "" {
		fmt.Fprintln(os.Stderr, "error: --from and --to are required")
		flag.Usage()
		os.Exit(2)
	}

	root, err := resolveRepoRoot(*repoRoot)
	if err != nil {
		fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	archetypes, err := selectArchetypes(root, *pathList)
	if err != nil {
		fatal(err)
	}
	if len(archetypes) == 0 {
		fatal(fmt.Errorf("no archetypes found under test/upgrade-paths/archetypes"))
	}

	docs := *docsRoot
	if docs == "" {
		docs = defaultDocsRoot(root)
	}

	runDir, err := os.MkdirTemp("", "upgrade-paths-run-")
	if err != nil {
		fatal(fmt.Errorf("create work dir: %w", err))
	}
	defer os.RemoveAll(runDir)

	renderer := NewHelmRenderer()

	// Non-fatal: charts vendor their dependencies, so this is normally a no-op.
	if err := renderer.DependencyBuild(ctx, ChartDir(root, *to)); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v (continuing; dependencies may be vendored)\n", err)
	}

	report := Report{
		Transition: fmt.Sprintf("%s-to-%s", *from, *to),
		From:       *from,
		To:         *to,
		Stage:      "render",
	}

	for _, name := range archetypes {
		t, err := LoadTransition(root, *from, *to, name)
		if err != nil {
			fatal(err)
		}

		a := Render(ctx, renderer, t, RunA, root, runDir)
		b := Render(ctx, renderer, t, RunB, root, runDir)

		if t.DeltaPath == "" {
			b = a
			b.Kind = RunB
		}

		outcome := Classify(a, b)
		pr := PathResult{
			Transition: report.Transition,
			Path:       name,
			Outcome:    outcome,
			RunA:       a,
			RunB:       b,
			HasDelta:   t.DeltaPath != "",
			HasRemedy:  t.RemedyPath != "",
			Findings:   BuildFindings(t, a, b, outcome),
		}
		pr.ScaffoldingKeys = chartvalues.LeafPaths(t.Delta.Scaffolding)

		if *discover && !a.Succeeded {
			workDir, err := os.MkdirTemp("", "upgrade-paths-"+name+"-")
			if err != nil {
				fatal(fmt.Errorf("create work dir: %w", err))
			}
			d, err := Discover(ctx, renderer, t, root, workDir)
			os.RemoveAll(workDir)
			if err != nil {
				fatal(err)
			}
			pr.Discovery = &d

			keys := make([]string, 0, len(d.Changes))
			for _, c := range d.Changes {
				keys = append(keys, c.Old)
			}
			cov, err := CheckDocsCoverage(docs, *from, *to, keys)
			if err != nil {
				fatal(fmt.Errorf("check docs coverage: %w", err))
			}
			pr.DocsCoverage = cov
		}

		report.Results = append(report.Results, pr)
	}

	md := report.Markdown()
	if !*quiet {
		fmt.Print(md)
	}
	if *mdOut != "" {
		if err := os.WriteFile(*mdOut, []byte(md), 0o644); err != nil {
			fatal(fmt.Errorf("write markdown: %w", err))
		}
	}
	if *jsonOut != "" {
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fatal(fmt.Errorf("marshal findings: %w", err))
		}
		if err := os.WriteFile(*jsonOut, append(b, '\n'), 0o644); err != nil {
			fatal(fmt.Errorf("write json: %w", err))
		}
	}

	os.Exit(report.ExitCode())
}

// selectArchetypes resolves the --path flag to a concrete list.
func selectArchetypes(root, list string) ([]string, error) {
	if strings.TrimSpace(list) == "" {
		return ListArchetypes(root)
	}
	var out []string
	for _, p := range strings.Split(list, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out, nil
}

// resolveRepoRoot finds the chart repo root by walking up for a charts/ dir.
func resolveRepoRoot(override string) (string, error) {
	if override != "" {
		return filepath.Abs(override)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if fileExists(filepath.Join(dir, "charts")) && fileExists(filepath.Join(dir, "Makefile")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate chart repo root from %s (pass --repo-root)", wd)
		}
		dir = parent
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(2)
}
