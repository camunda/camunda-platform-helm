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

package parity

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSDKVersionMatchesToolVersions guards the parity contract itself.
//
// Operator-rendered and CLI-rendered manifests are identical only because the
// embedded Helm SDK is the same version as the Helm CLI the repository pins. A
// dependency bump that moves one without the other would break parity silently,
// so it breaks this test loudly instead.
func TestSDKVersionMatchesToolVersions(t *testing.T) {
	pinned := helmVersionFromToolVersions(t, "../../../.tool-versions")
	embedded := embeddedHelmVersion(t)

	require.Equalf(t, "v"+pinned, embedded,
		"helm.sh/helm/v4 in operator/go.mod (%s) must match the helm CLI pinned in .tool-versions (%s); "+
			"bump both together or operator and Helm-CLI renders will diverge", embedded, pinned)
}

func helmVersionFromToolVersions(t *testing.T, path string) string {
	t.Helper()

	f, err := os.Open(filepath.Clean(path))
	require.NoError(t, err)
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "helm" {
			return fields[1]
		}
	}
	require.NoError(t, scanner.Err())

	t.Fatalf("no helm entry found in %s", path)
	return ""
}

func embeddedHelmVersion(t *testing.T) string {
	t.Helper()

	info, ok := debug.ReadBuildInfo()
	require.True(t, ok, "build info unavailable")

	for _, dep := range info.Deps {
		if dep.Path == "helm.sh/helm/v4" {
			return dep.Version
		}
	}

	t.Fatal("helm.sh/helm/v4 is not a dependency of this module")
	return ""
}
