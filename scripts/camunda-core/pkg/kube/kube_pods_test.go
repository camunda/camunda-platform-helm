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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestListPods(t *testing.T) {
	pod := func(name, namespace string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	}

	t.Run("returns pods scoped to the namespace", func(t *testing.T) {
		client := newTestClient(pod("a", "target"), pod("b", "target"), pod("c", "other"))

		pods, err := client.ListPods(context.Background(), "target")
		if err != nil {
			t.Fatalf("ListPods() error = %v", err)
		}
		if len(pods.Items) != 2 {
			t.Fatalf("got %d pods, want 2", len(pods.Items))
		}
		for _, p := range pods.Items {
			if p.Namespace != "target" {
				t.Errorf("pod %q leaked from namespace %q", p.Name, p.Namespace)
			}
		}
	})

	t.Run("carries container statuses through", func(t *testing.T) {
		p := pod("stuck", "target")
		p.Status.ContainerStatuses = []corev1.ContainerStatus{{
			Name:  "postgresql",
			Image: "registry.camunda.cloud/vendor-ee/postgresql:15.18.0-debian-12-r17",
			State: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{
					Reason:  "ImagePullBackOff",
					Message: "manifest unknown",
				},
			},
		}}
		client := newTestClient(p)

		pods, err := client.ListPods(context.Background(), "target")
		if err != nil {
			t.Fatalf("ListPods() error = %v", err)
		}
		if len(pods.Items) != 1 {
			t.Fatalf("got %d pods, want 1", len(pods.Items))
		}
		waiting := pods.Items[0].Status.ContainerStatuses[0].State.Waiting
		if waiting == nil || waiting.Reason != "ImagePullBackOff" {
			t.Fatalf("waiting state not preserved: %+v", waiting)
		}
	})

	t.Run("empty namespace is rejected", func(t *testing.T) {
		client := newTestClient()
		if _, err := client.ListPods(context.Background(), ""); err == nil {
			t.Fatal("expected an error for an empty namespace")
		}
	})

	t.Run("no pods is not an error", func(t *testing.T) {
		client := newTestClient()
		pods, err := client.ListPods(context.Background(), "empty")
		if err != nil {
			t.Fatalf("ListPods() error = %v", err)
		}
		if len(pods.Items) != 0 {
			t.Fatalf("got %d pods, want 0", len(pods.Items))
		}
	})
}
