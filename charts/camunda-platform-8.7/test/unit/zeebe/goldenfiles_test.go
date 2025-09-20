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

package zeebe

import (
	"camunda-platform/test/unit/utils"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGoldenDefaultsTemplate(t *testing.T) {
	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	templateNames := []string{"service", "serviceaccount", "statefulset", "configmap"}

	ignoredLines := []string{
		`\s+.*-secret:\s+.*`,    // secrets are auto-generated and need to be ignored.
		`\s+checksum/.+?:\s+.*`, // ignore configmap checksum.
	}

	utils.TestGoldenTemplates(t, chartPath, "zeebe", templateNames, ignoredLines, nil, nil)
}
