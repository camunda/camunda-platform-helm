// +build integration

package test

import (
	"context"
	"fmt"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/camunda-cloud/zeebe/clients/go/pkg/zbc"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
)

type integrationTest struct {
	suite.Suite
	chartPath   string
	release     string
	namespace   string
	kubeOptions *k8s.KubectlOptions
}

func TestIntegration(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../")
	require.NoError(t, err)

	namespace := "zeebe-" + strings.ToLower(random.UniqueId())

	// currently enforce the test context to be the Zeebe team's cluster, but could be change to some CI cluster
	kubeOptions := k8s.NewKubectlOptions("gke_zeebe-io_europe-west1-b_zeebe-cluster", "", namespace)

	suite.Run(t, &integrationTest{
		chartPath:   chartPath,
		release:     "zeebe-cluster-helm-it",
		namespace:   namespace,
		kubeOptions: kubeOptions,
	})
}

func (s *integrationTest) SetupTest() {
	k8s.CreateNamespace(s.T(), s.kubeOptions, s.namespace)
}

func (s *integrationTest) TearDownTest() {
	k8s.DeleteNamespace(s.T(), s.kubeOptions, s.namespace)
}

func (s *integrationTest) TestGatewayConnection() {
	options := &helm.Options{
		KubectlOptions: s.kubeOptions,
		SetValues: map[string]string{
			"image.tag": "1.3.1",
		},
	}

	// cannot add the delete part in the tear down since the options are per test
	helm.Install(s.T(), options, s.chartPath, s.release)
	defer helm.Delete(s.T(), options, s.release, true)

	// TODO: maybe find a better way than just guessing the service name?
	serviceName := fmt.Sprintf("%s-zeebe-gateway", s.release)
	client, closeFn, err := s.createPortForwardedClient(serviceName)
	s.Require().NoError(err, "failed to create Zeebe client")
	defer closeFn()

	ctx, cancelFn := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFn()
	topology, err := client.NewTopologyCommand().Send(ctx)

	s.Require().NoError(err, "failed to obtain gateway topology")
	s.Require().Equal(topology.GatewayVersion, "1.3.1")
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
