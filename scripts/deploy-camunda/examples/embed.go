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

// Package examples exposes the deploy-camunda starter configs shipped under
// scripts/deploy-camunda/examples/ as embedded bytes so `deploy-camunda config
// init --from-example <name>` works without a checked-out repo.
package examples

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

const suffix = ".deploy-camunda.yaml"

//go:embed *.deploy-camunda.yaml
var files embed.FS

// Load returns the raw bytes of the named example. The name is the base file
// name with the `.deploy-camunda.yaml` suffix stripped, e.g. "getting-started".
func Load(name string) ([]byte, error) {
	b, err := files.ReadFile(name + suffix)
	if err != nil {
		return nil, fmt.Errorf("example %q not found; run `deploy-camunda config init --list-examples` to see available templates", name)
	}
	return b, nil
}

// Names returns the available example names (suffix stripped, sorted).
func Names() []string {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), suffix))
	}
	sort.Strings(names)
	return names
}
