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
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/gruntwork-io/terratest/modules/random"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type namespaceSection struct {
	textVar string
	prefix  string
}

func getEnv(key string, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	envValue := getEnv(key, strconv.FormatBool(defaultValue))
	boolValue, err := strconv.ParseBool(envValue)
	if err != nil {
		log.Fatal(err)
	}
	return boolValue
}

func namespaceFormatWithEnvVars(nsBase string, nsSections []namespaceSection) string {
	for _, nss := range nsSections {
		if nssText, exist := os.LookupEnv(nss.textVar); exist {
			nsBase += fmt.Sprintf("-%s-%s", nss.prefix, nssText)
		}
	}
	return nsBase
}

func truncateString(str string, num int) string {
	shortenStr := str
	if len(str) > num {
		shortenStr = str[0:num]
	}
	return shortenStr
}

func createNamespaceObjectMeta() metav1.ObjectMeta {
	// if triggered by a github action the environment variable is set
	// we use it to better identify the test

	namespaceSections := []namespaceSection{
		{"GITHUB_PR_NUMBER", "pr"},
		{"GITHUB_PR_HEAD_SHA_SHORT", "sha"},
		{"GITHUB_WORKFLOW_RUN_ID", "run"},
	}
	namespace := namespaceFormatWithEnvVars("camunda-platform", namespaceSections)
	// In case the tests are running locally not in the CI.
	namespace += "-sfx-" + getEnv("GITHUB_WORKFLOW_JOB_ID", strings.ToLower(random.UniqueId()))

	return metav1.ObjectMeta{
		// max namespace length is 63 characters
		// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names
		Name: truncateString(namespace, 63),
		Labels: map[string]string{
			"github-pr-id":  getEnv("GITHUB_PR_NUMBER", ""),
			"git-sha-short": getEnv("GITHUB_PR_HEAD_SHA_SHORT", ""),
			"github-run-id": getEnv("GITHUB_WORKFLOW_RUN_ID", ""),
			"github-job-id": getEnv("GITHUB_WORKFLOW_JOB_ID", ""),
		},
	}
}

func getIntegrationSuiteOptions() integrationSuiteOptions {
	return integrationSuiteOptions{
		deleteNamespace: getEnvBool("CAMUNDA_DISTRO_TEST_DELETE_NAMESPACE", true),
	}
}
