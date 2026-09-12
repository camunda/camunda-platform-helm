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

package helm

import (
	"fmt"

	"helm.sh/helm/v4/pkg/chart/v2/loader"
)

// UpgradePhaseLabel is the pod label the chart stamps with the active phase.
//
// The Hub Services pin their selector to this label with the value "normal", so a
// pod in any other phase runs without receiving traffic. That is what makes the
// migration pod safe, and it is also how the operator observes convergence.
const UpgradePhaseLabel = "camunda.io/upgrade-phase"

// Capabilities describes chart features the operator depends on or can drive.
type Capabilities struct {
	// TopologyRoles is true when the chart understands global.topology.mode.
	//
	// This is a hard requirement, not an optional feature. Chart 14.x (8.9) has no
	// topology concept and its schema accepts unknown keys under global, so
	// setting the hub role there is silently ignored and the chart renders the
	// whole platform — including an Orchestration Cluster StatefulSet. A
	// CamundaHub that quietly deployed Zeebe would be a severe surprise, so the
	// operator refuses rather than relies on the key taking effect.
	TopologyRoles bool

	// UpgradePhases is true when the chart declares camundaHub.upgrade.phase.
	//
	// Detection is by presence in the chart's own default values rather than a
	// vendored list, so a chart line that does not have the key yet degrades to a
	// plain upgrade instead of failing its render on an unknown value.
	UpgradePhases bool
}

// Capabilities inspects a resolved chart.
func (d *Driver) Capabilities(chartPath string) (Capabilities, error) {
	ch, err := loader.Load(chartPath)
	if err != nil {
		return Capabilities{}, fmt.Errorf("load chart %q: %w", chartPath, err)
	}
	return Capabilities{
		TopologyRoles: hasPath(ch.Values, "global", "topology", "mode"),
		UpgradePhases: hasPath(ch.Values, "camundaHub", "upgrade", "phase"),
	}, nil
}

func hasPath(values map[string]any, path ...string) bool {
	current := values
	for i, key := range path {
		v, ok := current[key]
		if !ok {
			return false
		}
		if i == len(path)-1 {
			return true
		}
		next, ok := v.(map[string]any)
		if !ok {
			return false
		}
		current = next
	}
	return false
}

// WithUpgradePhase returns a copy of vals carrying camundaHub.upgrade.phase.
//
// The values are copied rather than mutated so the caller's checksum, which
// identifies the desired release, stays free of the transient phase.
func WithUpgradePhase(vals map[string]any, phase string) map[string]any {
	out := make(map[string]any, len(vals)+1)
	for k, v := range vals {
		out[k] = v
	}

	hub, _ := out["camundaHub"].(map[string]any)
	hubCopy := make(map[string]any, len(hub)+1)
	for k, v := range hub {
		hubCopy[k] = v
	}

	upgrade, _ := hubCopy["upgrade"].(map[string]any)
	upgradeCopy := make(map[string]any, len(upgrade)+1)
	for k, v := range upgrade {
		upgradeCopy[k] = v
	}

	upgradeCopy["phase"] = phase
	hubCopy["upgrade"] = upgradeCopy
	out["camundaHub"] = hubCopy
	return out
}
