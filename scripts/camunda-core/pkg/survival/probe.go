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

package survival

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Probe counts one kind of entity by running a command in a pod. The command
// must print a single integer on stdout.
type Probe struct {
	// Name identifies the entity in the report, e.g. "keycloak-users".
	Name string `yaml:"name" json:"name"`
	// Selector picks the pod to run in, e.g. "app.kubernetes.io/name=postgresql".
	Selector string `yaml:"selector" json:"selector"`
	// Container is optional when the pod has one container.
	Container string `yaml:"container,omitempty" json:"container,omitempty"`
	// Command is passed to sh -c inside the pod.
	Command string `yaml:"command" json:"command"`
}

// Runner executes a command in a pod chosen by label selector.
type Runner interface {
	ExecInPod(ctx context.Context, namespace, selector, container, command string) (string, error)
}

// Run executes every probe and returns a snapshot.
//
// A probe that fails is omitted rather than recorded as zero. A missing entry
// compares as NotProbed, so a broken probe reads as "unknown" instead of
// manufacturing a wipe — the report must not invent data loss.
func Run(ctx context.Context, r Runner, namespace string, probes []Probe) (Snapshot, []error) {
	snap := Snapshot{}
	var errs []error

	for _, p := range probes {
		out, err := r.ExecInPod(ctx, namespace, p.Selector, p.Container, p.Command)
		if err != nil {
			errs = append(errs, fmt.Errorf("probe %s: %w", p.Name, err))
			continue
		}
		n, err := ParseCount(out)
		if err != nil {
			errs = append(errs, fmt.Errorf("probe %s: %w", p.Name, err))
			continue
		}
		snap[p.Name] = n
	}
	return snap, errs
}

// ParseCount reads a single integer from command output, tolerating the
// surrounding whitespace and blank lines that psql and similar tools emit.
func ParseCount(out string) (int, error) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		n, err := strconv.Atoi(line)
		if err != nil {
			return 0, fmt.Errorf("expected a count, got %q", line)
		}
		return n, nil
	}
	return 0, fmt.Errorf("no output")
}
