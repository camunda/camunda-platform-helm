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

package parity

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// normalize makes two manifest streams comparable without weakening the claim.
//
// The Helm CLI wraps each document with a "# Source:" comment and its own leading
// separator; the SDK returns the same documents without that framing. Both are
// parsed, keyed by apiVersion/kind/namespace/name, sorted, and re-marshalled
// canonically. Object content is therefore compared exactly — only document
// framing and ordering are neutralised.
func normalize(t *testing.T, manifest string) string {
	t.Helper()

	type doc struct {
		key  string
		body string
	}

	var docs []doc
	for _, chunk := range strings.Split(manifest, "\n---") {
		trimmed := strings.TrimSpace(chunk)
		if trimmed == "" {
			continue
		}

		var obj map[string]any
		require.NoErrorf(t, yaml.Unmarshal([]byte(trimmed), &obj),
			"manifest document is not valid YAML:\n%s", trimmed)
		if len(obj) == 0 {
			continue
		}

		canonical, err := yaml.Marshal(obj)
		require.NoError(t, err)

		docs = append(docs, doc{key: objectKey(obj), body: string(canonical)})
	}

	sort.Slice(docs, func(i, j int) bool { return docs[i].key < docs[j].key })

	var b strings.Builder
	for _, d := range docs {
		fmt.Fprintf(&b, "# %s\n%s---\n", d.key, d.body)
	}
	return b.String()
}

func objectKey(obj map[string]any) string {
	apiVersion, _ := obj["apiVersion"].(string)
	kind, _ := obj["kind"].(string)

	var name, namespace string
	if meta, ok := obj["metadata"].(map[string]any); ok {
		name, _ = meta["name"].(string)
		namespace, _ = meta["namespace"].(string)
	}
	return fmt.Sprintf("%s/%s/%s/%s", apiVersion, kind, namespace, name)
}
