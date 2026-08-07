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
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"scripts/camunda-core/pkg/chartvalues"
)

// RunKind distinguishes the naive customer upgrade from the remediated one.
type RunKind string

const (
	// RunA renders the target chart with the source chart's values, unchanged.
	RunA RunKind = "A"
	// RunB additionally applies the transition's delta.
	RunB RunKind = "B"
)

// RenderResult captures one `helm template` invocation.
type RenderResult struct {
	Kind      RunKind       `json:"kind"`
	Succeeded bool          `json:"succeeded"`
	Stderr    string        `json:"stderr,omitempty"`
	Duration  time.Duration `json:"durationMs"`
	// Manifest is retained only on success.
	Manifest string `json:"-"`
	// ValuesFiles is the ordered -f list.
	ValuesFiles []string `json:"valuesFiles"`
}

// Renderer invokes Helm.
type Renderer interface {
	Template(ctx context.Context, chartDir string, valuesFiles []string) (stdout string, stderr string, err error)
	DependencyBuild(ctx context.Context, chartDir string) error
}

// HelmRenderer shells out to the real helm binary.
type HelmRenderer struct {
	Bin         string
	ReleaseName string
}

func NewHelmRenderer() *HelmRenderer {
	return &HelmRenderer{Bin: "helm", ReleaseName: "upgrade-path"}
}

func (h *HelmRenderer) Template(ctx context.Context, chartDir string, valuesFiles []string) (string, string, error) {
	args := []string{"template", h.ReleaseName, chartDir}
	for _, f := range valuesFiles {
		args = append(args, "-f", f)
	}
	cmd := exec.CommandContext(ctx, h.Bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// DependencyBuild resolves chart dependencies from Chart.lock.
func (h *HelmRenderer) DependencyBuild(ctx context.Context, chartDir string) error {
	cmd := exec.CommandContext(ctx, h.Bin, "dependency", "build", chartDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm dependency build: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// Render performs one A or B run.
//
// For Run B the delta is applied by consolidating the baseline layers into a
// single document and rewriting it, because a delta may remove keys and Helm
// cannot express removal through an additional -f.
func Render(ctx context.Context, r Renderer, t Transition, kind RunKind, repoRoot, workDir string) RenderResult {
	files := append([]string{}, t.BaselineLayers...)
	if kind == RunB && t.DeltaPath != "" {
		out := filepath.Join(workDir, fmt.Sprintf("run-b-%s.yaml", t.Archetype.Name))
		if _, err := chartvalues.Consolidate(t.BaselineLayers, t.DeltaPath, out); err != nil {
			return RenderResult{Kind: kind, ValuesFiles: files, Stderr: err.Error()}
		}
		files = []string{out}
	}

	res := RenderResult{Kind: kind, ValuesFiles: files}
	start := time.Now()
	stdout, stderr, err := r.Template(ctx, ChartDir(repoRoot, t.To), files)
	res.Duration = time.Since(start)
	res.Stderr = strings.TrimSpace(stderr)
	if err == nil {
		res.Succeeded = true
		res.Manifest = stdout
	}
	return res
}

var (
	reCamundaError = regexp.MustCompile(`\[camunda\]\[error\]\s*(.+)`)
	reTemplateAt   = regexp.MustCompile(`execution error at \(([^)]+)\)`)
	reHelmError    = regexp.MustCompile(`(?m)^Error:\s*(.+)$`)
)

// ErrorSignature is a stable, de-duplicatable summary of a render failure.
type ErrorSignature struct {
	Title  string
	Source string
	Raw    string
}

// Signature extracts a comparable signature from render stderr. The result
// must not vary with filesystem paths, timing, or ordering.
func Signature(stderr string) ErrorSignature {
	sig := ErrorSignature{Raw: strings.TrimSpace(stderr)}

	if m := reCamundaError.FindStringSubmatch(stderr); m != nil {
		sig.Title = collapseSpace(m[1])
	} else if m := reHelmError.FindStringSubmatch(stderr); m != nil {
		sig.Title = collapseSpace(m[1])
	} else if sig.Raw != "" {
		sig.Title = collapseSpace(firstLine(sig.Raw))
	}

	if m := reTemplateAt.FindStringSubmatch(stderr); m != nil {
		sig.Source = m[1]
	}

	// Helm prefixes the fail() message with "execution error at (...)".
	if sig.Source != "" {
		sig.Title = strings.TrimSpace(strings.TrimPrefix(sig.Title,
			fmt.Sprintf("execution error at (%s):", sig.Source)))
	}
	return sig
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
