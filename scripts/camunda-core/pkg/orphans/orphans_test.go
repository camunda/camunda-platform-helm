// Copyright 2026 Camunda Services GmbH
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

package orphans

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The 8.9-to-8.10 upgrade of the classic-bundled path, as observed on GKE.
// Removing the bundled subcharts stranded three claims while zeebe's and the
// companion Elasticsearch's remained in use.
func TestDetectReproducesObservedUpgrade(t *testing.T) {
	inv := Inventory{
		Claims: []string{
			"data-integration-elasticsearch-master-0",
			"data-integration-postgresql-0",
			"data-integration-postgresql-web-modeler-0",
			"data-integration-zeebe-0",
			"elasticsearch-master-elasticsearch-master-0",
			"migration-backup-pvc",
		},
		PodClaims: []string{"migration-backup-pvc"},
		StatefulSets: []StatefulSetRef{
			{Name: "integration-zeebe", ClaimTemplates: []string{"data"}, Replicas: 1},
			{Name: "elasticsearch-master", ClaimTemplates: []string{"elasticsearch-master"}, Replicas: 1},
		},
	}

	assert.Equal(t, []string{
		"data-integration-elasticsearch-master-0",
		"data-integration-postgresql-0",
		"data-integration-postgresql-web-modeler-0",
	}, Names(Detect(inv)))
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name string
		inv  Inventory
		want []string
	}{
		{
			name: "no claims",
			inv:  Inventory{},
			want: []string{},
		},
		{
			name: "claim referenced by a pod is not orphaned",
			inv:  Inventory{Claims: []string{"a"}, PodClaims: []string{"a"}},
			want: []string{},
		},
		{
			name: "claim with nothing referencing it is orphaned",
			inv:  Inventory{Claims: []string{"a"}},
			want: []string{"a"},
		},
		{
			name: "a scaled-down StatefulSet still claims its storage",
			inv: Inventory{
				Claims:       []string{"data-db-0", "data-db-1"},
				StatefulSets: []StatefulSetRef{{Name: "db", ClaimTemplates: []string{"data"}, Replicas: 2}},
			},
			want: []string{},
		},
		{
			name: "claims beyond the replica count remain associated",
			inv: Inventory{
				Claims:       []string{"data-db-0", "data-db-1", "data-db-2"},
				StatefulSets: []StatefulSetRef{{Name: "db", ClaimTemplates: []string{"data"}, Replicas: 2}},
			},
			want: []string{},
		},
		{
			name: "multiple templates on one StatefulSet",
			inv: Inventory{
				Claims: []string{"data-db-0", "logs-db-0", "stale-db-0"},
				StatefulSets: []StatefulSetRef{
					{Name: "db", ClaimTemplates: []string{"data", "logs"}, Replicas: 1},
				},
			},
			want: []string{"stale-db-0"},
		},
		{
			name: "a StatefulSet scaled to zero retains its claims",
			inv: Inventory{
				Claims:       []string{"data-db-0"},
				StatefulSets: []StatefulSetRef{{Name: "db", ClaimTemplates: []string{"data"}, Replicas: 0}},
			},
			want: []string{},
		},
		{
			name: "results are sorted",
			inv:  Inventory{Claims: []string{"c", "a", "b"}},
			want: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Names(Detect(tt.inv)))
		})
	}
}

func TestAppeared(t *testing.T) {
	before := []Orphan{{Claim: "already-stranded"}}
	after := []Orphan{{Claim: "already-stranded"}, {Claim: "newly-stranded"}}

	assert.Equal(t, []string{"newly-stranded"}, Names(Appeared(before, after)),
		"an upgrade is judged on what it strands, not on what it inherited")
}

func TestAppearedWithNoPriorOrphans(t *testing.T) {
	assert.Equal(t, []string{"a"}, Names(Appeared(nil, []Orphan{{Claim: "a"}})))
	assert.Empty(t, Names(Appeared([]Orphan{{Claim: "a"}}, nil)))
}
