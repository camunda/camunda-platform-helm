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

package values

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ComposeTest struct {
	suite.Suite
}

func TestCompose(t *testing.T) {
	suite.Run(t, new(ComposeTest))
}

// TestMapsDeepMergeArraysReplace pins the merge semantics the operator inherits
// from Helm. Getting this wrong is the difference between an overlay adding a
// node selector and an overlay silently dropping a tolerations list, and the
// repository's own AGENTS.md calls it out as a recurring source of confusion.
func (s *ComposeTest) TestMapsDeepMergeArraysReplace() {
	base := map[string]any{
		"identity": map[string]any{
			"enabled":      true,
			"nodeSelector": map[string]any{"tier": "compute"},
			"tolerations":  []any{"a", "b"},
		},
	}
	overlay := map[string]any{
		"identity": map[string]any{
			"nodeSelector": map[string]any{"zone": "us-east"},
			"tolerations":  []any{"c"},
		},
	}

	got, err := Compose([]map[string]any{base}, overlay)
	s.Require().NoError(err)

	identity := got["identity"].(map[string]any)
	s.Equal(true, identity["enabled"], "keys absent from the overlay must survive")
	s.Equal(map[string]any{"tier": "compute", "zone": "us-east"}, identity["nodeSelector"],
		"maps merge key by key")
	s.Equal([]any{"c"}, identity["tolerations"],
		"arrays replace wholesale; they do not append")
}

func (s *ComposeTest) TestLaterSourcesWin() {
	first := map[string]any{"identity": map[string]any{"replicas": 1}}
	second := map[string]any{"identity": map[string]any{"replicas": 3}}

	got, err := Compose([]map[string]any{first, second}, nil)
	s.Require().NoError(err)
	s.Equal(3, got["identity"].(map[string]any)["replicas"])
}

func (s *ComposeTest) TestInlineValuesWinOverSources() {
	source := map[string]any{"identity": map[string]any{"replicas": 1}}
	inline := map[string]any{"identity": map[string]any{"replicas": 9}}

	got, err := Compose([]map[string]any{source}, inline)
	s.Require().NoError(err)
	s.Equal(9, got["identity"].(map[string]any)["replicas"])
}

func (s *ComposeTest) TestHubTopologyIsForced() {
	got, err := Compose(nil, map[string]any{"identity": map[string]any{"enabled": true}})
	s.Require().NoError(err)

	topology := got["global"].(map[string]any)["topology"].(map[string]any)
	s.Equal("hub", topology["mode"])
}

func (s *ComposeTest) TestHubTopologyPreservesSiblingGlobalKeys() {
	inline := map[string]any{
		"global": map[string]any{
			"topology": map[string]any{"clusters": []any{}},
			"security": map[string]any{"authentication": map[string]any{"method": "oidc"}},
		},
	}

	got, err := Compose(nil, inline)
	s.Require().NoError(err)

	global := got["global"].(map[string]any)
	s.NotNil(global["security"], "forcing the topology mode must not drop sibling global keys")
	topology := global["topology"].(map[string]any)
	s.Equal("hub", topology["mode"])
	s.NotNil(topology["clusters"], "forcing the mode must not drop sibling topology keys")
}

func (s *ComposeTest) TestConflictingTopologyModeIsRejected() {
	inline := map[string]any{
		"global": map[string]any{"topology": map[string]any{"mode": "orchestration"}},
	}

	_, err := Compose(nil, inline)
	s.Require().Error(err)
	s.IsType(&TopologyConflictError{}, err)
	s.Contains(err.Error(), "orchestration")
}

func (s *ComposeTest) TestExplicitHubModeIsAccepted() {
	inline := map[string]any{
		"global": map[string]any{"topology": map[string]any{"mode": "hub"}},
	}

	_, err := Compose(nil, inline)
	s.NoError(err, "restating the mode the operator would set anyway is not a conflict")
}

func (s *ComposeTest) TestChecksumIsStableAcrossKeyOrder() {
	a := map[string]any{"b": 1, "a": 2, "c": map[string]any{"z": 1, "y": 2}}
	b := map[string]any{"a": 2, "c": map[string]any{"y": 2, "z": 1}, "b": 1}

	sumA, err := Checksum(a)
	s.Require().NoError(err)
	sumB, err := Checksum(b)
	s.Require().NoError(err)

	s.Equal(sumA, sumB)
}

func (s *ComposeTest) TestChecksumChangesWithContent() {
	sumA, err := Checksum(map[string]any{"replicas": 1})
	s.Require().NoError(err)
	sumB, err := Checksum(map[string]any{"replicas": 2})
	s.Require().NoError(err)

	s.NotEqual(sumA, sumB)
}

func (s *ComposeTest) TestDecodeEmptyYieldsNil() {
	got, err := Decode(nil)
	s.Require().NoError(err)
	s.Nil(got)
}
