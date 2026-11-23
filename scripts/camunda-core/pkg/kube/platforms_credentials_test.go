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

package kube

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestApplyExternalSecretsOtherAppliesAfterStaleCredentialsAreGone(t *testing.T) {
	t.Parallel()

	const name = "camunda-helm-existing-secrets-8-7"
	chart := t.TempDir()
	dir := filepath.Join(chart, "test", "integration", "external-secrets")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	manifest := "apiVersion: external-secrets.io/v1\nkind: ExternalSecret\nmetadata:\n  name: " + name + "\nspec:\n  target:\n    name: integration-test-credentials\n"
	if err := os.WriteFile(filepath.Join(dir, "external-secret-integration-test-credentials.yaml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{externalSecretGVR.GroupVersion()})
	mapper.Add(externalSecretGVR.GroupVersion().WithKind("ExternalSecret"), meta.RESTScopeNamespace)
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{externalSecretGVR: "ExternalSecretList"},
		externalSecret(name, "integration-test-credentials"),
		externalSecret("external-secret-certificates", "aws-camunda-cloud-tls"),
	)
	terminating := map[string]bool{}
	dynamicClient.PrependReactor("delete", "externalsecrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		terminating[action.(k8stesting.DeleteAction).GetName()] = true
		return true, nil, nil
	})
	dynamicClient.PrependReactor("get", "externalsecrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		got := action.(k8stesting.GetAction).GetName()
		if !terminating[got] {
			return false, nil, nil
		}
		delete(terminating, got)
		if err := dynamicClient.Tracker().Delete(externalSecretGVR, "ns", got); err != nil {
			return true, nil, err
		}
		return true, externalSecret(got, "integration-test-credentials"), nil
	})
	var applied, appliedWhileTerminating []string
	dynamicClient.PrependReactor("patch", "externalsecrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		patched := action.(k8stesting.PatchAction).GetName()
		applied = append(applied, patched)
		if terminating[patched] {
			appliedWhileTerminating = append(appliedWhileTerminating, patched)
		}
		return true, &unstructured.Unstructured{}, nil
	})
	client := &Client{dynamicClient: dynamicClient, restMapper: mapper}

	if err := applyExternalSecretsOther(context.Background(), client, t.TempDir(), chart, "ns", "", ""); err != nil {
		t.Fatalf("applyExternalSecretsOther() error = %v", err)
	}

	if len(appliedWhileTerminating) > 0 {
		t.Fatalf("applied ExternalSecrets %v while the deleted one was still terminating", appliedWhileTerminating)
	}
	if !slices.Equal(applied, []string{name}) {
		t.Fatalf("applied ExternalSecrets = %v, want [%s]", applied, name)
	}

	list, err := dynamicClient.Resource(externalSecretGVR).Namespace("ns").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	var left []string
	for _, item := range list.Items {
		left = append(left, item.GetName())
	}
	if !slices.Equal(left, []string{"external-secret-certificates"}) {
		t.Fatalf("remaining ExternalSecrets = %v, want [external-secret-certificates]", left)
	}
}

func TestIntegrationCredentialsManifest(t *testing.T) {
	chart := "/repo/charts/camunda-platform-8.10"
	shared := "/repo/.github/config/external-secret"
	chartFile := filepath.Join(chart, "test", "integration", "external-secrets", "external-secret-integration-test-credentials.yaml")
	chartVault := filepath.Join(chart, "test", "integration", "external-secrets", "external-secret-integration-test-credentials-vault.yaml")
	sharedFile := filepath.Join(shared, "external-secret-integration-test-credentials.yaml")
	scenario := "/tmp/credentials-manifest-1.yaml"

	cases := []struct {
		name     string
		suffix   string
		manifest string
		present  []string
		want     string
		wantErr  bool
	}{
		{name: "scenario manifest replaces the CI one", manifest: scenario, present: []string{scenario, chartFile, sharedFile}, want: scenario},
		{name: "missing scenario manifest is an error, not a CI fallback", manifest: scenario, present: []string{chartFile, sharedFile}, wantErr: true},
		{name: "chart-specific file wins over the shared one", present: []string{chartFile, sharedFile}, want: chartFile},
		{name: "shared fallback", present: []string{sharedFile}, want: sharedFile},
		{name: "vault suffix selects the vault file", suffix: "-vault", present: []string{chartFile, chartVault}, want: chartVault},
		{name: "none present", present: nil, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			set := map[string]bool{}
			for _, p := range tc.present {
				set[p] = true
			}
			got, err := integrationCredentialsManifest(chart, shared, tc.suffix, tc.manifest, func(p string) bool { return set[p] })
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
