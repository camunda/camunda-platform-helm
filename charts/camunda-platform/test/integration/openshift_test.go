// Copyright 2022 Camunda Services GmbH
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

//go:build integration && openshift
// +build integration,openshift

package integration

import (
	"context"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	openshiftv1 "github.com/openshift/api/project/v1"
	projectv1 "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"
	"testing"
)

type openshiftSuite struct {
	integrationSuite
	oc *projectv1.ProjectV1Client
}

func TestOpenShift(t *testing.T) {
	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	oc, err := getOpenshiftProjectClient(t)
	require.NoError(t, err)

	suite.Run(t, &openshiftSuite{integrationSuite{
		chartPath: chartPath,
		release:   "camunda-platform-it",
	}, oc})
}

func (s *openshiftSuite) SetupTest() {
	s.namespace = createNamespaceName()
	s.kubeOptions = k8s.NewKubectlOptions("", "", s.namespace)

	if !s.doesProjectExist() {
		err := s.createProject()
		s.Require().NoError(err)
	}
}

func (s *openshiftSuite) TearDownTest() {
	if s.doesProjectExist() {
		err := s.deleteProject()
		if err != nil {
			s.T().Logf("Failed to delete project: %s", err)
		}
	}
}

func (s *openshiftSuite) TestServicesEnd2End() {
	// given
	options := &helm.Options{
		KubectlOptions: s.kubeOptions,
		ValuesFiles: []string{
			"../../../../openshift/values.yaml",
			"../../../../openshift/values-patch.yaml",
		},
		ExtraArgs: map[string][]string{
			"install": {"--post-renderer", "../../../../openshift/patch.sh"},
		},
	}

	// when
	helm.Install(s.T(), options, s.chartPath, s.release)

	// then
	s.awaitAllPodsForThisRelease()
	s.createProcessInstance()

	s.awaitElasticPods()
	s.tryTologinToIdentity()
	s.assertProcessDefinitionFromOperate()
	s.assertTasksFromTasklist()
	s.tryToLoginToOptimize()
}

func (s *openshiftSuite) createProject() error {
	project := &openshiftv1.ProjectRequest{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NameSpace",
			APIVersion: "openshiftv1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: s.namespace,
		},
		DisplayName: s.namespace,
		Description: "Temporary project for camunda-platform-helm integration tests",
	}

	if _, err := s.oc.ProjectRequests().Create(context.Background(), project, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func (s *openshiftSuite) deleteProject() error {
	err := s.oc.Projects().Delete(context.Background(), s.namespace, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (s *openshiftSuite) doesProjectExist() bool {
	projects, err := s.oc.Projects().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		s.T().Logf("Cannot list projects; project %s may already exist (err: %s)", s.namespace, err)
		return false
	}

	for _, p := range projects.Items {
		if p.Name == s.namespace {
			return true
		}
	}

	return false
}

func getOpenshiftProjectClient(t *testing.T) (*projectv1.ProjectV1Client, error) {
	kubeConfig, err := k8s.GetKubeConfigPathE(t)
	if err != nil {
		return nil, err
	}

	config, err := k8s.LoadConfigFromPath(kubeConfig).ClientConfig()
	if err != nil {
		return nil, err
	}

	return projectv1.NewForConfig(config)
}
