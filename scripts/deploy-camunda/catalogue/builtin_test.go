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

package catalogue

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"scripts/deploy-camunda/matrix"
)

var updateGolden = flag.Bool("update-golden", false, "regenerate the builtin catalogue golden")

const goldenPath = "testdata/builtin_catalogue.golden"

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "charts", "chart-versions.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "repo root not found")
		dir = parent
	}
}

type manifestRows struct {
	Integration struct {
		Scenarios []struct {
			ID string `yaml:"id"`
		} `yaml:"scenarios"`
	} `yaml:"integration"`
}

func registryIDs(t *testing.T, root string) map[string][]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, "charts", chartDirPrefix+"*", "test", matrix.RegistryDirName, "manifest.yaml"))
	require.NoError(t, err)
	out := map[string][]string{}
	for _, p := range paths {
		rel, err := filepath.Rel(filepath.Join(root, "charts"), p)
		require.NoError(t, err)
		version := strings.TrimPrefix(strings.SplitN(filepath.ToSlash(rel), "/", 2)[0], chartDirPrefix)
		data, err := os.ReadFile(p)
		require.NoError(t, err)
		var m manifestRows
		require.NoError(t, yaml.Unmarshal(data, &m))
		for _, s := range m.Integration.Scenarios {
			out[version] = append(out[version], s.ID)
		}
	}
	return out
}

func TestBuiltinCatalogueGolden(t *testing.T) {
	root := repoRoot(t)
	recipes, err := LoadBuiltin(root)
	require.NoError(t, err)

	want := registryIDs(t, root)
	require.NotEmpty(t, want)
	got := map[string][]string{}
	total := 0
	for _, r := range recipes {
		assert.Equal(t, APIVersion, r.APIVersion)
		assert.Equal(t, KindRecipe, r.Kind)
		assert.Equal(t, BuiltinCatalogueName(r.Spec.Chart.Version), r.Metadata.Catalogue)
		got[r.Spec.Chart.Version] = append(got[r.Spec.Chart.Version], r.Metadata.Name)
	}
	for _, names := range want {
		total += len(names)
	}
	assert.Len(t, recipes, total, "recipe count must equal registry scenario count")
	assert.Equal(t, want, got)

	versions, err := matrix.LoadChartVersions(root)
	require.NoError(t, err)
	for _, v := range versions.ChartAutomation.RoutineVersions {
		assert.NotEmpty(t, got[v], "routine chart version %s has no builtin recipes", v)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	require.NoError(t, enc.Encode(recipes))
	require.NoError(t, enc.Close())

	if *updateGolden {
		require.NoError(t, os.WriteFile(goldenPath, buf.Bytes(), 0o644))
		return
	}
	golden, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "run: go test ./catalogue/ -run TestBuiltinCatalogueGolden -update-golden")
	assert.Equal(t, string(golden), buf.String())
}
