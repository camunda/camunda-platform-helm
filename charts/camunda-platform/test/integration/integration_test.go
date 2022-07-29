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

//go:build integration && !openshift
// +build integration,!openshift

package integration

import (
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestIntegration(t *testing.T) {
	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	suite.Run(t, &integrationSuite{
		chartPath: chartPath,
		release:   "camunda-platform-it",
	})
}

func (s *integrationSuite) SetupTest() {
	s.namespace = createNamespaceName()
	s.kubeOptions = k8s.NewKubectlOptions("gke_zeebe-io_europe-west1-b_zeebe-cluster", "", s.namespace)

	if _, err := k8s.GetNamespaceE(s.T(), s.kubeOptions, s.namespace); err != nil {
		k8s.CreateNamespace(s.T(), s.kubeOptions, s.namespace)
	} else {
		s.T().Logf("Namespace: %s already exist!", s.namespace)
	}
}

func (s *integrationSuite) TearDownTest() {
	if !s.T().Failed() {
		k8s.DeleteNamespace(s.T(), s.kubeOptions, s.namespace)
	} else {
		s.T().Logf("Test failed on namespace: %s!", s.namespace)
	}

}

func (s *integrationSuite) TestServicesEnd2End() {
	// given
	options := &helm.Options{
		KubectlOptions: s.kubeOptions,
	}

	// when
	if _, err := k8s.GetPodE(s.T(), s.kubeOptions, s.release+"-zeebe-0"); err != nil {
		helm.Install(s.T(), options, s.chartPath, s.release)
	} else {
		s.T().Logf("Helm chart was already installed, rerun assertions.")
	}

	// then
	s.awaitAllPodsForThisRelease()
	s.createProcessInstance()

	s.awaitElasticPods()
	s.tryTologinToIdentity()
	s.assertProcessDefinitionFromOperate()
	s.assertTasksFromTasklist()
	s.tryToLoginToOptimize()
}

func (s *integrationSuite) TestServicesEnd2EndShouldFailWithUpgrade() {
	// given
	options := &helm.Options{
		KubectlOptions: s.kubeOptions,
	}
	if _, err := k8s.GetPodE(s.T(), s.kubeOptions, s.release+"-zeebe-0"); err != nil {
		helm.Install(s.T(), options, s.chartPath, s.release)
	}

	// when
	err := helm.UpgradeE(s.T(), options, s.chartPath, s.release)

	// then
	s.Require().NotNil(err)
}

func (s *integrationSuite) TestServicesEnd2EndWithUpgrade() {
	// given
	options := &helm.Options{
		KubectlOptions: s.kubeOptions,
	}
	if _, err := k8s.GetPodE(s.T(), s.kubeOptions, s.release+"-zeebe-0"); err != nil {
		helm.Install(s.T(), options, s.chartPath, s.release)
	}
	tasklistSecret := s.getSecret("-tasklist-identity-secret", "tasklist-secret")
	operateSecret := s.getSecret("-operate-identity-secret", "operate-secret")
	optimizeSecret := s.getSecret("-optimize-identity-secret", "optimize-secret")
	keycloakAdminPassword := s.getSecret("-keycloak", "admin-password")
	keycloakManagementPassword := s.getSecret("-keycloak", "management-password")
	postgresqlPassword := s.getSecret("-postgresql", "postgres-password")

	// when
	upgradeOptions := &helm.Options{
		KubectlOptions: s.kubeOptions,
		SetStrValues: map[string]string{
			"global.identity.auth.tasklist.existingSecret": tasklistSecret,
			"global.identity.auth.optimize.existingSecret": optimizeSecret,
			"global.identity.auth.operate.existingSecret":  operateSecret,
			"identity.keycloak.auth.adminPassword":         keycloakAdminPassword,
			"identity.keycloak.auth.managementPassword":    keycloakManagementPassword,
			"identity.keycloak.postgresql.auth.password":   postgresqlPassword,
		},
	}
	helm.Upgrade(s.T(), upgradeOptions, s.chartPath, s.release)

	// then
	s.awaitAllPodsForThisRelease()
	s.createProcessInstance()

	s.awaitElasticPods()
	s.tryTologinToIdentity()
	s.assertProcessDefinitionFromOperate()
	s.assertTasksFromTasklist()
	s.tryToLoginToOptimize()
}

func (s *integrationSuite) TestServicesEnd2EndWithConfig() {
	// given
	options := &helm.Options{
		ValuesFiles:    []string{"it-values.yaml"},
		KubectlOptions: s.kubeOptions,
	}

	// when
	if _, err := k8s.GetPodE(s.T(), s.kubeOptions, s.release+"-zeebe-0"); err != nil {
		helm.Install(s.T(), options, s.chartPath, s.release)
	}

	// then
	s.awaitAllPodsForThisRelease()
	s.createProcessInstance()

	s.awaitElasticPods()
	s.tryTologinToIdentity()
	s.assertProcessDefinitionFromOperate()
	s.assertTasksFromTasklist()
}
