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

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/stretchr/testify/suite"

	"context"

	"github.com/camunda-cloud/zeebe/clients/go/pkg/pb"
	"github.com/camunda-cloud/zeebe/clients/go/pkg/zbc"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/retry"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type integrationSuite struct {
	suite.Suite
	chartPath   string
	release     string
	namespace   string
	kubeOptions *k8s.KubectlOptions
}

func (s *integrationSuite) getSecret(secretSuffix string, secretKey string) string {
	getSecret := k8s.GetSecret(s.T(), s.kubeOptions, s.release+secretSuffix)
	secret := string(getSecret.Data[secretKey])
	return secret
}

func (s *integrationSuite) assertProcessDefinitionFromOperate() {
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
			s.Require().GreaterOrEqual(float64(1), total)

			s.Require().Contains(jsonString, "it-test-process")

			return "Process definition 'it-test-process' successful queried from operate!", nil
		})
	s.T().Logf(message)
}

func (s *integrationSuite) assertTasksFromTasklist() {
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
			s.Require().GreaterOrEqual(1, len(tasks))
			task := tasks[0].(map[string]interface{})
			s.Require().Equal("It Test", task["name"])

			return "User Task 'It Test' successful queried from tasklist!", nil
		})
	s.T().Logf(message)
}

func (s *integrationSuite) tryToLoginToOptimize() {
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

func (s *integrationSuite) loginToOptimize() error {
	_, _, closeFn, err := s.doLogin("optimize", 8083, 8090)
	defer closeFn()
	if err != nil {
		return err
	}

	return nil
}

func (s *integrationSuite) queryTasksFromTasklist() (*bytes.Buffer, error) {
	endpoint, httpClient, closeFn, err := s.doLogin("tasklist", 8082, 8080)
	defer closeFn()
	if err != nil {
		return nil, err
	}

	// curl -i -H "Content-Type: application/json" -XPOST "http://localhost:8080/graphql" --cookie "ope-session"  -d '{"query": "{tasks(query:{}){name}}"}'
	return s.queryApi(httpClient, "http://"+endpoint+"/graphql", bytes.NewBufferString(`{"query": "{tasks(query:{}){name}}"}`))
}

func (s *integrationSuite) deployProcess(err error, client zbc.Client) *pb.DeployProcessResponse {
	ctx, cancelFn := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFn()
	deployProcessResponse, err := client.NewDeployProcessCommand().AddResourceFile("it-test-process.bpmn").Send(ctx)
	s.Require().NoError(err, "failed to deploy process model")
	s.Require().Equal(1, len(deployProcessResponse.Processes))
	return deployProcessResponse
}

func (s *integrationSuite) createProcessInstance() {
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

func (s *integrationSuite) tryTologinToIdentity() {
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

func (s *integrationSuite) assertLoginToIdentity() error {
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

func (s *integrationSuite) queryProcessDefinitionsFromOperate() (*bytes.Buffer, error) {
	endpoint, httpClient, closeFn, err := s.doLogin("operate", 8081, 8080)
	defer closeFn()
	if err != nil {
		return nil, err
	}

	return s.queryApi(httpClient, "http://"+endpoint+"/v1/process-definitions/search", bytes.NewBufferString("{}"))
}

func (s *integrationSuite) queryApi(httpClient http.Client, url string, jsonData *bytes.Buffer) (*bytes.Buffer, error) {
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

func (s *integrationSuite) awaitAllPodsForThisRelease() {
	// await that all Camunda Platform related pods become ready
	pods := k8s.ListPods(s.T(), s.kubeOptions, v1.ListOptions{LabelSelector: "app.kubernetes.io/instance=" + s.release})

	for _, pod := range pods {
		k8s.WaitUntilPodAvailable(s.T(), s.kubeOptions, pod.Name, 1000, 1*time.Second)
	}
}

func (s *integrationSuite) awaitElasticPods() {
	// await that all elastic related pods become ready, otherwise operate and tasklist can't answer requests
	pods := k8s.ListPods(s.T(), s.kubeOptions, v1.ListOptions{LabelSelector: "release=" + s.release})

	for _, pod := range pods {
		k8s.WaitUntilPodAvailable(s.T(), s.kubeOptions, pod.Name, 10, 10*time.Second)
	}
}

func (s *integrationSuite) assertGatewayTopology(err error, client zbc.Client) {
	ctx, cancelFn := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFn()
	topology, err := client.NewTopologyCommand().Send(ctx)

	s.Require().NoError(err, "failed to obtain gateway topology")
	s.Require().EqualValues(3, topology.ClusterSize)
	s.Require().EqualValues(3, topology.PartitionsCount)
	s.Require().EqualValues(3, topology.ReplicationFactor)
}

func (s *integrationSuite) resolveKeycloakServiceName() string {
	// Keycloak truncates at 20 chars since the node identifier in WildFly is limited to 23 characters.
	// see https://github.com/bitnami/charts/blob/master/bitnami/keycloak/templates/_helpers.tpl#L2
	keycloakServiceName := fmt.Sprintf("%s-keycl", s.release)
	keycloakServiceName = strings.TrimSuffix(keycloakServiceName[:20], "-")
	return keycloakServiceName
}
