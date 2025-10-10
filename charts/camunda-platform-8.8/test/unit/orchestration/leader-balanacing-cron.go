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

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	batchV1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
)

type LeaderBalancingCronTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestLeaderBalancingCronTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &LeaderBalancingCronTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/orchestration/leader-balancing-cron.yaml"},
	})
}

func (s *LeaderBalancingCronTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestContainerSetGlobalAnnotations",
			Values: map[string]string{
				"orchestration.leaderBalancing.enabled": "true",
				"global.annotations.foo":                "bar-global-annotation",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var service coreV1.Service
				helm.UnmarshalK8SYaml(s.T(), output, &service)

				// then
				s.Require().Equal("bar-global-annotation", service.ObjectMeta.Annotations["foo"])
			},
		}, {
			Name: "TestDifferentImage",
			Values: map[string]string{
				"orchestration.leaderBalancing.enabled": "true",
				"orchestration.leaderBalancing.image":   "xyz:latest",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// given
				var cronJob batchV1.CronJob
				expectedImage := "xyz:latest"

				// when
				helm.UnmarshalK8SYaml(s.T(), output, &cronJob)
				actualImage := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image

				// then
				s.Require().Equal(expectedImage, actualImage)
			},
		}, {
			Name: "TestDifferentSchedule",
			Values: map[string]string{
				"orchestration.leaderBalancing.enabled":  "true",
				"orchestration.leaderBalancing.schedule": "*/5 * * * *",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// given
				var cronJob batchV1.CronJob
				expectedSchedule := "*/5 * * * *"

				// when
				helm.UnmarshalK8SYaml(s.T(), output, &cronJob)
				actualSchedule := cronJob.Spec.Schedule

				// then
				s.Require().Equal(expectedSchedule, actualSchedule)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
