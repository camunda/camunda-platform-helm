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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Diagnostics is the in-memory representation of one diagnostics run, as
// produced by `deploy-camunda matrix run` under
// diagnostics/<namespace>/<timestamp>/.
type Diagnostics struct {
	Namespace      string
	CollectedAt    string
	KubeContext    string
	Pods           string
	Events         string
	TestOutputTail string
	PodLogs        []PodLog
	CollectErrors  []string
}

type PodLog struct {
	Pod  string
	Body string
}

// summaryFile mirrors the `diagnosticsSummary` struct in deploy-camunda's
// runner.go. Keep field names in sync.
type summaryFile struct {
	Namespace         string              `json:"namespace"`
	KubeContext       string              `json:"kubeContext,omitempty"`
	CollectedAt       string              `json:"collectedAt"`
	PodLogTailLines   int                 `json:"podLogTailLines"`
	Pods              string              `json:"pods,omitempty"`
	Events            string              `json:"events,omitempty"`
	PodLogs           []summaryFilePodLog `json:"podLogs,omitempty"`
	TestOutputLast200 string              `json:"testOutputLast200,omitempty"`
	Errors            []string            `json:"errors,omitempty"`
}

type summaryFilePodLog struct {
	Pod   string `json:"pod"`
	File  string `json:"file,omitempty"`
	Error string `json:"error,omitempty"`
}

// LoadDiagnostics walks `dir` looking for the most recent run directory
// (the one containing summary.json) and returns its parsed contents plus
// pod logs. `dir` is typically the artifact root downloaded by GH Actions.
func LoadDiagnostics(dir string) (*Diagnostics, error) {
	runDir, err := findRunDir(dir)
	if err != nil {
		return nil, err
	}

	summaryPath := filepath.Join(runDir, "summary.json")
	raw, err := os.ReadFile(summaryPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", summaryPath, err)
	}

	var s summaryFile
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", summaryPath, err)
	}

	d := &Diagnostics{
		Namespace:      s.Namespace,
		CollectedAt:    s.CollectedAt,
		KubeContext:    s.KubeContext,
		Pods:           s.Pods,
		Events:         s.Events,
		TestOutputTail: s.TestOutputLast200,
		CollectErrors:  s.Errors,
	}

	logsDir := filepath.Join(runDir, "logs")
	for _, entry := range s.PodLogs {
		if entry.File == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(logsDir, entry.File))
		if err != nil {
			d.CollectErrors = append(d.CollectErrors, fmt.Sprintf("read log %s: %v", entry.File, err))
			continue
		}
		d.PodLogs = append(d.PodLogs, PodLog{Pod: entry.Pod, Body: string(body)})
	}

	return d, nil
}

// findRunDir locates the most recent timestamped run directory. The artifact
// can either be the namespace directory itself (containing timestamps), or a
// parent directory containing one or more namespaces.
func findRunDir(root string) (string, error) {
	// Direct hit: `root` is already a run directory.
	if _, err := os.Stat(filepath.Join(root, "summary.json")); err == nil {
		return root, nil
	}

	candidates := []string{}
	err := filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort: skip unreadable entries
		}
		if filepath.Base(path) == "summary.json" {
			candidates = append(candidates, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk %s: %w", root, err)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no summary.json found under %s", root)
	}

	// Most-recent first (timestamps sort lexicographically).
	sort.Sort(sort.Reverse(sort.StringSlice(candidates)))
	return candidates[0], nil
}

// TrimLogs caps each pod log body to roughly maxBytes from the tail, the
// portion most likely to contain the failure cause. Always returns a fresh
// slice; never mutates the caller's data.
func (d *Diagnostics) TrimLogs(maxBytes int) {
	if maxBytes <= 0 {
		return
	}
	out := make([]PodLog, 0, len(d.PodLogs))
	for _, l := range d.PodLogs {
		body := l.Body
		if len(body) > maxBytes {
			// Cut at the next newline so we don't start mid-line.
			start := len(body) - maxBytes
			if nl := strings.IndexByte(body[start:], '\n'); nl >= 0 && start+nl+1 < len(body) {
				start += nl + 1
			}
			body = "...[truncated]...\n" + body[start:]
		}
		out = append(out, PodLog{Pod: l.Pod, Body: body})
	}
	d.PodLogs = out
}
