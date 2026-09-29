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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestResolveTopologyReleases_DerivesEveryDogfoodNamespace(t *testing.T) {
	releases, err := resolveTopologyReleases("../../..", "8.10", "dogfood", "dogfood")
	require.NoError(t, err)

	got := map[string]string{}
	for _, r := range releases {
		got[r.Suffix] = r.Namespace
	}

	assert.Equal(t, map[string]string{
		"hub":      "dogfood-hub",
		"plain":    "dogfood-plain",
		"mt":       "dogfood-mt",
		"pt":       "dogfood-pt",
		"optplain": "dogfood-optplain",
		"optmt":    "dogfood-optmt",
		"optptdef": "dogfood-optptdef",
		"optptta":  "dogfood-optptta",
		"optpttb":  "dogfood-optpttb",
	}, got)
	assert.Equal(t, "/optimize-plain", releases[4].OptimizeContextPath)
}

func TestResolveTopologyReleases_RejectsAScenarioWithoutATopology(t *testing.T) {
	_, err := resolveTopologyReleases("../../..", "8.10", "elasticsearch", "dogfood")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "declares no topology")
}

func TestResolveTopologyReleases_RejectsAnUnknownScenario(t *testing.T) {
	_, err := resolveTopologyReleases("../../..", "8.10", "not-a-scenario", "dogfood")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// The hub registers the clients every other release authenticates with, so a
// teardown that removed it first would strand them mid-delete.
func TestTeardownOrder_PutsTheHubLast(t *testing.T) {
	ordered := teardownOrder([]topologyRelease{
		{Role: "hub", Suffix: "hub", Namespace: "d-hub"},
		{Role: "orchestration", Suffix: "pt", Namespace: "d-pt"},
		{Role: "optimize", Suffix: "optptta", Namespace: "d-optptta"},
	})

	require.Len(t, ordered, 3)
	assert.Equal(t, []string{"d-pt", "d-optptta", "d-hub"}, []string{ordered[0].Namespace, ordered[1].Namespace, ordered[2].Namespace})
}

func pod(name string, ready ...bool) corev1.Pod {
	statuses := make([]corev1.ContainerStatus, 0, len(ready))
	for _, r := range ready {
		statuses = append(statuses, corev1.ContainerStatus{Ready: r})
	}
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     corev1.PodStatus{ContainerStatuses: statuses},
	}
}

func TestPodReadiness_CountsOnlyFullyReadyPods(t *testing.T) {
	ready, notReady := podReadiness([]corev1.Pod{
		pod("a", true, true),
		pod("b", true, false),
		pod("c", true),
	})

	assert.Equal(t, 2, ready)
	assert.Equal(t, []string{"b"}, notReady)
}

// A pod that reports no container statuses yet has not been observed ready, so
// counting it as ready would report a still-starting namespace as healthy.
func TestPodReadiness_TreatsAPodWithNoContainerStatusesAsNotReady(t *testing.T) {
	ready, notReady := podReadiness([]corev1.Pod{pod("pending")})

	assert.Equal(t, 0, ready)
	assert.Equal(t, []string{"pending"}, notReady)
}

func TestPodReadiness_CapsTheNotReadyList(t *testing.T) {
	pods := make([]corev1.Pod, 0, 8)
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		pods = append(pods, pod(name, false))
	}

	ready, notReady := podReadiness(pods)

	assert.Equal(t, 0, ready)
	assert.Len(t, notReady, 5)
}

func TestPersistTopologyNamespaces_AttemptsEveryExistingNamespace(t *testing.T) {
	releases := []topologyRelease{
		{Namespace: "dogfood-hub"},
		{Namespace: "dogfood-missing"},
		{Namespace: "dogfood-failing"},
		{Namespace: "dogfood-plain"},
	}
	var attempted []string
	out := &bytes.Buffer{}

	err := persistTopologyNamespaces(
		context.Background(), out, releases, "8760h",
		func(_ context.Context, namespace string) (bool, error) {
			return namespace != "dogfood-missing", nil
		},
		func(_ context.Context, namespace string) error {
			attempted = append(attempted, namespace)
			if namespace == "dogfood-failing" {
				return errors.New("apply failed")
			}
			return nil
		},
	)

	require.Error(t, err)
	assert.Equal(t, []string{"dogfood-hub", "dogfood-failing", "dogfood-plain"}, attempted)
	assert.Contains(t, err.Error(), "dogfood-missing")
	assert.Contains(t, err.Error(), "dogfood-failing")
	assert.Contains(t, out.String(), "persisted dogfood-plain")
}

func TestWriteTopologyStatus_DistinguishesMissingFromAPIErrors(t *testing.T) {
	releases := []topologyRelease{
		{Role: "hub", Namespace: "dogfood-hub"},
		{Role: "orchestration", Namespace: "dogfood-missing"},
		{Role: "orchestration", Namespace: "dogfood-error"},
	}
	out := &bytes.Buffer{}

	missing, unready, err := writeTopologyStatus(
		context.Background(), out,
		func(_ context.Context, namespace string) (*corev1.PodList, error) {
			switch namespace {
			case "dogfood-missing":
				return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, namespace)
			case "dogfood-error":
				return nil, errors.New("temporary API failure")
			default:
				return &corev1.PodList{Items: []corev1.Pod{pod("ready", true)}}, nil
			}
		},
		releases, "",
	)

	assert.Equal(t, 1, missing)
	assert.Equal(t, 0, unready)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dogfood-error")
	assert.Contains(t, out.String(), "| orchestration | `dogfood-missing` | absent |")
	assert.Contains(t, out.String(), "| orchestration | `dogfood-error` | error |")
}

func TestWriteTopologyStatus_UsesHubHostAndContextPathForOptimize(t *testing.T) {
	releases := []topologyRelease{
		{Role: "hub", Namespace: "dogfood-hub"},
		{Role: "optimize", Namespace: "dogfood-optplain", OptimizeContextPath: "/optimize-plain"},
	}
	out := &bytes.Buffer{}

	missing, unready, err := writeTopologyStatus(
		context.Background(), out,
		func(_ context.Context, namespace string) (*corev1.PodList, error) {
			return &corev1.PodList{Items: []corev1.Pod{pod(fmt.Sprintf("%s-pod", namespace), true)}}, nil
		},
		releases, "example.com",
	)

	require.NoError(t, err)
	assert.Zero(t, missing)
	assert.Zero(t, unready)
	assert.Contains(t, out.String(), "https://dogfood-hub.example.com/optimize-plain")
	assert.NotContains(t, out.String(), "https://dogfood-optplain.example.com")
}

func TestNewTopologyUninstallCommand_RejectsAMismatchedConfirmation(t *testing.T) {
	cmd := newTopologyUninstallCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--version", "8.10", "--scenario", "dogfood", "--base", "dogfood", "--confirm", "dogfoo"})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--confirm must repeat the base namespace")
}
