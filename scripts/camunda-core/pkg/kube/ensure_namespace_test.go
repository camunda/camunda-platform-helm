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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestEnsureNamespace_LeavesAnExistingNamespaceUntouched(t *testing.T) {
	meta := map[string]string{"cleaner/ttl": "8760h", "janitor/ttl": "8760h", "camunda.cloud/ephemeral": "false"}
	cs := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "persisted", Annotations: meta, Labels: map[string]string{"github-id": "x"},
	}})
	c := &Client{clientset: cs}

	created, err := c.EnsureNamespaceCreated(context.Background(), "persisted")
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Error("created = true for an existing namespace")
	}

	for _, a := range cs.Actions() {
		if verb := a.GetVerb(); verb != "get" && verb != "list" && verb != "watch" {
			t.Errorf("EnsureNamespace issued %q on an existing namespace; it must not modify it", verb)
		}
	}
	ns, err := cs.CoreV1().Namespaces().Get(context.Background(), "persisted", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ns.Annotations, meta) || ns.Labels["github-id"] != "x" {
		t.Errorf("metadata changed: annotations=%v labels=%v", ns.Annotations, ns.Labels)
	}
}

func TestEnsureNamespace_CreatesAMissingNamespace(t *testing.T) {
	cs := fake.NewSimpleClientset()
	c := &Client{clientset: cs}
	created, err := c.EnsureNamespaceCreated(context.Background(), "fresh")
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Error("created = false for a namespace this call created")
	}
	if _, err := cs.CoreV1().Namespaces().Get(context.Background(), "fresh", metav1.GetOptions{}); err != nil {
		t.Fatalf("namespace not created: %v", err)
	}
}
