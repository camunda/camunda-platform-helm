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

package matrix

import (
	"testing"
)

func TestTopologyPlaywrightProject(t *testing.T) {
	tests := []struct {
		name string
		leg  TopologyE2ELeg
		want string
	}{
		{
			name: "uses the topology project by default",
			leg:  TopologyE2ELeg{PlaywrightProject: "topology-orchestration"},
			want: "topology-orchestration",
		},
		{
			name: "uses the dependent additional project when configured",
			leg: TopologyE2ELeg{
				PlaywrightProject:           "topology-orchestration",
				AdditionalPlaywrightProject: "physical-tenants",
			},
			want: "physical-tenants",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := topologyPlaywrightProject(tt.leg); got != tt.want {
				t.Fatalf("topologyPlaywrightProject() = %q, want %q", got, tt.want)
			}
		})
	}
}
