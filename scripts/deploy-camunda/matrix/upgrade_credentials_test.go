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

package matrix

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// The current chart's credentials ExternalSecret must provide every key the
// previous chart's test scenarios reference, because an upgrade-minor Step 1
// installs the previous chart against the current chart's Secret. Keys the
// previous chart only carries for its own predecessor are not required.
func TestUpgradeMinorCredentialsCarryPreviousChartKeys(t *testing.T) {
	root := findRepoRoot(t)
	pairs := [][2]string{{"8.6", "8.7"}, {"8.7", "8.8"}, {"8.8", "8.9"}, {"8.9", "8.10"}}
	files := []string{
		"external-secret-integration-test-credentials.yaml",
		"external-secret-integration-test-credentials-vault.yaml",
	}

	for _, pair := range pairs {
		for _, file := range files {
			t.Run(pair[0]+"->"+pair[1]+"/"+file, func(t *testing.T) {
				previous := externalSecretKeys(t, root, pair[0], file)
				current := externalSecretKeys(t, root, pair[1], file)
				if previous == nil || current == nil {
					t.Skip("credentials ExternalSecret not present for both versions")
				}
				scenarios := scenarioText(t, root, pair[0])
				missing := []string{}
				for key := range previous {
					if !current[key] && referencesKey(scenarios, key) {
						missing = append(missing, key)
					}
				}
				sort.Strings(missing)
				assert.Empty(t, missing, "camunda-platform-%s %s lacks keys camunda-platform-%s reads", pair[1], file, pair[0])
			})
		}
	}
}

func externalSecretKeys(t *testing.T, root, version, file string) map[string]bool {
	t.Helper()
	path := filepath.Join(root, "charts", "camunda-platform-"+version, "test", "integration", "external-secrets", file)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)

	var doc struct {
		Spec struct {
			Target struct {
				Template struct {
					Data map[string]string `yaml:"data"`
				} `yaml:"template"`
			} `yaml:"target"`
			Data []struct {
				SecretKey string `yaml:"secretKey"`
			} `yaml:"data"`
		} `yaml:"spec"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &doc), path)

	keys := map[string]bool{}
	for key := range doc.Spec.Target.Template.Data {
		keys[key] = true
	}
	for _, entry := range doc.Spec.Data {
		keys[entry.SecretKey] = true
	}
	require.NotEmpty(t, keys, "no keys parsed from %s", path)
	return keys
}

func scenarioText(t *testing.T, root, version string) string {
	t.Helper()
	dir := filepath.Join(root, "charts", "camunda-platform-"+version, "test", "integration", "scenarios")
	var b strings.Builder
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b.Write(raw)
		b.WriteByte('\n')
		return nil
	})
	require.NoError(t, err)
	return b.String()
}

func referencesKey(text, key string) bool {
	return regexp.MustCompile(`(^|[^\w-])` + regexp.QuoteMeta(key) + `($|[^\w-])`).MatchString(text)
}
