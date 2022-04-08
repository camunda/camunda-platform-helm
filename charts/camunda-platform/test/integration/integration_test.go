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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"context"
	"testing"

	"github.com/camunda-cloud/zeebe/clients/go/pkg/pb"
	"github.com/camunda-cloud/zeebe/clients/go/pkg/zbc"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type integrationTest struct {
	suite.Suite
	chartPath   string
	release     string
	namespace   string
	kubeOptions *k8s.KubectlOptions
}

func TestIntegration(t *testing.T) {
	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	namespace := "camunda-platform-helm-test" // createNamespaceName()
	kubeOptions := k8s.NewKubectlOptions("gke_zeebe-io_europe-west1-b_zeebe-cluster", "", namespace)

	suite.Run(t, &integrationTest{
		chartPath:   chartPath,
		release:     "zeebe-cluster-helm-it",
		namespace:   namespace,
		kubeOptions: kubeOptions,
	})
}

func (s *integrationTest) SetupTest() {
	if _, err := k8s.GetNamespaceE(s.T(), s.kubeOptions, s.namespace); err != nil {
		k8s.CreateNamespace(s.T(), s.kubeOptions, s.namespace)
	}
}

func (s *integrationTest) TearDownTest() {
	if !s.T().Failed() {
		k8s.DeleteNamespace(s.T(), s.kubeOptions, s.namespace)
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
	}

	// then
	s.awaitCamundaPlatformPods()
	s.createProcessInstance()

	s.awaitElasticPods()
	JWTToken := s.tryTologinToIdentity()
	s.assertProcessDefinitionFromOperate(JWTToken)
	s.assertTasksFromTasklist()
}

func (s *integrationTest) assertProcessDefinitionFromOperate(jwtToken string) {
	message := retry.DoWithRetry(s.T(),
		"Try to query and assert process definition from operate",
		10,
		10*time.Second,
		func() (string, error) {
			responseBuf, err := s.queryProcessDefinitionsFromOperate(jwtToken)
			if err != nil {
				return "", err
			}

			jsonString := responseBuf.String()
			s.T().Logf("Request successful, got as response '%s'", jsonString)
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

			jsonString := responseBuf.String()
			s.T().Logf("Request successful, got as response '%s'", jsonString)
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


func (s *integrationTest) queryTasksFromTasklist() (*bytes.Buffer, error) {
	operateServiceName := fmt.Sprintf("%s-tasklist", s.release)
	endpoint, closeFn := s.createPortForwardedHttpClient(operateServiceName)
	defer closeFn()

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	httpClient := http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	err = s.loginOnService(endpoint, httpClient)
	if err != nil {
		return nil, err
	}

	// curl -i -H "Content-Type: application/json" -XPOST "http://localhost:8080/graphql" --cookie "ope-session"  -d '{"query": "{tasks(query:{}){name}}"}'
	return s.queryApi(httpClient, "http://"+endpoint+"/graphql", bytes.NewBufferString(`{"query": "{tasks(query:{}){name}}"}`))
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

func (s *integrationTest) resolveKeycloakServiceName() string {
	// Keycloak truncates at 20 chars since the node identifier in WildFly is limited to 23 characters.
	// see https://github.com/bitnami/charts/blob/master/bitnami/keycloak/templates/_helpers.tpl#L2
	keycloakServiceName := fmt.Sprintf("%s-keycl", s.release)
	keycloakServiceName = strings.TrimSuffix(keycloakServiceName[:20], "-")
	return keycloakServiceName
}

func (s *integrationTest) tryTologinToIdentity() string {
	return retry.DoWithRetry(s.T(),
		"Try to log in to Identity and verify returned JWT token",
		10,
		10*time.Second,
		func() (string, error) {
			jwt, err := s.assertLoginToIdentity()
			if err != nil {
				return "", err
			}
			s.T().Logf("Log in to Identity was successful, and JWT Token is valid.")
			return jwt, nil
		})
}

func (s *integrationTest) assertLoginToIdentity() (string, error) {
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

	// setup http client with cookie jar - necessary to store tokens
	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", err
	}
	httpClient := http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	sessionUrl, err := s.resolveSessionLoginUrl(err, "http://"+identityEndpoint+"/auth/login", httpClient)
	if err != nil {
		return "", err
	}
	s.T().Logf("Send log in request to %s", sessionUrl)

	// log in as demo:demo
	values := url.Values{
		"username": {"demo"},
		"password": {"demo"},
	}
	loginResponse, err := httpClient.PostForm(sessionUrl, values)
	if err != nil {
		return "", err
	}
	if loginResponse.StatusCode != 200 {
		return "", errors.New(fmt.Sprintf("On log in expected an 200 status code, but got %d", loginResponse.StatusCode))
	}
	s.T().Logf("Log in to identity sucessful! Trying JWT token now.")

	// The previous log in request caused to store a token in our cookie jar.
	// In order to verify whether this token is valid and works with identity we have to extract the token and set
	// the cookie value (JWT token) as authentication header.
	jwtToken, err := s.extractJWTTokenFromCookieJar(jar)
	if err != nil {
		return "", err
	}
	s.T().Logf("Extracted following JWT token from cookie jar '%s'.", jwtToken)

	verificationUrl := "http://" + identityEndpoint + "/api/clients"
	getRequest, err := http.NewRequest("GET", verificationUrl, nil)
	if err != nil {
		return "", err
	}
	getRequest.Header.Set("Authentication", "Bearer "+jwtToken)

	// verify the token with the get request
	getResponse, err := httpClient.Do(getRequest)
	if err != nil {
		return "", err
	}

	if getResponse.StatusCode != 200 {
		return "", errors.New(fmt.Sprintf("On validating JWT token expected an 200 status code, but got %d", getResponse.StatusCode))
	}
	return jwtToken, nil
}

func (s *integrationTest) resolveSessionLoginUrl(err error, loginUrl string, httpClient http.Client) (string, error) {
	// Send request to /auth/login, and follow redirect to keycloak to retrieve the login page.
	// We need to read the returned login page to get the correct URL with session code, only with this session code
	// we are able to log in correctly to keycloak / identity. Additionally, this kind of mimics the user interaction.

	request, err := http.NewRequest("GET", loginUrl, nil)
	if err != nil {
		return "", err
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return "", err
	}

	// The returned login page (from keycloak) is no valid html code, which means we can't use an HTML parser,
	// but we can extract the url via regex.
	//
	// Example form with corresponding URL we are looking for:
	//
	// <form id="kc-form-login" onsubmit="login.disabled = true; return true;"
	//		action="http://localhost:18080/auth/realms/camunda-platform/login-actions/authenticate?session_code=B0BxW2ST2DH0NYE1J-THQncuCVc2yPck5JFmgEnLWbM&amp;execution=be1c2750-2b28-4044-8cf3-22b1331efeae&amp;client_id=camunda-identity&amp;tab_id=tp2zBJnsh6o"
	//		method="post">
	//
	//
	defer response.Body.Close()
	body := response.Body
	b, err := io.ReadAll(body)

	regexCompiled := regexp.MustCompile("(action=\")(.*)(\"[\\s\\w]+=\")")
	match := regexCompiled.FindStringSubmatch(string(b))
	if len(match) < 3 {
		return "", errors.New(fmt.Sprintf("Expected to extract session url from response %s", string(b)))
	}
	sessionUrl := match[2]

	// the url is encoded in the html document, which means we need to replace some characters
	return strings.Replace(sessionUrl, "&amp;", "&", -1), nil
}

func (s *integrationTest) extractJWTTokenFromCookieJar(jar *cookiejar.Jar) (string, error) {
	cookies := jar.Cookies(&url.URL{Scheme: "http", Host: "localhost"})
	identityJWT := "IDENTITY_JWT"
	for _, cookie := range cookies {
		if cookie.Name == identityJWT {
			return cookie.Value, nil
		}
	}
	return "", errors.New("no JWT token found in cookie jar")
}

func (s *integrationTest) queryProcessDefinitionsFromOperate(jwtToken string) (*bytes.Buffer, error) {

	// in order to login to identity we need to port-forward to identity AND keycloak
	// identity needs to redirect (forward) requests to keycloak to enable the login

	// create keycloak port-forward
	keycloakServiceName := s.resolveKeycloakServiceName()
	_, closeKeycloakPortForward := s.createPortForwardedHttpClientWithPort(keycloakServiceName, 18080)
	defer closeKeycloakPortForward()

	// create operate port-forward
	operateServiceName := fmt.Sprintf("%s-operate", s.release)
	operateEndpoint, closeFn := s.createPortForwardedHttpClientWithPort(operateServiceName, 8080)
	defer closeFn()

	// setup http client with cookie jar - necessary to store tokens
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	httpClient := http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	sessionUrl, err := s.resolveSessionLoginUrl(err, "http://"+operateEndpoint+"/api/login", httpClient)
	if err != nil {
		return nil, err
	}
	s.T().Logf("Send log in request to %s", sessionUrl)

	// log in as demo:demo
	values := url.Values{
		"username": {"demo"},
		"password": {"demo"},
	}
	loginResponse, err := httpClient.PostForm(sessionUrl, values)
	if err != nil {
		return nil, err
	}
	if loginResponse.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("On log in expected an 200 status code, but got %d", loginResponse.StatusCode))
	}
	s.T().Logf("Log in to operate sucessful!")

	////////////////////////////////////////////////////////////////////////////////////////////////////////
	////////////////////////////////////////////////////////////////////////////////////////////////////////

	////////////////////////////////////////////////////////////////////////////////////////////////////////
	////////////////////////////////////////////////////////////////////////////////////////////////////////

	s.sendRequest(httpClient, "http://localhost:8080/v3/api-docs/v1")

	return s.queryApi(httpClient, "http://" + operateEndpoint + "/v1/process-definitions/search", bytes.NewBufferString("{}"))
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


func (s *integrationTest) sendRequest(httpClient http.Client, url string) (*bytes.Buffer, error) {
	s.T().Logf("Send GET request to '%s'", url)
	response, err := httpClient.Get(url)
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

func (s *integrationTest) loginOnService(endpoint string, httpClient http.Client) error {
	// curl --include --request POST --cookie-jar "ope-session" "http://localhost:8080/api/login?username=demo&password=demo"
	request, err := http.NewRequest("POST", "http://"+endpoint+"/api/login?username=demo&password=demo", nil)
	if err != nil {
		return err
	}
	request.Close = true

	_, err = httpClient.Do(request)
	if err != nil {
		return err
	}
	return nil
}

func (s *integrationTest) awaitCamundaPlatformPods() {
	// await that all Camunda Platform related pods become ready
	pods := k8s.ListPods(s.T(), s.kubeOptions, v1.ListOptions{LabelSelector: "app=camunda-platform"})

	for _, pod := range pods {
		k8s.WaitUntilPodAvailable(s.T(), s.kubeOptions, pod.Name, 1000, 1 * time.Second)
	}
}

func (s *integrationTest) awaitElasticPods() {
	// await that all elastic related pods become ready, otherwise operate and tasklist can't answer requests
	pods := k8s.ListPods(s.T(), s.kubeOptions, v1.ListOptions{LabelSelector: "app=elasticsearch-master"})

	for _, pod := range pods {
		k8s.WaitUntilPodAvailable(s.T(), s.kubeOptions, pod.Name, 10, 10*time.Second)
	}
}

func (s *integrationTest) deployProcess(err error, client zbc.Client) *pb.DeployProcessResponse {
	ctx, cancelFn := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFn()
	deployProcessResponse, err := client.NewDeployProcessCommand().AddResourceFile("it-test-process.bpmn").Send(ctx)
	s.Require().NoError(err, "failed to deploy process model")
	s.Require().Equal(1, len(deployProcessResponse.Processes))
	return deployProcessResponse
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

func (s *integrationTest) createPortForwardedClient(serviceName string) (zbc.Client, func(), error) {
	// NOTE: this only waits until the service is created, not until the underlying pods are ready to receive traffic
	k8s.WaitUntilServiceAvailable(s.T(), s.kubeOptions, serviceName, 90, 1*time.Second)

	// port forward the gateway service to avoid having to set up a public endpoint that the test can access externally
	localGatewayPort := k8s.GetAvailablePort(s.T())
	tunnel := k8s.NewTunnel(s.kubeOptions, k8s.ResourceTypeService, serviceName, localGatewayPort, 26500)

	// the gateway is not ready/receiving traffic until at least one leader is present
	s.waitUntilPortForwarded(tunnel, 30, 2*time.Second)

	endpoint := fmt.Sprintf("localhost:%d", localGatewayPort)
	client, err := zbc.NewClient(&zbc.ClientConfig{
		GatewayAddress:         endpoint,
		DialOpts:               []grpc.DialOption{},
		UsePlaintextConnection: true,
	})
	if err != nil {
		return nil, tunnel.Close, err
	}

	return client, func() { client.Close(); tunnel.Close() }, nil
}

func (s *integrationTest) createPortForwardedHttpClientWithPort(serviceName string, port int) (string, func()) {
	// NOTE: this only waits until the service is created, not until the underlying pods are ready to receive traffic
	k8s.WaitUntilServiceAvailable(s.T(), s.kubeOptions, serviceName, 90, 1*time.Second)

	// remote port needs to be container port - not service port!
	tunnel := k8s.NewTunnel(s.kubeOptions, k8s.ResourceTypeService, serviceName, port, 8080)

	// the gateway is not ready/receiving traffic until at least one leader is present
	s.waitUntilPortForwarded(tunnel, 30, 2*time.Second)

	endpoint := fmt.Sprintf("localhost:%d", port)
	return endpoint, func() {
		tunnel.Close()
	}
}

func (s *integrationTest) createPortForwardedHttpClient(serviceName string) (string, func()) {
	return s.createPortForwardedHttpClientWithPort(serviceName, k8s.GetAvailablePort(s.T()))
}

func (s *integrationTest) waitUntilPortForwarded(tunnel *k8s.Tunnel, retries int, sleepBetweenRetries time.Duration) {
	statusMsg := fmt.Sprintf("Waiting to port forward for endpoint %s", tunnel.Endpoint())
	message := retry.DoWithRetry(
		s.T(),
		statusMsg,
		retries,
		sleepBetweenRetries,
		func() (string, error) {
			err := tunnel.ForwardPortE(s.T())
			if err != nil {
				return "", err
			}

			return fmt.Sprintf("Endpoint %s is now forwarded", tunnel.Endpoint()), nil
		},
	)
	logger.Logf(s.T(), message)
}

func truncateString(str string, num int) string {
	shortenStr := str
	if len(str) > num {
		shortenStr = str[0:num]
	}
	return shortenStr
}

func createNamespaceName() string {
	// if triggered by a github action the environment variable is set
	// we use it to better identify the test
	commitSHA, exist := os.LookupEnv("GITHUB_SHA")
	namespace := "camunda-platform-"
	if !exist {
		namespace += strings.ToLower(random.UniqueId())
	} else {
		namespace += commitSHA
	}

	// max namespace length is 63 characters
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names
	return truncateString(namespace, 63)
}
