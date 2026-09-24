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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"scripts/deploy-camunda/matrix"
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

type fakeCredentialAPI struct {
	store      map[string]string
	exists     bool
	nsExists   bool
	creates    int
	updates    int
	raceOnce   map[string]string
	createErrs []error
	lastUpdate map[string]string
}

func (f *fakeCredentialAPI) api() topologyCredentialStore {
	return topologyCredentialStore{
		get: func(context.Context, string, string) (map[string]string, error) {
			if !f.exists {
				return nil, nil
			}
			return f.store, nil
		},
		create: func(_ context.Context, _, _ string, data map[string]string) error {
			f.creates++
			if f.raceOnce != nil {
				f.store, f.exists, f.raceOnce = f.raceOnce, true, nil
				return apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, "src")
			}
			f.store, f.exists = data, true
			return nil
		},
		update: func(_ context.Context, _, _ string, data map[string]string) error {
			f.updates++
			f.lastUpdate = data
			for k, v := range data {
				f.store[k] = v
			}
			return nil
		},
		namespaceExists: func(context.Context, string) (bool, error) { return f.nsExists, nil },
	}
}

func TestEnsureCredentials_WritesOnlyWhenSomethingIsMissing(t *testing.T) {
	ctx := context.Background()
	f := &fakeCredentialAPI{store: map[string]string{"prop-a": "a", "prop-b": "b"}, exists: true}

	var out bytes.Buffer
	if err := ensureCredentials(ctx, &out, []byte(twoPropertyManifest), "ns", nil, f.api()); err != nil {
		t.Fatal(err)
	}
	if f.updates+f.creates != 0 {
		t.Errorf("writes = %d, want 0 when every property exists", f.updates+f.creates)
	}
	if !strings.Contains(out.String(), "0 generated, 2 kept") {
		t.Errorf("output = %q", out.String())
	}

	delete(f.store, "prop-b")
	out.Reset()
	if err := ensureCredentials(ctx, &out, []byte(twoPropertyManifest), "ns", nil, f.api()); err != nil {
		t.Fatal(err)
	}
	if f.updates != 1 || f.creates != 0 || f.store["prop-a"] != "a" || f.store["prop-b"] == "" {
		t.Errorf("updates=%d creates=%d store=%v, want one update keeping prop-a and generating prop-b", f.updates, f.creates, f.store)
	}
	if strings.Contains(out.String(), f.store["prop-b"]) {
		t.Error("output must never contain a credential value")
	}
}

func TestEnsureCredentials_WritesOnlyGeneratedKeys(t *testing.T) {
	f := &fakeCredentialAPI{store: map[string]string{"prop-a": "a"}, exists: true}
	if err := ensureCredentials(context.Background(), &bytes.Buffer{}, []byte(twoPropertyManifest), "ns", nil, f.api()); err != nil {
		t.Fatal(err)
	}
	if _, touched := f.lastUpdate["prop-a"]; touched || len(f.lastUpdate) != 1 || f.lastUpdate["prop-b"] == "" {
		t.Errorf("update carried %v; it must contain only the generated prop-b, so a concurrently rotated prop-a is never rewritten", f.lastUpdate)
	}
}

func TestEnsureCredentials_CreatesMissingSourceForFreshEnvironment(t *testing.T) {
	f := &fakeCredentialAPI{}
	if err := ensureCredentials(context.Background(), &bytes.Buffer{}, []byte(twoPropertyManifest), "ns", []string{"env-hub"}, f.api()); err != nil {
		t.Fatal(err)
	}
	if f.creates != 1 || f.updates != 0 || len(f.store) != 2 {
		t.Errorf("creates=%d updates=%d store=%d keys, want a single create with both properties", f.creates, f.updates, len(f.store))
	}
}

func TestEnsureCredentials_RefusesToCreateSourceForDeployedEnvironment(t *testing.T) {
	f := &fakeCredentialAPI{nsExists: true}
	err := ensureCredentials(context.Background(), &bytes.Buffer{}, []byte(twoPropertyManifest), "ns", []string{"env-hub"}, f.api())
	if err == nil || !strings.Contains(err.Error(), "lock existing services out") {
		t.Fatalf("err = %v, want refusal", err)
	}
	if f.creates+f.updates != 0 {
		t.Error("must not write when refusing")
	}
}

func TestEnsureCredentials_GuardsOnAnySurvivingNamespace(t *testing.T) {
	f := &fakeCredentialAPI{}
	api := f.api()
	api.namespaceExists = func(_ context.Context, ns string) (bool, error) { return ns == "env-plain", nil }
	err := ensureCredentials(context.Background(), &bytes.Buffer{}, []byte(twoPropertyManifest), "ns", []string{"env-hub", "env-plain", "env-mt"}, api)
	if err == nil || !strings.Contains(err.Error(), "env-plain") {
		t.Fatalf("err = %v, want a refusal naming the surviving env-plain", err)
	}
	if f.creates+f.updates != 0 {
		t.Error("must not write when refusing")
	}
}

func TestEnsureCredentials_RefusesToTopUpSourceForDeployedEnvironment(t *testing.T) {
	f := &fakeCredentialAPI{store: map[string]string{"prop-a": "a"}, exists: true, nsExists: true}
	err := ensureCredentials(context.Background(), &bytes.Buffer{}, []byte(twoPropertyManifest), "ns", []string{"env-hub"}, f.api())
	if err == nil || !strings.Contains(err.Error(), "prop-b") {
		t.Fatalf("err = %v, want refusal naming the missing property", err)
	}
	if f.updates != 0 {
		t.Error("must not write when refusing")
	}
}

func TestEnsureCredentials_TopsUpSourceForFreshEnvironment(t *testing.T) {
	f := &fakeCredentialAPI{store: map[string]string{"prop-a": "a"}, exists: true}
	if err := ensureCredentials(context.Background(), &bytes.Buffer{}, []byte(twoPropertyManifest), "ns", []string{"env-hub"}, f.api()); err != nil {
		t.Fatal(err)
	}
	if f.updates != 1 || f.store["prop-a"] != "a" || f.store["prop-b"] == "" {
		t.Errorf("updates=%d store=%v", f.updates, f.store)
	}
}

func TestEnsureCredentials_ConcurrentCreateKeepsTheWinnersValues(t *testing.T) {
	winner := map[string]string{"prop-a": "won-a", "prop-b": "won-b"}
	f := &fakeCredentialAPI{raceOnce: winner}
	var out bytes.Buffer
	if err := ensureCredentials(context.Background(), &out, []byte(twoPropertyManifest), "ns", []string{"env-hub"}, f.api()); err != nil {
		t.Fatal(err)
	}
	if f.store["prop-a"] != "won-a" || f.store["prop-b"] != "won-b" || f.updates != 0 {
		t.Errorf("store=%v updates=%d, want the concurrently created values untouched", f.store, f.updates)
	}
	if !strings.Contains(out.String(), "created concurrently") {
		t.Errorf("output = %q", out.String())
	}
}

func TestEnsureCredentials_PropagatesReadError(t *testing.T) {
	api := topologyCredentialStore{
		get: func(context.Context, string, string) (map[string]string, error) { return nil, errors.New("boom") },
	}
	if err := ensureCredentials(context.Background(), &bytes.Buffer{}, []byte(twoPropertyManifest), "ns", nil, api); err == nil {
		t.Fatal("want error")
	}
}

func TestDogfoodCredentialsManifest_ReadsOneSourceWithDistinctProperties(t *testing.T) {
	path := filepath.Join("..", "..", "..", "charts", "camunda-platform-8.10", "test", "integration", "external-secrets", "dogfood-credentials.yaml")
	manifest, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err = matrix.RenderCredentialsManifest(manifest, "dogfood")
	if err != nil {
		t.Fatal(err)
	}
	src, err := parseCredentialSource(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if src.Name != "dogfood-credentials" {
		t.Errorf("source for base dogfood = %q, want dogfood-credentials", src.Name)
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
