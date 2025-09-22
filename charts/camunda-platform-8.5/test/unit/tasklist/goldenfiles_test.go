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

package tasklist

import (
	"path/filepath"
	"testing"

	"camunda-platform/test/unit/utils"

	"github.com/stretchr/testify/require"
)

func TestGoldenDefaultsTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	templateNames := []string{
		"deployment",
		"ingress",
		"service",
		"serviceaccount",
	}
	ignoredLines := []string{
		`\s+.*-secret:\s+.*`,    // secrets are auto-generated and need to be ignored.
		`\s+checksum/.+?:\s+.*`, // ignore configmap checksum.
	}
	setValues := map[string]string{
		"tasklist.ingress.enabled": "true",
	}

	utils.TestGoldenTemplates(t, chartPath, "tasklist", templateNames, ignoredLines, setValues, nil)
}
