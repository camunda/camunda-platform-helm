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
	"os"
	"strings"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/suite"
)

type integrationTest struct {
	suite.Suite
	chartPath   string
	release     string
	namespace   string
	kubeOptions *k8s.KubectlOptions
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
	namespace := "camunda-platform-" + strings.ToLower(random.UniqueId())
	if exist {
		namespace += "-" + commitSHA
	}

	// max namespace length is 63 characters
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names
	return truncateString(namespace, 63)
}

func (s *integrationTest) resolveKeycloakServiceName() string {
	// Keycloak truncates at 20 chars since the node identifier in WildFly is limited to 23 characters.
	// see https://github.com/bitnami/charts/blob/master/bitnami/keycloak/templates/_helpers.tpl#L2
	keycloakServiceName := fmt.Sprintf("%s-keycl", s.release)
	keycloakServiceName = strings.TrimSuffix(keycloakServiceName[:20], "-")
	return keycloakServiceName
}
