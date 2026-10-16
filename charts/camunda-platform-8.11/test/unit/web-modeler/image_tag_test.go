// Copyright 2026 Camunda Services GmbH
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

package web_modeler

import (
	"camunda-platform/test/unit/testhelpers"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
)

const (
	restapiTemplate    = "templates/web-modeler/deployment-restapi.yaml"
	websocketsTemplate = "templates/web-modeler/deployment-websockets.yaml"
)

type ImageTagTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
}

func TestImageTagTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ImageTagTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

func imageTagValues(extra map[string]string) map[string]string {
	values := map[string]string{
		"identity.enabled":                       "true",
		"webModeler.enabled":                     "true",
		"webModeler.restapi.mail.fromAddress":    "example@example.com",
		"webModeler.image.tag":                   "shared-tag",
		"webModeler.restapi.image.repository":    "example/restapi",
		"webModeler.websockets.image.repository": "example/websockets",
	}
	maps.Copy(values, extra)
	return values
}

func (s *ImageTagTest) requireContainerImage(output, want string) {
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	s.Require().Equal(want, deployment.Spec.Template.Spec.Containers[0].Image)
}

func (s *ImageTagTest) TestEmptySubcomponentTagIsRejectedBySchema() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "TestEmptyRestapiImageTag",
			Template: restapiTemplate,
			Values:   imageTagValues(map[string]string{"webModeler.restapi.image.tag": ""}),
			Expected: map[string]string{"ERROR": "'/webModeler/restapi/image/tag': minLength: got 0, want 1"},
		},
		{
			Name:     "TestEmptyWebsocketsImageTag",
			Template: websocketsTemplate,
			Values:   imageTagValues(map[string]string{"webModeler.websockets.image.tag": ""}),
			Expected: map[string]string{"ERROR": "'/webModeler/websockets/image/tag': minLength: got 0, want 1"},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

func (s *ImageTagTest) TestUnsetSubcomponentTagKeepsSharedTag() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "TestRestapiKeepsSharedTag",
			Template: restapiTemplate,
			Values:   imageTagValues(nil),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.requireContainerImage(output, "example/restapi:shared-tag")
			},
		},
		{
			Name:     "TestWebsocketsKeepsSharedTag",
			Template: websocketsTemplate,
			Values:   imageTagValues(nil),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.requireContainerImage(output, "example/websockets:shared-tag")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

func (s *ImageTagTest) TestNonEmptySubcomponentTagOverridesSharedTag() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "TestRestapiOverride",
			Template: restapiTemplate,
			Values:   imageTagValues(map[string]string{"webModeler.restapi.image.tag": "restapi-tag"}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.requireContainerImage(output, "example/restapi:restapi-tag")
			},
		},
		{
			Name:     "TestWebsocketsOverride",
			Template: websocketsTemplate,
			Values:   imageTagValues(map[string]string{"webModeler.websockets.image.tag": "websockets-tag"}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.requireContainerImage(output, "example/websockets:websockets-tag")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}
