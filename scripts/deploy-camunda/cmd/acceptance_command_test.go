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

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocumentStoreAcceptanceCommand(t *testing.T) {
	t.Parallel()
	parent := newAcceptanceCommand()
	command, _, err := parent.Find([]string{"documentstore-s3-compatible"})
	require.NoError(t, err)
	assert.Equal(t, "documentstore-s3-compatible", command.Name())
	parent.SetArgs([]string{"documentstore-s3-compatible", "--namespace=", "--release=integration"})
	require.ErrorContains(t, parent.Execute(), "namespace and release are required")
}
