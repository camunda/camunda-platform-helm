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

package orchestration

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type MigrationDataConfigMapEnvVarsTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestMigrationDataConfigMapEnvVarsTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &MigrationDataConfigMapEnvVarsTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/orchestration/migration-data-configmap-env-vars.yaml"},
	})
}

func (s *MigrationDataConfigMapEnvVarsTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestTasksRetryUsesTasksRetryValues",
			Values: map[string]string{
				"orchestration.migration.data.enabled":                            "true",
				"orchestration.migration.data.process.retry.maxRetries":           "7",
				"orchestration.migration.data.process.retry.maxRetryDelay":        "PT3M",
				"orchestration.migration.data.process.retry.minRetryDelay":        "PT30s",
				"orchestration.migration.data.process.retry.retryDelayMultiplier": "4",
				"orchestration.migration.data.tasks.retry.maxRetries":             "9",
				"orchestration.migration.data.tasks.retry.maxRetryDelay":          "PT2M",
				"orchestration.migration.data.tasks.retry.minRetryDelay":          "PT20s",
				"orchestration.migration.data.tasks.retry.retryDelayMultiplier":   "3",
			},
			Expected: map[string]string{
				"MIGRATION_PROCESS_RETRY_MAXRETRIES":           "7",
				"MIGRATION_PROCESS_RETRY_MAXRETRYDELAY":        "PT3M",
				"MIGRATION_PROCESS_RETRY_MINRETRYDELAY":        "PT30s",
				"MIGRATION_PROCESS_RETRY_RETRYDELAYMULTIPLIER": "4",
				"MIGRATION_TASKS_RETRY_MAXRETRIES":             "9",
				"MIGRATION_TASKS_RETRY_MAXRETRYDELAY":          "PT2M",
				"MIGRATION_TASKS_RETRY_MINRETRYDELAY":          "PT20s",
				"MIGRATION_TASKS_RETRY_RETRYDELAYMULTIPLIER":   "3",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
