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
	"testing"

	"scripts/camunda-core/pkg/versionmatrix"
)

func TestReadmeSectionsForRenderRefreshesOnlySelectedVersion(t *testing.T) {
	existing := `
## Helm chart 14.7.0

stale 14.7.0

## Helm chart 14.6.1

preserved 14.6.1
`
	entries := []versionmatrix.ChartEntry{
		{ChartVersion: "14.7.0"},
		{ChartVersion: "14.6.1"},
	}

	sections, err := readmeSectionsForRender(existing, entries, "14.7.0")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sections["14.7.0"]; ok {
		t.Error("selected section was not removed for regeneration")
	}
	if got := sections["14.6.1"]; got == "" {
		t.Error("unselected published section was not preserved")
	}
}

func TestReadmeSectionsForRenderRejectsUnknownVersion(t *testing.T) {
	entries := []versionmatrix.ChartEntry{{ChartVersion: "14.7.0"}}

	_, err := readmeSectionsForRender("", entries, "14.8.0")
	if err == nil {
		t.Fatal("expected unknown chart version error, got nil")
	}
}
