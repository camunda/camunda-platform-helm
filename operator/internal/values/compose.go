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

// Package values composes the chart values for a CamundaHub release.
//
// Merging uses Helm's own loader.MergeMaps, the same function behind
// `helm -f a.yaml -f b.yaml`, so a release composed from several ValuesFrom
// sources merges exactly as it would on the command line: maps merge key by key,
// arrays replace wholesale.
package values

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"helm.sh/helm/v4/pkg/chart/v2/loader"

	"operator/api/v1alpha1"
)

// TopologyConflictError reports that the user set global.topology.mode themselves.
//
// A CamundaHub is the hub role by construction, so the operator sets that key.
// Silently overwriting a conflicting value would hide a real misunderstanding
// about what the object does.
type TopologyConflictError struct {
	Got string
}

func (e *TopologyConflictError) Error() string {
	return fmt.Sprintf(
		"global.topology.mode is set to %q in spec.values; a CamundaHub always renders the %q role, "+
			"so remove the key or use a different resource for that topology",
		e.Got, v1alpha1.TopologyModeHub)
}

// Compose merges the ordered sources, then the inline values, then forces the hub
// topology role. Later sources win, matching repeated -f on the Helm CLI.
func Compose(sources []map[string]any, inline map[string]any) (map[string]any, error) {
	merged := map[string]any{}
	for _, src := range sources {
		merged = loader.MergeMaps(merged, src)
	}
	if inline != nil {
		merged = loader.MergeMaps(merged, inline)
	}

	if err := forceHubTopology(merged); err != nil {
		return nil, err
	}
	return merged, nil
}

func forceHubTopology(vals map[string]any) error {
	global, ok := vals["global"].(map[string]any)
	if !ok {
		global = map[string]any{}
		vals["global"] = global
	}

	topology, ok := global["topology"].(map[string]any)
	if !ok {
		topology = map[string]any{}
		global["topology"] = topology
	}

	if mode, set := topology["mode"]; set {
		if s, _ := mode.(string); s != v1alpha1.TopologyModeHub {
			return &TopologyConflictError{Got: fmt.Sprint(mode)}
		}
	}

	topology["mode"] = v1alpha1.TopologyModeHub
	return nil
}

// Decode unmarshals raw CRD JSON into a values map. A nil or empty input yields a
// nil map, which Helm treats as "no values".
func Decode(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode values: %w", err)
	}
	return out, nil
}

// Checksum identifies a values map. json.Marshal sorts map keys, so the result is
// stable across map iteration order.
func Checksum(vals map[string]any) (string, error) {
	canonical, err := json.Marshal(vals)
	if err != nil {
		return "", fmt.Errorf("checksum values: %w", err)
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}
