// Copyright Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package camunda

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNotesTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	output, err := exec.Command("helm", "install", "credential-output-test", chartPath,
		"--dry-run=client",
		"--set", "identity.firstUser.password=credential-output-canary-do-not-print",
		"--set", "identity.firstUser.existingSecret=",
		"--set", "orchestration.data.secondaryStorage.type=elasticsearch",
		"--set", "global.elasticsearch.enabled=true",
		"--set", "global.elasticsearch.external=true",
		"--set", "global.elasticsearch.url.host=elasticsearch",
	).CombinedOutput()
	require.NoError(t, err, string(output))

	_, notes, found := strings.Cut(string(output), "\nNOTES:\n")
	require.True(t, found)
	require.Contains(t, notes, `Default user: "demo".`)
	require.NotContains(t, notes, "credential-output-canary-do-not-print")
}
