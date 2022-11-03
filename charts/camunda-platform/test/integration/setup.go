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

	"github.com/gruntwork-io/terratest/modules/random"
)

type namespaceSection struct {
	textVar string
	prefix  string
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

func createNamespaceName() string {
	// if triggered by a github action the environment variable is set
	// we use it to better identify the test

	namespaceSections := []namespaceSection{
		{"GITHUB_PR_NUMBER", "pr"},
		{"GITHUB_PR_HEAD_SHA_SHORT", "sha"},
		{"GITHUB_WORKFLOW_RUN_ID", "run"},
	}
	namespace := namespaceFormatWithEnvVars("camunda-platform", namespaceSections)
	namespace += "-rnd-" + strings.ToLower(random.UniqueId())

	// max namespace length is 63 characters
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names
	return truncateString(namespace, 63)
}
