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
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"time"

	"github.com/camunda-cloud/zeebe/clients/go/pkg/zbc"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/retry"
	"google.golang.org/grpc"
)

func (s *integrationSuite) createPortForwardedClient(serviceName string) (zbc.Client, func(), error) {
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

func (s *integrationSuite) createPortForwardedHttpClientWithPort(serviceName string, port int) (string, func()) {
	return s.createPortForwardedHttpClientWithPortAndContainerPort(serviceName, port, 8080)
}

func (s *integrationSuite) createPortForwardedHttpClientWithPortAndContainerPort(serviceName string, port int, containerPort int) (string, func()) {
	// NOTE: this only waits until the service is created, not until the underlying pods are ready to receive traffic
	k8s.WaitUntilServiceAvailable(s.T(), s.kubeOptions, serviceName, 90, 1*time.Second)

	// remote port needs to be container port - not service port!
	tunnel := k8s.NewTunnel(s.kubeOptions, k8s.ResourceTypeService, serviceName, port, containerPort)

	// the gateway is not ready/receiving traffic until at least one leader is present
	s.waitUntilPortForwarded(tunnel, 30, 2*time.Second)

	endpoint := fmt.Sprintf("localhost:%d", port)
	return endpoint, tunnel.Close
}

func (s *integrationSuite) createPortForwardedHttpClient(serviceName string) (string, func()) {
	return s.createPortForwardedHttpClientWithPort(serviceName, k8s.GetAvailablePort(s.T()))
}

func (s *integrationSuite) waitUntilPortForwarded(tunnel *k8s.Tunnel, retries int, sleepBetweenRetries time.Duration) {
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

func (s *integrationSuite) createHttpClientWithJar() (http.Client, *cookiejar.Jar, error) {
	// setup http client with cookie jar - necessary to store tokens
	jar, err := cookiejar.New(nil)
	if err != nil {
		return http.Client{}, nil, err
	}
	httpClient := http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}
	return httpClient, jar, nil
}
