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

package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const twoPropertyManifest = `
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: creds
spec:
  data:
    - secretKey: a
      remoteRef: {key: src, property: prop-b}
    - secretKey: b
      remoteRef: {key: src, property: prop-a}
    - secretKey: c
      remoteRef: {key: src, property: prop-a}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ignored
`

func TestParseCredentialSource_CollectsSortedUniqueProperties(t *testing.T) {
	src, err := parseCredentialSource([]byte(twoPropertyManifest))
	if err != nil {
		t.Fatal(err)
	}
	if src.Name != "src" {
		t.Errorf("Name = %q, want src", src.Name)
	}
	if got := strings.Join(src.Properties, ","); got != "prop-a,prop-b" {
		t.Errorf("Properties = %q, want prop-a,prop-b", got)
	}
}

func TestParseCredentialSource_RejectsMultipleSources(t *testing.T) {
	manifest := `
kind: ExternalSecret
spec:
  data:
    - remoteRef: {key: one, property: p}
    - remoteRef: {key: two, property: p}
`
	if _, err := parseCredentialSource([]byte(manifest)); err == nil || !strings.Contains(err.Error(), "exactly one source secret") {
		t.Fatalf("err = %v, want exactly-one-source error", err)
	}
}

func TestParseCredentialSource_RejectsIncompleteRemoteRef(t *testing.T) {
	manifest := `
kind: ExternalSecret
spec:
  data:
    - remoteRef: {key: one}
`
	if _, err := parseCredentialSource([]byte(manifest)); err == nil {
		t.Fatal("want error for missing property")
	}
}

func TestPlanCredentials_KeepsExistingAndFillsMissing(t *testing.T) {
	existing := map[string]string{"kept": "old", "empty": "", "extra": "x"}
	n := 0
	gen := func() (string, error) { n++; return "new" + string(rune('0'+n)), nil }

	merged, created, err := planCredentials(existing, []string{"empty", "kept", "missing"}, gen)
	if err != nil {
		t.Fatal(err)
	}
	if merged["kept"] != "old" {
		t.Errorf("kept = %q, want unchanged", merged["kept"])
	}
	if merged["extra"] != "x" {
		t.Errorf("extra = %q, want preserved", merged["extra"])
	}
	if merged["empty"] == "" || merged["missing"] == "" {
		t.Errorf("empty/missing not generated: %v", merged)
	}
	if got := strings.Join(created, ","); got != "empty,missing" {
		t.Errorf("created = %q, want empty,missing", got)
	}
}

func TestGenerateCredential_LengthAndAlphabet(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		v, err := generateCredential()
		if err != nil {
			t.Fatal(err)
		}
		if len(v) != credentialLength {
			t.Fatalf("len = %d, want %d", len(v), credentialLength)
		}
		if strings.Trim(v, credentialAlphabet) != "" {
			t.Fatalf("%q has characters outside the alphabet", v)
		}
		seen[v] = true
	}
	if len(seen) != 20 {
		t.Errorf("got %d distinct values from 20 draws", len(seen))
	}
}

func TestEnsureCredentials_WritesOnlyWhenSomethingIsMissing(t *testing.T) {
	ctx := context.Background()
	store := map[string]string{"prop-a": "a", "prop-b": "b"}
	writes := 0
	get := func(context.Context, string, string) (map[string]string, error) { return store, nil }
	put := func(_ context.Context, _, _ string, data map[string]string) error { writes++; store = data; return nil }

	var out bytes.Buffer
	if err := ensureCredentials(ctx, &out, []byte(twoPropertyManifest), "ns", get, put); err != nil {
		t.Fatal(err)
	}
	if writes != 0 {
		t.Errorf("writes = %d, want 0 when every property exists", writes)
	}
	if !strings.Contains(out.String(), "0 generated, 2 kept") {
		t.Errorf("output = %q", out.String())
	}

	delete(store, "prop-b")
	out.Reset()
	if err := ensureCredentials(ctx, &out, []byte(twoPropertyManifest), "ns", get, put); err != nil {
		t.Fatal(err)
	}
	if writes != 1 || store["prop-a"] != "a" || store["prop-b"] == "" {
		t.Errorf("writes=%d store=%v, want one write keeping prop-a and generating prop-b", writes, store)
	}
	if strings.Contains(out.String(), store["prop-b"]) {
		t.Error("output must never contain a credential value")
	}
}

func TestEnsureCredentials_PropagatesReadError(t *testing.T) {
	get := func(context.Context, string, string) (map[string]string, error) { return nil, errors.New("boom") }
	put := func(context.Context, string, string, map[string]string) error { t.Fatal("must not write"); return nil }
	if err := ensureCredentials(context.Background(), &bytes.Buffer{}, []byte(twoPropertyManifest), "ns", get, put); err == nil {
		t.Fatal("want error")
	}
}

func TestDogfoodCredentialsManifest_ReadsOneSourceWithDistinctProperties(t *testing.T) {
	path := filepath.Join("..", "..", "..", "charts", "camunda-platform-8.10", "test", "integration", "external-secrets", "dogfood-credentials.yaml")
	manifest, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src, err := parseCredentialSource(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if src.Name == "integration-test" {
		t.Fatal("dogfood credentials must not read the CI-wide integration-test source")
	}
	var doc externalSecretDoc
	if err := yaml.Unmarshal(manifest, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Spec.Data) != len(src.Properties) {
		t.Errorf("%d target keys share %d source properties; every credential must have its own value", len(doc.Spec.Data), len(src.Properties))
	}
}
