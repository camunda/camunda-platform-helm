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

package chartvalues

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	return p
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()

	t.Run("parses keys", func(t *testing.T) {
		v, err := LoadFile(write(t, dir, "a.yaml", "global:\n  host: example.com\n"))
		require.NoError(t, err)
		assert.True(t, HasKey(v, "global.host"))
	})

	t.Run("comment-only file yields empty non-nil", func(t *testing.T) {
		v, err := LoadFile(write(t, dir, "c.yaml", "# nothing here\n"))
		require.NoError(t, err)
		require.NotNil(t, v)
		assert.Empty(t, v)
	})

	t.Run("missing file errors", func(t *testing.T) {
		_, err := LoadFile(filepath.Join(dir, "nope.yaml"))
		require.Error(t, err)
	})

	t.Run("malformed yaml errors", func(t *testing.T) {
		_, err := LoadFile(write(t, dir, "bad.yaml", "key: [unclosed\n"))
		require.Error(t, err)
	})
}

func TestMergeFiles(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "base.yaml",
		"global:\n  a: 1\n  nested:\n    x: base\nlist:\n  - one\n  - two\n")
	over := write(t, dir, "over.yaml",
		"global:\n  b: 2\n  nested:\n    y: over\nlist:\n  - only\n")

	got, err := MergeFiles([]string{base, over})
	require.NoError(t, err)

	g, ok := toMap(got["global"])
	require.True(t, ok)
	assert.Equal(t, 1, g["a"])
	assert.Equal(t, 2, g["b"])

	n, ok := toMap(g["nested"])
	require.True(t, ok)
	assert.Equal(t, "base", n["x"])
	assert.Equal(t, "over", n["y"], "maps merge recursively")

	assert.Len(t, got["list"], 1, "arrays are replaced wholesale, matching Helm")
}

func TestMergeDoesNotMutateInputs(t *testing.T) {
	a := Values{"global": Values{"x": 1}}
	b := Values{"global": Values{"y": 2}}

	_ = Merge(a, b)

	ag, _ := toMap(a["global"])
	assert.Len(t, ag, 1, "merge must not write into its arguments")
}

func TestDeleteKey(t *testing.T) {
	newVals := func() Values {
		return Values{
			"global": Values{"ingress": Values{"host": "example.com", "enabled": true}},
			"top":    "value",
		}
	}

	t.Run("removes a nested leaf and keeps siblings", func(t *testing.T) {
		v := newVals()
		assert.True(t, DeleteKey(v, "global.ingress.host"))
		assert.False(t, HasKey(v, "global.ingress.host"))
		assert.True(t, HasKey(v, "global.ingress.enabled"))
	})

	t.Run("removes a top-level key", func(t *testing.T) {
		v := newVals()
		assert.True(t, DeleteKey(v, "top"))
		assert.False(t, HasKey(v, "top"))
	})

	t.Run("leaves the parent map in place", func(t *testing.T) {
		v := newVals()
		DeleteKey(v, "global.ingress.host")
		DeleteKey(v, "global.ingress.enabled")
		assert.True(t, HasKey(v, "global.ingress"), "emptied parents are meaningful to the chart")
	})

	t.Run("reports absent paths", func(t *testing.T) {
		v := newVals()
		assert.False(t, DeleteKey(v, "global.ingress.missing"))
		assert.False(t, DeleteKey(v, "nope.nope"))
		assert.False(t, DeleteKey(v, "top.deeper"), "cannot descend through a scalar")
	})
}

func TestLoadDelta(t *testing.T) {
	dir := t.TempDir()

	t.Run("splits remove directive from set keys", func(t *testing.T) {
		p := write(t, dir, "d.yaml", `
$remove:
  - identityKeycloak
  - global.ingress.host
identity:
  externalDatabase:
    host: pg.example.com
`)
		d, err := LoadDelta(p)
		require.NoError(t, err)
		assert.Equal(t, []string{"global.ingress.host", "identityKeycloak"}, d.Remove,
			"sorted for deterministic application")
		assert.True(t, HasKey(d.Set, "identity.externalDatabase.host"))
		assert.NotContains(t, d.Set, RemoveDirective, "the directive is not a values key")
		assert.False(t, d.IsEmpty())
	})

	t.Run("empty file is an empty delta", func(t *testing.T) {
		d, err := LoadDelta(write(t, dir, "empty.yaml", "# no changes yet\n"))
		require.NoError(t, err)
		assert.True(t, d.IsEmpty())
	})

	t.Run("blank entries are skipped", func(t *testing.T) {
		d, err := LoadDelta(write(t, dir, "blank.yaml", "$remove:\n  - \"\"\n  - a.b\n"))
		require.NoError(t, err)
		assert.Equal(t, []string{"a.b"}, d.Remove)
	})

	t.Run("non-list remove directive errors", func(t *testing.T) {
		_, err := LoadDelta(write(t, dir, "bad1.yaml", "$remove: identityKeycloak\n"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a list")
	})

	t.Run("non-string entry errors", func(t *testing.T) {
		_, err := LoadDelta(write(t, dir, "bad2.yaml", "$remove:\n  - 42\n"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be strings")
	})
}

func TestDeltaApply(t *testing.T) {
	base := Values{
		"global":           Values{"ingress": Values{"host": "old.example.com"}},
		"identityKeycloak": Values{"enabled": true},
		"keep":             "untouched",
	}

	d := Delta{
		Remove: []string{"identityKeycloak", "global.ingress.host"},
		Set:    Values{"identity": Values{"externalDatabase": Values{"host": "pg"}}},
	}

	got := d.Apply(base)

	assert.False(t, HasKey(got, "identityKeycloak"))
	assert.False(t, HasKey(got, "global.ingress.host"))
	assert.True(t, HasKey(got, "identity.externalDatabase.host"))
	assert.Equal(t, "untouched", got["keep"])

	assert.True(t, HasKey(base, "identityKeycloak"), "Apply must not mutate its input")
}

func TestDeltaApplyRemovesBeforeSetting(t *testing.T) {
	base := Values{"auth": Values{"legacy": true}}
	d := Delta{
		Remove: []string{"auth.legacy"},
		Set:    Values{"auth": Values{"legacy": "re-added"}},
	}

	got := d.Apply(base)
	assert.Equal(t, "re-added", mustMap(t, got["auth"])["legacy"],
		"a delta can drop a key and re-add it")
}

func TestConsolidate(t *testing.T) {
	dir := t.TempDir()
	l1 := write(t, dir, "l1.yaml", "identityKeycloak:\n  enabled: true\nkeep: yes\n")
	l2 := write(t, dir, "l2.yaml", "global:\n  ingress:\n    host: old\n")
	delta := write(t, dir, "delta.yaml", "$remove:\n  - identityKeycloak\n  - global.ingress.host\nadded: true\n")
	out := filepath.Join(dir, "out.yaml")

	got, err := Consolidate([]string{l1, l2}, delta, out)
	require.NoError(t, err)

	assert.False(t, HasKey(got, "identityKeycloak"))
	assert.False(t, HasKey(got, "global.ingress.host"))
	assert.True(t, HasKey(got, "added"))

	reread, err := LoadFile(out)
	require.NoError(t, err)
	assert.Equal(t, got, reread, "the written file matches the returned document")
}

func TestConsolidateWithoutDelta(t *testing.T) {
	dir := t.TempDir()
	l1 := write(t, dir, "l1.yaml", "identityKeycloak:\n  enabled: true\n")
	out := filepath.Join(dir, "out.yaml")

	got, err := Consolidate([]string{l1}, "", out)
	require.NoError(t, err)
	assert.True(t, HasKey(got, "identityKeycloak"), "no delta leaves the merge untouched")
}

func mustMap(t *testing.T, v any) Values {
	t.Helper()
	m, ok := toMap(v)
	require.True(t, ok)
	return m
}

func TestLoadDeltaRename(t *testing.T) {
	dir := t.TempDir()

	t.Run("parses rename map", func(t *testing.T) {
		p := write(t, dir, "r.yaml", "$rename:\n  global.ingress.host: global.host\n$remove:\n  - elasticsearch\nkeep: 1\n")
		d, err := LoadDelta(p)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"global.ingress.host": "global.host"}, d.Rename)
		assert.Equal(t, []string{"elasticsearch"}, d.Remove)
		assert.NotContains(t, d.Set, RenameDirective)
		assert.Contains(t, d.Set, "keep")
	})

	t.Run("non-map rename errors", func(t *testing.T) {
		_, err := LoadDelta(write(t, dir, "r2.yaml", "$rename:\n  - a\n"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a map")
	})

	t.Run("non-string target errors", func(t *testing.T) {
		_, err := LoadDelta(write(t, dir, "r3.yaml", "$rename:\n  a.b: 42\n"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a string")
	})

	t.Run("rename alone is not empty", func(t *testing.T) {
		d, err := LoadDelta(write(t, dir, "r4.yaml", "$rename:\n  a: b\n"))
		require.NoError(t, err)
		assert.False(t, d.IsEmpty())
	})
}

func TestDeltaApplyRename(t *testing.T) {
	t.Run("moves the value and clears the old path", func(t *testing.T) {
		base := Values{"global": Values{"ingress": Values{"host": "ns.example.com"}}}
		d := Delta{Rename: map[string]string{"global.ingress.host": "global.host"}}

		got := d.Apply(base)

		v, ok := GetKey(got, "global.host")
		require.True(t, ok)
		assert.Equal(t, "ns.example.com", v, "the environment-specific value is preserved")
		assert.False(t, HasKey(got, "global.ingress.host"))
	})

	t.Run("absent source is skipped", func(t *testing.T) {
		base := Values{"other": 1}
		d := Delta{Rename: map[string]string{"global.ingress.host": "global.host"}}

		got := d.Apply(base)
		assert.False(t, HasKey(got, "global.host"))
	})

	t.Run("rename runs before remove", func(t *testing.T) {
		base := Values{"global": Values{"ingress": Values{"host": "h", "other": 1}}}
		d := Delta{
			Rename: map[string]string{"global.ingress.host": "global.host"},
			Remove: []string{"global.ingress"},
		}

		got := d.Apply(base)
		v, ok := GetKey(got, "global.host")
		require.True(t, ok, "the moved value survives removal of its old parent")
		assert.Equal(t, "h", v)
		assert.False(t, HasKey(got, "global.ingress"))
	})

	t.Run("does not mutate its input", func(t *testing.T) {
		base := Values{"global": Values{"ingress": Values{"host": "h"}}}
		d := Delta{Rename: map[string]string{"global.ingress.host": "global.host"}}
		_ = d.Apply(base)
		assert.True(t, HasKey(base, "global.ingress.host"))
	})
}

func TestSetKey(t *testing.T) {
	t.Run("creates intermediate maps", func(t *testing.T) {
		v := Values{}
		assert.True(t, SetKey(v, "a.b.c", "x"))
		got, ok := GetKey(v, "a.b.c")
		require.True(t, ok)
		assert.Equal(t, "x", got)
	})

	t.Run("refuses to descend through a scalar", func(t *testing.T) {
		v := Values{"a": "scalar"}
		assert.False(t, SetKey(v, "a.b", "x"))
		assert.Equal(t, "scalar", v["a"])
	})
}

func TestDeepCopyIsolatesNestedMutation(t *testing.T) {
	base := Values{
		"global": Values{"ingress": Values{"host": "h"}},
		"list":   []any{Values{"name": "a"}},
	}
	cp := DeepCopy(base)

	DeleteKey(cp, "global.ingress.host")
	mustMap(t, cp["list"].([]any)[0])["name"] = "changed"

	assert.True(t, HasKey(base, "global.ingress.host"), "nested maps are independent")
	assert.Equal(t, "a", mustMap(t, base["list"].([]any)[0])["name"], "slices are independent")
}

func TestDeltaApplyDoesNotMutateNestedInput(t *testing.T) {
	base := Values{"global": Values{"ingress": Values{"host": "h", "keep": 1}}}
	d := Delta{Remove: []string{"global.ingress.host"}}

	_ = d.Apply(base)

	assert.True(t, HasKey(base, "global.ingress.host"),
		"a nested removal must not reach back into the caller's document")
}

func TestLoadDeltaScaffolding(t *testing.T) {
	dir := t.TempDir()

	t.Run("splits scaffolding from customer-facing keys", func(t *testing.T) {
		p := write(t, dir, "s.yaml", `
$remove:
  - identityKeycloak
global:
  host: example.com
$scaffolding:
  identity:
    initContainers:
      - name: wait-for-keycloak
`)
		d, err := LoadDelta(p)
		require.NoError(t, err)
		assert.True(t, HasKey(d.Set, "global.host"), "product config stays customer-facing")
		assert.False(t, HasKey(d.Set, "identity.initContainers"), "scaffolding is not in Set")
		assert.True(t, HasKey(d.Scaffolding, "identity.initContainers"))
		assert.True(t, d.HasScaffolding())
	})

	t.Run("scaffolding alone is not an empty delta", func(t *testing.T) {
		d, err := LoadDelta(write(t, dir, "s2.yaml", "$scaffolding:\n  a: 1\n"))
		require.NoError(t, err)
		assert.False(t, d.IsEmpty())
	})

	t.Run("non-map scaffolding errors", func(t *testing.T) {
		_, err := LoadDelta(write(t, dir, "s3.yaml", "$scaffolding:\n  - a\n"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a map")
	})

	t.Run("no scaffolding leaves it absent", func(t *testing.T) {
		d, err := LoadDelta(write(t, dir, "s4.yaml", "$remove:\n  - a\n"))
		require.NoError(t, err)
		assert.False(t, d.HasScaffolding())
	})
}

func TestDeltaApplyAppliesScaffolding(t *testing.T) {
	base := Values{"identity": Values{"initContainers": []any{Values{"name": "old"}}}}
	d := Delta{
		Set:         Values{"global": Values{"host": "h"}},
		Scaffolding: Values{"identity": Values{"initContainers": []any{Values{"name": "new"}}}},
	}

	got := d.Apply(base)

	assert.True(t, HasKey(got, "global.host"))
	ics := got["identity"].(Values)["initContainers"].([]any)
	assert.Equal(t, "new", mustMap(t, ics[0])["name"],
		"scaffolding is applied so the run can succeed, even though it is not customer-facing")
}

func TestLeafPaths(t *testing.T) {
	v := Values{
		"identity": Values{
			"initContainers":   []any{Values{"name": "x"}},
			"externalDatabase": Values{"host": "pg", "port": 5432},
		},
		"top":   "scalar",
		"empty": Values{},
	}

	assert.Equal(t, []string{
		"empty",
		"identity.externalDatabase.host",
		"identity.externalDatabase.port",
		"identity.initContainers",
		"top",
	}, LeafPaths(v), "descent stops at the first non-map, so a list is named by its key")
}

func TestLeafPathsEmpty(t *testing.T) {
	assert.Empty(t, LeafPaths(Values{}))
}
