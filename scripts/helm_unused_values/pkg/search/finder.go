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

package search

import (
	"camunda.com/helmunusedvalues/pkg/keys"
	"camunda.com/helmunusedvalues/pkg/output"
	"camunda.com/helmunusedvalues/pkg/patterns"
)

type Finder struct {
	UseRipgrep   bool
	TemplatesDir string
	Registry     *patterns.Registry
	Parallelism  int // Number of parallel workers (0 = auto)
	Display      *output.Display
}

func NewFinder(templatesDir string, registry *patterns.Registry, useRipgrep bool, display *output.Display) *Finder {
	return &Finder{
		TemplatesDir: templatesDir,
		Registry:     registry,
		UseRipgrep:   useRipgrep,
		Parallelism:  0,
		Display:      display,
	}
}

func (f *Finder) FindUnusedKeys(keys []string, showProgress bool) ([]keys.KeyUsage, error) {
	return f.analyzeKeys(keys, showProgress)
}
