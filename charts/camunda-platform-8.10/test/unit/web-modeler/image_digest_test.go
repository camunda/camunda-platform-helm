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
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
)

const (
	restapiDigest    = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	websocketsDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func imageIs(want string) func(*testing.T, string, error) {
	return func(t *testing.T, output string, err error) {
		require.NoError(t, err)

		var deployment appsv1.Deployment
		helm.UnmarshalK8SYaml(t, output, &deployment)

		require.Equal(t, want, deployment.Spec.Template.Spec.Containers[0].Image)
	}
}

func (s *ImageTagTest) TestEmptyTagWithDigestKeepsDigestImage() {
	hubSharedEmptyTag := hubImageTagValues(map[string]string{
		"camundaHub.image.tag":               "",
		"camundaHub.restapi.image.digest":    restapiDigest,
		"camundaHub.websockets.image.digest": websocketsDigest,
	})

	testCases := []testhelpers.TestCase{
		{
			Name:     "TestLegacyRestapi",
			Template: restapiTemplate,
			Values: imageTagValues(map[string]string{
				"webModeler.restapi.image.tag":    "",
				"webModeler.restapi.image.digest": restapiDigest,
			}),
			Verifier: imageIs("example/restapi@" + restapiDigest),
		},
		{
			Name:     "TestLegacyWebsockets",
			Template: websocketsTemplate,
			Values: imageTagValues(map[string]string{
				"webModeler.websockets.image.tag":    "",
				"webModeler.websockets.image.digest": websocketsDigest,
			}),
			Verifier: imageIs("example/websockets@" + websocketsDigest),
		},
		{
			Name:     "TestHubRestapi",
			Template: restapiTemplate,
			Values: hubImageTagValues(map[string]string{
				"camundaHub.restapi.image.tag":    "",
				"camundaHub.restapi.image.digest": restapiDigest,
			}),
			Verifier: imageIs("example/restapi@" + restapiDigest),
		},
		{
			Name:     "TestHubWebsockets",
			Template: websocketsTemplate,
			Values: hubImageTagValues(map[string]string{
				"camundaHub.websockets.image.tag":    "",
				"camundaHub.websockets.image.digest": websocketsDigest,
			}),
			Verifier: imageIs("example/websockets@" + websocketsDigest),
		},
		{
			Name:     "TestHubSharedTagRestapi",
			Template: restapiTemplate,
			Values:   hubSharedEmptyTag,
			Verifier: imageIs("example/restapi@" + restapiDigest),
		},
		{
			Name:     "TestHubSharedTagWebsockets",
			Template: websocketsTemplate,
			Values:   hubSharedEmptyTag,
			Verifier: imageIs("example/websockets@" + websocketsDigest),
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

func (s *ImageTagTest) TestEmptyTagWithEmptyDigestIsRejectedBySchema() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "TestLegacyRestapi",
			Template: restapiTemplate,
			Values: imageTagValues(map[string]string{
				"webModeler.restapi.image.tag":    "",
				"webModeler.restapi.image.digest": "",
			}),
			Expected: map[string]string{"ERROR": "'/webModeler/restapi/image/tag': minLength: got 0, want 1"},
		},
		{
			Name:     "TestLegacyWebsockets",
			Template: websocketsTemplate,
			Values: imageTagValues(map[string]string{
				"webModeler.websockets.image.tag":    "",
				"webModeler.websockets.image.digest": "",
			}),
			Expected: map[string]string{"ERROR": "'/webModeler/websockets/image/tag': minLength: got 0, want 1"},
		},
		{
			Name:     "TestHubRestapi",
			Template: restapiTemplate,
			Values: hubImageTagValues(map[string]string{
				"camundaHub.restapi.image.tag":    "",
				"camundaHub.restapi.image.digest": "",
			}),
			Expected: map[string]string{"ERROR": "'/camundaHub/restapi/image/tag': minLength: got 0, want 1"},
		},
		{
			Name:     "TestHubWebsockets",
			Template: websocketsTemplate,
			Values: hubImageTagValues(map[string]string{
				"camundaHub.websockets.image.tag":    "",
				"camundaHub.websockets.image.digest": "",
			}),
			Expected: map[string]string{"ERROR": "'/camundaHub/websockets/image/tag': minLength: got 0, want 1"},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

func (s *ImageTagTest) TestEmptyHubSharedTagWithoutDigestFailsAtRender() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "TestHubSharedTag",
			Template: restapiTemplate,
			Values:   hubImageTagValues(map[string]string{"camundaHub.image.tag": ""}),
			Expected: map[string]string{"ERROR": "has neither a tag nor a digest"},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}
