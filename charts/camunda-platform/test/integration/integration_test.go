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

//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"context"
	"testing"

	"github.com/camunda-cloud/zeebe/clients/go/pkg/pb"
	"github.com/camunda-cloud/zeebe/clients/go/pkg/zbc"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIntegration(t *testing.T) {
	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	suite.Run(t, &integrationTest{
		chartPath: chartPath,
		release:   "camunda-platform-it",
	})
}

func (s *integrationTest) SetupTest() {
	s.namespace = createNamespaceName()
	s.kubeOptions = k8s.NewKubectlOptions("gke_zeebe-io_europe-west1-b_zeebe-cluster", "", s.namespace)

	if _, err := k8s.GetNamespaceE(s.T(), s.kubeOptions, s.namespace); err != nil {
		k8s.CreateNamespace(s.T(), s.kubeOptions, s.namespace)
	} else {
		s.T().Logf("Namespace: %s already exist!", s.namespace)
	}
}

func (s *integrationTest) TearDownTest() {
	if !s.T().Failed() {
		k8s.DeleteNamespace(s.T(), s.kubeOptions, s.namespace)
	} else {
		s.T().Logf("Test failed on namespace: %s!", s.namespace)
	}

}

func (s *integrationTest) TestServicesEnd2End() {
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

func (s *integrationTest) TestServicesEnd2EndShouldFailWithUpgrade() {
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

func (s *integrationTest) TestServicesEnd2EndWithUpgrade() {
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

func (s *integrationTest) getSecret(secretSuffix string, secretKey string) string {
	getSecret := k8s.GetSecret(s.T(), s.kubeOptions, s.release+secretSuffix)
	secret := string(getSecret.Data[secretKey])
	return secret
}

func (s *integrationTest) TestServicesEnd2EndWithConfig() {
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

func (s *integrationTest) assertProcessDefinitionFromOperate() {
	message := retry.DoWithRetry(s.T(),
		"Try to query and assert process definition from operate",
		10,
		10*time.Second,
		func() (string, error) {
			responseBuf, err := s.queryProcessDefinitionsFromOperate()
			if err != nil {
				return "", err
			}

			jsonString := responseBuf.String()
			var objectMap map[string]interface{}
			err = json.Unmarshal(responseBuf.Bytes(), &objectMap)
			if err != nil {
				return "", err
			}

			total := objectMap["total"].(float64)
			s.Require().GreaterOrEqual(total, float64(1))

			s.Require().Contains(jsonString, "it-test-process")

			return "Process definition 'it-test-process' successful queried from operate!", nil
		})
	s.T().Logf(message)
}

func (s *integrationTest) assertTasksFromTasklist() {
	message := retry.DoWithRetry(s.T(),
		"Try to query and assert process definition from operate",
		10,
		10*time.Second,
		func() (string, error) {
			responseBuf, err := s.queryTasksFromTasklist()
			if err != nil {
				return "", err
			}

			var objectMap map[string]interface{}
			err = json.Unmarshal(responseBuf.Bytes(), &objectMap)
			if err != nil {
				return "", err
			}

			data := objectMap["data"].(map[string]interface{})
			tasks := data["tasks"].([]interface{})
			s.Require().GreaterOrEqual(len(tasks), 1)
			task := tasks[0].(map[string]interface{})
			s.Require().Equal("It Test", task["name"])

			return "User Task 'It Test' successful queried from tasklist!", nil
		})
	s.T().Logf(message)
}

func (s *integrationTest) tryToLoginToOptimize() {
	message := retry.DoWithRetry(s.T(),
		"Try to login to Optimize",
		10,
		10*time.Second,
		func() (string, error) {
			err := s.loginToOptimize()
			if err != nil {
				return "", err
			}

			return "Login to Optimize successful!", nil
		})
	s.T().Logf(message)
}

func (s *integrationTest) loginToOptimize() error {
	_, _, closeFn, err := s.doLogin("optimize", 8083, 8090)
	defer closeFn()
	if err != nil {
		return err
	}

	return nil
}

func (s *integrationTest) queryTasksFromTasklist() (*bytes.Buffer, error) {
	endpoint, httpClient, closeFn, err := s.doLogin("tasklist", 8082, 8080)
	defer closeFn()
	if err != nil {
		return nil, err
	}

	// curl -i -H "Content-Type: application/json" -XPOST "http://localhost:8080/graphql" --cookie "ope-session"  -d '{"query": "{tasks(query:{}){name}}"}'
	return s.queryApi(httpClient, "http://"+endpoint+"/graphql", bytes.NewBufferString(`{"query": "{tasks(query:{}){name}}"}`))
}

func (s *integrationTest) deployProcess(err error, client zbc.Client) *pb.DeployProcessResponse {
	ctx, cancelFn := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFn()
	deployProcessResponse, err := client.NewDeployProcessCommand().AddResourceFile("it-test-process.bpmn").Send(ctx)
	s.Require().NoError(err, "failed to deploy process model")
	s.Require().Equal(1, len(deployProcessResponse.Processes))
	return deployProcessResponse
}

func (s *integrationTest) createProcessInstance() {
	serviceName := fmt.Sprintf("%s-zeebe-gateway", s.release)
	client, closeFn, err := s.createPortForwardedClient(serviceName)
	s.Require().NoError(err, "failed to create Zeebe client")
	defer closeFn()

	s.assertGatewayTopology(err, client)
	deployProcessResponse := s.deployProcess(err, client)
	ctx, cancelFn := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFn()

	message := retry.DoWithRetry(s.T(), "Try to create Process instance", 10, 1*time.Second, func() (string, error) {
		_, err = client.NewCreateInstanceCommand().ProcessDefinitionKey(deployProcessResponse.Processes[0].ProcessDefinitionKey).Send(ctx)
		return "Process instance created.", err
	})
	s.T().Logf(message)
}

func (s *integrationTest) tryTologinToIdentity() {
	retry.DoWithRetry(s.T(),
		"Try to log in to Identity and verify returned JWT token",
		10,
		10*time.Second,
		func() (string, error) {
			err := s.assertLoginToIdentity()
			if err != nil {
				return "", err
			}
			s.T().Logf("Log in to Identity was successful, and JWT Token is valid.")
			return "", nil
		})
}

func (s *integrationTest) assertLoginToIdentity() error {
	// in order to login to identity we need to port-forward to identity AND keycloak
	// identity needs to redirect (forward) requests to keycloak to enable the login

	// create keycloak port-forward
	keycloakServiceName := s.resolveKeycloakServiceName()
	_, closeKeycloakPortForward := s.createPortForwardedHttpClientWithPort(keycloakServiceName, 18080)
	defer closeKeycloakPortForward()

	// create identity port-forward
	identityServiceName := fmt.Sprintf("%s-identity", s.release)
	identityEndpoint, closeIdentityPortForward := s.createPortForwardedHttpClientWithPort(identityServiceName, 8080)
	defer closeIdentityPortForward()

	httpClient, jar, err := s.createHttpClientWithJar()
	if err != nil {
		return err
	}

	err = s.doSessionBasedLogin("http://"+identityEndpoint+"/auth/login", httpClient)
	if err != nil {
		return err
	}

	return s.doJWTBasedLogin(err, jar, identityEndpoint, httpClient)
}

func (s *integrationTest) queryProcessDefinitionsFromOperate() (*bytes.Buffer, error) {
	endpoint, httpClient, closeFn, err := s.doLogin("operate", 8081, 8080)
	defer closeFn()
	if err != nil {
		return nil, err
	}

	return s.queryApi(httpClient, "http://"+endpoint+"/v1/process-definitions/search", bytes.NewBufferString("{}"))
}

func (s *integrationTest) queryApi(httpClient http.Client, url string, jsonData *bytes.Buffer) (*bytes.Buffer, error) {
	s.T().Logf("Send POST request to '%s', with application/json data: '%s'", url, jsonData.String())
	response, err := httpClient.Post(url, "application/json", jsonData)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(response.Body)
	defer response.Body.Close()

	s.T().Logf("Got response: [statusCode: '%d', data:'%s']", response.StatusCode, buf.String())
	s.Require().Equal(200, response.StatusCode)

	if err != nil {
		return nil, err
	}
	return buf, nil
}

func (s *integrationTest) awaitAllPodsForThisRelease() {
	// await that all Camunda Platform related pods become ready
	pods := k8s.ListPods(s.T(), s.kubeOptions, v1.ListOptions{LabelSelector: "app.kubernetes.io/instance=" + s.release})

	for _, pod := range pods {
		k8s.WaitUntilPodAvailable(s.T(), s.kubeOptions, pod.Name, 1000, 1*time.Second)
	}
}

func (s *integrationTest) awaitElasticPods() {
	// await that all elastic related pods become ready, otherwise operate and tasklist can't answer requests
	pods := k8s.ListPods(s.T(), s.kubeOptions, v1.ListOptions{LabelSelector: "app=elasticsearch-master"})

	for _, pod := range pods {
		k8s.WaitUntilPodAvailable(s.T(), s.kubeOptions, pod.Name, 10, 10*time.Second)
	}
}

func (s *integrationTest) assertGatewayTopology(err error, client zbc.Client) {
	ctx, cancelFn := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFn()
	topology, err := client.NewTopologyCommand().Send(ctx)

	s.Require().NoError(err, "failed to obtain gateway topology")
	s.Require().EqualValues(3, topology.ClusterSize)
	s.Require().EqualValues(3, topology.PartitionsCount)
	s.Require().EqualValues(3, topology.ReplicationFactor)
}
