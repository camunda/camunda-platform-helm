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

package operate

import (
	"camunda-platform-helm/charts/camunda-platform/test/golden"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestGoldenConfigmapWithElasticsearchUrl(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	suite.Run(t, &golden.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
		GoldenFileName: "configmap-elastic-url",
		Templates:      []string{"charts/operate/templates/configmap.yaml"},
		SetValues:      map[string]string{"global.elasticsearch.url": "http://elasticmaster.org"},
	})
}
